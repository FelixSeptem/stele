package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryReadsCredentialAndChecksExactScopeGrant(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("FROM access_credentials c").WithArgs("stl_credential_1").WillReturnRows(pgxmock.NewRows([]string{"principal_id", "role", "principal_status", "principal_label", "principal_expires_at", "principal_created_at", "principal_updated_at", "credential_record_id", "credential_id", "credential_status", "salt", "digest", "credential_expires_at", "credential_created_at", "credential_disabled_at"}).AddRow("principal_1", string(auth.PrincipalRolePublic), string(auth.PrincipalStatusActive), "integration-a", nil, now, now, "credential-record_1", "stl_credential_1", string(auth.CredentialStatusActive), []byte("salt"), []byte("digest"), nil, now, nil))
	mock.ExpectQuery("FROM access_scope_grants").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"authorized"}).AddRow(true))
	repo := NewRepository(mock)
	principal, credential, err := repo.ReadPrincipalCredential(context.Background(), "stl_credential_1")
	if err != nil || principal.ID != "principal_1" || credential.ID != "credential-record_1" || credential.CredentialID != "stl_credential_1" {
		t.Fatalf("ReadPrincipalCredential() = %+v %+v %v", principal, credential, err)
	}
	authorized, err := repo.HasActiveScopeGrant(context.Background(), principal.ID, scope)
	if err != nil || !authorized {
		t.Fatalf("HasActiveScopeGrant() = %v, %v", authorized, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryChecksForActiveAdminPrincipal(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	mock.ExpectQuery("FROM access_principals").WithArgs(string(auth.PrincipalRoleAdmin), string(auth.PrincipalStatusActive)).WillReturnRows(pgxmock.NewRows([]string{"has_active_admin"}).AddRow(true))

	hasAdmin, err := NewRepository(mock).HasActiveAdminPrincipal(context.Background())
	if err != nil || !hasAdmin {
		t.Fatalf("HasActiveAdminPrincipal() = %v, %v; want true, nil", hasAdmin, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryIdempotentIngestReplaysCompletedMatchingClaim(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO event_idempotency_records").WithArgs(pgxmock.AnyArg(), "principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key", "fingerprint", pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM event_idempotency_records i[\\s\\S]*FOR UPDATE OF i").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key").WillReturnRows(pgxmock.NewRows([]string{"request_fingerprint", "status", "event_id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at", "admission", "lease_until"}).AddRow("fingerprint", "completed", "00000000-0000-0000-0000-000000000123", scope.Tenant, scope.Project, scope.Namespace, "conversation.message", "hello", now, now, []byte(`{"decision":"accept"}`), nil))
	mock.ExpectCommit()

	result, err := NewRepository(mock).IngestEventIdempotent(context.Background(), memory.IdempotentEventIngestInput{PrincipalID: "principal_1", IdempotencyKey: "request-key", RequestFingerprint: "fingerprint"}, memory.IngestEventInput{Scope: scope, EventType: "conversation.message", Content: "hello"}, memory.ProvenanceRecord{}, memory.AdmissionPressureReport{})
	if err != nil || !result.Replayed || result.Event.ID != "00000000-0000-0000-0000-000000000123" {
		t.Fatalf("IngestEventIdempotent() = %+v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryIdempotentIngestRejectsConflictingFingerprint(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO event_idempotency_records").WithArgs(pgxmock.AnyArg(), "principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key", "new-fingerprint", pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM event_idempotency_records i").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key").WillReturnRows(pgxmock.NewRows([]string{"request_fingerprint", "status", "event_id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at", "admission", "lease_until"}).AddRow("original-fingerprint", "pending", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	mock.ExpectRollback()

	_, err = NewRepository(mock).IngestEventIdempotent(context.Background(), memory.IdempotentEventIngestInput{PrincipalID: "principal_1", IdempotencyKey: "request-key", RequestFingerprint: "new-fingerprint"}, memory.IngestEventInput{Scope: scope, EventType: "conversation.message", Content: "changed"}, memory.ProvenanceRecord{}, memory.AdmissionPressureReport{})
	if err != memory.ErrIdempotencyConflict {
		t.Fatalf("IngestEventIdempotent() error = %v, want %v", err, memory.ErrIdempotencyConflict)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryConcurrentIdempotentClaimReturnsInProgressWithoutRawEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO event_idempotency_records").WithArgs(pgxmock.AnyArg(), "principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key", "fingerprint", pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM event_idempotency_records i").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace, "request-key").WillReturnRows(pgxmock.NewRows([]string{"request_fingerprint", "status", "event_id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at", "admission", "lease_until"}).AddRow("fingerprint", "pending", nil, nil, nil, nil, nil, nil, nil, nil, nil, time.Now().Add(time.Minute)))
	mock.ExpectRollback()

	_, err = NewRepository(mock).IngestEventIdempotent(context.Background(), memory.IdempotentEventIngestInput{PrincipalID: "principal_1", IdempotencyKey: "request-key", RequestFingerprint: "fingerprint"}, memory.IngestEventInput{Scope: scope, EventType: "conversation.message", Content: "hello"}, memory.ProvenanceRecord{}, memory.AdmissionPressureReport{})
	if err != memory.ErrIdempotencyInProgress {
		t.Fatalf("IngestEventIdempotent() error = %v, want %v", err, memory.ErrIdempotencyInProgress)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryIdempotentIngestRecoversExpiredLeaseWithoutNewClaim(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	sourceTime := time.Date(2026, 8, 16, 14, 0, 0, 0, time.UTC)
	input := memory.IngestEventInput{Scope: scope, EventType: "conversation.message", Content: "recovered", SourceTimestamp: sourceTime}
	provenance := memory.ProvenanceRecord{ID: "provenance_1", Scope: scope, Operation: "ingest_event", CreatedAt: sourceTime}
	claim := memory.IdempotentEventIngestInput{PrincipalID: "principal_1", IdempotencyKey: "request-key", RequestFingerprint: "fingerprint"}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO event_idempotency_records").WithArgs(pgxmock.AnyArg(), claim.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, claim.IdempotencyKey, claim.RequestFingerprint, pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("FROM event_idempotency_records i").WithArgs(claim.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, claim.IdempotencyKey).WillReturnRows(pgxmock.NewRows([]string{"request_fingerprint", "status", "event_id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at", "admission", "lease_until"}).AddRow(claim.RequestFingerprint, "pending", nil, nil, nil, nil, nil, nil, nil, nil, nil, time.Now().Add(-time.Minute)))
	mock.ExpectExec("UPDATE event_idempotency_records SET lease_until").WithArgs(pgxmock.AnyArg(), claim.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, claim.IdempotencyKey).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery("INSERT INTO raw_events").WithArgs(scope.Tenant, scope.Project, scope.Namespace, input.EventType, input.Content, pgxmock.AnyArg(), sourceTime).WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).AddRow("evt_123", scope.Tenant, scope.Project, scope.Namespace, input.EventType, input.Content, sourceTime, sourceTime))
	mock.ExpectExec("INSERT INTO provenance_links").WithArgs(provenance.ID, "evt_123", nil, nil, scope.Tenant, scope.Project, scope.Namespace, provenance.Operation, nil, nil, pgxmock.AnyArg(), provenance.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE event_idempotency_records").WithArgs("evt_123", pgxmock.AnyArg(), pgxmock.AnyArg(), claim.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, claim.IdempotencyKey).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	result, err := NewRepository(mock).IngestEventIdempotent(context.Background(), claim, input, provenance, memory.AdmissionPressureReport{})
	if err != nil || result.Replayed || result.Event.ID != "evt_123" {
		t.Fatalf("IngestEventIdempotent() = %+v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreatesPrincipalCredentialGrantAndAuditAtomically(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	principal := auth.Principal{ID: "principal_1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive, Label: "bootstrap", CreatedAt: now, UpdatedAt: now}
	credential := auth.Credential{ID: "credential_1", PrincipalID: principal.ID, Status: auth.CredentialStatusActive, CredentialID: "stl_credential_1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: now}
	grant := auth.ScopeGrant{ID: "grant_1", PrincipalID: principal.ID, Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Status: auth.ScopeGrantStatusActive, CreatedAt: now}
	audit := auth.AuditRecord{ID: "audit_1", PrincipalID: principal.ID, CredentialID: credential.ID, Action: "principal_created", Result: "success", CreatedAt: now}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO access_principals").WithArgs(principal.ID, string(principal.Role), string(principal.Status), principal.Label, nil, principal.CreatedAt, principal.UpdatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO access_credentials").WithArgs(credential.ID, credential.PrincipalID, string(credential.Status), credential.CredentialID, credential.Salt, credential.Digest, nil, credential.CreatedAt, nil).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO access_scope_grants").WithArgs(grant.ID, grant.PrincipalID, grant.Scope.Tenant, grant.Scope.Project, grant.Scope.Namespace, string(grant.Status), grant.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO access_audit_records").WithArgs(audit.ID, audit.PrincipalID, audit.CredentialID, nil, nil, nil, audit.Action, audit.Result, audit.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	if err := NewRepository(mock).CreatePrincipal(context.Background(), principal, credential, []auth.ScopeGrant{grant}, audit); err != nil {
		t.Fatalf("CreatePrincipal() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRotatesCredentialAndWritesAuditAtomically(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	credential := auth.Credential{ID: "credential_2", PrincipalID: "principal_1", Status: auth.CredentialStatusActive, CredentialID: "stl_credential_2", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: now}
	audit := auth.AuditRecord{ID: "audit_2", PrincipalID: "principal_1", CredentialID: credential.ID, Scope: scope, Action: "credential_rotated", Result: "success", CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM access_scope_grants").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"authorized"}).AddRow(true))
	mock.ExpectExec("UPDATE access_credentials").WithArgs("principal_1", now).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO access_credentials").WithArgs(credential.ID, credential.PrincipalID, string(credential.Status), credential.CredentialID, credential.Salt, credential.Digest, nil, credential.CreatedAt, nil).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO access_audit_records").WithArgs(audit.ID, audit.PrincipalID, audit.CredentialID, scope.Tenant, scope.Project, scope.Namespace, audit.Action, audit.Result, audit.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	if err := NewRepository(mock).RotateCredential(context.Background(), scope, "principal_1", credential, audit); err != nil {
		t.Fatalf("RotateCredential() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListsAccessAuditWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM access_audit_records").WithArgs(scope.Tenant, scope.Project, scope.Namespace, "principal_1", 10).WillReturnRows(pgxmock.NewRows([]string{"id", "principal_id", "credential_id", "tenant", "project", "namespace", "action", "result", "created_at"}).AddRow("audit_1", "principal_1", "credential_1", scope.Tenant, scope.Project, scope.Namespace, "credential_rotated", "success", now))

	records, err := NewRepository(mock).ListAccessAudit(context.Background(), scope, "principal_1", 10)
	if err != nil {
		t.Fatalf("ListAccessAudit() error = %v", err)
	}
	if len(records) != 1 || records[0].CredentialID != "credential_1" || records[0].Action != "credential_rotated" {
		t.Fatalf("ListAccessAudit() = %+v", records)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryDisablesPrincipalOnlyWithinGrantedScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 8, 16, 13, 0, 0, 0, time.UTC)
	audit := auth.AuditRecord{ID: "audit_3", PrincipalID: "principal_1", Scope: scope, Action: "principal_disabled", Result: "success", CreatedAt: now}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM access_scope_grants").WithArgs("principal_1", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"authorized"}).AddRow(true))
	mock.ExpectExec("UPDATE access_principals").WithArgs("principal_1", now).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("UPDATE access_credentials").WithArgs("principal_1", now).WillReturnResult(pgxmock.NewResult("UPDATE", 2))
	mock.ExpectExec("INSERT INTO access_audit_records").WithArgs(audit.ID, audit.PrincipalID, nil, scope.Tenant, scope.Project, scope.Namespace, audit.Action, audit.Result, audit.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	if err := NewRepository(mock).DisablePrincipal(context.Background(), scope, "principal_1", now, audit); err != nil {
		t.Fatalf("DisablePrincipal() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListsPrincipalsWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 8, 16, 13, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM access_principals p").WithArgs(scope.Tenant, scope.Project, scope.Namespace, 10).WillReturnRows(pgxmock.NewRows([]string{"id", "role", "status", "label", "expires_at", "created_at", "updated_at"}).AddRow("principal_1", string(auth.PrincipalRolePublic), string(auth.PrincipalStatusActive), "integration-a", nil, now, now))

	principals, err := NewRepository(mock).ListPrincipals(context.Background(), scope, 10)
	if err != nil {
		t.Fatalf("ListPrincipals() error = %v", err)
	}
	if len(principals) != 1 || principals[0].ID != "principal_1" || principals[0].Label != "integration-a" {
		t.Fatalf("ListPrincipals() = %+v", principals)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRevokesScopeGrantWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 8, 16, 15, 0, 0, 0, time.UTC)
	audit := auth.AuditRecord{ID: "audit_4", PrincipalID: "principal_1", Scope: scope, Action: "scope_grant_revoked", Result: "success", CreatedAt: now}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE access_scope_grants").WithArgs("grant_1", scope.Tenant, scope.Project, scope.Namespace, now).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO access_audit_records").WithArgs(audit.ID, audit.PrincipalID, nil, scope.Tenant, scope.Project, scope.Namespace, audit.Action, audit.Result, audit.CreatedAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	if err := NewRepository(mock).RevokeScopeGrant(context.Background(), scope, "grant_1", now, audit); err != nil {
		t.Fatalf("RevokeScopeGrant() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
