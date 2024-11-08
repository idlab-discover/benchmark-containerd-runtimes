package main

import (
	"benchmarks/internal/benchmark"
	"benchmarks/pkg/container"
	"bufio"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"
)

// Get current memory usage in kB
func currentUsedMemory() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Fatalf("Error opening /proc/meminfo: %v", err)
	}
	defer file.Close()

	var totalMem, availableMem uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "MemTotal:") {
			f := strings.Fields(line)
			totalMem, err = strconv.ParseUint(f[1], 10, 64)
			if err != nil {
				log.Fatalf("Error parsing total memory: %v", err)
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			f := strings.Fields(line)
			availableMem, err = strconv.ParseUint(f[1], 10, 64)
			if err != nil {
				log.Fatalf("Error parsing available memory: %v", err)
			}
		}

		if totalMem != 0 && availableMem != 0 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading /proc/meminfo: %v", err)
	}

	return totalMem - availableMem
}

var memories []float64

func memoryBenchmark(config benchmark.Config) {
	memories = make([]float64, 0, config.Iterations)

	c, ctx := container.Init()
	defer c.Close()

	image := container.PullImage(ctx, c, "docker.io/library/alpine:latest", config.Snapshotter)

	for i := 0; i < config.Iterations; i++ {
		containers := make([]container.Container, 0, 50)

		// Measure current memory usage
		time.Sleep(1 * time.Second)
		memStart := currentUsedMemory()
		log.Printf("(%d/%d) start memory usage: %d kB", i+1, config.Iterations, memStart)

		// Start 50 containers
		for j := 0; j < 50; j++ {
			containerID := fmt.Sprintf("container-%d-%d", j, rand.IntN(100000))

			container := container.CreateAndStartContainer(
				ctx,
				c,
				config,
				containerID,
				image,
				"/bin/sh", "-c", "sleep infinity",
			)

			containers = append(containers, container)
		}

		time.Sleep(3 * time.Second)

		// Measure new memory usage
		memEnd := currentUsedMemory()
		log.Printf("(%d/%d) end memory usage: %d kB", i+1, config.Iterations, memEnd)

		// Stop containers
		for _, c := range containers {
			c.RemoveContainer(true)
		}

		// Calculate memory usage/container
		memAllContainers := memEnd - memStart
		memPerContainer := float64(memAllContainers) / float64(50)

		memories = append(memories, memPerContainer)

		log.Printf("(%d/%d) memory usage by 50 containers: %d kB", i+1, config.Iterations, memAllContainers)
		log.Printf("(%d/%d) memory usage by 1 container: %f kB", i+1, config.Iterations, memPerContainer)
	}

	var avg float64 = 0
	for _, mem := range memories {
		avg += mem
	}
	avg /= float64(len(memories))
	log.Printf("Average memory usage per container over %d runs: %f kB", config.Iterations, avg)
}

func writeResult(config benchmark.Config) {
	err := os.MkdirAll("results/memory", 0o755)
	if err != nil {
		log.Fatalf("Failed to create result directory: %v", err)
	}

	f, err := os.CreateTemp("results/memory", fmt.Sprintf("memory-%s%s-%s-%d_", config.Runtime, config.RuntimeMeta, config.Snapshotter, config.Iterations))
	if err != nil {
		log.Fatalf("Failed to create result file: %v", err)
	}
	defer f.Close()

	buffer := bufio.NewWriter(f)
	defer buffer.Flush()

	_, err = fmt.Fprintf(buffer, "%s%s,%s,%d,kB\n", config.Runtime, config.RuntimeMeta, config.Snapshotter, config.Iterations)
	if err != nil {
		log.Fatalf("Error while writing to file: %v", err)
	}

	for _, mem := range memories {
		buffer.WriteString(fmt.Sprintf("%f\n", mem))
	}
}

func main() {
	config := benchmark.InitFlags()

	memoryBenchmark(config)
	writeResult(config)
}
