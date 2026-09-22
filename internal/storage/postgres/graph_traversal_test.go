package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryExpandGraphReturnsBoundedProofs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	limits := retrieval.DefaultGraphTraversalLimits()
	mock.ExpectQuery("WITH RECURSIVE graph AS").WithArgs(scope.Tenant, scope.Project, scope.Namespace, []string{"seed"}, limits.MaxHops, limits.MaxPathsPerRequest, nil, nil, string(memory.TemporalSelectionCurrent), now, nil, nil, nil).WillReturnRows(
		pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "root_seed", "depth", "edge_ids", "source_versions", "relation_types", "confidence", "graph_updated_at", "source_reliability", "cycle_detected"}).AddRow("endpoint", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "endpoint", now, now, "seed", 1, []string{"edge-1"}, []string{"7"}, []string{"knows"}, 1.0, now, 1.0, false),
	)
	repo := NewRepository(mock)
	repo.now = func() time.Time { return now }
	paths, err := repo.ExpandGraph(context.Background(), retrieval.GraphTraversalInput{Scope: scope, SeedMemoryIDs: []string{"seed"}, Limits: limits})
	if err != nil {
		t.Fatalf("ExpandGraph() error = %v", err)
	}
	if len(paths) != 1 || paths[0].Memory.ID != "endpoint" || paths[0].Proof.Hop != 1 {
		t.Fatalf("paths = %+v", paths)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations = %v", err)
	}
}

func TestRepositoryExpandGraphZeroHopDoesNotQueryProjectionStore(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-zero"}
	limits := retrieval.DefaultGraphTraversalLimits()
	limits.MaxHops = 0
	result, err := NewRepository(mock).ExpandGraphResult(context.Background(), retrieval.GraphTraversalInput{Scope: scope, SeedMemoryIDs: []string{"seed"}, Limits: limits})
	if err != nil {
		t.Fatalf("ExpandGraphResult() error = %v", err)
	}
	if len(result.Candidates) != 0 || result.Truncation != retrieval.GraphTraversalTruncationNone {
		t.Fatalf("zero-hop result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations = %v", err)
	}
}

func TestRepositoryExpandGraphReportsCycleAndPerSeedTruncation(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-cycle"}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	limits := retrieval.DefaultGraphTraversalLimits()
	limits.MaxPathsPerSeed = 1
	rows := pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "root_seed", "depth", "edge_ids", "source_versions", "relation_types", "confidence", "graph_updated_at", "source_reliability", "cycle_detected"})
	rows.AddRow("cycle", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "cycle", now, now, "seed", 2, []string{"edge-1", "edge-1"}, []string{"7", "7"}, []string{"knows", "knows"}, 0.8, now, 1.0, true)
	rows.AddRow("first", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "first", now, now, "seed", 1, []string{"edge-2"}, []string{"8"}, []string{"knows"}, 0.9, now, 1.0, false)
	rows.AddRow("second", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "second", now, now, "seed", 1, []string{"edge-3"}, []string{"9"}, []string{"knows"}, 0.7, now, 1.0, false)
	mock.ExpectQuery("WITH RECURSIVE graph AS").WithArgs(scope.Tenant, scope.Project, scope.Namespace, []string{"seed"}, limits.MaxHops, limits.MaxPathsPerRequest, nil, nil, string(memory.TemporalSelectionCurrent), now, nil, nil, nil).WillReturnRows(rows)
	repo := NewRepository(mock)
	repo.now = func() time.Time { return now }
	result, err := repo.ExpandGraphResult(context.Background(), retrieval.GraphTraversalInput{Scope: scope, SeedMemoryIDs: []string{"seed"}, Limits: limits})
	if err != nil {
		t.Fatalf("ExpandGraphResult() error = %v", err)
	}
	if result.Truncation != retrieval.GraphTraversalTruncationCycle || result.CyclesSeen != 1 || len(result.Candidates) != 1 || result.Candidates[0].Memory.ID != "first" {
		t.Fatalf("result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations = %v", err)
	}
}

func TestRepositoryExpandGraphOrdersByHopConfidenceFreshnessAndIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-order"}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	limits := retrieval.DefaultGraphTraversalLimits()
	rows := pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "root_seed", "depth", "edge_ids", "source_versions", "relation_types", "confidence", "graph_updated_at", "source_reliability", "cycle_detected"})
	rows.AddRow("z-hop2", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "z", now, now, "seed", 2, []string{"e2a", "e2b"}, []string{"2", "3"}, []string{"knows", "knows"}, 1.0, now, 1.0, false)
	rows.AddRow("b-hop1", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "b", now, now, "seed", 1, []string{"e1b"}, []string{"1"}, []string{"knows"}, 0.5, now, 1.0, false)
	rows.AddRow("a-hop1", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "a", now, now, "seed", 1, []string{"e1a"}, []string{"1"}, []string{"knows"}, 0.9, now, 1.0, false)
	mock.ExpectQuery("WITH RECURSIVE graph AS").WithArgs(scope.Tenant, scope.Project, scope.Namespace, []string{"seed"}, limits.MaxHops, limits.MaxPathsPerRequest, nil, nil, string(memory.TemporalSelectionCurrent), now, nil, nil, nil).WillReturnRows(rows)
	repo := NewRepository(mock)
	repo.now = func() time.Time { return now }
	result, err := repo.ExpandGraphResult(context.Background(), retrieval.GraphTraversalInput{Scope: scope, SeedMemoryIDs: []string{"seed"}, Limits: limits})
	if err != nil {
		t.Fatalf("ExpandGraphResult() error = %v", err)
	}
	if len(result.Candidates) != 3 {
		t.Fatalf("candidate count = %d result=%+v", len(result.Candidates), result)
	}
	if got := []string{result.Candidates[0].Memory.ID, result.Candidates[1].Memory.ID, result.Candidates[2].Memory.ID}; got[0] != "a-hop1" || got[1] != "b-hop1" || got[2] != "z-hop2" {
		t.Fatalf("ordered candidates = %v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations = %v", err)
	}
}
