package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/config"
)

var newRunner = app.NewRunner

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
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

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runner.Start(runContext)
}
