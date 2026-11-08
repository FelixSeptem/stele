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

func TestProceduralVerificationRepositoryClaimsExpiredRevalidationRun(t *testing.T) {
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

func TestProceduralVerificationRepositoryCreatesRevalidationRunByScopedWorkIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)
	run := proceduralRevalidationRun(now)
	mock.ExpectQuery("INSERT INTO procedural_memory_revalidation_runs").WithArgs(
		run.ID, run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace, run.VerificationID,
		run.MemoryID, run.TargetVersion, run.SourceWatermark, run.Trigger, run.WorkIdentity,
		run.Status, run.MaxAttempts, run.NextAttemptAt, run.CreatedAt, run.UpdatedAt,
	).WillReturnRows(proceduralRevalidationRunRows(now, run.Scope))

	stored, err := NewRepository(mock).CreateProceduralRevalidationRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != run.ID || stored.WorkIdentity != run.WorkIdentity || stored.Scope != run.Scope {
		t.Fatalf("stored = %+v, want %+v", stored, run)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProceduralVerificationRepositoryCompletesOnlyOwnedRun(t *testing.T) {
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

func TestProceduralVerificationRepositoryReadsRevalidationRunInExactScope(t *testing.T) {
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
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "verification_id", "memory_id", "target_version", "source_watermark", "trigger", "work_identity", "status", "attempt_count", "max_attempts", "lease_owner", "lease_expires_at", "next_attempt_at", "terminal_verification_id", "failure_category", "created_at", "updated_at", "terminal_at"}).
		AddRow("00000000-0000-0000-0000-000000000201", scope.Tenant, scope.Project, scope.Namespace, "00000000-0000-0000-0000-000000000101", "00000000-0000-0000-0000-000000000102", 3, "watermark-1", memory.ProceduralRevalidationTriggerAdministrator, "work-1", memory.ProceduralRevalidationRunStatusRunning, 2, 3, "worker-b", now.Add(time.Minute), now, nil, nil, now.Add(-time.Hour), now, nil)
}

func proceduralRevalidationRun(now time.Time) memory.ProceduralRevalidationRun {
	return memory.ProceduralRevalidationRun{
		ID: "00000000-0000-0000-0000-000000000201", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		VerificationID: "00000000-0000-0000-0000-000000000101", MemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		SourceWatermark: "watermark-1", Trigger: memory.ProceduralRevalidationTriggerAdministrator, WorkIdentity: "work-1",
		Status: memory.ProceduralRevalidationRunStatusPending, MaxAttempts: 3, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}
}
