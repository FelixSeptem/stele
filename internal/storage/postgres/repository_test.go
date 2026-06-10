package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
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
		RequestID: "req_123",
		Actor:     "operator-a",
		Operation: "ingest_event",
		CreatedAt: time.Date(2026, 5, 29, 23, 45, 0, 0, time.UTC),
		SourceContext: map[string]any{
			"reason": "manual override",
		},
	}

	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			record.ID,
			record.RawEventID,
			nil,
			nil,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			record.Operation,
			record.RequestID,
			record.Actor,
			pgxmock.AnyArg(),
			record.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewRepository(mock)
	if err := repo.WriteProvenance(context.Background(), record); err != nil {
		t.Fatalf("WriteProvenance() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReadCanonicalMemoryReturnsVisibleScopedRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs("mem_123", scope.Tenant, scope.Project, scope.Namespace, false).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			"mem_123", scope.Tenant, scope.Project, scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, "User prefers concise answers.", now.Add(-time.Hour), now,
		))

	repo := NewRepository(mock)
	got, err := repo.ReadCanonicalMemory(context.Background(), scope, "mem_123", false)
	if err != nil {
		t.Fatalf("ReadCanonicalMemory() error = %v", err)
	}

	if got.ID != "mem_123" {
		t.Fatalf("ID = %q, want mem_123", got.ID)
	}
}

func TestRepositoryReadMemoryProvenanceReturnsActorAndReason(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 16, 10, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM provenance_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "mem_123").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "raw_event_id", "candidate_memory_id", "memory_id", "tenant", "project", "namespace", "operation", "request_id", "actor", "source_context", "created_at",
		}).AddRow(
			"prov_1", nil, nil, "mem_123", scope.Tenant, scope.Project, scope.Namespace, "suppress_memory", "req_1", "operator-a", []byte(`{"reason":"manual override"}`), now,
		))

	repo := NewRepository(mock)
	records, err := repo.ReadMemoryProvenance(context.Background(), scope, "mem_123")
	if err != nil {
		t.Fatalf("ReadMemoryProvenance() error = %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}

	if records[0].Actor != "operator-a" {
		t.Fatalf("Actor = %q, want operator-a", records[0].Actor)
	}

	if got := records[0].SourceContext["reason"]; got != "manual override" {
		t.Fatalf("SourceContext[reason] = %v, want manual override", got)
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
		WithArgs(
			provenance.ID,
			"evt_123",
			nil,
			nil,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			provenance.Operation,
			nil,
			nil,
			pgxmock.AnyArg(),
			provenance.CreatedAt,
		).
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
		"CREATE EXTENSION IF NOT EXISTS vector",
		"CREATE TABLE IF NOT EXISTS raw_events",
		"governance_last_failed_at timestamptz",
		"governance_next_attempt_at timestamptz",
		"governance_exhausted_at timestamptz",
		"CREATE TABLE IF NOT EXISTS candidate_memories",
		"CREATE TABLE IF NOT EXISTS canonical_memories",
		"retention_class text NOT NULL",
		"CREATE TABLE IF NOT EXISTS memory_versions",
		"CREATE TABLE IF NOT EXISTS provenance_links",
		"CREATE TABLE IF NOT EXISTS relation_projections",
		"CREATE TABLE IF NOT EXISTS job_executions",
		"modified_by text NOT NULL",
		"CREATE INDEX IF NOT EXISTS raw_events_governance_claim_idx",
		"CREATE INDEX IF NOT EXISTS canonical_memories_search_text_idx",
		"CREATE INDEX IF NOT EXISTS relation_projections_search_text_idx",
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

func TestRepositoryReadGovernanceStatusReturnsBacklogSnapshot(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 6, 18, 30, 0, 0, time.UTC)
	oldestPending := now.Add(-10 * time.Minute)
	mock.ExpectQuery("SELECT[\\s\\S]*FROM raw_events").
		WithArgs(now).
		WillReturnRows(pgxmock.NewRows([]string{
			"pending_raw_events", "leased_raw_events", "processed_raw_events", "oldest_pending_created_at",
		}).AddRow(
			int64(7),
			int64(2),
			int64(19),
			oldestPending,
		))

	repo := NewRepository(mock)
	status, err := repo.ReadGovernanceStatus(context.Background(), now)
	if err != nil {
		t.Fatalf("ReadGovernanceStatus() error = %v", err)
	}

	if status.PendingRawEvents != 7 || status.LeasedRawEvents != 2 || status.ProcessedRawEvents != 19 {
		t.Fatalf("status = %+v, want expected counts", status)
	}

	if !status.OldestPendingCreatedAt.Equal(oldestPending) {
		t.Fatalf("OldestPendingCreatedAt = %v, want %v", status.OldestPendingCreatedAt, oldestPending)
	}
}

func TestRepositoryReadMemoryHistoryReturnsCanonicalVersionsAndProvenance(t *testing.T) {
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
	now := time.Date(2026, 6, 6, 19, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs("mem_hidden", scope.Tenant, scope.Project, scope.Namespace, true).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			"mem_hidden",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassProfile,
			memory.MemoryStateForgotten,
			"Old preference",
			now.Add(-time.Hour),
			now,
		))

	mock.ExpectQuery("SELECT[\\s\\S]*FROM memory_versions").
		WithArgs("mem_hidden").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			"ver_2",
			"mem_hidden",
			int64(2),
			memory.MemoryStateForgotten,
			"Old preference",
			now,
			"cand_2",
		))

	mock.ExpectQuery("SELECT[\\s\\S]*FROM provenance_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "mem_hidden").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "raw_event_id", "candidate_memory_id", "memory_id", "tenant", "project", "namespace", "operation", "request_id", "actor", "source_context", "created_at",
		}).AddRow(
			"prov_1",
			"evt_1",
			"cand_2",
			"mem_hidden",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"promote_candidate",
			nil,
			nil,
			[]byte(`{}`),
			now,
		))

	repo := NewRepository(mock)
	history, err := repo.ReadMemoryHistory(context.Background(), scope, "mem_hidden", true)
	if err != nil {
		t.Fatalf("ReadMemoryHistory() error = %v", err)
	}

	if history.Memory.ID != "mem_hidden" {
		t.Fatalf("Memory.ID = %q, want %q", history.Memory.ID, "mem_hidden")
	}

	if len(history.Versions) != 1 || history.Versions[0].ID != "ver_2" {
		t.Fatalf("Versions = %+v, want one version", history.Versions)
	}

	if len(history.Provenance) != 1 || history.Provenance[0].ID != "prov_1" {
		t.Fatalf("Provenance = %+v, want one provenance record", history.Provenance)
	}
}

func TestRepositoryBeginJobExecutionInsertsNewExecution(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	startedAt := time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC)
	execution := jobs.JobExecution{
		JobName:        "summary_compaction",
		Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TriggerSource:  "scheduler",
		IdempotencyKey: "summary_compaction:tenant-a:project-a:namespace-a:2026-06-07T11:00:00Z",
		StartedAt:      startedAt,
	}

	mock.ExpectExec("INSERT INTO job_executions").
		WithArgs(
			execution.JobName,
			execution.Scope.Tenant,
			execution.Scope.Project,
			execution.Scope.Namespace,
			execution.TriggerSource,
			execution.IdempotencyKey,
			jobs.JobExecutionStatusRunning,
			execution.StartedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewRepository(mock)
	started, err := repo.BeginJobExecution(context.Background(), execution)
	if err != nil {
		t.Fatalf("BeginJobExecution() error = %v", err)
	}

	if !started {
		t.Fatal("started = false, want true")
	}
}

func TestRepositoryBeginJobExecutionSkipsCompletedIdempotencyKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	execution := jobs.JobExecution{
		JobName:        "summary_compaction",
		Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TriggerSource:  "scheduler",
		IdempotencyKey: "summary_compaction:tenant-a:project-a:namespace-a:2026-06-07T11:00:00Z",
		StartedAt:      time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC),
	}

	mock.ExpectExec("INSERT INTO job_executions").
		WithArgs(
			execution.JobName,
			execution.Scope.Tenant,
			execution.Scope.Project,
			execution.Scope.Namespace,
			execution.TriggerSource,
			execution.IdempotencyKey,
			jobs.JobExecutionStatusRunning,
			execution.StartedAt,
		).
		WillReturnError(&pgconn.PgError{Code: "23505"})
	mock.ExpectQuery("SELECT status FROM job_executions WHERE idempotency_key = \\$1").
		WithArgs(execution.IdempotencyKey).
		WillReturnRows(pgxmock.NewRows([]string{"status"}).AddRow(string(jobs.JobExecutionStatusCompleted)))

	repo := NewRepository(mock)
	started, err := repo.BeginJobExecution(context.Background(), execution)
	if err != nil {
		t.Fatalf("BeginJobExecution() error = %v", err)
	}

	if started {
		t.Fatal("started = true, want false for completed idempotency key")
	}
}

func TestRepositoryListRecentJobExecutionsReturnsNewestFirst(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 11, 30, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM job_executions").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 5).
		WillReturnRows(pgxmock.NewRows([]string{
			"job_name", "tenant", "project", "namespace", "trigger_source", "idempotency_key", "status", "attempt", "processed_count", "error_message", "started_at", "finished_at",
		}).AddRow(
			"summary_compaction",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"scheduler",
			"run-a",
			string(jobs.JobExecutionStatusCompleted),
			1,
			1,
			nil,
			now,
			now.Add(time.Second),
		))

	repo := NewRepository(mock)
	records, err := repo.ListRecentJobExecutions(context.Background(), scope, 5)
	if err != nil {
		t.Fatalf("ListRecentJobExecutions() error = %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}

	if records[0].JobName != "summary_compaction" || records[0].Status != jobs.JobExecutionStatusCompleted {
		t.Fatalf("record = %+v, want returned job execution", records[0])
	}
}

func TestRepositoryListRetentionTargetsReturnsActiveScopedTargets(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, memory.MemoryStateActive, 10).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "retention_class", "updated_at",
		}).AddRow(
			"mem_123",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"ephemeral",
			now,
		))

	repo := NewRepository(mock)
	targets, err := repo.ListRetentionTargets(context.Background(), scope, 10)
	if err != nil {
		t.Fatalf("ListRetentionTargets() error = %v", err)
	}

	if len(targets) != 1 || targets[0].MemoryID != "mem_123" {
		t.Fatalf("targets = %+v, want one target mem_123", targets)
	}
}

func TestRepositoryDeleteJobExecutionsBeforeReturnsDeletedCount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("DELETE FROM job_executions").
		WithArgs(cutoff).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	repo := NewRepository(mock)
	deleted, err := repo.DeleteJobExecutionsBefore(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("DeleteJobExecutionsBefore() error = %v", err)
	}

	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

func TestRepositoryClaimPendingRawEventsSkipsRetryWaitAndExhausted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	input := governance.ClaimPendingRawEventsInput{
		WorkerID:      "worker-a",
		BatchSize:     8,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	}

	mock.ExpectQuery("WITH claimed AS").
		WithArgs(input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.BatchSize).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at",
			"governance_worker_id", "governance_claimed_at", "governance_lease_until", "governance_attempt",
		}))

	repo := NewRepository(mock)
	claims, err := repo.ClaimPendingRawEvents(context.Background(), input)
	if err != nil {
		t.Fatalf("ClaimPendingRawEvents() error = %v", err)
	}

	if len(claims) != 0 {
		t.Fatalf("len(claims) = %d, want 0", len(claims))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRenewClaimedRawEventLease(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	renewedAt := time.Date(2026, 6, 10, 12, 10, 0, 0, time.UTC)
	input := governance.RenewClaimedRawEventLeaseInput{
		RawEventID: "evt_123",
		WorkerID:   "worker-a",
		RenewedAt:  renewedAt,
		LeaseUntil: renewedAt.Add(2 * time.Minute),
	}

	mock.ExpectExec("UPDATE raw_events").
		WithArgs(input.RawEventID, input.WorkerID, input.RenewedAt, input.LeaseUntil).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.RenewClaimedRawEventLease(context.Background(), input); err != nil {
		t.Fatalf("RenewClaimedRawEventLease() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRecordClaimedRawEventFailureSchedulesRetry(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	failedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	nextAttemptAt := failedAt.Add(30 * time.Second)
	input := governance.RecordClaimedRawEventFailureInput{
		RawEventID:    "evt_123",
		WorkerID:      "worker-a",
		FailedAt:      failedAt,
		ErrorMessage:  "candidate extraction failed",
		NextAttemptAt: nextAttemptAt,
	}

	mock.ExpectExec("UPDATE raw_events").
		WithArgs(
			input.RawEventID,
			input.WorkerID,
			input.FailedAt,
			input.ErrorMessage,
			input.NextAttemptAt,
			nil,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.RecordClaimedRawEventFailure(context.Background(), input); err != nil {
		t.Fatalf("RecordClaimedRawEventFailure() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRecordClaimedRawEventFailureMarksExhausted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	failedAt := time.Date(2026, 6, 10, 12, 5, 0, 0, time.UTC)
	input := governance.RecordClaimedRawEventFailureInput{
		RawEventID:   "evt_123",
		WorkerID:     "worker-a",
		FailedAt:     failedAt,
		ErrorMessage: "candidate extraction failed",
		ExhaustedAt:  failedAt,
	}

	mock.ExpectExec("UPDATE raw_events").
		WithArgs(
			input.RawEventID,
			input.WorkerID,
			input.FailedAt,
			input.ErrorMessage,
			nil,
			input.ExhaustedAt,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.RecordClaimedRawEventFailure(context.Background(), input); err != nil {
		t.Fatalf("RecordClaimedRawEventFailure() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRenewClaimedRawEventLeaseRejectsLostOwnership(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	renewedAt := time.Date(2026, 6, 10, 12, 10, 0, 0, time.UTC)
	input := governance.RenewClaimedRawEventLeaseInput{
		RawEventID: "evt_123",
		WorkerID:   "worker-a",
		RenewedAt:  renewedAt,
		LeaseUntil: renewedAt.Add(2 * time.Minute),
	}

	mock.ExpectExec("UPDATE raw_events").
		WithArgs(input.RawEventID, input.WorkerID, input.RenewedAt, input.LeaseUntil).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewRepository(mock)
	if err := repo.RenewClaimedRawEventLease(context.Background(), input); !errors.Is(err, governance.ErrClaimOwnershipLost) {
		t.Fatalf("RenewClaimedRawEventLease() error = %v, want ErrClaimOwnershipLost", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListMaintenanceScopesReturnsDistinctEligibleScopes(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT tenant, project, namespace FROM \\(").
		WithArgs(50).
		WillReturnRows(pgxmock.NewRows([]string{"tenant", "project", "namespace"}).
			AddRow("tenant-a", "project-a", "namespace-a").
			AddRow("tenant-b", "project-b", "namespace-b"))

	repo := NewRepository(mock)
	scopes, err := repo.ListMaintenanceScopes(context.Background(), 50)
	if err != nil {
		t.Fatalf("ListMaintenanceScopes() error = %v", err)
	}

	if len(scopes) != 2 {
		t.Fatalf("len(scopes) = %d, want 2", len(scopes))
	}

	if scopes[0].Namespace != "namespace-a" || scopes[1].Namespace != "namespace-b" {
		t.Fatalf("scopes = %+v, want namespace-a/namespace-b", scopes)
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
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM memory_versions").
		WithArgs(input.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(0)))
	mock.ExpectQuery("INSERT INTO canonical_memories[\\s\\S]*search_text").
		WithArgs(
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			input.Candidate.Class,
			memory.MemoryStateActive,
			input.Candidate.RetentionClass,
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

	if version.Version != 1 {
		t.Fatalf("version.Version = %d, want %d", version.Version, 1)
	}
}

func TestRepositoryPromoteCandidateSupersedeUpdatesCanonicalAndAppendsVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC)
	input := governance.CanonicalPromotion{
		Candidate: governance.CandidateMemory{
			ID:               "cand_456",
			SourceRawEventID: "evt_456",
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			Class:          memory.MemoryClassProfile,
			Content:        "User prefers concise answers.",
			Confidence:     0.94,
			Importance:     0.87,
			Freshness:      0.66,
			Sensitivity:    governance.SensitivityLow,
			Mutability:     governance.MutabilityMutable,
			RetentionClass: policy.RetentionClassDurable,
			Status:         governance.CandidateStatusPending,
			CreatedAt:      updatedAt,
			UpdatedAt:      updatedAt,
		},
		MemoryID:  "mem_existing",
		VersionID: "ver_456",
		Version:   1,
		CreatedAt: updatedAt,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM memory_versions").
		WithArgs(input.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(1)))
	mock.ExpectQuery("UPDATE canonical_memories[\\s\\S]*search_text").
		WithArgs(
			input.MemoryID,
			memory.MemoryStateActive,
			input.Candidate.RetentionClass,
			input.Candidate.Content,
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
			createdAt,
			input.CreatedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(
			input.VersionID,
			input.MemoryID,
			int64(2),
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
			int64(2),
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

	if version.Version != 2 {
		t.Fatalf("version.Version = %d, want %d", version.Version, 2)
	}
}

func TestRepositoryPromoteCandidateRelationUpsertsProjection(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 15, 12, 0, 0, time.UTC)
	input := governance.CanonicalPromotion{
		Candidate: governance.CandidateMemory{
			ID:               "cand_relation",
			SourceRawEventID: "evt_relation",
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			Class:          memory.MemoryClassRelation,
			Content:        "entity:user relation:interested_in target:travel",
			Confidence:     0.93,
			Importance:     0.72,
			Freshness:      0.81,
			Sensitivity:    governance.SensitivityLow,
			Mutability:     governance.MutabilityMutable,
			RetentionClass: policy.RetentionClassDurable,
			Status:         governance.CandidateStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		MemoryID:  "mem_relation",
		VersionID: "ver_relation",
		Version:   1,
		CreatedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM memory_versions").
		WithArgs(input.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(0)))
	mock.ExpectQuery("INSERT INTO canonical_memories[\\s\\S]*search_text").
		WithArgs(
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			input.Candidate.Class,
			memory.MemoryStateActive,
			input.Candidate.RetentionClass,
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
	mock.ExpectExec("INSERT INTO relation_projections").
		WithArgs(
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			"user",
			"interested_in",
			"travel",
			input.Candidate.Content,
			input.CreatedAt,
			input.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

	if version.Version != 1 {
		t.Fatalf("version.Version = %d, want %d", version.Version, 1)
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
	mock.ExpectQuery("INSERT INTO canonical_memories[\\s\\S]*search_text").
		WithArgs(
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			memory.MemoryClassSummary,
			memory.MemoryStateActive,
			policy.RetentionClassDurable,
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
			nil,
			nil,
			pgxmock.AnyArg(),
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
			nil,
			nil,
			pgxmock.AnyArg(),
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

func TestRepositoryListCandidatesForCompactionReturnsPromotedCandidatesBeforeCutoff(t *testing.T) {
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
	cutoff := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .* FROM candidate_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, governance.CandidateStatusPromoted, cutoff, 10).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "source_raw_event_id", "tenant", "project", "namespace", "class", "content",
			"confidence", "importance", "freshness", "sensitivity", "mutability", "retention_class",
			"status", "created_at", "updated_at",
		}).AddRow(
			"cand_123",
			"evt_123",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassEpisodic,
			"User asked about hotels.",
			0.88,
			0.61,
			0.43,
			governance.SensitivityLow,
			governance.MutabilityImmutable,
			policy.RetentionClassSession,
			governance.CandidateStatusPromoted,
			cutoff.Add(-2*time.Hour),
			cutoff.Add(-time.Hour),
		))

	repo := NewRepository(mock)
	candidates, err := repo.ListCandidatesForCompaction(context.Background(), scope, cutoff, 10)
	if err != nil {
		t.Fatalf("ListCandidatesForCompaction() error = %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want %d", len(candidates), 1)
	}

	if candidates[0].ID != "cand_123" {
		t.Fatalf("candidate.ID = %q, want %q", candidates[0].ID, "cand_123")
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
		Reason:    "manual override",
		Actor:     "operator-a",
		RequestID: "req_123",
		AppliedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(action.MemoryID, action.Scope.Tenant, action.Scope.Project, action.Scope.Namespace, action.TargetState(), now).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			memory.MemoryClassProfile,
			action.TargetState(),
			"User prefers concise answers.",
			now.Add(-time.Hour),
			now,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			"suppress_memory",
			action.RequestID,
			action.Actor,
			pgxmock.AnyArg(),
			now,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.ApplyLifecycleAction(context.Background(), action)
	if err != nil {
		t.Fatalf("ApplyLifecycleAction() error = %v", err)
	}

	if canonical.State != memory.MemoryStateSuppressed {
		t.Fatalf("State = %q, want %q", canonical.State, memory.MemoryStateSuppressed)
	}
}

func TestRepositoryApplyLifecycleActionDeleteClearsPayloadAndWritesDeletionMarker(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 17, 25, 0, 0, time.UTC)
	action := governance.LifecycleAction{
		MemoryID: "mem_456",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Action:    policy.ForgettingActionDelete,
		Reason:    "compliance request",
		Actor:     "operator-a",
		RequestID: "req_456",
		AppliedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(action.MemoryID, action.Scope.Tenant, action.Scope.Project, action.Scope.Namespace, action.TargetState(), now).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			memory.MemoryClassProfile,
			action.TargetState(),
			"",
			now.Add(-time.Hour),
			now,
		))
	mock.ExpectExec("INSERT INTO deletion_markers").
		WithArgs(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			action.Reason,
			now,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			"delete_memory",
			action.RequestID,
			action.Actor,
			pgxmock.AnyArg(),
			now,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.ApplyLifecycleAction(context.Background(), action)
	if err != nil {
		t.Fatalf("ApplyLifecycleAction() error = %v", err)
	}

	if canonical.State != memory.MemoryStateDeleted {
		t.Fatalf("State = %q, want %q", canonical.State, memory.MemoryStateDeleted)
	}

	if canonical.Content != "" {
		t.Fatalf("Content = %q, want empty payload after delete", canonical.Content)
	}
}

func TestRepositoryApplyLifecycleActionDeleteRemovesRelationProjection(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 6, 1, 17, 27, 0, 0, time.UTC)
	action := governance.LifecycleAction{
		MemoryID: "mem_relation",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Action:    policy.ForgettingActionDelete,
		Reason:    "compliance request",
		Actor:     "operator-a",
		RequestID: "req_relation",
		AppliedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(action.MemoryID, action.Scope.Tenant, action.Scope.Project, action.Scope.Namespace, action.TargetState(), now).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			memory.MemoryClassRelation,
			action.TargetState(),
			"",
			now.Add(-time.Hour),
			now,
		))
	mock.ExpectExec("DELETE FROM relation_projections").
		WithArgs(action.MemoryID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec("INSERT INTO deletion_markers").
		WithArgs(
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			action.Reason,
			now,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			action.MemoryID,
			action.Scope.Tenant,
			action.Scope.Project,
			action.Scope.Namespace,
			"delete_memory",
			action.RequestID,
			action.Actor,
			pgxmock.AnyArg(),
			now,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.ApplyLifecycleAction(context.Background(), action)
	if err != nil {
		t.Fatalf("ApplyLifecycleAction() error = %v", err)
	}

	if canonical.State != memory.MemoryStateDeleted {
		t.Fatalf("State = %q, want %q", canonical.State, memory.MemoryStateDeleted)
	}
}

func TestRepositoryCreateMemoryWritesCanonicalVersionAndProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ManualCreateMemoryRecord{
		MemoryID:  "mem_manual",
		VersionID: "ver_manual",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:     memory.MemoryClassProfile,
		Content:   "Seeded operator memory.",
		Reason:    "seed profile",
		Actor:     "operator-a",
		RequestID: "req_manual",
		CreatedAt: time.Date(2026, 6, 7, 20, 20, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(0)))
	mock.ExpectQuery("INSERT INTO canonical_memories").
		WithArgs(
			record.MemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			record.Class,
			memory.MemoryStateActive,
			policy.RetentionClassDurable,
			record.Content,
			record.CreatedAt,
			record.CreatedAt,
		).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.MemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			record.Class,
			memory.MemoryStateActive,
			record.Content,
			record.CreatedAt,
			record.CreatedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(record.VersionID, record.MemoryID, int64(1), memory.MemoryStateActive, record.Content, record.CreatedAt, record.Actor).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			record.VersionID, record.MemoryID, int64(1), memory.MemoryStateActive, record.Content, record.CreatedAt, record.Actor,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			record.MemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			"manual_create_memory",
			record.RequestID,
			record.Actor,
			pgxmock.AnyArg(),
			record.CreatedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.CreateMemory(context.Background(), record)
	if err != nil {
		t.Fatalf("CreateMemory() error = %v", err)
	}

	if canonical.ID != record.MemoryID {
		t.Fatalf("ID = %q, want %q", canonical.ID, record.MemoryID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryUpdateMemoryRejectsVersionConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ManualUpdateMemoryRecord{
		MemoryID:        "mem_manual",
		VersionID:       "ver_manual",
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Content:         "Corrected content.",
		ExpectedVersion: 2,
		Reason:          "correct typo",
		Actor:           "operator-a",
		RequestID:       "req_manual",
		UpdatedAt:       time.Date(2026, 6, 7, 20, 25, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(3)))
	mock.ExpectRollback()

	repo := NewRepository(mock)
	_, err = repo.UpdateMemory(context.Background(), record)
	if !errors.Is(err, memory.ErrManualMutationVersionConflict) {
		t.Fatalf("UpdateMemory() error = %v, want version conflict", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryMergeMemorySuppressesSourceAndWritesTargetVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ManualMergeMemoryRecord{
		TargetMemoryID:  "mem_target",
		SourceMemoryID:  "mem_source",
		VersionID:       "ver_merge",
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Content:         "Merged profile content.",
		ExpectedVersion: 2,
		Reason:          "dedupe duplicate",
		Actor:           "operator-a",
		RequestID:       "req_merge",
		AppliedAt:       time.Date(2026, 6, 7, 20, 30, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.TargetMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.TargetMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, "Old target", record.AppliedAt.Add(-2*time.Hour), record.AppliedAt.Add(-time.Hour),
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.SourceMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.SourceMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, "Source duplicate", record.AppliedAt.Add(-3*time.Hour), record.AppliedAt.Add(-90*time.Minute),
		))
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.TargetMemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(2)))
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(record.TargetMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Content, record.AppliedAt).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.TargetMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, record.Content, record.AppliedAt.Add(-2*time.Hour), record.AppliedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(record.VersionID, record.TargetMemoryID, int64(3), memory.MemoryStateActive, record.Content, record.AppliedAt, record.Actor).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			record.VersionID, record.TargetMemoryID, int64(3), memory.MemoryStateActive, record.Content, record.AppliedAt, record.Actor,
		))
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(record.SourceMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, memory.MemoryStateSuppressed, record.AppliedAt).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.SourceMemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateSuppressed, "Source duplicate", record.AppliedAt.Add(-3*time.Hour), record.AppliedAt,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			record.TargetMemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			"manual_merge_memory",
			record.RequestID,
			record.Actor,
			pgxmock.AnyArg(),
			record.AppliedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			record.SourceMemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			"manual_merge_memory",
			record.RequestID,
			record.Actor,
			pgxmock.AnyArg(),
			record.AppliedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.MergeMemory(context.Background(), record)
	if err != nil {
		t.Fatalf("MergeMemory() error = %v", err)
	}

	if canonical.ID != record.TargetMemoryID {
		t.Fatalf("ID = %q, want %q", canonical.ID, record.TargetMemoryID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReclassifyMemoryUpdatesClassAndWritesProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ManualReclassifyMemoryRecord{
		MemoryID:        "mem_manual",
		VersionID:       "ver_reclass",
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TargetClass:     memory.MemoryClassProcedural,
		ExpectedVersion: 1,
		Reason:          "fix class",
		Actor:           "operator-a",
		RequestID:       "req_reclass",
		AppliedAt:       time.Date(2026, 6, 7, 20, 35, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs(record.MemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.MemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, "Respond concisely.", record.AppliedAt.Add(-2*time.Hour), record.AppliedAt.Add(-time.Hour),
		))
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)").
		WithArgs(record.MemoryID).
		WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(int64(1)))
	mock.ExpectQuery("UPDATE canonical_memories").
		WithArgs(record.MemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.TargetClass, record.AppliedAt).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			record.MemoryID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
			record.TargetClass, memory.MemoryStateActive, "Respond concisely.", record.AppliedAt.Add(-2*time.Hour), record.AppliedAt,
		))
	mock.ExpectQuery("INSERT INTO memory_versions").
		WithArgs(record.VersionID, record.MemoryID, int64(2), memory.MemoryStateActive, "Respond concisely.", record.AppliedAt, record.Actor).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "memory_id", "version", "state", "content", "created_at", "modified_by",
		}).AddRow(
			record.VersionID, record.MemoryID, int64(2), memory.MemoryStateActive, "Respond concisely.", record.AppliedAt, record.Actor,
		))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			nil,
			nil,
			record.MemoryID,
			record.Scope.Tenant,
			record.Scope.Project,
			record.Scope.Namespace,
			"manual_reclassify_memory",
			record.RequestID,
			record.Actor,
			pgxmock.AnyArg(),
			record.AppliedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	canonical, err := repo.ReclassifyMemory(context.Background(), record)
	if err != nil {
		t.Fatalf("ReclassifyMemory() error = %v", err)
	}

	if canonical.Class != record.TargetClass {
		t.Fatalf("Class = %q, want %q", canonical.Class, record.TargetClass)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositorySearchLexicalReturnsVisibleHits(t *testing.T) {
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
	now := time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .*lexical_score.*FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "concise answers", nil, nil, 3).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "lexical_score",
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
			0.88,
		))

	repo := NewRepository(mock)
	hits, err := repo.SearchLexical(context.Background(), retrieval.SearchInput{
		Scope: scope,
		Query: "concise answers",
		TopK:  3,
	})
	if err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
	}

	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want %d", len(hits), 1)
	}

	if hits[0].Memory.ID != "mem_123" {
		t.Fatalf("hits[0].Memory.ID = %q, want %q", hits[0].Memory.ID, "mem_123")
	}
}

func TestRepositoryListCitationsByMemoryIDs(t *testing.T) {
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

	mock.ExpectQuery("SELECT memory_id, raw_event_id, operation FROM provenance_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"memory_id", "raw_event_id", "operation"}).
			AddRow("mem_123", "evt_123", "promote_candidate").
			AddRow("mem_123", "evt_456", "create_summary_memory"))

	repo := NewRepository(mock)
	citations, err := repo.ListCitations(context.Background(), scope, []string{"mem_123"})
	if err != nil {
		t.Fatalf("ListCitations() error = %v", err)
	}

	if len(citations["mem_123"]) != 2 {
		t.Fatalf("len(citations[mem_123]) = %d, want %d", len(citations["mem_123"]), 2)
	}
}

func TestRepositorySearchSemanticReturnsVisibleSummaryHits(t *testing.T) {
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
	now := time.Date(2026, 6, 6, 14, 10, 0, 0, time.UTC)
	queryEmbedding := []float32{0.1, 0.2, 0.3}

	mock.ExpectQuery("SELECT .*semantic_score.*FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "[0.1,0.2,0.3]", nil, nil, 5).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "semantic_score",
		}).AddRow(
			"mem_summary",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassSummary,
			memory.MemoryStateActive,
			"Summary: the user is planning weekend travel.",
			now.Add(-time.Hour),
			now,
			0.76,
		))

	repo := NewRepository(mock)
	hits, err := repo.SearchSemantic(context.Background(), retrieval.SearchInput{
		Scope:            scope,
		Query:            "travel planning",
		QueryEmbedding:   queryEmbedding,
		TopK:             5,
		IncludeSummaries: true,
	})
	if err != nil {
		t.Fatalf("SearchSemantic() error = %v", err)
	}

	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want %d", len(hits), 1)
	}

	if hits[0].Memory.Class != memory.MemoryClassSummary {
		t.Fatalf("hits[0].Memory.Class = %q, want %q", hits[0].Memory.Class, memory.MemoryClassSummary)
	}
}

func TestRepositorySearchSemanticWithoutEmbeddingReturnsNoHits(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	hits, err := repo.SearchSemantic(context.Background(), retrieval.SearchInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Query: "travel planning",
		TopK:  5,
	})
	if err != nil {
		t.Fatalf("SearchSemantic() error = %v", err)
	}

	if len(hits) != 0 {
		t.Fatalf("len(hits) = %d, want %d", len(hits), 0)
	}
}

func TestRepositorySearchRelationsReturnsRelationHitsOnlyWhenEnabled(t *testing.T) {
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
	now := time.Date(2026, 6, 6, 14, 20, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .*relation_score.*FROM relation_projections").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "travel", "travel", "travel", "travel", "travel", nil, nil, 4).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "relation_score",
		}).AddRow(
			"mem_relation",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			memory.MemoryClassRelation,
			memory.MemoryStateActive,
			"entity:user relation:interested_in target:travel",
			now.Add(-time.Hour),
			now,
			0.64,
		))

	repo := NewRepository(mock)
	hits, err := repo.SearchRelations(context.Background(), retrieval.SearchInput{
		Scope:            scope,
		Query:            "travel",
		TopK:             4,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("SearchRelations() error = %v", err)
	}

	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want %d", len(hits), 1)
	}

	if hits[0].Memory.ID != "mem_relation" {
		t.Fatalf("hits[0].Memory.ID = %q, want %q", hits[0].Memory.ID, "mem_relation")
	}
}

func TestRepositorySearchLexicalPassesTimeWindowFilters(t *testing.T) {
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
	from := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT .*lexical_score.*FROM canonical_memories").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "travel", from, to, 2).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at", "lexical_score",
		}))

	repo := NewRepository(mock)
	_, err = repo.SearchLexical(context.Background(), retrieval.SearchInput{
		Scope:    scope,
		Query:    "travel",
		TopK:     2,
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
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
