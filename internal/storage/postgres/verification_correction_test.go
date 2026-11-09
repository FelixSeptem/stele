package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestVerificationCorrectionAdmissionRejectsMismatchedTargetBeforeHandoff(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	input := memory.MemoryIntentInput{
		Scope: scope, MemoryPath: "procedures/recovery", Type: memory.MemoryIntentUpdate,
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		Content: "corrected procedure", Actor: "operator", Reason: "refuted verification",
		RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idempotency-1",
	}
	attribution := memory.VerificationAttribution{
		VerificationID: "00000000-0000-0000-0000-000000000101",
		TargetMemoryID: "00000000-0000-0000-0000-000000000999", TargetVersion: 3,
		SourceWatermark: "watermark-1", PolicyVersion: "policy-v1",
	}

	_, err = NewRepository(mock).SubmitVerificationAttributedIntent(context.Background(), input, attribution)
	if !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("SubmitVerificationAttributedIntent() error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationCorrectionAdmissionPersistsAttributionAndHandoffAtomically(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	now := time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC)
	verification := postgresProceduralVerification()
	verification.Disposition = memory.ProceduralVerificationDispositionRefuted
	verification.CorrectedContent = "corrected procedure"
	input := memory.MemoryIntentInput{
		Scope: verification.Scope, MemoryPath: "procedures/recovery", Type: memory.MemoryIntentUpdate,
		TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion,
		Content: verification.CorrectedContent, Actor: "operator", Reason: "refuted verification",
		RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idempotency-1",
		Evidence: []memory.MemoryIntentEvidence{{Scope: verification.Scope, Kind: "procedural_verification", ID: verification.ID, Version: verification.TargetVersion}},
	}
	attribution := memory.VerificationAttribution{
		VerificationID: verification.ID, TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion,
		SourceWatermark: verification.SourceWatermark, PolicyVersion: verification.PolicyVersion,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM procedural_memory_verifications").
		WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.ID).
		WillReturnRows(verificationCorrectionRows(verification))
	mock.ExpectQuery("FROM canonical_memories m").
		WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.MemoryID, verification.TargetVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	mock.ExpectQuery("FROM raw_events").
		WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.Evidence[0].ID).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))

	created := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: input.Scope, MemoryPath: input.MemoryPath,
		Type: input.Type, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion,
		Content: input.Content, Actor: input.Actor, Reason: input.Reason, RequestID: input.RequestID,
		OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey,
		Evidence: input.Evidence, PolicyVersion: attribution.PolicyVersion,
		Status: memory.MemoryIntentStatusAccepted, CreatedAt: now,
	}
	mock.ExpectQuery("INSERT INTO memory_intents").WithArgs(anyArgs(25)...).WillReturnRows(verificationAttributedIntentRows(created))
	mock.ExpectExec("INSERT INTO memory_intent_verification_attributions").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_intent_transitions").WithArgs(anyArgs(14)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_intent_river_jobs").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	repo.SetMemoryIntentRiverInserter(verificationRiverInserter{jobID: 17})
	stored, err := repo.SubmitVerificationAttributedIntent(context.Background(), input, attribution)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != created.ID || stored.Verification == nil || *stored.Verification != attribution {
		t.Fatalf("stored = %+v, want immutable verification attribution", stored)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReadMemoryIntentRehydratesVerificationAttribution(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	attribution := memory.VerificationAttribution{
		VerificationID: "00000000-0000-0000-0000-000000000101",
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		SourceWatermark: "watermark-1", PolicyVersion: "policy-v1",
	}
	record := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: scope, MemoryPath: "procedures/recovery",
		Type: memory.MemoryIntentUpdate, TargetMemoryID: attribution.TargetMemoryID, TargetVersion: attribution.TargetVersion,
		Content: "corrected procedure", Actor: "operator", Reason: "refuted verification", RequestID: "request-1",
		OperationID: "operation-1", IdempotencyKey: "idempotency-1", PolicyVersion: attribution.PolicyVersion,
		Evidence: []memory.MemoryIntentEvidence{{Scope: scope, Kind: "procedural_verification", ID: attribution.VerificationID, Version: attribution.TargetVersion}},
		Status:   memory.MemoryIntentStatusAccepted, CreatedAt: time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC),
	}
	mock.ExpectQuery("FROM memory_intents").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(verificationAttributedIntentRows(record))
	mock.ExpectQuery("FROM memory_intent_correction_attributions").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"verification_id", "target_memory_id", "target_version", "source_watermark", "policy_version"}).
			AddRow(attribution.VerificationID, attribution.TargetMemoryID, attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion))

	stored, err := NewRepository(mock).ReadMemoryIntent(context.Background(), scope, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Verification == nil || *stored.Verification != attribution {
		t.Fatalf("stored verification = %+v, want %+v", stored.Verification, attribution)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListMemoryIntentsRehydratesVerificationAttribution(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	attribution := memory.VerificationAttribution{
		VerificationID: "00000000-0000-0000-0000-000000000101",
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		SourceWatermark: "watermark-1", PolicyVersion: "policy-v1",
	}
	record := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: scope, MemoryPath: "procedures/recovery",
		Type: memory.MemoryIntentUpdate, TargetMemoryID: attribution.TargetMemoryID, TargetVersion: attribution.TargetVersion,
		Content: "corrected procedure", Actor: "operator", Reason: "refuted verification", RequestID: "request-1",
		OperationID: "operation-1", IdempotencyKey: "idempotency-1", PolicyVersion: attribution.PolicyVersion,
		Evidence: []memory.MemoryIntentEvidence{{Scope: scope, Kind: "procedural_verification", ID: attribution.VerificationID, Version: attribution.TargetVersion}},
		Status:   memory.MemoryIntentStatusAccepted, CreatedAt: time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC),
	}
	mock.ExpectQuery("FROM memory_intents i").WithArgs(scope.Tenant, scope.Project, scope.Namespace, nil, 10).
		WillReturnRows(verificationAttributedIntentListRows(record))
	mock.ExpectQuery("FROM memory_intent_verification_attributions").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"verification_id", "target_memory_id", "target_version", "source_watermark", "policy_version"}).
			AddRow(attribution.VerificationID, attribution.TargetMemoryID, attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion))

	items, err := NewRepository(mock).ListMemoryIntents(context.Background(), memory.MemoryIntentListInput{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Verification == nil || *items[0].Verification != attribution {
		t.Fatalf("items = %+v, want one verification-attributed intent", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitVerificationAttributedIntentReplaysOnlyMatchingAttribution(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	verification := postgresProceduralVerification()
	verification.Disposition = memory.ProceduralVerificationDispositionRefuted
	verification.CorrectedContent = "corrected procedure"
	input := memory.MemoryIntentInput{
		Scope: verification.Scope, MemoryPath: "procedures/recovery", Type: memory.MemoryIntentUpdate,
		TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion,
		Content: verification.CorrectedContent, Actor: "operator", Reason: "refuted verification",
		RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idempotency-1",
		Evidence: []memory.MemoryIntentEvidence{{Scope: verification.Scope, Kind: "procedural_verification", ID: verification.ID, Version: verification.TargetVersion}},
	}
	attribution := memory.VerificationAttribution{
		VerificationID: verification.ID, TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion,
		SourceWatermark: verification.SourceWatermark, PolicyVersion: verification.PolicyVersion,
	}
	fingerprint, err := memory.VerificationIntentFingerprint(input, attribution)
	if err != nil {
		t.Fatal(err)
	}
	replayed := memory.MemoryIntentRecord{
		ID: "00000000-0000-0000-0000-000000000401", Scope: input.Scope, MemoryPath: input.MemoryPath,
		Type: input.Type, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion,
		Content: input.Content, Actor: input.Actor, Reason: input.Reason, RequestID: input.RequestID,
		OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey, Evidence: input.Evidence,
		PolicyVersion: attribution.PolicyVersion, Status: memory.MemoryIntentStatusAccepted,
		CreatedAt: time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM procedural_memory_verifications").WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.ID).
		WillReturnRows(verificationCorrectionRows(verification))
	mock.ExpectQuery("FROM canonical_memories m").WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.MemoryID, verification.TargetVersion).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	mock.ExpectQuery("FROM raw_events").WithArgs(verification.Scope.Tenant, verification.Scope.Project, verification.Scope.Namespace, verification.Evidence[0].ID).
		WillReturnRows(pgxmock.NewRows([]string{"eligible"}).AddRow(true))
	mock.ExpectQuery("INSERT INTO memory_intents").WithArgs(anyArgs(25)...).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT request_fingerprint").WithArgs(input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.IdempotencyKey, input.OperationID).
		WillReturnRows(verificationAttributedIntentReplayRows(replayed, fingerprint))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM memory_intent_verification_attributions").WithArgs(replayed.ID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, attribution.VerificationID, attribution.TargetMemoryID, attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()

	stored, err := NewRepository(mock).SubmitVerificationAttributedIntent(context.Background(), input, attribution)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != replayed.ID || stored.Status != memory.MemoryIntentStatusReplayed || stored.Verification == nil || *stored.Verification != attribution {
		t.Fatalf("replayed intent = %+v, want original verification-attributed result", stored)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationAttributedIntentRejectsConflictingReplayAttribution(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	record := memory.MemoryIntentRecord{ID: "00000000-0000-0000-0000-000000000401", Scope: scope}
	attribution := memory.VerificationAttribution{
		VerificationID: "00000000-0000-0000-0000-000000000101",
		TargetMemoryID: "00000000-0000-0000-0000-000000000102", TargetVersion: 3,
		SourceWatermark: "watermark-2", PolicyVersion: "policy-v1",
	}
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM memory_intent_verification_attributions").
		WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace, attribution.VerificationID, attribution.TargetMemoryID, attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	err = NewRepository(mock).verifyReplayVerificationAttribution(context.Background(), mock, record, attribution)
	if !errors.Is(err, memory.ErrIdempotencyConflict) {
		t.Fatalf("verifyReplayVerificationAttribution() error = %v, want ErrIdempotencyConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type verificationRiverInserter struct{ jobID int64 }

func (s verificationRiverInserter) InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{ID: s.jobID}}, nil
}

func verificationCorrectionRows(record memory.ProceduralMemoryVerification) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_id", "target_version", "fingerprint", "claim_fingerprint", "applicability_fingerprint", "disposition", "evidence", "source_watermark", "policy_version", "actor", "reason", "corrected_content", "created_at"}).
		AddRow(record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion, strings.Repeat("a", 64), record.ClaimFingerprint, record.ApplicabilityFingerprint, record.Disposition, []byte(`[{"scope":{"tenant":"tenant-a","project":"project-a","namespace":"namespace-a"},"kind":"tool_observation","source_kind":"raw_event","polarity":"","id":"00000000-0000-0000-0000-000000000103","source_version":1,"corroborates":true}]`), record.SourceWatermark, record.PolicyVersion, record.Actor, record.Reason, record.CorrectedContent, record.CreatedAt)
}

func verificationAttributedIntentRows(record memory.MemoryIntentRecord) *pgxmock.Rows {
	targetID := record.TargetMemoryID
	targetVersion := record.TargetVersion
	policyVersion := record.PolicyVersion
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "intent_type", "actor", "reason", "provenance", "request_id", "operation_id", "idempotency_key", "target_memory_id", "target_version", "target_insight_id", "evidence_refs", "outcome_reference", "policy_version", "precedence_version", "precedence_stage", "precedence_outcome", "payload", "status", "created_at"}).
		AddRow(record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryPath, record.Type, record.Actor, record.Reason, []byte(`{}`), record.RequestID, record.OperationID, record.IdempotencyKey, &targetID, &targetVersion, nil, []byte(`[{"scope":{"tenant":"tenant-a","project":"project-a","namespace":"namespace-a"},"kind":"procedural_verification","id":"00000000-0000-0000-0000-000000000101","version":3}]`), nil, &policyVersion, nil, nil, nil, []byte(`{"content":"corrected procedure","memory_path":"procedures/recovery"}`), record.Status, record.CreatedAt)
}

func verificationAttributedIntentListRows(record memory.MemoryIntentRecord) *pgxmock.Rows {
	targetID := record.TargetMemoryID
	targetVersion := record.TargetVersion
	policyVersion := record.PolicyVersion
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "intent_type", "actor", "reason", "provenance", "request_id", "operation_id", "idempotency_key", "target_memory_id", "target_version", "target_insight_id", "evidence_refs", "outcome_reference", "policy_version", "precedence_version", "precedence_stage", "precedence_outcome", "payload", "status", "created_at", "projection_id", "source_revision", "binding_id", "principal_id", "attribution_state"}).
		AddRow(record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryPath, record.Type, record.Actor, record.Reason, []byte(`{}`), record.RequestID, record.OperationID, record.IdempotencyKey, &targetID, &targetVersion, nil, []byte(`[{"scope":{"tenant":"tenant-a","project":"project-a","namespace":"namespace-a"},"kind":"procedural_verification","id":"00000000-0000-0000-0000-000000000101","version":3}]`), nil, &policyVersion, nil, nil, nil, []byte(`{"content":"corrected procedure","memory_path":"procedures/recovery"}`), record.Status, record.CreatedAt, nil, nil, nil, nil, nil)
}

func verificationAttributedIntentReplayRows(record memory.MemoryIntentRecord, fingerprint string) *pgxmock.Rows {
	targetID := record.TargetMemoryID
	targetVersion := record.TargetVersion
	policyVersion := record.PolicyVersion
	return pgxmock.NewRows([]string{"request_fingerprint", "id", "tenant", "project", "namespace", "memory_path", "intent_type", "actor", "reason", "provenance", "request_id", "operation_id", "idempotency_key", "target_memory_id", "target_version", "target_insight_id", "evidence_refs", "outcome_reference", "policy_version", "precedence_version", "precedence_stage", "precedence_outcome", "payload", "status", "created_at"}).
		AddRow(fingerprint, record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryPath, record.Type, record.Actor, record.Reason, []byte(`{}`), record.RequestID, record.OperationID, record.IdempotencyKey, &targetID, &targetVersion, nil, []byte(`[{"scope":{"tenant":"tenant-a","project":"project-a","namespace":"namespace-a"},"kind":"procedural_verification","id":"00000000-0000-0000-0000-000000000101","version":3}]`), nil, &policyVersion, nil, nil, nil, []byte(`{"content":"corrected procedure","memory_path":"procedures/recovery"}`), record.Status, record.CreatedAt)
}

func anyArgs(count int) []any {
	args := make([]any, count)
	for index := range args {
		args[index] = pgxmock.AnyArg()
	}
	return args
}
