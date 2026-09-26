package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type rememberIntentStub struct{ err error }

func (s rememberIntentStub) Submit(context.Context, memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	return memory.MemoryIntentRecord{}, s.err
}

func TestRememberMapsDurableIdempotencyConflictToBoundedConflict(t *testing.T) {
	conflict, ok := safeMutationError(memory.ErrIdempotencyConflict).(MCPError)
	if !ok || conflict.Category != ErrorValidation || conflict.Code != "idempotency_conflict" {
		t.Fatalf("safeMutationError(idempotency conflict) = %#v, want bounded validation conflict", conflict)
	}
	dependency, ok := safeMutationError(errors.New("database unavailable")).(MCPError)
	if !ok || dependency.Category != ErrorDependency || dependency.Code != "mutation_failed" {
		t.Fatalf("safeMutationError(dependency error) = %#v, want bounded dependency failure", dependency)
	}
}
