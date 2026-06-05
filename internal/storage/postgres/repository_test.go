package postgres

import (
	"context"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

func TestRepositoryWriteRawEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 5, 29, 23, 40, 0, 0, time.UTC)
	input := memory.IngestEventInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		Metadata:        map[string]any{"channel": "chat"},
		SourceTimestamp: time.Date(2026, 5, 29, 23, 39, 0, 0, time.UTC),
	}

	rows := pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).
		AddRow("evt_123", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, input.SourceTimestamp, createdAt)
	mock.ExpectQuery("INSERT INTO raw_events").
		WithArgs(input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, pgxmock.AnyArg(), input.SourceTimestamp).
		WillReturnRows(rows)

	repo := NewRepository(mock)
	event, err := repo.WriteRawEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("WriteRawEvent() error = %v", err)
	}

	if event.ID != "evt_123" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "evt_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryWriteProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ProvenanceRecord{
		ID:         "prov_123",
		RawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Operation: "ingest_event",
		CreatedAt: time.Date(2026, 5, 29, 23, 45, 0, 0, time.UTC),
	}

	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(record.ID, record.RawEventID, nil, nil, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Operation, record.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewRepository(mock)
	if err := repo.WriteProvenance(context.Background(), record); err != nil {
		t.Fatalf("WriteProvenance() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryIngestEventWritesRawEventAndProvenanceInOneTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 5, 29, 23, 50, 0, 0, time.UTC)
	input := memory.IngestEventInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		Metadata:        map[string]any{"channel": "chat"},
		SourceTimestamp: time.Date(2026, 5, 29, 23, 49, 0, 0, time.UTC),
	}
	provenance := memory.ProvenanceRecord{
		ID:        "prov_123",
		Scope:     input.Scope,
		Operation: "ingest_event",
		CreatedAt: createdAt,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO raw_events").
		WithArgs(input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, pgxmock.AnyArg(), input.SourceTimestamp).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).
			AddRow("evt_123", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, input.SourceTimestamp, createdAt))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(provenance.ID, "evt_123", nil, nil, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, provenance.Operation, provenance.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	event, err := repo.IngestEvent(context.Background(), input, provenance)
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	if event.ID != "evt_123" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "evt_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestMigrateRunsBaseSchema(t *testing.T) {
	sql, err := BaseSchemaSQL()
	if err != nil {
		t.Fatalf("BaseSchemaSQL() error = %v", err)
	}

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS raw_events",
		"CREATE TABLE IF NOT EXISTS candidate_memories",
		"CREATE TABLE IF NOT EXISTS canonical_memories",
		"CREATE TABLE IF NOT EXISTS memory_versions",
		"CREATE TABLE IF NOT EXISTS provenance_links",
	} {
		if !containsSQL(sql, want) {
			t.Fatalf("BaseSchemaSQL() missing %q", want)
		}
	}
}

func TestRepositoryCreateCandidateWritesCandidateAndProvenanceInOneTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 5, 31, 16, 45, 0, 0, time.UTC)
	candidate := governance.CandidateMemory{
		ID:               "cand_123",
		SourceRawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.82,
		Freshness:      0.77,
		Sensitivity:    governance.SensitivityLow,
		Mutability:     governance.MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         governance.CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	provenance := memory.ProvenanceRecord{
		ID:                "prov_123",
		RawEventID:        candidate.SourceRawEventID,
		CandidateMemoryID: candidate.ID,
		Scope:             candidate.Scope,
		Operation:         "create_candidate",
		CreatedAt:         now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO candidate_memories").
		WithArgs(
			candidate.ID,
			candidate.SourceRawEventID,
			candidate.Scope.Tenant,
			candidate.Scope.Project,
			candidate.Scope.Namespace,
			candidate.Class,
			candidate.Content,
			candidate.Confidence,
			candidate.Importance,
			candidate.Freshness,
			candidate.Sensitivity,
			candidate.Mutability,
			candidate.RetentionClass,
			candidate.Status,
			candidate.CreatedAt,
			candidate.UpdatedAt,
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "source_raw_event_id", "tenant", "project", "namespace", "class", "content",
			"confidence", "importance", "freshness", "sensitivity", "mutability", "retention_class",
			"status", "created_at", "updated_at",
		}).AddRow(
			candidate.ID,
			candidate.SourceRawEventID,
			candidate.Scope.Tenant,
			candidate.Scope.Project,
			candidate.Scope.Namespace,
			candidate.Class,
			candidate.Content,
			candidate.Confidence,
			candidate.Importance,
			candidate.Freshness,
			candidate.Sensitivity,
			candidate.Mutability,
			candidate.RetentionClass,
			candidate.Status,
			candidate.CreatedAt,
			candidate.UpdatedAt,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			provenance.ID,
			provenance.RawEventID,
			provenance.CandidateMemoryID,
			nil,
			candidate.Scope.Tenant,
			candidate.Scope.Project,
			candidate.Scope.Namespace,
			provenance.Operation,
			provenance.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	created, err := repo.CreateCandidate(context.Background(), candidate, provenance)
	if err != nil {
		t.Fatalf("CreateCandidate() error = %v", err)
	}

	if created.ID != candidate.ID {
		t.Fatalf("created.ID = %q, want %q", created.ID, candidate.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListCandidatesByRawEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 5, 31, 16, 50, 0, 0, time.UTC)
	rows := pgxmock.NewRows([]string{
		"id", "source_raw_event_id", "tenant", "project", "namespace", "class", "content",
		"confidence", "importance", "freshness", "sensitivity", "mutability", "retention_class",
		"status", "created_at", "updated_at",
	}).AddRow(
		"cand_123",
		"evt_123",
		"tenant-a",
		"project-a",
		"namespace-a",
		memory.MemoryClassProfile,
		"User prefers concise answers.",
		0.91,
		0.82,
		0.77,
		governance.SensitivityLow,
		governance.MutabilityMutable,
		policy.RetentionClassDurable,
		governance.CandidateStatusPending,
		now,
		now,
	)

	mock.ExpectQuery("SELECT .* FROM candidate_memories").
		WithArgs("evt_123").
		WillReturnRows(rows)

	repo := NewRepository(mock)
	candidates, err := repo.ListCandidatesByRawEvent(context.Background(), "evt_123")
	if err != nil {
		t.Fatalf("ListCandidatesByRawEvent() error = %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want %d", len(candidates), 1)
	}

	if candidates[0].ID != "cand_123" {
		t.Fatalf("candidates[0].ID = %q, want %q", candidates[0].ID, "cand_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryTransitionCandidateStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 5, 31, 17, 0, 0, 0, time.UTC)
	transition := governance.CandidateStatusTransition{
		CandidateID: "cand_123",
		ToStatus:    governance.CandidateStatusPromoted,
		UpdatedAt:   now,
	}
	provenance := memory.ProvenanceRecord{
		ID:                "prov_456",
		CandidateMemoryID: "cand_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Operation: "promote_candidate",
		CreatedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE candidate_memories").
		WithArgs(transition.CandidateID, transition.ToStatus, transition.UpdatedAt).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "source_raw_event_id", "tenant", "project", "namespace", "class", "content",
			"confidence", "importance", "freshness", "sensitivity", "mutability", "retention_class",
			"status", "created_at", "updated_at",
		}).AddRow(
			"cand_123",
			"evt_123",
			"tenant-a",
			"project-a",
			"namespace-a",
			memory.MemoryClassProfile,
			"User prefers concise answers.",
			0.91,
			0.82,
			0.77,
			governance.SensitivityLow,
			governance.MutabilityMutable,
			policy.RetentionClassDurable,
			governance.CandidateStatusPromoted,
			now.Add(-time.Minute),
			now,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			provenance.ID,
			nil,
			provenance.CandidateMemoryID,
			nil,
			provenance.Scope.Tenant,
			provenance.Scope.Project,
			provenance.Scope.Namespace,
			provenance.Operation,
			provenance.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	candidate, err := repo.TransitionCandidateStatus(context.Background(), transition, provenance)
	if err != nil {
		t.Fatalf("TransitionCandidateStatus() error = %v", err)
	}

	if candidate.Status != governance.CandidateStatusPromoted {
		t.Fatalf("candidate.Status = %q, want %q", candidate.Status, governance.CandidateStatusPromoted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryClaimPendingRawEvents(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC)
	input := governance.ClaimPendingRawEventsInput{
		WorkerID:      "worker-a",
		BatchSize:     2,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	}

	mock.ExpectQuery("WITH claimed AS").
		WithArgs(input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.BatchSize).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at",
			"governance_worker_id", "governance_claimed_at", "governance_lease_until", "governance_attempt",
		}).AddRow(
			"evt_123",
			"tenant-a",
			"project-a",
			"namespace-a",
			"conversation.message",
			"hello",
			now.Add(-time.Minute),
			now.Add(-time.Minute),
			"worker-a",
			now,
			now.Add(2*time.Minute),
			1,
		))

	repo := NewRepository(mock)
	claims, err := repo.ClaimPendingRawEvents(context.Background(), input)
	if err != nil {
		t.Fatalf("ClaimPendingRawEvents() error = %v", err)
	}

	if len(claims) != 1 {
		t.Fatalf("len(claims) = %d, want %d", len(claims), 1)
	}

	if claims[0].Event.ID != "evt_123" {
		t.Fatalf("claims[0].Event.ID = %q, want %q", claims[0].Event.ID, "evt_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryMarkRawEventProcessed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	input := governance.CompleteClaimedRawEventInput{
		RawEventID:  "evt_123",
		WorkerID:    "worker-a",
		ProcessedAt: time.Date(2026, 6, 1, 11, 5, 0, 0, time.UTC),
	}

	mock.ExpectExec("UPDATE raw_events").
		WithArgs(input.RawEventID, input.WorkerID, input.ProcessedAt).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.MarkRawEventProcessed(context.Background(), input); err != nil {
		t.Fatalf("MarkRawEventProcessed() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryGetLatestCanonicalByScopeAndClass(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	mock.ExpectQuery("SELECT .* FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, memory.MemoryClassProfile, memory.MemoryStateActive).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			"mem_123",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassProfile,
			memory.MemoryStateActive,
			"User prefers concise answers.",
			now.Add(-time.Hour),
			now,
		))

	repo := NewRepository(mock)
	canonical, found, err := repo.GetLatestCanonicalByScopeAndClass(context.Background(), scope, memory.MemoryClassProfile)
	if err != nil {
		t.Fatalf("GetLatestCanonicalByScopeAndClass() error = %v", err)
	}

	if !found {
		t.Fatal("found = false, want true")
	}

	if canonical.ID != "mem_123" {
		t.Fatalf("canonical.ID = %q, want %q", canonical.ID, "mem_123")
	}
}

func TestRepositoryPromoteCandidateCreatesCanonicalMemoryVersionAndProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC)
	input := governance.CanonicalPromotion{
		Candidate: governance.CandidateMemory{
			ID:               "cand_123",
			SourceRawEventID: "evt_123",
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			Class:          memory.MemoryClassProfile,
			Content:        "User prefers concise answers.",
			Confidence:     0.91,
			Importance:     0.84,
			Freshness:      0.74,
			Sensitivity:    governance.SensitivityLow,
			Mutability:     governance.MutabilityMutable,
			RetentionClass: policy.RetentionClassDurable,
			Status:         governance.CandidateStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		MemoryID:  "mem_123",
		VersionID: "ver_123",
		Version:   1,
		CreatedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO canonical_memories").
		WithArgs(
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			input.Candidate.Class,
			memory.MemoryStateActive,
			input.Candidate.Content,
			input.CreatedAt,
			input.CreatedAt,
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			input.Candidate.Class,
			memory.MemoryStateActive,
			input.Candidate.Content,
			input.CreatedAt,
			input.CreatedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(
			input.VersionID,
			input.MemoryID,
			input.Version,
			memory.MemoryStateActive,
			input.Candidate.Content,
			input.CreatedAt,
			input.Candidate.ID,
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			input.VersionID,
			input.MemoryID,
			input.Version,
			memory.MemoryStateActive,
			input.Candidate.Content,
			input.CreatedAt,
			input.Candidate.ID,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			input.Candidate.SourceRawEventID,
			input.Candidate.ID,
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			"promote_candidate",
			input.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, version, err := repo.PromoteCandidate(context.Background(), input)
	if err != nil {
		t.Fatalf("PromoteCandidate() error = %v", err)
	}

	if canonical.ID != input.MemoryID {
		t.Fatalf("canonical.ID = %q, want %q", canonical.ID, input.MemoryID)
	}

	if version.ID != input.VersionID {
		t.Fatalf("version.ID = %q, want %q", version.ID, input.VersionID)
	}
}

func TestRepositoryCreateSummaryMemoryWritesCanonicalVersionAndEvidenceProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 16, 20, 0, 0, time.UTC)
	input := governance.SummaryMemoryRecord{
		MemoryID:  "sum_123",
		VersionID: "sum_ver_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Content:             "Summary: user asked about hotels; agent recommended two options.",
		EvidenceRawEventIDs: []string{"evt_1", "evt_2"},
		CreatedAt:           now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO canonical_memories").
		WithArgs(
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			memory.MemoryClassSummary,
			memory.MemoryStateActive,
			input.Content,
			input.CreatedAt,
			input.CreatedAt,
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			memory.MemoryClassSummary,
			memory.MemoryStateActive,
			input.Content,
			input.CreatedAt,
			input.CreatedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(
			input.VersionID,
			input.MemoryID,
			int64(1),
			memory.MemoryStateActive,
			input.Content,
			input.CreatedAt,
			"summary_compactor",
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			input.VersionID,
			input.MemoryID,
			int64(1),
			memory.MemoryStateActive,
			input.Content,
			input.CreatedAt,
			"summary_compactor",
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			"evt_1",
			nil,
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			"create_summary_memory",
			input.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			"evt_2",
			nil,
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			"create_summary_memory",
			input.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, version, err := repo.CreateSummaryMemory(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateSummaryMemory() error = %v", err)
	}

	if canonical.ID != input.MemoryID {
		t.Fatalf("canonical.ID = %q, want %q", canonical.ID, input.MemoryID)
	}

	if version.ID != input.VersionID {
		t.Fatalf("version.ID = %q, want %q", version.ID, input.VersionID)
	}
}

func TestRepositoryListVisibleCanonicalMemoriesExcludesHiddenStatesByDefault(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}
	now := time.Date(2026, 6, 1, 17, 15, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .* FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, false).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			"mem_active",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassProfile,
			memory.MemoryStateActive,
			"User prefers concise answers.",
			now.Add(-time.Hour),
			now,
		))

	repo := NewRepository(mock)
	memories, err := repo.ListCanonicalMemories(context.Background(), scope, false)
	if err != nil {
		t.Fatalf("ListCanonicalMemories() error = %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("len(memories) = %d, want %d", len(memories), 1)
	}

	if memories[0].State != memory.MemoryStateActive {
		t.Fatalf("State = %q, want %q", memories[0].State, memory.MemoryStateActive)
	}
}

func TestRepositoryApplyLifecycleActionUpdatesCanonicalState(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 17, 20, 0, 0, time.UTC)
	action := governance.LifecycleAction{
		MemoryID: "mem_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Action:    policy.ForgettingActionSuppress,
		Content:   "User prefers concise answers.",
		AppliedAt: now,
	}

	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(action.MemoryID, action.TargetState(), now).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			memory.MemoryClassProfile,
			action.TargetState(),
			action.Content,
			now.Add(-time.Hour),
			now,
		))

	repo := NewRepository(mock)
	canonical, err := repo.ApplyLifecycleAction(context.Background(), action)
	if err != nil {
		t.Fatalf("ApplyLifecycleAction() error = %v", err)
	}

	if canonical.State != memory.MemoryStateSuppressed {
		t.Fatalf("State = %q, want %q", canonical.State, memory.MemoryStateSuppressed)
	}
}

func containsSQL(haystack, needle string) bool {
	return len(haystack) >= len(needle) && context.Background() != nil && (stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
