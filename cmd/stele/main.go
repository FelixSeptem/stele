package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
)

var newRunner = app.NewRunner

func main() {
	if len(os.Args) > 1 && os.Args[1] == "benchmark" {
		if err := runBenchmark(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("benchmark command must be one of: list, run-smoke")
	}
	encoder := json.NewEncoder(stdout)
	switch args[0] {
	case "list":
		return encoder.Encode(benchmark.DefaultRegistry().List())
	case "run-smoke":
		dataDir := os.Getenv("STELE_BENCHMARK_DATA_DIR")
		if dataDir == "" {
			return fmt.Errorf("STELE_BENCHMARK_DATA_DIR is required for benchmark run-smoke")
		}
		result, err := benchmark.RunLoCoMoSmoke(benchmark.NewCache(dataDir), memory.Scope{Tenant: "benchmark", Project: "locomo-smoke", Namespace: "offline"})
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	default:
		_, _ = fmt.Fprintln(stderr, "benchmark command must be one of: list, run-smoke")
		return fmt.Errorf("unsupported benchmark command %q", args[0])
	}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	runner, err := newRunner(cfg)
	if err != nil {
		return err
	}

	return runner.Start(context.Background())
}
