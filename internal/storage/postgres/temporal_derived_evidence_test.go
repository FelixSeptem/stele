package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
)

// Section 5 covers derived evidence: relations, chunks, embeddings/rebuild
// items, and context projections. None of them owns fact validity -- each one
// carries an immutable snapshot of the source version it was built from, so a
// successor can be excluded rather than silently substituted, and a rebuild can
// be attributed to the window it actually described.

// derivedSourceSnapshot is an explicit, non-legacy validity snapshot. Using an
// explicit source (rather than a backfilled one) is what makes these tests able
// to tell "carried the source's identity" apart from "invented one".
func derivedSourceSnapshot(t *testing.T) memory.TemporalValidity {
	t.Helper()
	validTo := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	validity := memory.TemporalValidity{
		TemporalFactID: "fact-derived-1",
		IngestedAt:     time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		ValidFrom:      time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		ValidTo:        &validTo,
		ValiditySource: memory.TemporalValiditySourceExplicit,
	}
	if err := validity.Validate(); err != nil {
		t.Fatalf("fixture must be a valid explicit snapshot: %v", err)
	}
	return validity
}

// TestUpsertRelationProjectionPersistsSourceVersionAndValiditySnapshot proves
// 5.1 for relation projections: the row records which canonical version it was
// derived from and that version's validity window, rather than asserting a
// window of its own.
func TestUpsertRelationProjectionPersistsSourceVersionAndValiditySnapshot(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 6, 14, 20, 0, 0, time.UTC)
	snapshot := derivedSourceSnapshot(t)
	canonical := memory.CanonicalMemory{
		ID:               "11111111-1111-1111-1111-111111111111",
		Scope:            memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:            memory.MemoryClassRelation,
		State:            memory.MemoryStateActive,
		Content:          "entity:user relation:interested_in target:travel",
		CreatedAt:        now.Add(-time.Hour),
		ModifiedAt:       now,
		TemporalValidity: snapshot,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO relation_projections").
		WithArgs(
			canonical.ID,
			canonical.Scope.Tenant,
			canonical.Scope.Project,
			canonical.Scope.Namespace,
			"user",
			"interested_in",
			"travel",
			canonical.Content,
			canonical.CreatedAt,
			now,
			int64(3),
			snapshot.TemporalFactID,
			snapshot.ValidFrom,
			*snapshot.ValidTo,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	tx, err := mock.Begin(context.Background())
	if err != nil {
		t.Fatalf("mock.Begin() error = %v", err)
	}
	if err := upsertRelationProjection(context.Background(), tx, canonical, 3, now); err != nil {
		t.Fatalf("upsertRelationProjection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestUpsertRelationProjectionLeavesDerivedRelationWithoutInventedWindow proves
// the other half of 5.1: a relation is a derived artifact, so when its source
// carries no fact validity the projection must stay unset instead of fabricating
// an interval.
func TestUpsertRelationProjectionLeavesDerivedRelationWithoutInventedWindow(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 6, 14, 20, 0, 0, time.UTC)
	canonical := memory.CanonicalMemory{
		ID:        "22222222-2222-2222-2222-222222222222",
		Scope:     memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:     memory.MemoryClassRelation,
		State:     memory.MemoryStateActive,
		Content:   "entity:user relation:works_at target:acme",
		CreatedAt: now.Add(-time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO relation_projections").
		WithArgs(
			canonical.ID,
			canonical.Scope.Tenant,
			canonical.Scope.Project,
			canonical.Scope.Namespace,
			"user",
			"works_at",
			"acme",
			canonical.Content,
			canonical.CreatedAt,
			now,
			int64(1),
			nil,
			nil,
			nil,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	tx, err := mock.Begin(context.Background())
	if err != nil {
		t.Fatalf("mock.Begin() error = %v", err)
	}
	if err := upsertRelationProjection(context.Background(), tx, canonical, 1, now); err != nil {
		t.Fatalf("upsertRelationProjection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestRelationSourceCurrencyPredicateIsLegacySafeAndRollbackAware pins the two
// edge cases of the read-side guard: it must compare only when both sides are
// known (so an unmigrated or partially migrated row is never hidden), and it
// must disappear entirely under operational rollback.
func TestRelationSourceCurrencyPredicateIsLegacySafeAndRollbackAware(t *testing.T) {
	guard := relationSourceCurrencyPredicate(temporalSelection{Mode: string(memory.TemporalSelectionCurrent)})
	if guard == "" {
		t.Fatal("guard must be rendered for an enabled policy")
	}
	if !strings.Contains(guard, "rp.source_version IS NULL") || !strings.Contains(guard, "cm.temporal_head_version IS NULL") {
		t.Fatalf("guard = %q, want it to tolerate unknown source version and unknown head", guard)
	}
	if !strings.Contains(guard, "rp.source_version = cm.temporal_head_version") {
		t.Fatalf("guard = %q, want it to require the projection to name the current head", guard)
	}

	// Rollback: no temporal filtering at all, which is not the same as an
	// unsatisfiable predicate.
	if rolledBack := relationSourceCurrencyPredicate(temporalSelection{Disabled: true}); rolledBack != "" {
		t.Fatalf("rollback guard = %q, want empty", rolledBack)
	}
}

// TestRepositorySearchRelationsExcludesProjectionSupersededBySuccessor proves
// 5.2's "relation points to an expired source" scenario. The expectation only
// matches a query that includes the currency guard, so removing the guard -- or
// replacing it with a guard that ignores the head -- turns this test red.
func TestRepositorySearchRelationsExcludesProjectionSupersededBySuccessor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	instant := time.Date(2026, 6, 6, 14, 20, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT .*relation_score.*FROM relation_projections[\s\S]*rp\.source_version = cm\.temporal_head_version`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "travel", "travel", "travel", "travel", "travel", nil, nil, 4,
			string(memory.TemporalSelectionCurrent), instant, nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "relation_score",
		}))

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }
	hits, err := repo.SearchRelations(context.Background(), retrieval.SearchInput{
		Scope:            scope,
		Query:            "travel",
		TopK:             4,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("SearchRelations() error = %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("len(hits) = %d, want 0", len(hits))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionRebuildsRelationProjectionForSuccessor proves
// 5.2's "relation projection is rebuilt after correction": the successor version
// gets its own projection lineage, and the snapshot comes from the successor
// version row rather than the canonical head.
func TestAppendTemporalCorrectionRebuildsRelationProjectionForSuccessor(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	predecessor := int64(1)
	correction := memory.TemporalCorrection{
		Scope:              scope,
		TemporalFactID:     "fact-derived-1",
		MemoryID:           temporalCorrectionTestMemoryID,
		PredecessorVersion: &predecessor,
		SuccessorVersion:   2,
		Actor:              "reviewer-1",
		Reason:             "employer changed",
		Disposition:        memory.TemporalConflictResolved,
		CreatedAt:          now,
	}

	successorSnapshot := derivedSourceSnapshot(t)
	validTo := successorSnapshot.ValidTo

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\r\n\t ]*COALESCE\\(temporal_fact_id, id::text\\)").
		WithArgs(correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRowsForClass("fact-derived-1", 1, memory.MemoryClassRelation))
	mock.ExpectExec("INSERT INTO temporal_corrections").
		WithArgs(
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			correction.TemporalFactID,
			correction.MemoryID,
			int64(1),
			int64(2),
			correction.Actor,
			correction.Reason,
			string(correction.Disposition),
			correction.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE canonical_memories").
		WithArgs(correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace, correction.TemporalFactID, int64(2)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery("SELECT[\r\n\t ]*id,[\r\n\t ]*tenant,[\r\n\t ]*project,[\r\n\t ]*namespace,[\r\n\t ]*class").
		WithArgs(correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
			"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source",
		}).AddRow(
			correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace, string(memory.MemoryClassRelation),
			string(memory.MemoryStateActive), "entity:user relation:works_at target:acme", now.Add(-time.Hour), now,
			"fact-derived-1", successorSnapshot.IngestedAt, successorSnapshot.ValidFrom, validTo, string(memory.TemporalValiditySourceExplicit),
		))
	mock.ExpectQuery("FROM memory_versions").
		WithArgs(correction.MemoryID, int64(2)).
		WillReturnRows(pgxmock.NewRows([]string{
			"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source",
		}).AddRow(
			"fact-derived-1", successorSnapshot.IngestedAt, successorSnapshot.ValidFrom, validTo, string(memory.TemporalValiditySourceExplicit),
		))
	// The rebuild must name the successor version and carry the successor's own
	// snapshot, not the head's.
	mock.ExpectExec("INSERT INTO relation_projections").
		WithArgs(
			correction.MemoryID,
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"user",
			"works_at",
			"acme",
			"entity:user relation:works_at target:acme",
			now.Add(-time.Hour),
			now,
			int64(2),
			"fact-derived-1",
			successorSnapshot.ValidFrom,
			*validTo,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	if err := repo.AppendTemporalCorrection(context.Background(), correction); err != nil {
		t.Fatalf("AppendTemporalCorrection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAppendTemporalCorrectionSkipsDerivedRebuildForFactualClass proves the
// rebuild is scoped: a factual correction must not pay for, or perturb, a
// derived projection it does not own.
func TestAppendTemporalCorrectionSkipsDerivedRebuildForFactualClass(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	predecessor := int64(1)
	correction := memory.TemporalCorrection{
		Scope:              scope,
		TemporalFactID:     "fact-derived-1",
		MemoryID:           temporalCorrectionTestMemoryID,
		PredecessorVersion: &predecessor,
		SuccessorVersion:   2,
		Actor:              "reviewer-1",
		Reason:             "profile corrected",
		Disposition:        memory.TemporalConflictResolved,
		CreatedAt:          now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\r\n\t ]*COALESCE\\(temporal_fact_id, id::text\\)").
		WithArgs(correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(temporalHeadGuardRowsForClass("fact-derived-1", 1, memory.MemoryClassProfile))
	mock.ExpectExec("INSERT INTO temporal_corrections").
		WithArgs(
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			correction.TemporalFactID,
			correction.MemoryID,
			int64(1),
			int64(2),
			correction.Actor,
			correction.Reason,
			string(correction.Disposition),
			correction.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE canonical_memories").
		WithArgs(correction.MemoryID, scope.Tenant, scope.Project, scope.Namespace, correction.TemporalFactID, int64(2)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// No canonical read, no version read, and above all no relation rebuild.
	mock.ExpectCommit()

	repo := NewRepository(mock)
	if err := repo.AppendTemporalCorrection(context.Background(), correction); err != nil {
		t.Fatalf("AppendTemporalCorrection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestCreateMemoryChunksPersistsSourceVersionValiditySnapshot proves 5.1 and the
// persistence half of 5.3: a chunk derivation records the validity identity of
// the version it was materialized from.
func TestCreateMemoryChunksPersistsSourceVersionValiditySnapshot(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	createdAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)
	snapshot := derivedSourceSnapshot(t)
	content := "User prefers concise answers."

	chunk := memory.MemoryChunk{
		ID:     "chunk-1",
		Scope:  scope,
		Class:  memory.MemoryClassProfile,
		Source: memory.ChunkSourceReference{Kind: memory.ChunkSourceKindCanonicalVersion, ID: "33333333-3333-3333-3333-333333333333", MemoryID: "44444444-4444-4444-4444-444444444444", Version: 2, Scope: scope},
		// The chunk inherits the source version's validity; it does not choose one.
		TemporalValidity: snapshot,
		Ordinal:          0,
		Content:          content,
		SourceRange:      memory.ChunkRange{Start: 0, End: len(content)},
		CharacterCount:   len([]rune(content)),
		TokenCount:       len(strings.Fields(content)),
		LifecycleState:   memory.MemoryStateActive,
		PolicyVersion:    "policy-v1",
		RendererVersion:  "renderer-v1",
		CreatedAt:        createdAt,
	}

	derivationID, watermarkHash, contentHash, watermark, err := memoryChunkDerivationIdentity(chunk, "counter-v1")
	if err != nil {
		t.Fatalf("memoryChunkDerivationIdentity() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT mv.state, cm.state").
		WithArgs(chunk.Source.ID, chunk.Source.MemoryID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"mv.state", "cm.state"}).AddRow("active", "active"))
	mock.ExpectExec("INSERT INTO memory_chunk_derivations").
		WithArgs(
			derivationID, scope.Tenant, scope.Project, scope.Namespace,
			string(chunk.Source.Kind), chunk.Source.ID, chunk.Source.Version, chunk.Source.MemoryID,
			chunk.Source.SessionID, chunk.Source.UserID, watermark, watermarkHash, contentHash,
			chunk.PolicyVersion, chunk.RendererVersion, "counter-v1", createdAt,
			snapshot.TemporalFactID, snapshot.ValidFrom, *snapshot.ValidTo,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_chunk_items").
		WithArgs(
			chunk.ID, derivationID, scope.Tenant, scope.Project, scope.Namespace,
			chunk.Source.SessionID, chunk.Source.UserID, chunk.Ordinal, string(chunk.Class), chunk.Content,
			chunk.SourceRange.Start, chunk.SourceRange.End, chunk.CharacterCount, chunk.TokenCount,
			memoryChunkContentHash(chunk.Content), createdAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery("SELECT id FROM memory_chunk_items").
		WithArgs(derivationID, chunk.Ordinal).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(chunk.ID))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	stored, err := repo.CreateMemoryChunks(context.Background(), []memory.MemoryChunk{chunk}, "counter-v1")
	if err != nil {
		t.Fatalf("CreateMemoryChunks() error = %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored) = %d, want 1", len(stored))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// chunkVisibleRow returns the full column set visibleMemoryChunkSelect reads,
// so a test can prove the temporal snapshot survives a round trip.
func chunkVisibleRow(chunk memory.MemoryChunk) []any {
	// valid_to is scanned into sql.NullTime, so an absent value is a bare nil
	// and a present one is the instant, not a pointer to it.
	var validTo any
	if chunk.TemporalValidity.ValidTo != nil {
		validTo = *chunk.TemporalValidity.ValidTo
	}
	return []any{
		chunk.ID, chunk.Scope.Tenant, chunk.Scope.Project, chunk.Scope.Namespace,
		string(chunk.Source.Kind), chunk.Source.ID, chunk.Source.Version, chunk.Source.MemoryID,
		chunk.Source.SessionID, chunk.Source.UserID, string(chunk.Class), chunk.Ordinal, chunk.Content,
		chunk.SourceRange.Start, chunk.SourceRange.End, chunk.CharacterCount, chunk.TokenCount,
		string(chunk.LifecycleState), chunk.PolicyVersion, chunk.RendererVersion, chunk.CreatedAt,
		chunk.TemporalValidity.TemporalFactID, chunk.TemporalValidity.ValidFrom, validTo,
	}
}

func chunkVisibleColumns() []string {
	return []string{
		"id", "tenant", "project", "namespace", "source_kind", "source_id", "source_version", "parent_memory_id",
		"source_session_id", "source_user_id", "class", "ordinal", "content", "source_start", "source_end",
		"character_count", "token_count", "lifecycle_state", "policy_version", "renderer_version", "created_at",
		"temporal_fact_id", "valid_from", "valid_to",
	}
}

// TestReadMemoryChunkAppliesSourceVersionValidityPredicate proves 5.3's read
// half: an ordinary chunk read re-proves that the version the chunk was built
// from is still fact-valid, and returns the snapshot so a caller can see which
// window the evidence described.
func TestReadMemoryChunkAppliesSourceVersionValidityPredicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	instant := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	content := "User prefers concise answers."
	chunk := memory.MemoryChunk{
		ID:              "chunk-1",
		Scope:           scope,
		Class:           memory.MemoryClassProfile,
		Source:          memory.ChunkSourceReference{Kind: memory.ChunkSourceKindCanonicalVersion, ID: "33333333-3333-3333-3333-333333333333", MemoryID: "44444444-4444-4444-4444-444444444444", Version: 2, Scope: scope},
		Ordinal:         0,
		Content:         content,
		SourceRange:     memory.ChunkRange{Start: 0, End: len(content)},
		CharacterCount:  len([]rune(content)),
		TokenCount:      len(strings.Fields(content)),
		LifecycleState:  memory.MemoryStateActive,
		PolicyVersion:   "policy-v1",
		RendererVersion: "renderer-v1",
		CreatedAt:       instant.Add(-time.Hour),
		TemporalValidity: memory.TemporalValidity{
			TemporalFactID: "fact-derived-1",
			ValidFrom:      instant.Add(-2 * time.Hour),
		},
	}

	// The regex demands the shared validity predicate appear inside the
	// canonical EXISTS, so dropping the chunk-side filter turns this red.
	mock.ExpectQuery(`SELECT i\.id[\s\S]*COALESCE\(mv\.valid_from, mv\.created_at\)`).
		WithArgs(chunk.ID, scope.Tenant, scope.Project, scope.Namespace,
			string(memory.TemporalSelectionCurrent), instant, nil, nil, nil).
		WillReturnRows(pgxmock.NewRows(chunkVisibleColumns()).AddRow(chunkVisibleRow(chunk)...))

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }
	stored, err := repo.ReadMemoryChunk(context.Background(), scope, chunk.ID)
	if err != nil {
		t.Fatalf("ReadMemoryChunk() error = %v", err)
	}
	if stored.TemporalValidity.TemporalFactID != "fact-derived-1" {
		t.Fatalf("TemporalFactID = %q, want fact-derived-1", stored.TemporalValidity.TemporalFactID)
	}
	if !stored.TemporalValidity.ValidFrom.Equal(instant.Add(-2 * time.Hour)) {
		t.Fatalf("ValidFrom = %v, want %v", stored.TemporalValidity.ValidFrom, instant.Add(-2*time.Hour))
	}
	if stored.TemporalValidity.ValidTo != nil {
		t.Fatalf("ValidTo = %v, want open-ended", stored.TemporalValidity.ValidTo)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadMemoryChunkParentIsExcludedWhenSourceVersionIsExpired proves the
// parent/adjacent half of 5.3: an expired source cannot leak its parent identity
// through a fallback read. The parent lookup delegates to the same visibility
// query, so an ordinary read with no visible chunk fails closed.
func TestReadMemoryChunkParentIsExcludedWhenSourceVersionIsExpired(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	instant := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT i\.id[\s\S]*COALESCE\(mv\.valid_from, mv\.created_at\)`).
		WithArgs("chunk-expired", scope.Tenant, scope.Project, scope.Namespace,
			string(memory.TemporalSelectionCurrent), instant, nil, nil, nil).
		WillReturnRows(pgxmock.NewRows(chunkVisibleColumns()))

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }
	if _, err := repo.ReadMemoryChunkParent(context.Background(), scope, "chunk-expired"); err == nil {
		t.Fatal("ReadMemoryChunkParent() error = nil, want the expired source to be excluded")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestReadMemoryChunkRollbackReturnsPreTemporalBaseline proves the chunk read
// honours the operational rollback: with the policy off the query carries no
// validity predicate and no temporal bind values, so the row set is exactly the
// pre-temporal baseline.
func TestReadMemoryChunkRollbackReturnsPreTemporalBaseline(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	instant := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	content := "User prefers concise answers."
	chunk := memory.MemoryChunk{
		ID:              "chunk-1",
		Scope:           scope,
		Class:           memory.MemoryClassProfile,
		Source:          memory.ChunkSourceReference{Kind: memory.ChunkSourceKindCanonicalVersion, ID: "33333333-3333-3333-3333-333333333333", MemoryID: "44444444-4444-4444-4444-444444444444", Version: 2, Scope: scope},
		Ordinal:         0,
		Content:         content,
		SourceRange:     memory.ChunkRange{Start: 0, End: len(content)},
		CharacterCount:  len([]rune(content)),
		TokenCount:      len(strings.Fields(content)),
		LifecycleState:  memory.MemoryStateActive,
		PolicyVersion:   "policy-v1",
		RendererVersion: "renderer-v1",
		CreatedAt:       instant.Add(-time.Hour),
	}

	// No COALESCE(mv.valid_from, ...) clause and no temporal bind values: the
	// rollback query is byte-for-byte the approved baseline.
	mock.ExpectQuery(`SELECT i\.id[\s\S]*FROM memory_chunk_items`).
		WithArgs(chunk.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows(chunkVisibleColumns()).AddRow(chunkVisibleRow(chunk)...))

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }
	repo.SetTemporalPolicyDisabled(true)
	stored, err := repo.ReadMemoryChunk(context.Background(), scope, chunk.ID)
	if err != nil {
		t.Fatalf("ReadMemoryChunk() error = %v", err)
	}
	if stored.ID != chunk.ID {
		t.Fatalf("stored.ID = %q, want %q", stored.ID, chunk.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestRepositorySearchSemanticFallsBackWithoutSubstitutingAnotherVersion proves
// 5.4: when no active vector exists for the requested source version, the
// semantic channel yields nothing for that memory. It must never repair the gap
// by borrowing another version's vector, or another scope's.
func TestRepositorySearchSemanticFallsBackWithoutSubstitutingAnotherVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	instant := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)

	// The regex pins both the exact-scope joins and the version match, so
	// dropping either one -- which is what would let a stale or foreign vector
	// stand in for the missing one -- turns this test red.
	mock.ExpectQuery(`SELECT[\s\S]*FROM canonical_memories cm[\s\S]*JOIN embedding_rebuilds er[\s\S]*JOIN vector_revisions vr[\s\S]*vr\.source_version = er\.source_version[\s\S]*`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "[0.1,0.2,0.3]", nil, nil, 5,
			string(memory.TemporalSelectionCurrent), instant, nil, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "semantic_score",
		}))

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }
	hits, err := repo.SearchSemantic(context.Background(), retrieval.SearchInput{
		Scope:          scope,
		Query:          "travel planning",
		QueryEmbedding: []float32{0.1, 0.2, 0.3},
		TopK:           5,
	})
	if err != nil {
		t.Fatalf("SearchSemantic() error = %v", err)
	}
	// A missing historical vector is a bounded fallback: the channel is simply
	// empty for this memory. It is not an error, and it is not a substitution.
	if len(hits) != 0 {
		t.Fatalf("len(hits) = %d, want 0: a missing version vector must not be substituted", len(hits))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestProjectionCitationCarriesSourceVersion proves the citation half of 5.4: a
// citation names the exact canonical version it was rendered from, so two
// projections of the same memory at different versions stay distinguishable.
func TestProjectionCitationCarriesSourceVersion(t *testing.T) {
	citation := memory.ProjectionCitation{MemoryID: "44444444-4444-4444-4444-444444444444", Operation: "context_projection", SourceVersion: 3}
	encoded, err := json.Marshal(citation)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"source_version":3`) {
		t.Fatalf("citation = %s, want the source version to be part of the lineage", encoded)
	}

	// An unset version stays absent rather than serializing as zero, so a legacy
	// citation is unchanged by this addition.
	legacy, err := json.Marshal(memory.ProjectionCitation{MemoryID: "44444444-4444-4444-4444-444444444444", Operation: "context_projection"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(legacy), "source_version") {
		t.Fatalf("legacy citation = %s, want the version omitted when unset", legacy)
	}
}

// TestReadContextProjectionReturnsDeterministicOrderAndSourceSnapshots proves
// 5.5: an ordinary projection read is ordered by a total key, so two rebuilds of
// the same evidence produce identical output, and each item still names the
// validity window it was rendered from.
func TestReadContextProjectionReturnsDeterministicOrderAndSourceSnapshots(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	projectionID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

	// The regex pins two things at once: the read must select the source
	// validity snapshot, and it must order by a total key -- sort_key, then
	// source identity, then id. Rows are deliberately handed back out of order
	// to prove the read itself is deterministic rather than dependent on how the
	// database happened to return them.
	mock.ExpectQuery(`SELECT id, source_kind, source_id[\s\S]*COALESCE\(temporal_fact_id, ''\), valid_from, valid_to[\s\S]*ORDER BY sort_key ASC, source_kind ASC, source_id ASC, id ASC`).
		WithArgs(projectionID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "source_kind", "source_id", "source_version", "memory_id", "class", "lifecycle_state",
			"rendered_text", "sort_key", "citation", "temporal_fact_id", "valid_from", "valid_to",
		}).
			AddRow("item-b", "canonical", "99999999-9999-9999-9999-999999999999", int64(2), nil, "episodic", "active", "second", "02", []byte(`{}`), "fact-b", now, nil).
			AddRow("item-a", "canonical", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", int64(1), nil, "profile", "active", "first", "01", []byte(`{}`), "fact-a", now, validTo))

	repo := NewRepository(mock)
	projection := memory.ContextProjection{
		ID:    projectionID.String(),
		Scope: scope,
		Kind:  memory.ContextProjectionKindAlwaysVisible,
	}
	items, err := repo.readContextProjectionItems(context.Background(), projection)
	if err != nil {
		t.Fatalf("readContextProjectionItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// The read normalizes to the total ordering, so a rebuild that materializes
	// the same evidence produces identical output no matter what order the rows
	// came back in.
	if items[0].SortKey != "01" || items[1].SortKey != "02" {
		t.Fatalf("items = %q, %q, want normalized sort order", items[0].SortKey, items[1].SortKey)
	}
	if items[0].TemporalValidity.TemporalFactID != "fact-a" || items[0].TemporalValidity.ValidTo == nil {
		t.Fatalf("items[0].TemporalValidity = %+v, want closed fact-a", items[0].TemporalValidity)
	}
	if items[1].TemporalValidity.TemporalFactID != "fact-b" || items[1].TemporalValidity.ValidTo != nil {
		t.Fatalf("items[1].TemporalValidity = %+v, want open-ended fact-b", items[1].TemporalValidity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
