package jobs

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type chunkMaterializationStoreStub struct {
	inputs []memory.ChunkingInput
	seen   int
}

func (s *chunkMaterializationStoreStub) ListChunkingInputs(_ context.Context, _ memory.Scope, _ int) ([]memory.ChunkingInput, error) {
	return s.inputs, nil
}

func (s *chunkMaterializationStoreStub) CreateMemoryChunks(_ context.Context, chunks []memory.MemoryChunk, _ string) ([]memory.MemoryChunk, error) {
	s.seen += len(chunks)
	return chunks, nil
}

func TestMemoryChunkMaterializationJobProcessesBoundedInputs(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &chunkMaterializationStoreStub{inputs: []memory.ChunkingInput{{Scope: scope, Source: memory.ChunkSourceReference{Kind: memory.ChunkSourceKindRawEvent, ID: "e", Version: 1, Scope: scope}, Class: memory.MemoryClassEpisodic, Content: "one\n\ntwo", Policy: memory.DefaultChunkPolicy("p", "r")}}}
	job := MemoryChunkMaterializationJob{Scope: scope, Store: store, Limit: 1}
	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 2 || store.seen != 2 {
		t.Fatalf("processed=%d seen=%d, want two derived chunks", processed, store.seen)
	}
}

func TestMemoryChunkMaterializationJobRequiresExactScopeAndDependencies(t *testing.T) {
	if _, err := (MemoryChunkMaterializationJob{}).Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil without scope/store")
	}
	if _, err := (MemoryChunkMaterializationJob{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}}).Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil without store")
	}
}
