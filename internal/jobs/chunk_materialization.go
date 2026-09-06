package jobs

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

// MemoryChunkMaterializationStore provides bounded, exact-scope source inputs
// and durable idempotent chunk writes.
type MemoryChunkMaterializationStore interface {
	ListChunkingInputs(context.Context, memory.Scope, int) ([]memory.ChunkingInput, error)
	CreateMemoryChunks(context.Context, []memory.MemoryChunk, string) ([]memory.MemoryChunk, error)
}

// MemoryChunkMaterializationJob is an opt-in maintenance job. It does not
// discover sources outside the supplied scope and leaves rollout decisions to
// retrieval configuration.
type MemoryChunkMaterializationJob struct {
	Scope memory.Scope
	Store MemoryChunkMaterializationStore
	Limit int
}

func (j MemoryChunkMaterializationJob) Name() string { return "memory_chunk_materialization" }

func (j MemoryChunkMaterializationJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Store == nil {
		return 0, fmt.Errorf("memory chunk materialization store is required")
	}
	limit := j.Limit
	if limit <= 0 {
		limit = 100
	}
	inputs, err := j.Store.ListChunkingInputs(ctx, j.Scope, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, input := range inputs {
		if input.Scope.Normalized() != j.Scope.Normalized() {
			continue
		}
		chunks, err := memory.ChunkText(input)
		if err != nil {
			return processed, err
		}
		if _, err := j.Store.CreateMemoryChunks(ctx, chunks, input.Policy.CounterVersion); err != nil {
			return processed, err
		}
		processed += len(chunks)
	}
	return processed, nil
}
