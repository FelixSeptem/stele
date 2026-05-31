package main

import (
	"context"
	"log"

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

	return runner.Start(context.Background())
}
