package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestProceduralVerificationRepositoryAppendsScopedVerification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	record := postgresProceduralVerification()
	fingerprint, err := memory.ProceduralMemoryVerificationFingerprint(record)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("INSERT INTO procedural_memory_verifications").WithArgs(
		record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
		record.MemoryID, record.TargetVersion, fingerprint, record.ClaimFingerprint,
		record.ApplicabilityFingerprint, record.Disposition, pgxmock.AnyArg(), record.SourceWatermark,
		record.PolicyVersion, record.Actor, record.Reason, pgxmock.AnyArg(), record.CreatedAt,
	).WillReturnRows(proceduralVerificationRows(record, fingerprint))

	stored, err := NewRepository(mock).AppendProceduralMemoryVerification(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != record.ID || stored.Scope != record.Scope || stored.TargetVersion != record.TargetVersion {
		t.Fatalf("stored = %+v, want scoped target %+v", stored, record)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralVerificationRepositoryReadIsExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	record := postgresProceduralVerification()
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs("other", record.Scope.Project, record.Scope.Namespace, record.ID).WillReturnError(pgx.ErrNoRows)

	_, err = NewRepository(mock).ReadProceduralMemoryVerification(context.Background(), memory.Scope{Tenant: "other", Project: record.Scope.Project, Namespace: record.Scope.Namespace}, record.ID)
	if !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("foreign scope read error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryRecoversExpiredLease(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("WITH claimed AS").WithArgs(scope.Tenant, scope.Project, scope.Namespace, "worker-b", now, now.Add(time.Minute), 1).
		WillReturnRows(proceduralRevalidationRunRows(now, scope))

	runs, err := NewRepository(mock).ClaimProceduralRevalidationRuns(context.Background(), memory.ClaimProceduralRevalidationRunsInput{Scope: scope, WorkerID: "worker-b", Now: now, LeaseDuration: time.Minute, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].LeaseOwner != "worker-b" || runs[0].Attempt != 2 {
		t.Fatalf("runs = %+v, want one reclaimed run", runs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryClaimsScopedPendingRunWithLeaseOwnership(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("UPDATE procedural_memory_revalidation_runs").WithArgs(scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000201", "worker-a", now, now.Add(time.Minute)).
		WillReturnRows(proceduralRevalidationRunRows(now, scope))

	run, err := NewRepository(mock).ClaimProceduralRevalidationRun(context.Background(), memory.ClaimProceduralRevalidationRunInput{Scope: scope, RunID: "00000000-0000-0000-0000-000000000201", WorkerID: "worker-a", Now: now, LeaseDuration: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if run.LeaseOwner != "worker-b" {
		t.Fatalf("run lease owner = %q", run.LeaseOwner)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryCoalescesScopedWorkIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	run := proceduralRevalidationRun(now)
	const existingID = "00000000-0000-0000-0000-000000000299"
	mock.ExpectQuery("INSERT INTO procedural_memory_revalidation_runs").WithArgs(
		run.ID, run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace, run.VerificationID,
		run.MemoryID, run.TargetVersion, run.SourceWatermark, run.Trigger, run.WorkIdentity,
		run.Status, run.MaxAttempts, run.NextAttemptAt, run.CreatedAt, run.UpdatedAt,
	).WillReturnRows(proceduralRevalidationRunRowsWithID(now, run.Scope, existingID))

	stored, err := NewRepository(mock).CreateProceduralRevalidationRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != existingID || stored.WorkIdentity != run.WorkIdentity || stored.Scope != run.Scope {
		t.Fatalf("stored = %+v, want existing durable work identity %+v", stored, run)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryRejectsCompletionWithoutLeaseOwnership(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectExec("UPDATE procedural_memory_revalidation_runs").WithArgs(
		scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000201", "worker-a",
		memory.ProceduralRevalidationRunStatusCompleted, "00000000-0000-0000-0000-000000000301", now,
	).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))

	err = NewRepository(mock).CompleteProceduralRevalidationRun(context.Background(), memory.CompleteProceduralRevalidationRunInput{
		Scope: scope, RunID: "00000000-0000-0000-0000-000000000201", WorkerID: "worker-a",
		Status: memory.ProceduralRevalidationRunStatusCompleted, TerminalVerificationID: "00000000-0000-0000-0000-000000000301", CompletedAt: now,
	})
	if !errors.Is(err, memory.ErrProceduralRevalidationLeaseLost) {
		t.Fatalf("complete error = %v, want lease lost", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryExhaustsOwnedRunningRun(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectExec("UPDATE procedural_memory_revalidation_runs").WithArgs(
		scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000201", "worker-a", now,
	).WillReturnResult(pgconn.NewCommandTag("UPDATE 1"))

	err = NewRepository(mock).ExhaustProceduralRevalidationRun(context.Background(), memory.ExhaustProceduralRevalidationRunInput{
		Scope: scope, RunID: "00000000-0000-0000-0000-000000000201", WorkerID: "worker-a", ExhaustedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralVerificationRepositoryListsTargetHistoryInExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	record := postgresProceduralVerification()
	fingerprint, err := memory.ProceduralMemoryVerificationFingerprint(record)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, 10).
		WillReturnRows(proceduralVerificationRows(record, fingerprint))

	history, err := NewRepository(mock).ListProceduralMemoryVerificationHistory(context.Background(), record.Scope, record.MemoryID, record.TargetVersion, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != record.ID {
		t.Fatalf("history = %+v", history)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralVerificationRepositoryResolvesCurrentTargetAndTypedEvidence(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	record := postgresProceduralVerification()
	mock.ExpectQuery("FROM canonical_memories m").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	mock.ExpectQuery("FROM raw_events").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Evidence[0].ID).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	repo := NewRepository(mock)
	if err := repo.ValidateProceduralVerificationTarget(context.Background(), record); err != nil {
		t.Fatalf("target validation: %v", err)
	}
	if err := repo.ValidateProceduralVerificationEvidence(context.Background(), record.Scope, record.Evidence[0]); err != nil {
		t.Fatalf("evidence validation: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCorrectionWorkerRechecksAttributionInExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	record := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: scope, Type: memory.MemoryIntentUpdate,
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3, Content: "corrected procedure",
		Verification: &memory.VerificationAttribution{VerificationID: "00000000-0000-0000-0000-000000000101", TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3, SourceWatermark: "watermark-1", PolicyVersion: "policy-v1"},
	}
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace, record.Verification.VerificationID, record.TargetMemoryID, record.TargetVersion, record.Verification.SourceWatermark, record.Verification.PolicyVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	verification := postgresProceduralVerification()
	verification.Disposition = memory.ProceduralVerificationDispositionRefuted
	verification.CorrectedContent = record.Content
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs(scope.Tenant, scope.Project, scope.Namespace, record.Verification.VerificationID).
		WillReturnRows(proceduralVerificationRows(verification, strings.Repeat("a", 64)))
	mock.ExpectQuery("FROM canonical_memories m").WithArgs(scope.Tenant, scope.Project, scope.Namespace, record.TargetMemoryID, record.TargetVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	mock.ExpectQuery("FROM raw_events").WithArgs(scope.Tenant, scope.Project, scope.Namespace, verification.Evidence[0].ID).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	if err := NewRepository(mock).ValidateMemoryIntentVerification(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCorrectionWorkerRejectsTargetConflictBeforeCanonicalMutation(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	record := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: scope, Type: memory.MemoryIntentUpdate,
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		Verification: &memory.VerificationAttribution{VerificationID: "00000000-0000-0000-0000-000000000101", TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3, SourceWatermark: "watermark-1", PolicyVersion: "policy-v1"},
	}
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace, record.Verification.VerificationID, record.TargetMemoryID, record.TargetVersion, record.Verification.SourceWatermark, record.Verification.PolicyVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(false))
	if err := NewRepository(mock).ValidateMemoryIntentVerification(context.Background(), record); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("ValidateMemoryIntentVerification() error = %v, want lifecycle-safe conflict refusal", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryReadsRunInExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("FROM procedural_memory_revalidation_runs").WithArgs(scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000201").
		WillReturnRows(proceduralRevalidationRunRows(now, scope))

	run, err := NewRepository(mock).ReadProceduralRevalidationRun(context.Background(), scope, "00000000-0000-0000-0000-000000000201")
	if err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.Scope != scope || run.TargetVersion != 3 {
		t.Fatalf("run = %+v", run)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralRevalidationRepositoryListsOnlyPendingRunsForRestartRecovery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("FROM procedural_memory_revalidation_runs").WithArgs(scope.Tenant, scope.Project, scope.Namespace, now, 10).
		WillReturnRows(proceduralPendingRevalidationRunRows(now, scope))

	runs, err := NewRepository(mock).ListPendingProceduralRevalidationRuns(context.Background(), memory.ListPendingProceduralRevalidationRunsInput{Scope: scope, Now: now, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != memory.ProceduralRevalidationRunStatusPending || runs[0].Scope != scope {
		t.Fatalf("pending runs = %+v", runs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCorrectionWorkerPersistsBoundedIntentCandidateSuccessorLineage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	record := postgresProceduralVerification()
	fingerprint, err := memory.ProceduralMemoryVerificationFingerprint(record)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(proceduralVerificationRows(record, fingerprint))
	mock.ExpectQuery("FROM procedural_memory_revalidation_runs").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(proceduralRevalidationRunRows(now, record.Scope))
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(pgxmock.NewRows([]string{"verification_id", "intent_id", "status", "candidate_id", "successor_memory_id", "successor_version"}).
			AddRow(record.ID, "00000000-0000-0000-0000-000000000401", memory.MemoryIntentStatusActive, "00000000-0000-0000-0000-000000000501", record.MemoryID, int64(4)))

	lineage, err := NewRepository(mock).ReadProceduralVerificationLineage(context.Background(), record.Scope, record.MemoryID, record.TargetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(lineage.Verifications) != 1 || len(lineage.Runs) != 1 || len(lineage.Intents) != 1 || lineage.Verifications[0].ID != record.ID {
		t.Fatalf("lineage = %+v", lineage)
	}
	if link := lineage.Intents[0]; link.VerificationID != record.ID || link.IntentID != "00000000-0000-0000-0000-000000000401" || link.CandidateID == "" || link.SuccessorVersion != 4 {
		t.Fatalf("lineage intent link = %+v, want bounded verification-to-successor chain", link)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralVerificationHistoryRedactsForeignIntentCandidateAndSuccessorLinks(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	record := postgresProceduralVerification()
	foreign := memory.Scope{Tenant: "tenant-b", Project: record.Scope.Project, Namespace: record.Scope.Namespace}
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs(foreign.Tenant, foreign.Project, foreign.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_id", "target_version", "fingerprint", "claim_fingerprint", "applicability_fingerprint", "disposition", "evidence", "source_watermark", "policy_version", "actor", "reason", "corrected_content", "created_at"}))
	mock.ExpectQuery("FROM procedural_memory_revalidation_runs").WithArgs(foreign.Tenant, foreign.Project, foreign.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "verification_id", "memory_id", "target_version", "source_watermark", "trigger", "work_identity", "status", "attempt_count", "max_attempts", "lease_owner", "lease_expires_at", "next_attempt_at", "terminal_verification_id", "failure_category", "created_at", "updated_at", "terminal_at"}))
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(foreign.Tenant, foreign.Project, foreign.Namespace, record.MemoryID, record.TargetVersion, 100).
		WillReturnRows(pgxmock.NewRows([]string{"verification_id", "intent_id", "status", "candidate_id", "successor_memory_id", "successor_version"}))

	lineage, err := NewRepository(mock).ReadProceduralVerificationLineage(context.Background(), foreign, record.MemoryID, record.TargetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(lineage.Verifications) != 0 || len(lineage.Runs) != 0 || len(lineage.Intents) != 0 {
		t.Fatalf("foreign lineage = %+v, want redacted empty lineage", lineage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func postgresProceduralVerification() memory.ProceduralMemoryVerification {
	return memory.ProceduralMemoryVerification{
		ID: "00000000-0000-0000-0000-000000000101", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64),
		Disposition: memory.ProceduralVerificationDispositionSupported,
		Evidence:    []memory.ProceduralVerificationEvidence{{Kind: memory.ProceduralVerificationEvidenceToolObservation, SourceKind: memory.ProceduralVerificationEvidenceSourceRawEvent, ID: "00000000-0000-0000-0000-000000000103", SourceVersion: 1, Corroborates: true}},
		Actor:       "operator", Reason: "verified", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1",
		CreatedAt: time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
	}
}

func proceduralVerificationRows(record memory.ProceduralMemoryVerification, fingerprint string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_id", "target_version", "fingerprint", "claim_fingerprint", "applicability_fingerprint", "disposition", "evidence", "source_watermark", "policy_version", "actor", "reason", "corrected_content", "created_at"}).
		AddRow(record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, fingerprint, record.ClaimFingerprint, record.ApplicabilityFingerprint, record.Disposition, []byte(`[{"scope":{"tenant":"tenant-a","project":"project-a","namespace":"namespace-a"},"kind":"tool_observation","source_kind":"raw_event","polarity":"","id":"00000000-0000-0000-0000-000000000103","source_version":1,"corroborates":true}]`), record.SourceWatermark, record.PolicyVersion, record.Actor, record.Reason, nil, record.CreatedAt)
}

func proceduralRevalidationRunRows(now time.Time, scope memory.Scope) *pgxmock.Rows {
	return proceduralRevalidationRunRowsWithID(now, scope, "00000000-0000-0000-0000-000000000201")
}

func proceduralRevalidationRunRowsWithID(now time.Time, scope memory.Scope, id string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "verification_id", "memory_id", "target_version", "source_watermark", "trigger", "work_identity", "status", "attempt_count", "max_attempts", "lease_owner", "lease_expires_at", "next_attempt_at", "terminal_verification_id", "failure_category", "created_at", "updated_at", "terminal_at"}).
		AddRow(id, scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000101", "00000000-0000-0000-0000-000000000102", 3, "watermark-1", memory.ProceduralRevalidationTriggerAdministrator, "work-1", memory.ProceduralRevalidationRunStatusRunning, 2, 3, "worker-b", now.Add(time.Minute), now, nil, nil, now.Add(-time.Hour), now, nil)
}

func proceduralPendingRevalidationRunRows(now time.Time, scope memory.Scope) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "verification_id", "memory_id", "target_version", "source_watermark", "trigger", "work_identity", "status", "attempt_count", "max_attempts", "lease_owner", "lease_expires_at", "next_attempt_at", "terminal_verification_id", "failure_category", "created_at", "updated_at", "terminal_at"}).
		AddRow("00000000-0000-0000-0000-000000000201", scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000101", "00000000-0000-0000-0000-000000000102", 3, "watermark-1", memory.ProceduralRevalidationTriggerAdministrator, "work-1", memory.ProceduralRevalidationRunStatusPending, 0, 3, nil, nil, now, nil, nil, now.Add(-time.Hour), now, nil)
}

func proceduralRevalidationRun(now time.Time) memory.ProceduralRevalidationRun {
	return memory.ProceduralRevalidationRun{
		ID: "00000000-0000-0000-0000-000000000201", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		VerificationID: "00000000-0000-0000-0000-000000000101", MemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		SourceWatermark: "watermark-1", Trigger: memory.ProceduralRevalidationTriggerAdministrator, WorkIdentity: "work-1",
		Status: memory.ProceduralRevalidationRunStatusPending, MaxAttempts: 3, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}
}
