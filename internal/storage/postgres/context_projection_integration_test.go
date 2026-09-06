package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

func TestContextProjectionPostgresConcurrentRebuildIsScopedAndIdempotent(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_PROJECTION_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_PROJECTION_DSN is not configured; skipping real projection integration test")
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
	scope := memory.Scope{Tenant: "projection-it", Project: "p", Namespace: "n"}
	input := memory.MaterializeContextProjectionInput{Scope: scope, Kind: memory.ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "s", RendererVersion: "r", Policy: memory.DefaultContextProjectionPolicy("p"), Sources: []memory.ContextProjectionCandidate{{Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "00000000-0000-0000-0000-000000000001", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "value", Confidence: 1}}}
	projection, err := memory.MaterializeContextProjection(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	results := make(chan memory.ContextProjection, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := repo.CreateContextProjection(ctx, projection)
			if e == nil {
				results <- p
			}
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for range results {
		count++
	}
	if count != 2 {
		t.Fatalf("successful concurrent writes = %d, want 2 idempotent responses", count)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM context_projections WHERE tenant=$1 AND project=$2 AND namespace=$3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("projection rows = %d, want one idempotent row", rows)
	}
	foreign := scope
	foreign.Tenant = "foreign"
	if _, err := repo.ReadLatestContextProjection(ctx, foreign, memory.ContextProjectionKindAlwaysVisible); err == nil {
		t.Fatal("foreign scope unexpectedly read projection")
	}
}

func TestContextProjectionPostgresRebuildUsesLatestInScopeCanonicalVersion(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_PROJECTION_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_PROJECTION_DSN is not configured; skipping real projection integration test")
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

	scope := memory.Scope{Tenant: "projection-version-" + uuid.NewString(), Project: "p", Namespace: "n"}
	foreign := memory.Scope{Tenant: "foreign-" + uuid.NewString(), Project: "p", Namespace: "n"}
	memoryID := uuid.New()
	supersededVersionID := uuid.New()
	latestVersionID := uuid.New()
	foreignMemoryID := uuid.New()
	foreignVersionID := uuid.New()
	for _, row := range []struct {
		id      uuid.UUID
		scope   memory.Scope
		content string
	}{
		{id: memoryID, scope: scope, content: "canonical shell content"},
		{id: foreignMemoryID, scope: foreign, content: "foreign canonical content"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO canonical_memories (id, tenant, project, namespace, class, state, content)
VALUES ($1, $2, $3, $4, 'profile', 'active', $5)`, row.id, row.scope.Tenant, row.scope.Project, row.scope.Namespace, row.content); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		id       uuid.UUID
		memoryID uuid.UUID
		version  int
		content  string
	}{
		{id: supersededVersionID, memoryID: memoryID, version: 1, content: "superseded projection content"},
		{id: latestVersionID, memoryID: memoryID, version: 2, content: "latest projection content"},
		{id: foreignVersionID, memoryID: foreignMemoryID, version: 9, content: "foreign projection content"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO memory_versions (id, memory_id, version, state, content, modified_by)
VALUES ($1, $2, $3, 'active', $4, 'projection-integration-test')`, row.id, row.memoryID, row.version, row.content); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewRepository(pool)
	candidates, err := repo.ListContextProjectionCandidates(ctx, scope, memory.ContextProjectionKindAlwaysVisible, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1 scoped canonical candidate", len(candidates))
	}
	if candidates[0].Source.ID != latestVersionID.String() || candidates[0].Source.Version != 2 || candidates[0].Content != "latest projection content" {
		t.Fatalf("candidate = %+v, want latest in-scope canonical version", candidates[0])
	}

	projection, err := memory.RebuildContextProjectionFromStore(ctx, memory.ContextProjectionRebuildRequest{
		Scope:           scope,
		Kind:            memory.ContextProjectionKindAlwaysVisible,
		Limit:           10,
		SchemaVersion:   "schema-v1",
		Policy:          memory.DefaultContextProjectionPolicy("policy-v1"),
		RendererVersion: "renderer-v1",
	}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Items) != 1 || projection.Items[0].Source.ID != latestVersionID.String() || projection.Items[0].Text != "latest projection content" {
		t.Fatalf("projection items = %+v, want only latest in-scope version", projection.Items)
	}
	read, err := repo.ReadLatestContextProjection(ctx, scope, memory.ContextProjectionKindAlwaysVisible)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Items) != 1 || read.Items[0].Source.ID != latestVersionID.String() || read.Items[0].Text != "latest projection content" {
		t.Fatalf("stored items = %+v, want latest in-scope version only", read.Items)
	}
}
