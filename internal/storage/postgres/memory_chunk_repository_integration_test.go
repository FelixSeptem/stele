package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// These tests intentionally require an explicit, operator-owned DSN. They do
// not create or destroy databases and are therefore safe to run against a
// disposable PostgreSQL + pgvector instance in CI.
func TestMemoryChunkPostgresConcurrentMaterializationIsIdempotentAndScoped(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_CHUNK_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_CHUNK_DSN is not configured; skipping chunk integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "chunk-it", Project: "p", Namespace: "n"}
	eventID := "00000000-0000-0000-0000-000000000101"
	if _, err := pool.Exec(ctx, `INSERT INTO raw_events (id,tenant,project,namespace,event_type,content,memory_session_id) VALUES ($1,$2,$3,$4,'conversation.message','first paragraph.',$5) ON CONFLICT DO NOTHING`, eventID, scope.Tenant, scope.Project, scope.Namespace, "session-1"); err != nil {
		t.Fatal(err)
	}
	chunks, err := memory.ChunkText(memory.ChunkingInput{
		Source: memory.ChunkSourceReference{Kind: memory.ChunkSourceKindRawEvent, ID: eventID, Version: 1, Scope: scope, SessionID: "session-1", UserID: "user-1"},
		Scope:  scope, Class: memory.MemoryClassEpisodic, Content: "first paragraph.", Policy: memory.DefaultChunkPolicy("policy-it", "renderer-it"),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.CreateMemoryChunks(ctx, chunks, "whitespace-v1"); err != nil {
				t.Errorf("CreateMemoryChunks() error = %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := repo.ListMemoryChunksBySource(ctx, scope, chunks[0].Source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(chunks) {
		t.Fatalf("visible chunks = %d, want %d", len(got), len(chunks))
	}
	foreign := scope
	foreign.Tenant = "foreign"
	if _, err := repo.ListMemoryChunksBySource(ctx, foreign, chunks[0].Source); err == nil {
		t.Fatal("foreign scope unexpectedly read chunks")
	}
}
