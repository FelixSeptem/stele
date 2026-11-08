package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) AppendProceduralMemoryVerification(ctx context.Context, record memory.ProceduralMemoryVerification) (memory.ProceduralMemoryVerification, error) {
	if err := record.Validate(); err != nil {
		return memory.ProceduralMemoryVerification{}, err
	}
	fingerprint, err := memory.ProceduralMemoryVerificationFingerprint(record)
	if err != nil {
		return memory.ProceduralMemoryVerification{}, err
	}
	evidence := append([]memory.ProceduralVerificationEvidence(nil), record.Evidence...)
	for index := range evidence {
		if evidence[index].Scope == (memory.Scope{}) {
			evidence[index].Scope = record.Scope.Normalized()
		}
	}
	encodedEvidence, err := json.Marshal(evidence)
	if err != nil {
		return memory.ProceduralMemoryVerification{}, fmt.Errorf("marshal procedural verification evidence: %w", err)
	}
	const query = `
INSERT INTO procedural_memory_verifications (
    id, tenant, project, namespace, memory_id, target_version, fingerprint,
    claim_fingerprint, applicability_fingerprint, disposition, evidence,
    source_watermark, policy_version, actor, reason, corrected_content, created_at
)
VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $15, $16, $17)
ON CONFLICT (tenant, project, namespace, fingerprint) DO NOTHING
RETURNING id::text, tenant, project, namespace, memory_id::text, target_version, fingerprint,
    claim_fingerprint, applicability_fingerprint, disposition, evidence,
    source_watermark, policy_version, actor, reason, corrected_content, created_at
`
	stored, err := scanProceduralMemoryVerification(r.queryer(ctx).QueryRow(ctx, query,
		record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace,
		record.MemoryID, record.TargetVersion, fingerprint, record.ClaimFingerprint,
		record.ApplicabilityFingerprint, record.Disposition, encodedEvidence, record.SourceWatermark,
		record.PolicyVersion, record.Actor, record.Reason, nullableString(record.CorrectedContent), record.CreatedAt,
	))
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return memory.ProceduralMemoryVerification{}, fmt.Errorf("append procedural verification: %w", err)
	}
	// The fingerprint is server-derived. A conflict can therefore only be a
	// replay of the same immutable assertion, never a caller-selected reuse.
	return r.readProceduralMemoryVerificationByFingerprint(ctx, record.Scope, fingerprint)
}

// ValidateProceduralVerificationTarget is the admission-time proof that the
// target is still an active, current procedural version in the asserted scope.
// It deliberately returns the same not-found category for foreign, hidden,
// stale, and nonprocedural targets.
func (r *Repository) ValidateProceduralVerificationTarget(ctx context.Context, record memory.ProceduralMemoryVerification) error {
	const query = `
SELECT EXISTS (
    SELECT 1
    FROM canonical_memories m
    JOIN memory_versions v ON v.memory_id = m.id AND v.version = $5
    WHERE m.tenant = $1 AND m.project = $2 AND m.namespace = $3 AND m.id = $4::uuid
      AND m.class = 'procedural' AND m.state = 'active'
      AND NOT EXISTS (SELECT 1 FROM memory_versions successor WHERE successor.memory_id = m.id AND successor.version > v.version)
) AS eligible
`
	var eligible bool
	if err := r.queryer(ctx).QueryRow(ctx, query, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryID, record.TargetVersion).Scan(&eligible); err != nil {
		return fmt.Errorf("resolve procedural verification target: %w", err)
	}
	if !eligible {
		return memory.ErrNotFound
	}
	return nil
}

// ValidateProceduralVerificationEvidence proves a durable, exact-scope source
// remains eligible. It never searches for replacements: the kind, identifier,
// and observed version recorded in the immutable evidence row are its entire
// authority.
func (r *Repository) ValidateProceduralVerificationEvidence(ctx context.Context, scope memory.Scope, evidence memory.ProceduralVerificationEvidence) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := evidence.Validate(scope); err != nil {
		return err
	}
	var query string
	args := []any{scope.Tenant, scope.Project, scope.Namespace, evidence.ID}
	switch evidence.SourceKind {
	case memory.ProceduralVerificationEvidenceSourceRawEvent:
		// Raw events are immutable. Version one denotes the only durable source
		// revision and makes drift detectable if this contract later evolves.
		if evidence.SourceVersion != 1 {
			return memory.ErrNotFound
		}
		query = `SELECT EXISTS (SELECT 1 FROM raw_events WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid) AS eligible`
	case memory.ProceduralVerificationEvidenceSourceMemoryVersion:
		query = `SELECT EXISTS (
            SELECT 1 FROM memory_versions v JOIN canonical_memories m ON m.id = v.memory_id
            WHERE m.tenant = $1 AND m.project = $2 AND m.namespace = $3
              AND v.id = $4::uuid AND v.version = $5 AND m.state = 'active'
        ) AS eligible`
		args = append(args, evidence.SourceVersion)
	case memory.ProceduralVerificationEvidenceSourceUsefulnessFeedback:
		if evidence.SourceVersion != 1 {
			return memory.ErrNotFound
		}
		query = `SELECT EXISTS (SELECT 1 FROM usefulness_feedback WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4 AND superseded_at IS NULL) AS eligible`
	case memory.ProceduralVerificationEvidenceSourceExternalEvaluation:
		if evidence.SourceVersion != 1 {
			return memory.ErrNotFound
		}
		query = `SELECT EXISTS (SELECT 1 FROM task_evaluations WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid AND correction_state = 'active') AS eligible`
	default:
		return memory.ErrNotFound
	}
	var eligible bool
	if err := r.queryer(ctx).QueryRow(ctx, query, args...).Scan(&eligible); err != nil {
		return fmt.Errorf("resolve procedural verification evidence: %w", err)
	}
	if !eligible {
		return memory.ErrNotFound
	}
	return nil
}

func (r *Repository) ReadProceduralMemoryVerification(ctx context.Context, scope memory.Scope, id string) (memory.ProceduralMemoryVerification, error) {
	if err := scope.Validate(); err != nil {
		return memory.ProceduralMemoryVerification{}, err
	}
	const query = `
SELECT id::text, tenant, project, namespace, memory_id::text, target_version, fingerprint,
    claim_fingerprint, applicability_fingerprint, disposition, evidence,
    source_watermark, policy_version, actor, reason, corrected_content, created_at
FROM procedural_memory_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid
`
	record, err := scanProceduralMemoryVerification(r.queryer(ctx).QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return memory.ProceduralMemoryVerification{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.ProceduralMemoryVerification{}, fmt.Errorf("read procedural verification: %w", err)
	}
	return record, nil
}

func (r *Repository) ListProceduralMemoryVerificationHistory(ctx context.Context, scope memory.Scope, memoryID string, targetVersion int64, limit int) ([]memory.ProceduralMemoryVerification, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if memoryID == "" || targetVersion <= 0 || limit <= 0 || limit > 100 {
		return nil, fmt.Errorf("procedural verification history input is invalid")
	}
	const query = `
SELECT id::text, tenant, project, namespace, memory_id::text, target_version, fingerprint,
    claim_fingerprint, applicability_fingerprint, disposition, evidence,
    source_watermark, policy_version, actor, reason, corrected_content, created_at
FROM procedural_memory_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND memory_id = $4::uuid AND target_version = $5
ORDER BY created_at DESC, id DESC
LIMIT $6
`
	rows, err := r.queryer(ctx).Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memoryID, targetVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("list procedural verification history: %w", err)
	}
	defer rows.Close()
	history := make([]memory.ProceduralMemoryVerification, 0, limit)
	for rows.Next() {
		record, err := scanProceduralMemoryVerification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan procedural verification history: %w", err)
		}
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate procedural verification history: %w", err)
	}
	return history, nil
}

// ReadProceduralVerificationLineage is deliberately separate from ordinary
// canonical-memory history. Its caller is the privileged admin boundary that
// has already proved scope authority; this repository method reinforces that
// scope on each ledger and run query.
func (r *Repository) ReadProceduralVerificationLineage(ctx context.Context, scope memory.Scope, memoryID string, targetVersion int64) (memory.ProceduralVerificationLineage, error) {
	if err := scope.Validate(); err != nil {
		return memory.ProceduralVerificationLineage{}, err
	}
	if memoryID == "" || targetVersion <= 0 {
		return memory.ProceduralVerificationLineage{}, fmt.Errorf("procedural verification lineage input is invalid")
	}
	verifications, err := r.ListProceduralMemoryVerificationHistory(ctx, scope, memoryID, targetVersion, 100)
	if err != nil {
		return memory.ProceduralVerificationLineage{}, err
	}
	const query = `
SELECT id::text, tenant, project, namespace, verification_id::text,
    memory_id::text, target_version, source_watermark, trigger, work_identity,
    status, attempt_count, max_attempts, lease_owner, lease_expires_at,
    next_attempt_at, terminal_verification_id::text, failure_category,
    created_at, updated_at, terminal_at
FROM procedural_memory_revalidation_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND memory_id = $4::uuid AND target_version = $5
ORDER BY created_at DESC, id DESC
LIMIT $6
`
	rows, err := r.queryer(ctx).Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memoryID, targetVersion, 100)
	if err != nil {
		return memory.ProceduralVerificationLineage{}, fmt.Errorf("read procedural revalidation lineage: %w", err)
	}
	defer rows.Close()
	lineage := memory.ProceduralVerificationLineage{Scope: scope, MemoryID: memoryID, TargetVersion: targetVersion, Verifications: verifications}
	for rows.Next() {
		run, err := scanProceduralRevalidationRun(rows)
		if err != nil {
			return memory.ProceduralVerificationLineage{}, fmt.Errorf("scan procedural revalidation lineage: %w", err)
		}
		lineage.Runs = append(lineage.Runs, run)
	}
	if err := rows.Err(); err != nil {
		return memory.ProceduralVerificationLineage{}, fmt.Errorf("iterate procedural revalidation lineage: %w", err)
	}
	if err := lineage.Validate(); err != nil {
		return memory.ProceduralVerificationLineage{}, err
	}
	return lineage, nil
}

func (r *Repository) readProceduralMemoryVerificationByFingerprint(ctx context.Context, scope memory.Scope, fingerprint string) (memory.ProceduralMemoryVerification, error) {
	const query = `
SELECT id::text, tenant, project, namespace, memory_id::text, target_version, fingerprint,
    claim_fingerprint, applicability_fingerprint, disposition, evidence,
    source_watermark, policy_version, actor, reason, corrected_content, created_at
FROM procedural_memory_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND fingerprint = $4
`
	record, err := scanProceduralMemoryVerification(r.queryer(ctx).QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, fingerprint))
	if err != nil {
		return memory.ProceduralMemoryVerification{}, fmt.Errorf("read replayed procedural verification: %w", err)
	}
	return record, nil
}

// ClaimProceduralRevalidationRuns atomically claims pending work or work whose
// former lease expired. The query contains scope predicates in both selection
// and update paths so an ID can never act as cross-scope authority.
func (r *Repository) ClaimProceduralRevalidationRuns(ctx context.Context, input memory.ClaimProceduralRevalidationRunsInput) ([]memory.ProceduralRevalidationRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH claimed AS (
    SELECT id
    FROM procedural_memory_revalidation_runs
    WHERE tenant = $1 AND project = $2 AND namespace = $3
      AND status IN ('pending', 'running')
      AND next_attempt_at <= $5
      AND (lease_expires_at IS NULL OR lease_expires_at <= $5)
    ORDER BY created_at ASC, id ASC
    LIMIT $7
    FOR UPDATE SKIP LOCKED
)
UPDATE procedural_memory_revalidation_runs r
SET status = 'running', lease_owner = $4, lease_expires_at = $6,
    attempt_count = r.attempt_count + 1, updated_at = $5
FROM claimed
WHERE r.id = claimed.id
  AND r.tenant = $1 AND r.project = $2 AND r.namespace = $3
RETURNING r.id::text, r.tenant, r.project, r.namespace, r.verification_id::text,
    r.memory_id::text, r.target_version, r.source_watermark, r.trigger,
    r.work_identity, r.status, r.attempt_count, r.max_attempts, r.lease_owner,
    r.lease_expires_at, r.next_attempt_at, r.terminal_verification_id::text,
    r.failure_category, r.created_at, r.updated_at, r.terminal_at
`
	rows, err := r.queryer(ctx).Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.Now.UTC(), input.Now.UTC().Add(input.LeaseDuration), input.Limit)
	if err != nil {
		return nil, fmt.Errorf("claim procedural revalidation runs: %w", err)
	}
	defer rows.Close()
	runs := make([]memory.ProceduralRevalidationRun, 0, input.Limit)
	for rows.Next() {
		run, err := scanProceduralRevalidationRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed procedural revalidation runs: %w", err)
	}
	return runs, nil
}

// ClaimProceduralRevalidationRun binds a delivered derived-work reference to a
// lease on exactly one durable run. A redelivery while the lease is live fails
// closed; an expired lease is safely reclaimable after process restart.
func (r *Repository) ClaimProceduralRevalidationRun(ctx context.Context, input memory.ClaimProceduralRevalidationRunInput) (memory.ProceduralRevalidationRun, error) {
	if err := input.Validate(); err != nil {
		return memory.ProceduralRevalidationRun{}, err
	}
	const query = `
UPDATE procedural_memory_revalidation_runs
SET status = 'running', lease_owner = $5, lease_expires_at = $7,
    attempt_count = attempt_count + 1, updated_at = $6
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid
  AND status IN ('pending', 'running')
  AND next_attempt_at <= $6
  AND (lease_expires_at IS NULL OR lease_expires_at <= $6)
RETURNING id::text, tenant, project, namespace, verification_id::text,
    memory_id::text, target_version, source_watermark, trigger, work_identity,
    status, attempt_count, max_attempts, lease_owner, lease_expires_at,
    next_attempt_at, terminal_verification_id::text, failure_category,
    created_at, updated_at, terminal_at
`
	run, err := scanProceduralRevalidationRun(r.queryer(ctx).QueryRow(ctx, query,
		input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID, input.WorkerID,
		input.Now.UTC(), input.Now.UTC().Add(input.LeaseDuration),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return memory.ProceduralRevalidationRun{}, memory.ErrProceduralRevalidationLeaseLost
	}
	if err != nil {
		return memory.ProceduralRevalidationRun{}, fmt.Errorf("claim procedural revalidation run: %w", err)
	}
	return run, nil
}

func (r *Repository) CreateProceduralRevalidationRun(ctx context.Context, run memory.ProceduralRevalidationRun) (memory.ProceduralRevalidationRun, error) {
	if err := run.Validate(); err != nil {
		return memory.ProceduralRevalidationRun{}, err
	}
	nextAttemptAt := run.NextAttemptAt
	if nextAttemptAt.IsZero() {
		nextAttemptAt = run.CreatedAt
	}
	updatedAt := run.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = run.CreatedAt
	}
	const query = `
INSERT INTO procedural_memory_revalidation_runs (
    id, tenant, project, namespace, verification_id, memory_id, target_version,
    source_watermark, trigger, work_identity, status, max_attempts,
    next_attempt_at, created_at, updated_at
)
VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6::uuid, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (tenant, project, namespace, work_identity) DO UPDATE
SET work_identity = procedural_memory_revalidation_runs.work_identity
RETURNING id::text, tenant, project, namespace, verification_id::text,
    memory_id::text, target_version, source_watermark, trigger, work_identity,
    status, attempt_count, max_attempts, lease_owner, lease_expires_at,
    next_attempt_at, terminal_verification_id::text, failure_category,
    created_at, updated_at, terminal_at
`
	stored, err := scanProceduralRevalidationRun(r.queryer(ctx).QueryRow(ctx, query,
		run.ID, run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace, run.VerificationID,
		run.MemoryID, run.TargetVersion, run.SourceWatermark, run.Trigger, run.WorkIdentity,
		run.Status, run.MaxAttempts, nextAttemptAt, run.CreatedAt, updatedAt,
	))
	if err != nil {
		return memory.ProceduralRevalidationRun{}, fmt.Errorf("create procedural revalidation run: %w", err)
	}
	return stored, nil
}

func (r *Repository) ReadProceduralRevalidationRun(ctx context.Context, scope memory.Scope, runID string) (memory.ProceduralRevalidationRun, error) {
	if err := scope.Validate(); err != nil {
		return memory.ProceduralRevalidationRun{}, err
	}
	if runID == "" {
		return memory.ProceduralRevalidationRun{}, fmt.Errorf("procedural revalidation run id is required")
	}
	const query = `
SELECT id::text, tenant, project, namespace, verification_id::text,
    memory_id::text, target_version, source_watermark, trigger, work_identity,
    status, attempt_count, max_attempts, lease_owner, lease_expires_at,
    next_attempt_at, terminal_verification_id::text, failure_category,
    created_at, updated_at, terminal_at
FROM procedural_memory_revalidation_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid
`
	run, err := scanProceduralRevalidationRun(r.queryer(ctx).QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return memory.ProceduralRevalidationRun{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.ProceduralRevalidationRun{}, fmt.Errorf("read procedural revalidation run: %w", err)
	}
	return run, nil
}

func (r *Repository) CompleteProceduralRevalidationRun(ctx context.Context, input memory.CompleteProceduralRevalidationRunInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE procedural_memory_revalidation_runs
SET status = $6, terminal_verification_id = $7::uuid, terminal_at = $8,
    updated_at = $8, lease_owner = NULL, lease_expires_at = NULL
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4::uuid
  AND lease_owner = $5 AND status = 'running'
`
	tag, err := r.queryer(ctx).Exec(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID, input.WorkerID, input.Status, input.TerminalVerificationID, input.CompletedAt.UTC())
	if err != nil {
		return fmt.Errorf("complete procedural revalidation run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return memory.ErrProceduralRevalidationLeaseLost
	}
	return nil
}

type proceduralVerificationScanner interface{ Scan(...any) error }

func scanProceduralMemoryVerification(scanner proceduralVerificationScanner) (memory.ProceduralMemoryVerification, error) {
	var record memory.ProceduralMemoryVerification
	var fingerprint string
	var evidence []byte
	var corrected sql.NullString
	if err := scanner.Scan(
		&record.ID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace,
		&record.MemoryID, &record.TargetVersion, &fingerprint, &record.ClaimFingerprint,
		&record.ApplicabilityFingerprint, &record.Disposition, &evidence, &record.SourceWatermark,
		&record.PolicyVersion, &record.Actor, &record.Reason, &corrected, &record.CreatedAt,
	); err != nil {
		return memory.ProceduralMemoryVerification{}, err
	}
	if err := json.Unmarshal(evidence, &record.Evidence); err != nil {
		return memory.ProceduralMemoryVerification{}, fmt.Errorf("unmarshal procedural verification evidence: %w", err)
	}
	if corrected.Valid {
		record.CorrectedContent = corrected.String
	}
	return record, nil
}

func scanProceduralRevalidationRun(scanner proceduralVerificationScanner) (memory.ProceduralRevalidationRun, error) {
	var run memory.ProceduralRevalidationRun
	var leaseOwner, terminalVerificationID, failureCategory sql.NullString
	var leaseExpiresAt, terminalAt sql.NullTime
	if err := scanner.Scan(
		&run.ID, &run.Scope.Tenant, &run.Scope.Project, &run.Scope.Namespace,
		&run.VerificationID, &run.MemoryID, &run.TargetVersion, &run.SourceWatermark,
		&run.Trigger, &run.WorkIdentity, &run.Status, &run.Attempt, &run.MaxAttempts,
		&leaseOwner, &leaseExpiresAt, &run.NextAttemptAt, &terminalVerificationID,
		&failureCategory, &run.CreatedAt, &run.UpdatedAt, &terminalAt,
	); err != nil {
		return memory.ProceduralRevalidationRun{}, err
	}
	if leaseOwner.Valid {
		run.LeaseOwner = leaseOwner.String
	}
	if leaseExpiresAt.Valid {
		run.LeaseExpiresAt = leaseExpiresAt.Time
	}
	if terminalVerificationID.Valid {
		run.TerminalVerificationID = terminalVerificationID.String
	}
	if failureCategory.Valid {
		run.FailureCategory = failureCategory.String
	}
	if terminalAt.Valid {
		// The public domain keeps only the status/reference. The terminal
		// timestamp remains a bounded audit field in PostgreSQL for admin views.
	}
	return run, nil
}
