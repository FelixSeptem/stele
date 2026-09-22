package postgres

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/pashagolub/pgxmock/v4"
)

// TestGraphTraversalFixedClockFixtureReplayAndBoundaries keeps the graph
// traversal fixture intentionally deterministic.  The rows are returned in a
// deliberately non-priority order so the repository's stable ordering is part
// of the replay contract rather than an artifact of PostgreSQL insertion order.
func TestGraphTraversalFixedClockFixtureReplayAndBoundaries(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-fixed", Project: "project-fixed", Namespace: "namespace-fixed"}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	rows := func() *pgxmock.Rows {
		return pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content",
			"created_at", "updated_at", "root_seed", "depth", "edge_ids",
			"source_versions", "relation_types", "confidence", "graph_updated_at",
			"source_reliability", "cycle_detected",
		}).
			AddRow("hop3", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "hop3", now.Add(-3*time.Minute), now.Add(-3*time.Minute), "seed-high-degree", 3, []string{"e1", "e2", "e3"}, []string{"v1", "v2", "v3"}, []string{"knows", "supports", "depends_on"}, 0.7, now.Add(-3*time.Minute), 1.0, false).
			AddRow("duplicate-b", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "duplicate-b", now.Add(-2*time.Minute), now.Add(-2*time.Minute), "seed-high-degree", 2, []string{"e1", "e2b"}, []string{"v1", "v2b"}, []string{"knows", "supports"}, 0.8, now.Add(-2*time.Minute), 1.0, false).
			AddRow("cycle", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "cycle", now, now, "seed-high-degree", 2, []string{"e1", "e1"}, []string{"v1", "v1"}, []string{"knows", "knows"}, 0.99, now, 1.0, true).
			AddRow("duplicate-a", scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassRelation, memory.MemoryStateActive, "duplicate-a", now.Add(-1*time.Minute), now.Add(-1*time.Minute), "seed-high-degree", 1, []string{"e1"}, []string{"v1"}, []string{"knows"}, 0.9, now.Add(-1*time.Minute), 1.0, false)
	}

	queryWithAllBoundaries := `WITH RECURSIVE graph AS`

	tests := []struct {
		name          string
		limits        retrieval.GraphTraversalLimits
		constraint    memory.TemporalConstraint
		rows          func() *pgxmock.Rows
		queryPattern  string
		wantIDs       []string
		wantTruncate  retrieval.GraphTraversalTruncation
		wantCycles    int
		wantPathsSeen int
	}{
		{
			name: "default one hop",
			limits: func() retrieval.GraphTraversalLimits {
				limits := retrieval.DefaultGraphTraversalLimits()
				limits.MaxHops = 1
				return limits
			}(),
			constraint:    memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent},
			rows:          rows,
			queryPattern:  queryWithAllBoundaries,
			wantIDs:       []string{"duplicate-a"},
			wantTruncate:  retrieval.GraphTraversalTruncationCycle,
			wantCycles:    1,
			wantPathsSeen: 4,
		},
		{
			name: "two hop multi-hop evidence",
			limits: func() retrieval.GraphTraversalLimits {
				limits := retrieval.DefaultGraphTraversalLimits()
				limits.MaxHops = 2
				return limits
			}(),
			constraint:    memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: ptrTime(now.Add(-30 * time.Second))},
			rows:          rows,
			queryPattern:  queryWithAllBoundaries,
			wantIDs:       []string{"duplicate-a", "duplicate-b"},
			wantTruncate:  retrieval.GraphTraversalTruncationCycle,
			wantCycles:    1,
			wantPathsSeen: 4,
		},
		{
			name: "three hop bounded path",
			limits: func() retrieval.GraphTraversalLimits {
				limits := retrieval.DefaultGraphTraversalLimits()
				limits.MaxHops = 3
				return limits
			}(),
			constraint:    memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: ptrTime(now.Add(-5 * time.Minute)), ValidTo: ptrTime(now.Add(time.Minute))},
			rows:          rows,
			queryPattern:  queryWithAllBoundaries,
			wantIDs:       []string{"duplicate-a", "duplicate-b", "hop3"},
			wantTruncate:  retrieval.GraphTraversalTruncationCycle,
			wantCycles:    1,
			wantPathsSeen: 4,
		},
		{
			name:   "missing projection is empty and stable",
			limits: retrieval.DefaultGraphTraversalLimits(),
			rows: func() *pgxmock.Rows {
				return pgxmock.NewRows([]string{
					"id", "tenant", "project", "namespace", "class", "state", "content",
					"created_at", "updated_at", "root_seed", "depth", "edge_ids",
					"source_versions", "relation_types", "confidence", "graph_updated_at",
					"source_reliability", "cycle_detected",
				})
			},
			queryPattern:  `WITH RECURSIVE graph AS`,
			wantIDs:       []string{},
			wantTruncate:  retrieval.GraphTraversalTruncationNone,
			wantCycles:    0,
			wantPathsSeen: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runFixedGraphFixture(t, scope, now, test.limits, test.constraint, test.rows(), test.queryPattern)
			if got.err != nil {
				t.Fatal(got.err)
			}
			if got.ids == nil {
				got.ids = []string{}
			}
			if !reflect.DeepEqual(got.ids, test.wantIDs) {
				t.Fatalf("ordered endpoint IDs = %v, want %v", got.ids, test.wantIDs)
			}
			if got.result.Truncation != test.wantTruncate || got.result.CyclesSeen != test.wantCycles || got.result.PathsSeen != test.wantPathsSeen {
				t.Fatalf("result = %+v, want truncation=%q cycles=%d paths=%d", got.result, test.wantTruncate, test.wantCycles, test.wantPathsSeen)
			}
		})
	}

	// Re-run the same fixed-clock fixture twice through fresh pools.  This
	// catches accidental dependence on database row order or wall-clock time.
	first := runFixedGraphFixture(t, scope, now, func() retrieval.GraphTraversalLimits {
		limits := retrieval.DefaultGraphTraversalLimits()
		limits.MaxHops = 3
		return limits
	}(), memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}, rows(), queryWithAllBoundaries)
	second := runFixedGraphFixture(t, scope, now, func() retrieval.GraphTraversalLimits {
		limits := retrieval.DefaultGraphTraversalLimits()
		limits.MaxHops = 3
		return limits
	}(), memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}, rows(), queryWithAllBoundaries)
	if first.err != nil || second.err != nil {
		t.Fatalf("replay fixture errors: first=%v second=%v", first.err, second.err)
	}
	if !reflect.DeepEqual(first.ids, second.ids) || !reflect.DeepEqual(first.result, second.result) {
		t.Fatalf("fixed-clock replay diverged: first=(%v,%+v) second=(%v,%+v)", first.ids, first.result, second.ids, second.result)
	}
}

type fixedGraphFixtureResult struct {
	ids    []string
	result retrieval.GraphTraversalResult
	err    error
}

func runFixedGraphFixture(t *testing.T, scope memory.Scope, now time.Time, limits retrieval.GraphTraversalLimits, constraint memory.TemporalConstraint, rows *pgxmock.Rows, queryPattern string) fixedGraphFixtureResult {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		return fixedGraphFixtureResult{err: err}
	}
	defer mock.Close()
	var asOf, validFrom, validTo any
	mode := constraint.Mode
	if mode == "" {
		mode = memory.TemporalSelectionCurrent
	}
	if constraint.AsOf != nil {
		asOf = *constraint.AsOf
	}
	if constraint.ValidFrom != nil {
		validFrom = *constraint.ValidFrom
	}
	if constraint.ValidTo != nil {
		validTo = *constraint.ValidTo
	}
	mock.ExpectQuery(queryPattern).WithArgs(scope.Tenant, scope.Project, scope.Namespace, []string{"seed-high-degree"}, limits.MaxHops, limits.MaxPathsPerRequest, nil, nil, string(mode), now, asOf, validFrom, validTo).WillReturnRows(rows)
	repo := NewRepository(mock)
	repo.now = func() time.Time { return now }
	result, err := repo.ExpandGraphResult(context.Background(), retrieval.GraphTraversalInput{Scope: scope, SeedMemoryIDs: []string{"seed-high-degree"}, TemporalConstraint: constraint, Limits: limits})
	if err != nil {
		return fixedGraphFixtureResult{result: result, err: err}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		return fixedGraphFixtureResult{result: result, err: err}
	}
	ids := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.Memory.ID)
	}
	return fixedGraphFixtureResult{ids: ids, result: result}
}

func ptrTime(value time.Time) *time.Time { return &value }
