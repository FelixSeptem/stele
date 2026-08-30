package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
)

var newRunner = app.NewRunner

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("benchmark command must be one of: list, run-smoke, run-postgres-smoke")
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
	case "run-postgres-smoke":
		dsn := os.Getenv("STELE_POSTGRES_DSN")
		if dsn == "" {
			return fmt.Errorf("STELE_POSTGRES_DSN is required for benchmark run-postgres-smoke")
		}
		result, err := benchmark.RunLoCoMoPostgresSmoke(context.Background(), dsn, memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "local"})
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	default:
		_, _ = fmt.Fprintln(stderr, "benchmark command must be one of: list, run-smoke, run-postgres-smoke")
		return fmt.Errorf("unsupported benchmark command %q", args[0])
	}
}

func run() error {
	return runArgs(os.Args[1:])
}

func runArgs(args []string) error {
	if len(args) > 0 && args[0] == "benchmark" {
		return runBenchmark(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "migrate" {
		return runMigrate(args[1:])
	}
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	runner, err := newRunner(cfg)
	if err != nil {
		return err
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runner.Start(runContext)
}

func runMigrate(args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "up") {
		return fmt.Errorf("usage: stele migrate <status|up>")
	}
	dsn := os.Getenv("STELE_POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("STELE_POSTGRES_DSN is required")
	}
	runner := postgres.NewMigrationRunner()
	ctx := context.Background()
	if args[0] == "status" {
		state, err := runner.Status(ctx, dsn)
		if err != nil {
			return err
		}
		if os.Getenv("STELE_MIGRATION_OUTPUT") == "json" {
			return json.NewEncoder(os.Stdout).Encode(state)
		}
		fmt.Fprintf(os.Stdout, "status=%s current_version=%d latest_version=%d dirty=%t pending=%t\n", state.Status, state.CurrentVersion, state.LatestVersion, state.Dirty, state.Pending)
		return nil
	}
	return runner.Apply(ctx, dsn)
}
