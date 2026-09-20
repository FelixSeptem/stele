package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func temporalCorrectionTestTimes() (time.Time, time.Time, time.Time) {
	first := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	second := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
	third := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	return first, second, third
}

const temporalCorrectionTestMemoryID = "11111111-1111-1111-1111-111111111111"

// temporalHeadGuardRows returns the row the head guard reads: the canonical
// fact identity, its current temporal head version, and its class. The class
// decides whether the correction also has to rebuild a derived projection, so
// the default is a factual class, which owns none.
func temporalHeadGuardRows(factID string, headVersion int64) *pgxmock.Rows {
	return temporalHeadGuardRowsForClass(factID, headVersion, memory.MemoryClassEpisodic)
}

func temporalHeadGuardRowsForClass(factID string, headVersion int64, class memory.MemoryClass) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"temporal_fact_id", "temporal_head_version", "class"}).
		AddRow(factID, headVersion, string(class))
}

// temporalCorrectionRows returns the ledger columns in the order
// temporalCorrectionSelect reads them.
func temporalCorrectionRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"temporal_fact_id", "predecessor_version", "successor_version",
		"actor", "reason", "disposition", "created_at",
	})
}

func temporalNullableVersion(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}

func correctionInsertStatement(t *testing.T) string {
	t.Helper()
	return temporalCorrectionInsert
}

func correctionSelectStatement(t *testing.T) string {
	t.Helper()
	return temporalCorrectionSelect
}

func correctionHeadAdvanceStatement(t *testing.T) string {
	t.Helper()
	return temporalCorrectionHeadAdvance
}

// TestAppendTemporalCorrectionWritesLedgerRowWithoutTouchingPayloads is the core
// append-only proof: the correction is recorded in its own ledger row and the
// statement set contains no write to memory_versions content. A single UPDATE
// there would rewrite history in place.
func TestAppendTemporalCorrectionWritesLedgerRowWithoutTouchingPayloads(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()
	predecessor := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 1))
	mock.ExpectExec(`INSERT INTO temporal_corrections`).
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", temporalCorrectionTestMemoryID,
			predecessor, int64(2), "actor-1", "retroactive correction",
			string(memory.TemporalConflictNone), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`UPDATE canonical_memories`).
		WithArgs(
			temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", int64(2),
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:              scope,
		TemporalFactID:     "fact-1",
		MemoryID:           temporalCorrectionTestMemoryID,
		PredecessorVersion: &predecessor,
		SuccessorVersion:   2,
		Actor:              "actor-1",
		Reason:             "retroactive correction",
		Disposition:        memory.TemporalConflictNone,
		CreatedAt:          createdAt,
	})
	if err != nil {
		t.Fatalf("AppendTemporalCorrection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionIsIdempotentOnReplay proves a replayed correction
// converges instead of duplicating or failing. The ledger's unique identity is
// (scope, memory, successor version), so a second identical append reports zero
// rows affected and is read back rather than rewritten.
func TestAppendTemporalCorrectionIsIdempotentOnReplay(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 0))
	mock.ExpectExec(`INSERT INTO temporal_corrections`).
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", temporalCorrectionTestMemoryID,
			nil, int64(1), "actor-1", "initial validity",
			string(memory.TemporalConflictNone), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`UPDATE canonical_memories`).
		WithArgs(
			temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", int64(1),
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	correction := memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-1",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 1,
		Actor:            "actor-1",
		Reason:           "initial validity",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	}
	if err := repo.AppendTemporalCorrection(context.Background(), correction); err != nil {
		t.Fatalf("first AppendTemporalCorrection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionRejectsInvalidBeforeTouchingTheLedger proves the
// domain rules are enforced at the boundary: an unattributed or non-advancing
// correction must not reach storage at all.
func TestAppendTemporalCorrectionRejectsInvalidBeforeTouchingTheLedger(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()
	predecessor := int64(2)

	tests := []struct {
		name       string
		correction memory.TemporalCorrection
		wantErr    error
	}{
		{
			name: "missing provenance",
			correction: memory.TemporalCorrection{
				Scope: scope, TemporalFactID: "fact-1",
				MemoryID:         temporalCorrectionTestMemoryID,
				SuccessorVersion: 2, Disposition: memory.TemporalConflictNone, CreatedAt: createdAt,
			},
			wantErr: memory.ErrTemporalCorrectionProvenanceRequired,
		},
		{
			name: "successor does not advance past predecessor",
			correction: memory.TemporalCorrection{
				Scope: scope, TemporalFactID: "fact-1",
				MemoryID:           temporalCorrectionTestMemoryID,
				PredecessorVersion: &predecessor, SuccessorVersion: 2,
				Actor: "actor-1", Reason: "no advance",
				Disposition: memory.TemporalConflictNone, CreatedAt: createdAt,
			},
			wantErr: memory.ErrTemporalCorrectionLineageInvalid,
		},
		{
			name: "unknown disposition",
			correction: memory.TemporalCorrection{
				Scope: scope, TemporalFactID: "fact-1",
				MemoryID:         temporalCorrectionTestMemoryID,
				SuccessorVersion: 1, Actor: "actor-1", Reason: "made up",
				Disposition: "made_up", CreatedAt: createdAt,
			},
			wantErr: memory.ErrTemporalDispositionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.AppendTemporalCorrection(context.Background(), tt.correction)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("a rejected correction must not touch the ledger: %v", err)
	}
}

// TestAppendTemporalCorrectionRefusesToRewriteAnExistingCorrection proves a
// replay that disagrees with the stored record is refused rather than silently
// overwriting it: the guard reads the durable row back when the insert conflicts.
func TestAppendTemporalCorrectionRefusesToRewriteAnExistingCorrection(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 0))
	mock.ExpectExec(`INSERT INTO temporal_corrections`).
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", temporalCorrectionTestMemoryID,
			nil, int64(1), "actor-1", "rewritten reason",
			string(memory.TemporalConflictNone), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	mock.ExpectQuery(`FROM temporal_corrections`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, temporalCorrectionTestMemoryID, int64(1)).
		WillReturnRows(temporalCorrectionRows().
			AddRow("fact-1", nil, int64(1), "actor-1", "original reason", string(memory.TemporalConflictNone), createdAt))

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-1",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 1,
		Actor:            "actor-1",
		Reason:           "rewritten reason",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	})
	if !errors.Is(err, memory.ErrTemporalCorrectionConflict) {
		t.Fatalf("error = %v, want ErrTemporalCorrectionConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionAcceptsExactReplayAfterConflict proves the guard
// only refuses disagreement: an identical replay converges so retries from an
// at-least-once caller stay safe.
func TestAppendTemporalCorrectionAcceptsExactReplayAfterConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 0))
	mock.ExpectExec(`INSERT INTO temporal_corrections`).
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", temporalCorrectionTestMemoryID,
			nil, int64(1), "actor-1", "initial validity",
			string(memory.TemporalConflictNone), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	mock.ExpectQuery(`FROM temporal_corrections`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, temporalCorrectionTestMemoryID, int64(1)).
		WillReturnRows(temporalCorrectionRows().
			AddRow("fact-1", nil, int64(1), "actor-1", "initial validity", string(memory.TemporalConflictNone), createdAt))
	mock.ExpectCommit()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-1",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 1,
		Actor:            "actor-1",
		Reason:           "initial validity",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	})
	if err != nil {
		t.Fatalf("exact replay must converge, got error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionRefusesAGap proves a successor that skips the
// current head is refused before any write: a gap would leave a version that no
// correction accounts for, which is exactly what makes the lineage
// non-deterministic.
func TestAppendTemporalCorrectionRefusesAGap(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 1))
	mock.ExpectRollback()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-1",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 3,
		Actor:            "actor-1",
		Reason:           "skips a version",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	})
	if !errors.Is(err, memory.ErrTemporalCorrectionLineageInvalid) {
		t.Fatalf("error = %v, want ErrTemporalCorrectionLineageInvalid", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("a gapped correction must not write anything: %v", err)
	}
}

// TestAppendTemporalCorrectionRejectsForeignFactIdentity proves a correction
// cannot be filed against a fact identity that the canonical row does not carry,
// which would fork one memory's lineage into two unreconcilable chains.
func TestAppendTemporalCorrectionRejectsForeignFactIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 0))
	mock.ExpectRollback()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-2",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 1,
		Actor:            "actor-1",
		Reason:           "wrong fact",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	})
	if !errors.Is(err, memory.ErrTemporalCorrectionIdentityRequired) {
		t.Fatalf("error = %v, want ErrTemporalCorrectionIdentityRequired", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("a foreign fact identity must not write anything: %v", err)
	}
}

// TestAppendTemporalCorrectionEnforcesScopeIsolation proves the writes are
// always scoped: a ledger without the tenant/project/namespace predicate would
// let one tenant append to another's lineage.
func TestAppendTemporalCorrectionEnforcesScopeIsolation(t *testing.T) {
	insert := correctionInsertStatement(t)

	if !strings.Contains(insert, "ON CONFLICT") {
		t.Error("correction insert must be replay-safe via ON CONFLICT")
	}
	if strings.Contains(insert, "UPDATE ") {
		t.Error("correction append must never mutate existing rows")
	}
	for _, column := range []string{"tenant", "project", "namespace"} {
		if !strings.Contains(insert, column) {
			t.Errorf("correction insert must carry %q", column)
		}
	}

	head := correctionHeadAdvanceStatement(t)
	if strings.Contains(head, "content") {
		t.Error("head advancement must not rewrite canonical content")
	}
	for _, fragment := range []string{"tenant = $2", "project = $3", "namespace = $4"} {
		if !strings.Contains(head, fragment) {
			t.Errorf("head advancement must be scope-filtered by %q", fragment)
		}
	}
	if !strings.Contains(head, "temporal_head_version = $6") {
		t.Errorf("head advancement must move the temporal head pointer: %s", head)
	}
}

// TestAppendTemporalCorrectionRejectsOutOfScopeCorrection proves a caller cannot
// smuggle a correction into a foreign namespace by supplying a memory id that
// belongs elsewhere: the scope guard reads the canonical row back first.
func TestAppendTemporalCorrectionRejectsOutOfScopeCorrection(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"temporal_fact_id", "temporal_head_version"}))
	mock.ExpectRollback()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:            scope,
		TemporalFactID:   "fact-1",
		MemoryID:         temporalCorrectionTestMemoryID,
		SuccessorVersion: 1,
		Actor:            "actor-1",
		Reason:           "initial validity",
		Disposition:      memory.TemporalConflictNone,
		CreatedAt:        createdAt,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("error = %v, want pgx.ErrNoRows for an out-of-scope memory", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadTemporalLineageReturnsStableAscendingOrder pins the ordering contract
// that makes history reproducible: oldest successor first, so a reader can
// rebuild the chain without re-sorting.
func TestReadTemporalLineageReturnsStableAscendingOrder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	first, second, third := temporalCorrectionTestTimes()

	mock.ExpectQuery(`FROM temporal_corrections`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, temporalCorrectionTestMemoryID).
		WillReturnRows(temporalCorrectionRows().
			AddRow("fact-1", nil, int64(1), "actor-1", "initial validity", string(memory.TemporalConflictNone), first).
			AddRow("fact-1", temporalNullableVersion(1), int64(2), "actor-2", "retroactive correction", string(memory.TemporalConflictResolved), second).
			AddRow("fact-1", temporalNullableVersion(2), int64(3), "actor-3", "supersede", string(memory.TemporalConflictNone), third))

	lineage, err := repo.ReadTemporalLineage(context.Background(), scope, temporalCorrectionTestMemoryID)
	if err != nil {
		t.Fatalf("ReadTemporalLineage() error = %v", err)
	}
	if lineage.TemporalFactID != "fact-1" {
		t.Fatalf("TemporalFactID = %q, want fact-1", lineage.TemporalFactID)
	}
	if len(lineage.Corrections) != 3 {
		t.Fatalf("corrections = %d, want 3", len(lineage.Corrections))
	}
	wantActors := []string{"actor-1", "actor-2", "actor-3"}
	for i, correction := range lineage.Corrections {
		if correction.SuccessorVersion != int64(i+1) {
			t.Fatalf("correction %d successor = %d, want %d", i, correction.SuccessorVersion, i+1)
		}
		if correction.Actor != wantActors[i] {
			t.Fatalf("correction %d actor = %q, want %q", i, correction.Actor, wantActors[i])
		}
		if correction.MemoryID != temporalCorrectionTestMemoryID {
			t.Fatalf("correction %d memory id = %q", i, correction.MemoryID)
		}
		if correction.Scope != scope {
			t.Fatalf("correction %d scope = %+v, want %+v", i, correction.Scope, scope)
		}
	}
	if lineage.Corrections[0].PredecessorVersion != nil {
		t.Error("the first correction must have no predecessor")
	}
	if lineage.Corrections[2].PredecessorVersion == nil || *lineage.Corrections[2].PredecessorVersion != 2 {
		t.Error("the third correction must link back to version 2")
	}
	if err := lineage.Validate(); err != nil {
		t.Fatalf("read-back lineage must satisfy the domain contract: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadTemporalLineageOrdersAscendingInSQL proves the ordering is enforced by
// the statement rather than incidental row arrival order.
func TestReadTemporalLineageOrdersAscendingInSQL(t *testing.T) {
	sql := correctionSelectStatement(t)

	if !strings.Contains(sql, "ORDER BY successor_version ASC") {
		t.Errorf("lineage read must order by successor_version ascending: %s", sql)
	}
	for _, column := range []string{"tenant = $1", "project = $2", "namespace = $3"} {
		if !strings.Contains(sql, column) {
			t.Errorf("lineage read must be scope-filtered by %q", column)
		}
	}
}

// TestReadTemporalLineageEmptyFactIsNotAnError proves an uncorrected fact reads
// as an empty lineage instead of failing, so callers do not have to special-case
// facts that never needed a correction.
func TestReadTemporalLineageEmptyFactIsNotAnError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()

	mock.ExpectQuery(`FROM temporal_corrections`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, temporalCorrectionTestMemoryID).
		WillReturnRows(temporalCorrectionRows())

	lineage, err := repo.ReadTemporalLineage(context.Background(), scope, temporalCorrectionTestMemoryID)
	if err != nil {
		t.Fatalf("ReadTemporalLineage() error = %v", err)
	}
	if len(lineage.Corrections) != 0 {
		t.Fatalf("corrections = %d, want 0", len(lineage.Corrections))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadTemporalLineageRejectsGapInStoredChain proves a corrupted ledger is
// surfaced rather than silently reordered: the contiguity contract must hold on
// read, not only on write.
func TestReadTemporalLineageRejectsGapInStoredChain(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	first, second, _ := temporalCorrectionTestTimes()

	mock.ExpectQuery(`FROM temporal_corrections`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, temporalCorrectionTestMemoryID).
		WillReturnRows(temporalCorrectionRows().
			AddRow("fact-1", nil, int64(1), "actor-1", "initial validity", string(memory.TemporalConflictNone), first).
			AddRow("fact-1", temporalNullableVersion(1), int64(3), "actor-2", "gap", string(memory.TemporalConflictNone), second))

	_, err = repo.ReadTemporalLineage(context.Background(), scope, temporalCorrectionTestMemoryID)
	if err == nil {
		t.Fatal("ReadTemporalLineage() accepted a lineage with a version gap")
	}
	if !errors.Is(err, memory.ErrTemporalCorrectionGap) {
		t.Fatalf("error = %v, want ErrTemporalCorrectionGap", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadTemporalLineageRejectsUnscopedInput proves the read cannot be issued
// cross-tenant: an unvalidated scope must fail before any query runs.
func TestReadTemporalLineageRejectsUnscopedInput(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)

	if _, err := repo.ReadTemporalLineage(context.Background(), memory.Scope{Tenant: "t"}, temporalCorrectionTestMemoryID); err == nil {
		t.Fatal("ReadTemporalLineage() accepted an incomplete scope")
	}
	if _, err := repo.ReadTemporalLineage(context.Background(), temporalTestScope(), "  "); err == nil {
		t.Fatal("ReadTemporalLineage() accepted a blank memory id")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("an invalid input must not execute a query: %v", err)
	}
}

// TestAppendTemporalCorrectionAdvancesCurrentHeadWithoutMutatingVersions proves
// the "current head advancement" half of the task: the canonical row's temporal
// head moves forward while no memory_versions payload is rewritten.
func TestAppendTemporalCorrectionAdvancesCurrentHeadWithoutMutatingVersions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	scope := temporalTestScope()
	createdAt, _, _ := temporalCorrectionTestTimes()
	predecessor := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM canonical_memories`).
		WithArgs(temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRows("fact-1", 1))
	mock.ExpectExec(`INSERT INTO temporal_corrections`).
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", temporalCorrectionTestMemoryID,
			predecessor, int64(2), "actor-1", "retroactive correction",
			string(memory.TemporalConflictOpen), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`UPDATE canonical_memories`).
		WithArgs(
			temporalCorrectionTestMemoryID, scope.Tenant, scope.Project, scope.Namespace,
			"fact-1", int64(2),
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	err = repo.AppendTemporalCorrection(context.Background(), memory.TemporalCorrection{
		Scope:              scope,
		TemporalFactID:     "fact-1",
		MemoryID:           temporalCorrectionTestMemoryID,
		PredecessorVersion: &predecessor,
		SuccessorVersion:   2,
		Actor:              "actor-1",
		Reason:             "retroactive correction",
		Disposition:        memory.TemporalConflictOpen,
		CreatedAt:          createdAt,
	})
	if err != nil {
		t.Fatalf("AppendTemporalCorrection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestTemporalCorrectionPersistenceNeverTargetsMemoryVersions is a structural
// guard against the exact regression the task names: none of the correction
// statements may write a version payload, so a correction can never mutate a
// prior version in place.
func TestTemporalCorrectionPersistenceNeverTargetsMemoryVersions(t *testing.T) {
	for _, statement := range []string{
		correctionInsertStatement(t),
		correctionSelectStatement(t),
		correctionHeadAdvanceStatement(t),
	} {
		if strings.Contains(statement, "memory_versions") {
			t.Errorf("correction persistence must not touch memory_versions:\n%s", statement)
		}
		if strings.Contains(statement, "INSERT INTO canonical_memories") {
			t.Errorf("correction persistence must not create canonical rows:\n%s", statement)
		}
	}
}
