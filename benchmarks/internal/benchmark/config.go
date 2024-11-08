package benchmark

import "flag"

type Config struct {
	Iterations     int
	Runtime        string
	RuntimeMeta    string
	Snapshotter    string
	NetworkGateway string
}

func InitFlags() Config {
	iterations := flag.Int("iterations", 10, "Amount of benchmark iterations")
	runtime := flag.String("runtime", "io.containerd.runc.v2", "The runtime to run the benchmark for")
	runtimeMeta := flag.String("meta", "", "Additional metadata. Can for example be '-firecracker' to indicate firecracker VMM.")
	snapshotter := flag.String("snapshotter", "overlayfs", "The snapshotter that should be used. For Firecracker this should probably be 'devmapper'")
	networkGateway := flag.String("network-gateway", "10.4.0.1", "The gateway of the CNI network. This can be different depending on CNI configuration")

	flag.Parse()

	return Config{
		Iterations:     *iterations,
		Runtime:        *runtime,
		RuntimeMeta:    *runtimeMeta,
		Snapshotter:    *snapshotter,
		NetworkGateway: *networkGateway,
	}
}
