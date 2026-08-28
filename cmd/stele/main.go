package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
)

var newRunner = app.NewRunner

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	return runArgs(os.Args[1:])
}

func runArgs(args []string) error {
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
