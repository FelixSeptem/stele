package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/pashagolub/pgxmock/v4"
)

// These tests prove the second half of task 3.3: a write that violates the
// temporal identity/interval rules must be rejected *before* any content is
// persisted, so canonical and provenance state are left byte-for-byte unchanged.
//
// The mechanism that makes this possible is ordering: each path reads and
// validates the existing fact before issuing its first content-bearing
// statement. pgxmock enforces ordering strictly, so a test that expects
// BeginTx -> read -> Rollback with *no* INSERT/UPDATE expectation will fail
// loudly if the implementation ever starts writing before it validates.

func rejectTestScope() memory.Scope {
	return memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
}

// canonicalReadRow builds the 14-column canonical row used by the pre-write
// read on every mutation path.
func canonicalReadRow(memoryID string, scope memory.Scope, class memory.MemoryClass, content string, createdAt, updatedAt time.Time, validity memory.TemporalValidity) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source",
	}).AddRow(
		memoryID, scope.Tenant, scope.Project, scope.Namespace,
		class, memory.MemoryStateActive, content, createdAt, updatedAt,
		validity.TemporalFactID, validity.IngestedAt, validity.ValidFrom, validity.ValidTo, string(validity.ValiditySource),
	)
}

// TestRepositoryUpdateMemoryRejectsPartiallyPopulatedTemporalSnapshot proves a
// malformed existing snapshot fails the transition closed before any write.
func TestRepositoryUpdateMemoryRejectsPartiallyPopulatedTemporalSnapshot(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := rejectTestScope()
	now := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)

	record := memory.ManualUpdateMemoryRecord{
		MemoryID:        "mem_partial",
		VersionID:       "ver_partial",
		Scope:           scope,
		Content:         "Corrected content.",
		ExpectedVersion: 1,
		Reason:          "correct typo",
		Actor:           "operator-a",
		RequestID:       "req_partial",
		UpdatedAt:       now,
	}

	// A closed interval (valid_to == valid_from) is malformed under the half-open
	// [valid_from, valid_to) contract and must not be silently repaired into a
	// well-formed one.
	closed := now.Add(-2 * time.Hour)
	malformed := memory.TemporalValidity{
		TemporalFactID: "mem_partial",
		IngestedAt:     now.Add(-2 * time.Hour),
		ValidFrom:      now.Add(-2 * time.Hour),
		ValidTo:        &closed,
		ValiditySource: memory.TemporalValiditySourceLegacyCompatible,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(1)))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories[\\s\\S]*WHERE id = \\$1").
		WithArgs(record.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(canonicalReadRow(
			record.MemoryID, scope, memory.MemoryClassProfile, "Original content.",
			now.Add(-2*time.Hour), now.Add(-time.Hour), malformed,
		))
	// No UPDATE canonical_memories, no INSERT INTO memory_versions, no
	// provenance write: validation fails before any content statement runs.
	mock.ExpectRollback()

	repo := NewRepositoryWithEmbeddingRouter(mock, testEmbeddingRouter())
	if _, err := repo.UpdateMemory(context.Background(), record); err == nil {
		t.Fatal("UpdateMemory() error = nil, want a temporal validation failure")
	} else if !errors.Is(err, memory.ErrTemporalIntervalInvalid) {
		t.Fatalf("UpdateMemory() error = %v, want ErrTemporalIntervalInvalid", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

// TestRepositoryUpdateMemoryPreservesExistingIdentityAcrossContentEdit proves a
// content-only edit carries the existing temporal identity forward instead of
// re-deriving a fresh one, which would fork the fact's lineage.
//
// Note on coverage: the identity *change* guard in ValidateTemporalTransition is
// not reachable through this path, because resolveManualTransitionValidity always
// derives the proposed snapshot from the existing one. It is exercised directly
// at the domain level; here we pin the preservation behaviour that makes it
// unreachable.
func TestRepositoryUpdateMemoryPreservesExistingIdentityAcrossContentEdit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := rejectTestScope()
	now := time.Date(2026, 6, 20, 9, 5, 0, 0, time.UTC)

	record := memory.ManualUpdateMemoryRecord{
		MemoryID:        "mem_identity",
		VersionID:       "ver_identity",
		Scope:           scope,
		Content:         "Corrected content.",
		ExpectedVersion: 1,
		Reason:          "correct typo",
		Actor:           "operator-a",
		RequestID:       "req_identity",
		UpdatedAt:       now,
	}

	existing := memory.TemporalValidity{
		TemporalFactID: "fact_stable",
		IngestedAt:     now.Add(-2 * time.Hour),
		ValidFrom:      now.Add(-2 * time.Hour),
		ValiditySource: memory.TemporalValiditySourceExplicit,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(1)))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories[\\s\\S]*WHERE id = \\$1").
		WithArgs(record.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(canonicalReadRow(
			record.MemoryID, scope, memory.MemoryClassProfile, "Original content.",
			now.Add(-2*time.Hour), now.Add(-time.Hour), existing,
		))
	// The update must re-assert the same fact identity and window, not a new one.
	mock.ExpectQuery("UPDATE canonical_memories[\\s\\S]*search_text[\\s\\S]*embedding = NULL").
		WithArgs(
			record.MemoryID, scope.Tenant, scope.Project, scope.Namespace, record.Content, record.UpdatedAt,
			existing.TemporalFactID, existing.IngestedAt, existing.ValidFrom, existing.ValidTo, string(existing.ValiditySource),
		).
		WillReturnRows(canonicalReadRow(
			record.MemoryID, scope, memory.MemoryClassProfile, record.Content,
			now.Add(-2*time.Hour), record.UpdatedAt, existing,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(
			record.VersionID, record.MemoryID, int64(2), memory.MemoryStateActive, record.Content, record.UpdatedAt, record.Actor,
			existing.TemporalFactID, existing.IngestedAt, existing.ValidFrom, existing.ValidTo, string(existing.ValiditySource),
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
			"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source",
		}).AddRow(
			record.VersionID, record.MemoryID, int64(2), memory.MemoryStateActive, record.Content, record.UpdatedAt, record.Actor,
			existing.TemporalFactID, existing.IngestedAt, existing.ValidFrom, existing.ValidTo, string(existing.ValiditySource),
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(), nil, nil,
			record.MemoryID, scope.Tenant, scope.Project, scope.Namespace,
			"manual_update_memory", record.RequestID, record.Actor, pgxmock.AnyArg(), record.UpdatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO embedding_rebuilds").
		WithArgs(
			record.MemoryID, scope.Tenant, scope.Project, scope.Namespace,
			int64(2), testContentHash(record.Content), "openai", "text-embedding-3-small", 1536,
			memory.EmbeddingRebuildStatusPending, nil, record.UpdatedAt, nil, nil,
			existing.TemporalFactID, existing.ValidFrom, nil,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepositoryWithEmbeddingRouter(mock, testEmbeddingRouter())
	canonical, err := repo.UpdateMemory(context.Background(), record)
	if err != nil {
		t.Fatalf("UpdateMemory() error = %v", err)
	}

	if canonical.TemporalValidity.TemporalFactID != existing.TemporalFactID {
		t.Fatalf("TemporalFactID = %q, want %q", canonical.TemporalValidity.TemporalFactID, existing.TemporalFactID)
	}
	if !canonical.TemporalValidity.ValidFrom.Equal(existing.ValidFrom) {
		t.Fatalf("ValidFrom = %v, want %v", canonical.TemporalValidity.ValidFrom, existing.ValidFrom)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

// TestRepositoryMergeMemoryRejectsClassMismatchLeavingStateUnchanged proves the
// merge guard fires after the reads but before any content write.
func TestRepositoryMergeMemoryRejectsClassMismatchLeavingStateUnchanged(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := rejectTestScope()
	now := time.Date(2026, 6, 20, 9, 10, 0, 0, time.UTC)

	record := memory.ManualMergeMemoryRecord{
		TargetMemoryID:  "mem_target",
		SourceMemoryID:  "mem_source",
		VersionID:       "ver_merge",
		Scope:           scope,
		Content:         "Merged content.",
		ExpectedVersion: 1,
		Reason:          "dedupe",
		Actor:           "operator-a",
		RequestID:       "req_merge_reject",
		AppliedAt:       now,
	}

	targetLegacy := memory.TemporalValidity{}.LegacyCurrentCompatible(now.Add(-2 * time.Hour))
	sourceLegacy := memory.TemporalValidity{}.LegacyCurrentCompatible(now.Add(-3 * time.Hour))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.TargetMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(canonicalReadRow(
			record.TargetMemoryID, scope, memory.MemoryClassProfile, "Old target",
			now.Add(-2*time.Hour), now.Add(-time.Hour), targetLegacy,
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.SourceMemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(canonicalReadRow(
			record.SourceMemoryID, scope, memory.MemoryClassEpisodic, "Source duplicate",
			now.Add(-3*time.Hour), now.Add(-90*time.Minute), sourceLegacy,
		))
	// Class mismatch: rejected right here, so the target UPDATE, the version
	// insert, the suppression UPDATE, and both provenance inserts never run.
	mock.ExpectRollback()

	repo := NewRepositoryWithEmbeddingRouter(mock, testEmbeddingRouter())
	if _, err := repo.MergeMemory(context.Background(), record); !errors.Is(err, memory.ErrManualMutationRejected) {
		t.Fatalf("MergeMemory() error = %v, want ErrManualMutationRejected", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

// TestRepositoryReclassifyMemoryRejectsInactiveLeavingStateUnchanged proves the
// reclassify guard fires after the read but before any content write.
func TestRepositoryReclassifyMemoryRejectsInactiveLeavingStateUnchanged(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := rejectTestScope()
	now := time.Date(2026, 6, 20, 9, 15, 0, 0, time.UTC)

	record := memory.ManualReclassifyMemoryRecord{
		MemoryID:        "mem_inactive",
		VersionID:       "ver_reclass_reject",
		Scope:           scope,
		TargetClass:     memory.MemoryClassProcedural,
		ExpectedVersion: 1,
		Reason:          "fix class",
		Actor:           "operator-a",
		RequestID:       "req_reclass_reject",
		AppliedAt:       now,
	}

	row := pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source",
	}).AddRow(
		record.MemoryID, scope.Tenant, scope.Project, scope.Namespace,
		memory.MemoryClassProfile, memory.MemoryStateSuppressed, "Respond concisely.",
		now.Add(-2*time.Hour), now.Add(-time.Hour),
		"", nil, nil, nil, "",
	)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(row)
	mock.ExpectRollback()

	repo := NewRepositoryWithEmbeddingRouter(mock, testEmbeddingRouter())
	if _, err := repo.ReclassifyMemory(context.Background(), record); !errors.Is(err, memory.ErrManualMutationRejected) {
		t.Fatalf("ReclassifyMemory() error = %v, want ErrManualMutationRejected", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

// TestRepositoryUpdateMemoryRejectsExpiredVersionLeavingStateUnchanged proves a
// stale-writer rejection happens before the validity read and before any write,
// so a lost update cannot perturb canonical or provenance state.
func TestRepositoryUpdateMemoryRejectsExpiredVersionLeavingStateUnchanged(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := rejectTestScope()
	now := time.Date(2026, 6, 20, 9, 20, 0, 0, time.UTC)

	record := memory.ManualUpdateMemoryRecord{
		MemoryID:        "mem_stale",
		VersionID:       "ver_stale",
		Scope:           scope,
		Content:         "Corrected content.",
		ExpectedVersion: 4,
		Reason:          "correct typo",
		Actor:           "operator-a",
		RequestID:       "req_stale",
		UpdatedAt:       now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(7)))
	mock.ExpectRollback()

	repo := NewRepositoryWithEmbeddingRouter(mock, testEmbeddingRouter())
	if _, err := repo.UpdateMemory(context.Background(), record); !errors.Is(err, memory.ErrManualMutationVersionConflict) {
		t.Fatalf("UpdateMemory() error = %v, want ErrManualMutationVersionConflict", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
