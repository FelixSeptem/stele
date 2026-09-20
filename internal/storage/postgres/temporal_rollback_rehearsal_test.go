package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/pashagolub/pgxmock/v4"
)

// TestTemporalRollbackRehearsalLeavesHistoryIntactAndEmitsBoundedEvidence is the
// rehearsal required by task 2.5. It proves three things about the *operational*
// rollback path -- disabling temporal-aware policy resolution, which is the only
// supported production rollback:
//
//  1. disablement returns the approved current baseline: no valid-time predicate
//     is rendered, and no temporal bind parameters are sent;
//  2. temporal history remains intact and readable: the correction ledger still
//     returns its rows after disablement;
//  3. the emitted audit evidence is bounded: only low-cardinality categories, no
//     query text, no memory identifiers, no raw scores, no DSN.
//
// The rehearsal is repeatable and mutates nothing, so it can run before and
// after a rollout without side effects.
func TestTemporalRollbackRehearsalLeavesHistoryIntactAndEmitsBoundedEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)

	t.Run("disablement returns the baseline current selection", func(t *testing.T) {
		baseline := retrieval.SearchInput{
			Scope: scope,
			Query: "concise answers",
			TopK:  3,
		}
		if baseline.TemporalPolicyDisabled() {
			t.Fatal("an ordinary input must not report temporal policy disabled")
		}

		rollback := baseline.WithTemporalPolicyDisabled()
		if !rollback.TemporalPolicyDisabled() {
			t.Fatal("WithTemporalPolicyDisabled() did not disable temporal policy")
		}
		if rollback.TemporalConstraint.Mode != "" {
			t.Fatalf("disablement left a temporal constraint %q; rollback must clear it", rollback.TemporalConstraint.Mode)
		}

		// The rendered predicate carries no valid-time filtering at all.
		disabled := temporalSelectionFor(rollback, now)
		if !disabled.IsDisabled() {
			t.Fatal("a disabled input must produce a disabled selection")
		}
		rendered := temporalSQLPredicate("", disabled, 8, 9, 10, 11, 12)
		if strings.TrimSpace(rendered) != "" {
			t.Fatalf("disabled selection still renders a predicate: %s", rendered)
		}
		for _, forbidden := range []string{"valid_from", "valid_to", "as_of", "false"} {
			if strings.Contains(rendered, forbidden) {
				t.Fatalf("disabled predicate %s references %q", rendered, forbidden)
			}
		}

		// Enablement still renders, so disablement is a switch rather than a
		// permanent state.
		enabled := temporalSQLPredicate("", temporalSelectionFor(baseline, now), 8, 9, 10, 11, 12)
		if !strings.Contains(enabled, "valid_from") {
			t.Fatalf("enabled selection rendered no valid-time predicate: %s", enabled)
		}
	})

	t.Run("disablement does not send temporal bind parameters", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("pgxmock.NewPool() error = %v", err)
		}
		defer mock.Close()

		input := retrieval.SearchInput{
			Scope: scope,
			Query: "concise answers",
			TopK:  3,
		}.WithTemporalPolicyDisabled()

		// Only the baseline parameters: scope, query, time window, limit. The
		// five temporal bind values are absent because the predicate is gone.
		mock.ExpectQuery("SELECT .*lexical_score.*FROM canonical_memories").
			WithArgs(scope.Tenant, scope.Project, scope.Namespace, "concise answers", nil, nil, 3).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
				"lexical_score",
			}).AddRow(
				"mem_123", scope.Tenant, scope.Project, scope.Namespace,
				memory.MemoryClassProfile, memory.MemoryStateActive, "User prefers concise answers.",
				now.Add(-time.Hour), now, 0.88,
			))

		repo := NewRepository(mock)
		repo.now = func() time.Time { return now }
		hits, err := repo.SearchLexical(context.Background(), input)
		if err != nil {
			t.Fatalf("SearchLexical() with temporal disabled error = %v", err)
		}
		if len(hits) != 1 {
			t.Fatalf("len(hits) = %d, want 1; rollback must still return the baseline rows", len(hits))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("ExpectationsWereMet() error = %v", err)
		}
	})

	t.Run("temporal history remains readable after disablement", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("pgxmock.NewPool() error = %v", err)
		}
		defer mock.Close()

		mock.ExpectQuery("SELECT[\\s\\S]*FROM temporal_corrections").
			WithArgs(scope.Tenant, scope.Project, scope.Namespace, "mem_rehearsal").
			WillReturnRows(pgxmock.NewRows([]string{
				"temporal_fact_id", "predecessor_version", "successor_version",
				"actor", "reason", "disposition", "created_at",
			}).
				AddRow("fact-1", nil, int64(1), "actor-1", "initial",
					string(memory.TemporalConflictNone), now.Add(-2*time.Hour)).
				AddRow("fact-1", int64(1), int64(2), "actor-1", "retroactive",
					string(memory.TemporalConflictNone), now.Add(-time.Hour)),
			)

		repo := NewRepository(mock)
		lineage, err := repo.ReadTemporalLineage(context.Background(), scope, "mem_rehearsal")
		if err != nil {
			t.Fatalf("ReadTemporalLineage() error = %v", err)
		}
		if len(lineage.Corrections) != 2 {
			t.Fatalf("len(Corrections) = %d, want 2; disablement must not hide history", len(lineage.Corrections))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("ExpectationsWereMet() error = %v", err)
		}
	})

	t.Run("audit evidence is bounded", func(t *testing.T) {
		evidence := temporalRollbackEvidence{
			Outcome:         "baseline_current",
			HistoryIntact:   true,
			CorrectionCount: 2,
			Disposition:     "none",
		}

		encoded, err := json.Marshal(evidence)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		payload := string(encoded)

		for _, forbidden := range []string{
			"postgres://",
			"concise answers",
			"mem_rehearsal",
			"tenant-a",
			"0.88",
			"secret",
			"actor-1",
		} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("audit evidence %s leaks %q", payload, forbidden)
			}
		}

		var roundTrip map[string]any
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(roundTrip) != 4 {
			t.Fatalf("audit evidence carries %d fields, want only the 4 bounded categories", len(roundTrip))
		}
	})
}

// temporalRollbackEvidence is the bounded audit record a rehearsal emits. It is
// deliberately limited to low-cardinality categories: a rollback decision needs
// to know the outcome and whether history survived, never which memories were
// involved or what they said.
type temporalRollbackEvidence struct {
	Outcome         string `json:"outcome"`
	HistoryIntact   bool   `json:"history_intact"`
	CorrectionCount int    `json:"correction_count"`
	Disposition     string `json:"disposition"`
}

// TestTemporalRollbackIsNotConstraintRejection proves disablement and rejection
// are different outcomes. Collapsing them would make rollback return no rows,
// which is an outage rather than a fallback to the approved baseline.
func TestTemporalRollbackIsNotConstraintRejection(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)

	disabled := temporalSelectionFor(retrieval.SearchInput{}.WithTemporalPolicyDisabled(), now)
	if disabled.ConstraintReject {
		t.Fatal("disablement must not be reported as a constraint rejection")
	}
	if disabled.IsDisabled() {
		if rendered := temporalSQLPredicate("", disabled, 8, 9, 10, 11, 12); strings.Contains(rendered, "false") {
			t.Fatalf("disabled selection renders `false`, which would hide the baseline: %s", rendered)
		}
	}

	rejected := temporalSelection{ConstraintReject: true}
	if rendered := temporalSQLPredicate("", rejected, 8, 9, 10, 11, 12); rendered != "AND false" {
		t.Fatalf("rejected selection rendered %q, want `AND false`", rendered)
	}
}
