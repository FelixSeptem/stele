package memory

import (
	"context"
	"testing"
)

type memoryChunkStoreStub struct {
	chunks         []MemoryChunk
	counterVersion string
}

func (s *memoryChunkStoreStub) CreateMemoryChunks(_ context.Context, chunks []MemoryChunk, counterVersion string) ([]MemoryChunk, error) {
	s.chunks = append([]MemoryChunk(nil), chunks...)
	s.counterVersion = counterVersion
	return chunks, nil
}

func TestMemoryChunkMaterializerMaterializesExactVisibleSource(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &memoryChunkStoreStub{}
	service := MemoryChunkMaterializer{Store: store}
	chunks, err := service.MaterializeMemoryChunks(context.Background(), ChunkingInput{
		Scope:  scope,
		Source: ChunkSourceReference{Kind: ChunkSourceKindRawEvent, ID: "event-1", Version: 1, Scope: scope, LifecycleState: MemoryStateActive},
		Class:  MemoryClassEpisodic, Content: "first event\n\nsecond event", Policy: DefaultChunkPolicy("policy-v1", "renderer-v1"),
	})
	if err != nil {
		t.Fatalf("MaterializeMemoryChunks() error = %v", err)
	}
	if len(chunks) != 2 || len(store.chunks) != 2 || store.counterVersion != "whitespace-v1" {
		t.Fatalf("chunks=%+v stored=%+v counter=%q", chunks, store.chunks, store.counterVersion)
	}
}

func TestMemoryChunkMaterializerFailsClosedWithoutStoreOrVisibleSource(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	input := ChunkingInput{Scope: scope, Source: ChunkSourceReference{Kind: ChunkSourceKindRawEvent, ID: "event-1", Version: 1, Scope: scope, LifecycleState: MemoryStateSuppressed}, Class: MemoryClassEpisodic, Content: "secret", Policy: DefaultChunkPolicy("policy-v1", "renderer-v1")}
	if _, err := (MemoryChunkMaterializer{}).MaterializeMemoryChunks(context.Background(), input); err == nil {
		t.Fatal("MaterializeMemoryChunks() error = nil without store")
	}
	if _, err := (MemoryChunkMaterializer{Store: &memoryChunkStoreStub{}}).MaterializeMemoryChunks(context.Background(), input); err == nil {
		t.Fatal("MaterializeMemoryChunks() error = nil for hidden source")
	}
}
