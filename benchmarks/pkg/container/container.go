package container

import (
	"benchmarks/internal/benchmark"
	"context"
	"fmt"
	"log"
	"syscall"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/netns"
	"github.com/containerd/containerd/v2/pkg/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Container struct {
	client     *containerd.Client
	ctx        context.Context
	id         string
	cni        gocni.CNI
	netNS      *netns.NetNS
	container  containerd.Container
	task       containerd.Task
	exitStatus <-chan containerd.ExitStatus
}

func Init() (*containerd.Client, context.Context) {
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.Fatalf("Could not connect to containerd: %v", err)
	}

	return client, namespaces.WithNamespace(context.Background(), "default")
}

func PullImage(ctx context.Context, client *containerd.Client, ref, snapshotter string) containerd.Image {
	image, err := client.Pull(
		ctx,
		ref,
		containerd.WithPullUnpack,
		containerd.WithPullSnapshotter(snapshotter),
	)
	if err != nil {
		log.Fatalf("Could not pull %s image: %v", ref, err)
	}
	log.Printf("Successfully pulled %s image\n", image.Name())

	return image
}

func CreateAndStartContainer(ctx context.Context, client *containerd.Client, config benchmark.Config, id string, image containerd.Image, processArgs ...string) Container {
	// Setup CNI networking
	cniNetwork, err := gocni.New(
		gocni.WithMinNetworkCount(2),
		gocni.WithPluginConfDir("/etc/cni/net.d"),
		gocni.WithPluginDir([]string{"/opt/cni/bin"}),
	)
	if err != nil {
		log.Fatalf("Failed to initialize CNI library: %v", err)
	}

	// Create new network namespace
	netNS, err := netns.NewNetNS("/var/run/netns")
	if err != nil {
		log.Fatalf("Failed to create new netns: %v", err)
	}

	if err := cniNetwork.Load(gocni.WithLoNetwork, gocni.WithDefaultConf); err != nil {
		log.Fatalf("Failed to load CNI configuration: %v", err)
	}

	if _, err := cniNetwork.Setup(ctx, fmt.Sprintf("default-%s", id), netNS.GetPath()); err != nil {
		log.Fatalf("Failed to setup CNI network: %v", err)
	}

	ociAnnotations := make(map[string]string)

	// Add additional CRI annotations when running with gVisor due to bug in the containerd-shim-runsc-v1 which expects CRI annotations
	// See https://github.com/google/gvisor/issues/4544
	if config.Runtime == "io.containerd.runsc.v1" {
		// Create container in new sandbox instead of already present sandbox
		ociAnnotations["io.kubernetes.cri.container-type"] = "sandbox" // https://pkg.go.dev/github.com/containerd/containerd/pkg/cri/annotations
	}

	ctr, err := client.NewContainer(
		ctx,
		id,
		containerd.WithImage(image),
		containerd.WithSnapshotter(config.Snapshotter),
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", id), image),
		containerd.WithRuntime(config.Runtime, nil),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithAnnotations(ociAnnotations),
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.NetworkNamespace,
				Path: netNS.GetPath(),
			}),
			oci.WithProcessArgs(processArgs...),
		),
	)
	if err != nil {
		log.Fatalf("Could not create container: %v", err)
	}

	task, err := ctr.NewTask(ctx, cio.NullIO)
	// task, err := ctr.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		log.Fatalf("Could not create task: %v", err)
	}

	// Wait on task before starting it to prevent race conditions
	// https://github.com/containerd/containerd/blob/main/docs/getting-started.md#task-wait-and-start
	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		log.Fatalf("Error waiting on task: %v", err)
	}

	if err := task.Start(ctx); err != nil {
		log.Fatalf("Error starting task: %v", err)
	}

	return Container{
		client:     client,
		ctx:        ctx,
		id:         id,
		cni:        cniNetwork,
		netNS:      netNS,
		container:  ctr,
		task:       task,
		exitStatus: exitStatusC,
	}

}

func (c *Container) RemoveContainer(kill bool) {
	if kill {
		s, err := c.task.Status(c.ctx)
		if err != nil {
			log.Fatalf("Failed to retrieve task status: %v", err)
		}
		if s.Status != containerd.Stopped {
			c.task.Kill(c.ctx, syscall.SIGTERM)
		}

		s, err = c.task.Status(c.ctx)
		if err != nil {
			log.Fatalf("Failed to retrieve task status: %v", err)
		}
		if s.Status != containerd.Stopped {
			c.task.Kill(c.ctx, syscall.SIGKILL)
		}
	}

	// Wait on channel: make sure that container has fully exited
	status := <-c.exitStatus
	code, exitedAt, err := status.Result()
	if err != nil {
		log.Printf("Error retrieving exit status code from channel: %v", err)
	} else {
		log.Printf("Container exited at %v with status %v", exitedAt, code)
	}

	c.task.Delete(c.ctx, containerd.WithProcessKill)
	c.container.Delete(c.ctx, containerd.WithSnapshotCleanup)

	// Clean up networking
	if err := c.cni.Remove(c.ctx, fmt.Sprintf("default-%s", c.id), ""); err != nil {
		log.Fatalf("Failed to teardown CNI network: %v", err)
	}
	if err := c.netNS.Remove(); err != nil {
		log.Fatalf("Failed to remove netns: %v", err)
	}
}
