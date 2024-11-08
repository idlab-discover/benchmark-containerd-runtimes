package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"benchmarks/internal/benchmark"
	"benchmarks/pkg/container"
)

var startTime time.Time
var startTimes []time.Duration

func startupBenchmark(config benchmark.Config) {
	startTimes = make([]time.Duration, 0, config.Iterations)

	c, ctx := container.Init()
	defer c.Close()

	image := container.PullImage(ctx, c, "docker.io/library/alpine:latest", config.Snapshotter)

	for i := 0; i < config.Iterations; i++ {
		func() {
			containerID := fmt.Sprintf("container-%d", i)

			startTime = time.Now()
			log.Printf("%s start at %v\n", containerID, startTime)

			container := container.CreateAndStartContainer(
				ctx,
				c,
				config,
				containerID,
				image,
				"/bin/sh", "-c", fmt.Sprintf("wget -q http://%s:5000/started", config.NetworkGateway),
			)
			defer container.RemoveContainer(false)
		}()
	}
}

func writeResult(config benchmark.Config) {
	err := os.MkdirAll("results/start", 0o755)
	if err != nil {
		log.Fatalf("Failed to create result directory: %v", err)
	}

	f, err := os.CreateTemp("results/start", fmt.Sprintf("start-%s%s-%s-%d_", config.Runtime, config.RuntimeMeta, config.Snapshotter, config.Iterations))
	if err != nil {
		log.Fatalf("Failed to create result file: %v", err)
	}
	defer f.Close()

	buffer := bufio.NewWriter(f)
	defer buffer.Flush()

	_, err = fmt.Fprintf(buffer, "%s%s,%s,%d,ms\n", config.Runtime, config.RuntimeMeta, config.Snapshotter, config.Iterations)
	if err != nil {
		log.Fatalf("Error while writing to file: %v", err)
	}

	for _, t := range startTimes {
		buffer.WriteString(strconv.FormatInt(t.Milliseconds(), 10) + "\n")
	}
}

func getStarted(w http.ResponseWriter, r *http.Request) {
	elapsedTime := time.Since(startTime)
	startTimes = append(startTimes, elapsedTime)

	log.Printf("Started in %v\n", elapsedTime)

	w.WriteHeader(http.StatusOK)
}

func main() {
	config := benchmark.InitFlags()

	http.HandleFunc("/started", getStarted)

	server := http.Server{Addr: ":5000"}

	go func() {
		err := server.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error starting http server: %s\n", err)
		}
	}()

	startupBenchmark(config)
	writeResult(config)

	if err := server.Shutdown(context.TODO()); err != nil {
		log.Fatalf("Could not shut down server: %v", err)
	}
}
