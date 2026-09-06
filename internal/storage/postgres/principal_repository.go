package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ReadPrincipalCredential(ctx context.Context, credentialID string) (auth.Principal, auth.Credential, error) {
	const query = `
SELECT p.id, p.role, p.status, p.label, p.expires_at, p.created_at, p.updated_at,
       c.id, c.credential_id, c.status, c.salt, c.digest, c.expires_at, c.created_at, c.disabled_at
FROM access_credentials c
JOIN access_principals p ON p.id = c.principal_id
WHERE c.credential_id = $1
`
	var principal auth.Principal
	var credential auth.Credential
	var principalExpires, credentialExpires, credentialDisabled sql.NullTime
	if err := r.db.QueryRow(ctx, query, credentialID).Scan(
		&principal.ID, &principal.Role, &principal.Status, &principal.Label, &principalExpires, &principal.CreatedAt, &principal.UpdatedAt,
		&credential.ID, &credential.CredentialID, &credential.Status, &credential.Salt, &credential.Digest, &credentialExpires, &credential.CreatedAt, &credentialDisabled,
	); err != nil {
		if err == pgx.ErrNoRows {
			return auth.Principal{}, auth.Credential{}, err
		}
		return auth.Principal{}, auth.Credential{}, fmt.Errorf("read principal credential: %w", err)
	}
	credential.PrincipalID = principal.ID
	if principalExpires.Valid {
		principal.ExpiresAt = principalExpires.Time
	}
	if credentialExpires.Valid {
		credential.ExpiresAt = credentialExpires.Time
	}
	if credentialDisabled.Valid {
		credential.DisabledAt = credentialDisabled.Time
	}
	return principal, credential, nil
}

func (r *Repository) HasActiveScopeGrant(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	const query = `
SELECT EXISTS(
    SELECT 1 FROM access_scope_grants
    WHERE principal_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4 AND status = 'active'
) AS authorized
`
	var authorized bool
	if err := r.db.QueryRow(ctx, query, principalID, scope.Tenant, scope.Project, scope.Namespace).Scan(&authorized); err != nil {
		return false, fmt.Errorf("check principal scope grant: %w", err)
	}
	return authorized, nil
}

func (r *Repository) HasActiveAdminPrincipal(ctx context.Context) (bool, error) {
	const query = `
SELECT EXISTS(
    SELECT 1 FROM access_principals
    WHERE role = $1 AND status = $2 AND (expires_at IS NULL OR expires_at > now())
) AS has_active_admin
`
	var hasAdmin bool
	if err := r.db.QueryRow(ctx, query, string(auth.PrincipalRoleAdmin), string(auth.PrincipalStatusActive)).Scan(&hasAdmin); err != nil {
		return false, fmt.Errorf("check active admin principal: %w", err)
	}
	return hasAdmin, nil
}

func (r *Repository) IngestEventIdempotent(ctx context.Context, claim memory.IdempotentEventIngestInput, input memory.IngestEventInput, provenance memory.ProvenanceRecord, admission memory.AdmissionPressureReport) (memory.IdempotentEventIngestResult, error) {
	if err := claim.Validate(); err != nil {
		return memory.IdempotentEventIngestResult{}, err
	}
	if err := input.Validate(); err != nil {
		return memory.IdempotentEventIngestResult{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.IdempotentEventIngestResult{}, fmt.Errorf("begin idempotent ingest transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	const claimQuery = `
INSERT INTO event_idempotency_records (
    id, principal_id, tenant, project, namespace, idempotency_key, request_fingerprint, status, lease_until
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
ON CONFLICT (principal_id, tenant, project, namespace, idempotency_key) DO NOTHING
RETURNING id
`
	var claimID string
	newClaim := true
	if err := tx.QueryRow(ctx, claimQuery, uuid.NewString(), claim.PrincipalID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, claim.IdempotencyKey, claim.RequestFingerprint, now.Add(time.Minute)).Scan(&claimID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return memory.IdempotentEventIngestResult{}, fmt.Errorf("create idempotency claim: %w", err)
		}
		newClaim = false
	}

	if !newClaim {
		const existingQuery = `
SELECT i.request_fingerprint, i.status, e.id, e.tenant, e.project, e.namespace, e.event_type, e.content, e.source_timestamp, e.created_at, i.admission, i.lease_until
FROM event_idempotency_records i
LEFT JOIN raw_events e ON e.id = i.raw_event_id
WHERE i.principal_id = $1 AND i.tenant = $2 AND i.project = $3 AND i.namespace = $4 AND i.idempotency_key = $5
FOR UPDATE OF i
`
		var fingerprint, status string
		var event memory.RawEvent
		var eventID sql.NullString
		var sourceTimestamp, createdAt sql.NullTime
		var admissionRaw []byte
		var leaseUntil sql.NullTime
		if err := tx.QueryRow(ctx, existingQuery, claim.PrincipalID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, claim.IdempotencyKey).Scan(
			&fingerprint, &status, &eventID, &event.Scope.Tenant, &event.Scope.Project, &event.Scope.Namespace, &event.EventType, &event.Content, &sourceTimestamp, &createdAt, &admissionRaw, &leaseUntil,
		); err != nil {
			return memory.IdempotentEventIngestResult{}, fmt.Errorf("read idempotency claim: %w", err)
		}
		if fingerprint != claim.RequestFingerprint {
			return memory.IdempotentEventIngestResult{}, memory.ErrIdempotencyConflict
		}
		if status == "completed" && eventID.Valid {
			event.ID = eventID.String
			if sourceTimestamp.Valid {
				event.SourceTimestamp = sourceTimestamp.Time
			}
			if createdAt.Valid {
				event.CreatedAt = createdAt.Time
			}
			if len(admissionRaw) > 0 && string(admissionRaw) != "null" {
				var original memory.AdmissionPressureReport
				if err := json.Unmarshal(admissionRaw, &original); err != nil {
					return memory.IdempotentEventIngestResult{}, fmt.Errorf("decode replay admission: %w", err)
				}
				event.Admission = &original
			}
			if err := tx.Commit(ctx); err != nil {
				return memory.IdempotentEventIngestResult{}, fmt.Errorf("commit idempotency replay: %w", err)
			}
			return memory.IdempotentEventIngestResult{Event: event, Replayed: true}, nil
		}
		if status == "pending" && leaseUntil.Valid && leaseUntil.Time.After(now) {
			return memory.IdempotentEventIngestResult{}, memory.ErrIdempotencyInProgress
		}
		const recoverQuery = `UPDATE event_idempotency_records SET lease_until = $1 WHERE principal_id = $2 AND tenant = $3 AND project = $4 AND namespace = $5 AND idempotency_key = $6`
		if _, err := tx.Exec(ctx, recoverQuery, now.Add(time.Minute), claim.PrincipalID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, claim.IdempotencyKey); err != nil {
			return memory.IdempotentEventIngestResult{}, fmt.Errorf("recover idempotency claim: %w", err)
		}
	}

	event, err := writeRawEvent(ctx, tx, input)
	if err != nil {
		return memory.IdempotentEventIngestResult{}, err
	}
	provenance.RawEventID = event.ID
	if err := writeProvenance(ctx, tx, provenance); err != nil {
		return memory.IdempotentEventIngestResult{}, err
	}
	admissionRaw, err := json.Marshal(admission)
	if err != nil {
		return memory.IdempotentEventIngestResult{}, fmt.Errorf("encode admission result: %w", err)
	}
	const completeQuery = `
UPDATE event_idempotency_records
SET status = 'completed', raw_event_id = $1, admission = $2, lease_until = NULL, completed_at = $3
WHERE principal_id = $4 AND tenant = $5 AND project = $6 AND namespace = $7 AND idempotency_key = $8
`
	if _, err := tx.Exec(ctx, completeQuery, event.ID, admissionRaw, now, claim.PrincipalID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, claim.IdempotencyKey); err != nil {
		return memory.IdempotentEventIngestResult{}, fmt.Errorf("complete idempotency claim: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.IdempotentEventIngestResult{}, fmt.Errorf("commit idempotent ingest transaction: %w", err)
	}
	return memory.IdempotentEventIngestResult{Event: event}, nil
}

func (r *Repository) CreatePrincipal(ctx context.Context, principal auth.Principal, credential auth.Credential, grants []auth.ScopeGrant, audit auth.AuditRecord) error {
	if err := principal.Validate(); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	if len(grants) == 0 {
		return fmt.Errorf("at least one scope grant is required")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin create principal transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	const principalQuery = `INSERT INTO access_principals (id, role, status, label, expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.Exec(ctx, principalQuery, principal.ID, string(principal.Role), string(principal.Status), principal.Label, nullableTime(principal.ExpiresAt), principal.CreatedAt, principal.UpdatedAt); err != nil {
		return fmt.Errorf("insert principal: %w", err)
	}
	const credentialQuery = `INSERT INTO access_credentials (id, principal_id, status, credential_id, salt, digest, expires_at, created_at, disabled_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := tx.Exec(ctx, credentialQuery, credential.ID, credential.PrincipalID, string(credential.Status), credential.CredentialID, credential.Salt, credential.Digest, nullableTime(credential.ExpiresAt), credential.CreatedAt, nullableTime(credential.DisabledAt)); err != nil {
		return fmt.Errorf("insert credential: %w", err)
	}
	const grantQuery = `INSERT INTO access_scope_grants (id, principal_id, tenant, project, namespace, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, grant := range grants {
		if err := grant.Validate(); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, grantQuery, grant.ID, grant.PrincipalID, grant.Scope.Tenant, grant.Scope.Project, grant.Scope.Namespace, string(grant.Status), grant.CreatedAt); err != nil {
			return fmt.Errorf("insert scope grant: %w", err)
		}
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create principal transaction: %w", err)
	}
	return nil
}

func (r *Repository) RotateCredential(ctx context.Context, scope memory.Scope, principalID string, credential auth.Credential, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	if credential.PrincipalID != principalID {
		return fmt.Errorf("credential principal id does not match rotation principal")
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin credential rotation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureActivePrincipalGrant(ctx, tx, principalID, scope); err != nil {
		return err
	}

	const revokeQuery = `
UPDATE access_credentials
SET status = 'revoked', disabled_at = $2
WHERE principal_id = $1 AND status = 'active'
`
	if _, err := tx.Exec(ctx, revokeQuery, principalID, credential.CreatedAt); err != nil {
		return fmt.Errorf("revoke prior credentials: %w", err)
	}

	const credentialQuery = `INSERT INTO access_credentials (id, principal_id, status, credential_id, salt, digest, expires_at, created_at, disabled_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := tx.Exec(ctx, credentialQuery, credential.ID, credential.PrincipalID, string(credential.Status), credential.CredentialID, credential.Salt, credential.Digest, nullableTime(credential.ExpiresAt), credential.CreatedAt, nullableTime(credential.DisabledAt)); err != nil {
		return fmt.Errorf("insert rotated credential: %w", err)
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit credential rotation transaction: %w", err)
	}
	return nil
}

func (r *Repository) DisablePrincipal(ctx context.Context, scope memory.Scope, principalID string, disabledAt time.Time, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if principalID == "" || disabledAt.IsZero() {
		return fmt.Errorf("principal id and disabled at are required")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin principal disable transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := ensureActivePrincipalGrant(ctx, tx, principalID, scope); err != nil {
		return err
	}
	const disablePrincipalQuery = `UPDATE access_principals SET status = 'disabled', disabled_at = $2, updated_at = $2 WHERE id = $1`
	if _, err := tx.Exec(ctx, disablePrincipalQuery, principalID, disabledAt.UTC()); err != nil {
		return fmt.Errorf("disable principal: %w", err)
	}
	const disableCredentialsQuery = `UPDATE access_credentials SET status = 'disabled', disabled_at = $2 WHERE principal_id = $1 AND status = 'active'`
	if _, err := tx.Exec(ctx, disableCredentialsQuery, principalID, disabledAt.UTC()); err != nil {
		return fmt.Errorf("disable principal credentials: %w", err)
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit principal disable transaction: %w", err)
	}
	return nil
}

func (r *Repository) ExpirePrincipal(ctx context.Context, scope memory.Scope, principalID string, expiresAt time.Time, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if principalID == "" || expiresAt.IsZero() {
		return fmt.Errorf("principal id and expires at are required")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin principal expiry transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := ensureActivePrincipalGrant(ctx, tx, principalID, scope); err != nil {
		return err
	}
	const expirePrincipalQuery = `UPDATE access_principals SET expires_at = $2, updated_at = $3 WHERE id = $1`
	if _, err := tx.Exec(ctx, expirePrincipalQuery, principalID, expiresAt.UTC(), audit.CreatedAt.UTC()); err != nil {
		return fmt.Errorf("expire principal: %w", err)
	}
	const expireCredentialsQuery = `UPDATE access_credentials SET expires_at = $2 WHERE principal_id = $1 AND status = 'active'`
	if _, err := tx.Exec(ctx, expireCredentialsQuery, principalID, expiresAt.UTC()); err != nil {
		return fmt.Errorf("expire principal credentials: %w", err)
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit principal expiry transaction: %w", err)
	}
	return nil
}

func (r *Repository) ListPrincipals(ctx context.Context, scope memory.Scope, limit int) ([]auth.Principal, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("principal limit must be between 1 and 100")
	}
	const query = `
SELECT p.id, p.role, p.status, p.label, p.expires_at, p.created_at, p.updated_at
FROM access_principals p
WHERE EXISTS (
    SELECT 1 FROM access_scope_grants g
    WHERE g.principal_id = p.id AND g.tenant = $1 AND g.project = $2 AND g.namespace = $3
)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list principals: %w", err)
	}
	defer rows.Close()
	principals := make([]auth.Principal, 0)
	for rows.Next() {
		principal, err := scanAccessPrincipal(rows)
		if err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate principals: %w", err)
	}
	return principals, nil
}

func (r *Repository) ReadPrincipal(ctx context.Context, scope memory.Scope, principalID string) (auth.Principal, error) {
	if err := scope.Validate(); err != nil {
		return auth.Principal{}, err
	}
	const query = `
SELECT p.id, p.role, p.status, p.label, p.expires_at, p.created_at, p.updated_at
FROM access_principals p
WHERE p.id = $1 AND EXISTS (
    SELECT 1 FROM access_scope_grants g
    WHERE g.principal_id = p.id AND g.tenant = $2 AND g.project = $3 AND g.namespace = $4
)
`
	principal, err := scanAccessPrincipal(r.db.QueryRow(ctx, query, principalID, scope.Tenant, scope.Project, scope.Namespace))
	if err != nil {
		return auth.Principal{}, fmt.Errorf("read scoped principal: %w", err)
	}
	return principal, nil
}

func (r *Repository) ListScopeGrants(ctx context.Context, scope memory.Scope, principalID string) ([]auth.ScopeGrant, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, principal_id, tenant, project, namespace, status, created_at, revoked_at
FROM access_scope_grants
WHERE principal_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
ORDER BY created_at DESC, id DESC
`
	rows, err := r.db.Query(ctx, query, principalID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list scope grants: %w", err)
	}
	defer rows.Close()
	grants := make([]auth.ScopeGrant, 0)
	for rows.Next() {
		grant, err := scanAccessScopeGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scope grants: %w", err)
	}
	return grants, nil
}

func (r *Repository) CreateScopeGrant(ctx context.Context, scope memory.Scope, grant auth.ScopeGrant, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := grant.Validate(); err != nil {
		return err
	}
	if grant.Scope != scope {
		return fmt.Errorf("scope grant must match authorized scope")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin scope grant transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := ensureActivePrincipalGrant(ctx, tx, grant.PrincipalID, scope); err != nil {
		return err
	}
	const query = `INSERT INTO access_scope_grants (id, principal_id, tenant, project, namespace, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.Exec(ctx, query, grant.ID, grant.PrincipalID, grant.Scope.Tenant, grant.Scope.Project, grant.Scope.Namespace, string(grant.Status), grant.CreatedAt); err != nil {
		return fmt.Errorf("insert scope grant: %w", err)
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit scope grant transaction: %w", err)
	}
	return nil
}

func (r *Repository) RevokeScopeGrant(ctx context.Context, scope memory.Scope, grantID string, revokedAt time.Time, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if grantID == "" || revokedAt.IsZero() {
		return fmt.Errorf("scope grant id and revoked at are required")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin scope grant revoke transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	const query = `
UPDATE access_scope_grants
SET status = 'revoked', revoked_at = $5
WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4 AND status = 'active'
`
	result, err := tx.Exec(ctx, query, grantID, scope.Tenant, scope.Project, scope.Namespace, revokedAt.UTC())
	if err != nil {
		return fmt.Errorf("revoke scope grant: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := writeAccessAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit scope grant revoke transaction: %w", err)
	}
	return nil
}

func (r *Repository) ListAccessAudit(ctx context.Context, scope memory.Scope, principalID string, limit int) ([]auth.AuditRecord, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("audit limit must be between 1 and 100")
	}

	const query = `
SELECT id, principal_id, credential_id, tenant, project, namespace, action, result, created_at
FROM access_audit_records
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND ($4 = '' OR principal_id = $4)
ORDER BY created_at DESC, id DESC
LIMIT $5
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, principalID, limit)
	if err != nil {
		return nil, fmt.Errorf("list access audit: %w", err)
	}
	defer rows.Close()

	records := make([]auth.AuditRecord, 0)
	for rows.Next() {
		record, err := scanAccessAudit(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate access audit: %w", err)
	}
	return records, nil
}

func ensureActivePrincipalGrant(ctx context.Context, db queryRower, principalID string, scope memory.Scope) error {
	const query = `
SELECT EXISTS(
    SELECT 1 FROM access_scope_grants
    WHERE principal_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4 AND status = 'active'
) AS authorized
`
	var authorized bool
	if err := db.QueryRow(ctx, query, principalID, scope.Tenant, scope.Project, scope.Namespace).Scan(&authorized); err != nil {
		return fmt.Errorf("check principal scope grant for lifecycle action: %w", err)
	}
	if !authorized {
		return pgx.ErrNoRows
	}
	return nil
}

func scanAccessAudit(row interface{ Scan(dest ...any) error }) (auth.AuditRecord, error) {
	var record auth.AuditRecord
	var principalID, credentialID, tenant, project, namespace sql.NullString
	if err := row.Scan(&record.ID, &principalID, &credentialID, &tenant, &project, &namespace, &record.Action, &record.Result, &record.CreatedAt); err != nil {
		return auth.AuditRecord{}, fmt.Errorf("scan access audit: %w", err)
	}
	record.PrincipalID = principalID.String
	record.CredentialID = credentialID.String
	record.Scope = memory.Scope{Tenant: tenant.String, Project: project.String, Namespace: namespace.String}
	return record, nil
}

func scanAccessPrincipal(row interface{ Scan(dest ...any) error }) (auth.Principal, error) {
	var principal auth.Principal
	var expiresAt sql.NullTime
	if err := row.Scan(&principal.ID, &principal.Role, &principal.Status, &principal.Label, &expiresAt, &principal.CreatedAt, &principal.UpdatedAt); err != nil {
		return auth.Principal{}, fmt.Errorf("scan principal: %w", err)
	}
	if expiresAt.Valid {
		principal.ExpiresAt = expiresAt.Time
	}
	return principal, nil
}

func scanAccessScopeGrant(row interface{ Scan(dest ...any) error }) (auth.ScopeGrant, error) {
	var grant auth.ScopeGrant
	var revokedAt sql.NullTime
	if err := row.Scan(&grant.ID, &grant.PrincipalID, &grant.Scope.Tenant, &grant.Scope.Project, &grant.Scope.Namespace, &grant.Status, &grant.CreatedAt, &revokedAt); err != nil {
		return auth.ScopeGrant{}, fmt.Errorf("scan scope grant: %w", err)
	}
	if revokedAt.Valid {
		grant.RevokedAt = revokedAt.Time
	}
	return grant, nil
}

func writeAccessAudit(ctx context.Context, db queryRower, audit auth.AuditRecord) error {
	const query = `INSERT INTO access_audit_records (id, principal_id, credential_id, tenant, project, namespace, action, result, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := db.Exec(ctx, query, audit.ID, nullableString(audit.PrincipalID), nullableString(audit.CredentialID), nullableString(audit.Scope.Tenant), nullableString(audit.Scope.Project), nullableString(audit.Scope.Namespace), audit.Action, audit.Result, audit.CreatedAt); err != nil {
		return fmt.Errorf("insert access audit: %w", err)
	}
	return nil
}
