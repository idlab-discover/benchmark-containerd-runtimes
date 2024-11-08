# Benchmark containerd Runtimes

This repo contains a Go program that allows you to perform benchmarks on different low-level container runtimes that are OCI-compliant. It uses the containerd client to spawn a configurable amount of containers.

## Benchmarks

Current supported benchmark types are **startup times** and **memory usage**.

### Results

There are benchmark run results available in the [results](./results) directory. These results are startup and memory benchmarks for runc, Kata Containers (Kata 2.x & Kata 3.x, using different hypervisors (QEMU, Firecracker, Cloud Hypervisor, Dragonball)), and gVisor (KVM & systrap platforms).

## Building

You can build the binaries in the `benchmarks` module as follows:

### Startup

```bash
CGO_ENABLED=0 go build ./cmd/startup/
```

### Memory

```bash
CGO_ENABLED=0 go build ./cmd/memory/
```

## Usage

```bash
# Replace [benchmark] with either `startup` or `memory`
❯ ./[benchmark] --help
Usage of ./[benchmark]:
  -iterations int
        Amount of benchmark iterations (default 10)
  -meta string
        Additional metadata. Can for example be '-firecracker' to indicate firecracker VMM.
  -network-gateway string
        The gateway of the CNI network. This can be different depending on CNI configuration (default "10.4.0.1")
  -runtime string
        The runtime to run the benchmark for (default "io.containerd.runc.v2")
  -snapshotter string
        The snapshotter that should be used. For Firecracker this should probably be 'devmapper' (default "overlayfs")
```

