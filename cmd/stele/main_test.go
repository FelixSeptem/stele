package main

import (
	"context"
	"errors"
	"testing"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/config"
)

type stubRunner struct {
	called bool
	err    error
}

func (s *stubRunner) Start(ctx context.Context) error {
	s.called = true
	return s.err
}

func TestRunReturnsConfigErrorForInvalidEnv(t *testing.T) {
	t.Setenv("STELE_MODE", "bad-mode")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	if err := run(); err == nil {
		t.Fatal("run() error = nil, want config error")
	}
}

func TestRunStartsApplicationForValidEnv(t *testing.T) {
	t.Setenv("STELE_MODE", string(config.ModeAPI))
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	original := newRunner
	defer func() {
		newRunner = original
	}()

	runner := &stubRunner{}
	newRunner = func(cfg config.Config) (app.Runner, error) {
		return runner, nil
	}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !runner.called {
		t.Fatal("runner was not started")
	}
}

func TestRunReturnsRunnerConstructionFailure(t *testing.T) {
	t.Setenv("STELE_MODE", string(config.ModeAPI))
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	original := newRunner
	defer func() {
		newRunner = original
	}()

	newRunner = func(cfg config.Config) (app.Runner, error) {
		return nil, errors.New("runner build failed")
	}

	if err := run(); err == nil {
		t.Fatal("run() error = nil, want runner construction failure")
	}
}
