package app

import (
	"context"
	"errors"
	"testing"
)

func TestReadinessGateBecomesNotReadyWhenDraining(t *testing.T) {
	gate := &readinessGate{checker: readinessFunc(func(context.Context) error { return nil })}
	if err := gate.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() before drain = %v, want nil", err)
	}
	gate.BeginDrain()
	if err := gate.Ready(context.Background()); !errors.Is(err, errRuntimeDraining) {
		t.Fatalf("Ready() during drain = %v, want draining error", err)
	}
}
