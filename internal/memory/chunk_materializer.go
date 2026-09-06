package memory

import (
	"context"
	"fmt"
)

// MemoryChunkStore persists derived chunks without making them canonical.
type MemoryChunkStore interface {
	CreateMemoryChunks(context.Context, []MemoryChunk, string) ([]MemoryChunk, error)
}

// MemoryChunkMaterializer turns one authorized immutable source snapshot into
// derived chunks and delegates durable idempotency to the store.
type MemoryChunkMaterializer struct {
	Store MemoryChunkStore
}

func (m MemoryChunkMaterializer) MaterializeMemoryChunks(ctx context.Context, input ChunkingInput) ([]MemoryChunk, error) {
	if m.Store == nil {
		return nil, fmt.Errorf("memory chunk store is required")
	}
	if input.Source.LifecycleState != "" && input.Source.LifecycleState != MemoryStateActive {
		return nil, fmt.Errorf("chunk source lifecycle state %q is not visible", input.Source.LifecycleState)
	}
	chunks, err := ChunkText(input)
	if err != nil {
		return nil, err
	}
	return m.Store.CreateMemoryChunks(ctx, chunks, input.Policy.CounterVersion)
}
