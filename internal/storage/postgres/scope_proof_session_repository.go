package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func (r *Repository) CreateScopeProofRun(ctx context.Context, run memory.ScopeProofRun) (memory.ScopeProofRun, error) {
	if err := run.Scope.Validate(); err != nil {
		return memory.ScopeProofRun{}, err
	}
	summary, err := json.Marshal(normalizeAnyMap(run.Summary))
	if err != nil {
		return memory.ScopeProofRun{}, fmt.Errorf("marshal scope proof summary: %w", err)
	}
	const query = `
INSERT INTO scope_proof_runs (
	id, tenant, project, namespace, status, verdict, checks, fixture_mode,
	actor, reason, rerun_of, linked_session_id, failure_category, summary,
	created_at, updated_at, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING id, tenant, project, namespace, status, verdict, checks, fixture_mode,
	actor, reason, rerun_of, linked_session_id, failure_category, summary,
	created_at, updated_at, started_at, finished_at
`
	return scanScopeProofRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.Status,
		run.Verdict,
		scopeProofCheckStrings(run.Checks),
		run.FixtureMode,
		run.Actor,
		run.Reason,
		nullableString(run.RerunOf),
		nullableString(run.LinkedSessionID),
		nullableString(string(run.FailureCategory)),
		summary,
		run.CreatedAt,
		run.UpdatedAt,
		nullableTime(run.StartedAt),
		nullableTime(run.FinishedAt),
	))
}

func (r *Repository) CreateScopeProofStep(ctx context.Context, step memory.ScopeProofStep) (memory.ScopeProofStep, error) {
	if err := step.Scope.Validate(); err != nil {
		return memory.ScopeProofStep{}, err
	}
	evidence, err := json.Marshal(normalizeAnyMap(step.Evidence))
	if err != nil {
		return memory.ScopeProofStep{}, fmt.Errorf("marshal scope proof step evidence: %w", err)
	}
	const query = `
INSERT INTO scope_proof_steps (
	id, proof_id, tenant, project, namespace, step, status, verdict, failure_category,
	evidence, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING id, proof_id, tenant, project, namespace, step, status, verdict, failure_category,
	evidence, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
`
	return scanScopeProofStep(r.db.QueryRow(
		ctx,
		query,
		step.ID,
		step.ProofID,
		step.Scope.Tenant,
		step.Scope.Project,
		step.Scope.Namespace,
		step.Step,
		step.Status,
		nullableString(string(step.Verdict)),
		nullableString(string(step.FailureCategory)),
		evidence,
		step.Attempt,
		nullableString(step.WorkerID),
		nullableTime(step.LeaseUntil),
		nullableString(step.LastError),
		nullableTime(step.NextAttemptAt),
		step.CreatedAt,
		step.UpdatedAt,
		nullableTime(step.CompletedAt),
	))
}

func (r *Repository) ReadScopeProofRun(ctx context.Context, input memory.ReadScopeProofRunInput) (memory.ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return memory.ScopeProofRun{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, verdict, checks, fixture_mode,
	actor, reason, rerun_of, linked_session_id, failure_category, summary,
	created_at, updated_at, started_at, finished_at
FROM scope_proof_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	run, err := scanScopeProofRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.ProofID))
	if err != nil {
		return memory.ScopeProofRun{}, err
	}
	steps, err := r.listScopeProofSteps(ctx, input.Scope, input.ProofID)
	if err != nil {
		return memory.ScopeProofRun{}, err
	}
	run.Steps = steps
	return run, nil
}

func (r *Repository) ListScopeProofRuns(ctx context.Context, input memory.ListScopeProofRunsInput) ([]memory.ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, tenant, project, namespace, status, verdict, checks, fixture_mode,
	actor, reason, rerun_of, linked_session_id, failure_category, summary,
	created_at, updated_at, started_at, finished_at
FROM scope_proof_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY updated_at DESC, id ASC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list scope proof runs: %w", err)
	}
	defer rows.Close()
	runs := make([]memory.ScopeProofRun, 0)
	for rows.Next() {
		run, err := scanScopeProofRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scope proof runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) UpdateScopeProofRunStatus(ctx context.Context, input memory.UpdateScopeProofRunStatusInput) (memory.ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return memory.ScopeProofRun{}, err
	}
	summary, err := json.Marshal(normalizeAnyMap(input.Summary))
	if err != nil {
		return memory.ScopeProofRun{}, fmt.Errorf("marshal scope proof summary: %w", err)
	}
	const query = `
UPDATE scope_proof_runs
SET status = $5,
	verdict = $6,
	failure_category = $7,
	summary = $8,
	updated_at = $9,
	started_at = COALESCE($10, started_at),
	finished_at = $11
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, verdict, checks, fixture_mode,
	actor, reason, rerun_of, linked_session_id, failure_category, summary,
	created_at, updated_at, started_at, finished_at
`
	return scanScopeProofRun(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ProofID,
		input.Status,
		input.Verdict,
		nullableString(string(input.FailureCategory)),
		summary,
		input.UpdatedAt,
		nullableTime(input.StartedAt),
		nullableTime(input.FinishedAt),
	))
}

func (r *Repository) ClaimScopeProofSteps(ctx context.Context, input memory.ClaimScopeProofStepsInput) ([]memory.ScopeProofStep, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH claimed AS (
	SELECT id
	FROM scope_proof_steps
	WHERE tenant = $1
	  AND project = $2
	  AND namespace = $3
	  AND status IN ($9, $10)
	  AND (next_attempt_at IS NULL OR next_attempt_at <= $5)
	  AND (lease_until IS NULL OR lease_until <= $5)
	ORDER BY updated_at ASC, id ASC
	LIMIT $7
	FOR UPDATE SKIP LOCKED
)
UPDATE scope_proof_steps s
SET status = $8,
	worker_id = $4,
	lease_until = $6,
	attempt = s.attempt + 1,
	updated_at = $5
FROM claimed
WHERE s.id = claimed.id
RETURNING s.id, s.proof_id, s.tenant, s.project, s.namespace, s.step, s.status, s.verdict, s.failure_category,
	s.evidence, s.attempt, s.worker_id, s.lease_until, s.last_error, s.next_attempt_at,
	s.created_at, s.updated_at, s.completed_at
`
	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.WorkerID,
		input.Now,
		input.Now.Add(input.LeaseDuration),
		input.Limit,
		memory.ScopeProofStepStatusRunning,
		memory.ScopeProofStepStatusPending,
		memory.ScopeProofStepStatusFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("claim scope proof steps: %w", err)
	}
	defer rows.Close()
	steps := make([]memory.ScopeProofStep, 0)
	for rows.Next() {
		step, err := scanScopeProofStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed scope proof steps: %w", err)
	}
	return steps, nil
}

func (r *Repository) CompleteScopeProofStep(ctx context.Context, input memory.CompleteScopeProofStepInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	evidence, err := json.Marshal(normalizeAnyMap(input.Evidence))
	if err != nil {
		return fmt.Errorf("marshal scope proof step evidence: %w", err)
	}
	const query = `
UPDATE scope_proof_steps
SET status = $7,
	verdict = $8,
	failure_category = $9,
	evidence = $10,
	worker_id = NULL,
	lease_until = NULL,
	last_error = NULL,
	next_attempt_at = NULL,
	updated_at = $11,
	completed_at = $11
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND proof_id = $4 AND id = $5 AND worker_id = $6
`
	if _, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ProofID,
		input.StepID,
		input.WorkerID,
		input.Status,
		input.Verdict,
		nullableString(string(input.FailureCategory)),
		evidence,
		input.CompletedAt,
	); err != nil {
		return fmt.Errorf("complete scope proof step: %w", err)
	}
	return nil
}

func (r *Repository) RecordScopeProofStepFailure(ctx context.Context, input memory.RecordScopeProofStepFailureInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	completedAt := time.Time{}
	if input.Status == memory.ScopeProofStepStatusExhausted || input.Status == memory.ScopeProofStepStatusManualReview {
		completedAt = input.FailedAt
	}
	const query = `
UPDATE scope_proof_steps
SET status = $7,
	verdict = $8,
	failure_category = $9,
	worker_id = NULL,
	lease_until = NULL,
	last_error = $10,
	next_attempt_at = $11,
	updated_at = $12,
	completed_at = COALESCE($13, completed_at)
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND proof_id = $4 AND id = $5 AND worker_id = $6
`
	if _, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ProofID,
		input.StepID,
		input.WorkerID,
		input.Status,
		nullableString(string(input.Verdict)),
		nullableString(string(input.FailureCategory)),
		input.ErrorMessage,
		nullableTime(input.NextAttemptAt),
		input.FailedAt,
		nullableTime(completedAt),
	); err != nil {
		return fmt.Errorf("record scope proof step failure: %w", err)
	}
	return nil
}

func (r *Repository) listScopeProofSteps(ctx context.Context, scope memory.Scope, proofID string) ([]memory.ScopeProofStep, error) {
	const query = `
SELECT id, proof_id, tenant, project, namespace, step, status, verdict, failure_category,
	evidence, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
FROM scope_proof_steps
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND proof_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, proofID)
	if err != nil {
		return nil, fmt.Errorf("list scope proof steps: %w", err)
	}
	defer rows.Close()
	steps := make([]memory.ScopeProofStep, 0)
	for rows.Next() {
		step, err := scanScopeProofStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scope proof steps: %w", err)
	}
	return steps, nil
}

func (r *Repository) CreateMemorySessionRun(ctx context.Context, session memory.MemorySessionRun) (memory.MemorySessionRun, error) {
	if err := session.Scope.Validate(); err != nil {
		return memory.MemorySessionRun{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(session.Metadata))
	if err != nil {
		return memory.MemorySessionRun{}, fmt.Errorf("marshal memory session metadata: %w", err)
	}
	const query = `
INSERT INTO memory_session_runs (
	id, tenant, project, namespace, status, verdict, actor, reason, metadata,
	failure_category, created_at, updated_at, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, tenant, project, namespace, status, verdict, actor, reason, metadata,
	failure_category, created_at, updated_at, started_at, finished_at
`
	return scanMemorySessionRun(r.db.QueryRow(
		ctx,
		query,
		session.ID,
		session.Scope.Tenant,
		session.Scope.Project,
		session.Scope.Namespace,
		session.Status,
		session.Verdict,
		nullableString(session.Actor),
		nullableString(session.Reason),
		metadata,
		nullableString(string(session.FailureCategory)),
		session.CreatedAt,
		session.UpdatedAt,
		nullableTime(session.StartedAt),
		nullableTime(session.FinishedAt),
	))
}

func (r *Repository) CreateMemorySessionTurn(ctx context.Context, turn memory.MemorySessionTurn) (memory.MemorySessionTurn, error) {
	if err := turn.Scope.Validate(); err != nil {
		return memory.MemorySessionTurn{}, err
	}
	contextEvidence, err := json.Marshal(normalizeAnyMap(turn.ContextEvidence))
	if err != nil {
		return memory.MemorySessionTurn{}, fmt.Errorf("marshal memory session turn context evidence: %w", err)
	}
	const query = `
INSERT INTO memory_session_turns (
	id, session_id, tenant, project, namespace, idempotency_key, status, query, context_evidence,
	outcome_event_ids, expected_recall, verification_status, failure_category,
	created_at, updated_at, verified_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (tenant, project, namespace, session_id, idempotency_key)
WHERE idempotency_key IS NOT NULL
DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
RETURNING id, session_id, tenant, project, namespace, idempotency_key, outcome_idempotency_key, status, query, context_evidence,
	outcome_event_ids, expected_recall, verification_status, failure_category,
	created_at, updated_at, verified_at
`
	return scanMemorySessionTurn(r.db.QueryRow(
		ctx,
		query,
		turn.ID,
		turn.SessionID,
		turn.Scope.Tenant,
		turn.Scope.Project,
		turn.Scope.Namespace,
		nullableString(turn.IdempotencyKey),
		turn.Status,
		turn.Query,
		contextEvidence,
		turn.OutcomeEventIDs,
		turn.ExpectedRecall,
		nullableString(string(turn.VerificationStatus)),
		nullableString(string(turn.FailureCategory)),
		turn.CreatedAt,
		turn.UpdatedAt,
		nullableTime(turn.VerifiedAt),
	))
}

func (r *Repository) ReadMemorySessionRun(ctx context.Context, input memory.ReadMemorySessionRunInput) (memory.MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return memory.MemorySessionRun{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, verdict, actor, reason, metadata,
	failure_category, created_at, updated_at, started_at, finished_at
FROM memory_session_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	session, err := scanMemorySessionRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.SessionID))
	if err != nil {
		return memory.MemorySessionRun{}, err
	}
	turns, err := r.listMemorySessionTurns(ctx, input.Scope, input.SessionID)
	if err != nil {
		return memory.MemorySessionRun{}, err
	}
	verifications, err := r.listMemorySessionVerifications(ctx, input.Scope, input.SessionID)
	if err != nil {
		return memory.MemorySessionRun{}, err
	}
	session.Turns = turns
	session.Verifications = verifications
	return session, nil
}

func (r *Repository) ListMemorySessionRuns(ctx context.Context, input memory.ListMemorySessionRunsInput) ([]memory.MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, tenant, project, namespace, status, verdict, actor, reason, metadata,
	failure_category, created_at, updated_at, started_at, finished_at
FROM memory_session_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY updated_at DESC, id ASC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list memory session runs: %w", err)
	}
	defer rows.Close()
	sessions := make([]memory.MemorySessionRun, 0)
	for rows.Next() {
		session, err := scanMemorySessionRun(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory session runs: %w", err)
	}
	return sessions, nil
}

func (r *Repository) UpdateMemorySessionRunStatus(ctx context.Context, input memory.UpdateMemorySessionRunStatusInput) (memory.MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return memory.MemorySessionRun{}, err
	}
	const query = `
UPDATE memory_session_runs
SET status = $5,
	verdict = $6,
	failure_category = $7,
	updated_at = $8,
	started_at = COALESCE($9, started_at),
	finished_at = $10
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, verdict, actor, reason, metadata,
	failure_category, created_at, updated_at, started_at, finished_at
`
	return scanMemorySessionRun(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.SessionID,
		input.Status,
		input.Verdict,
		nullableString(string(input.FailureCategory)),
		input.UpdatedAt,
		nullableTime(input.StartedAt),
		nullableTime(input.FinishedAt),
	))
}

func (r *Repository) UpdateMemorySessionTurnOutcome(ctx context.Context, input memory.UpdateMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error) {
	if err := input.Validate(); err != nil {
		return memory.MemorySessionTurn{}, err
	}
	const query = `
UPDATE memory_session_turns
SET status = CASE WHEN outcome_idempotency_key = $6 THEN status ELSE $7 END,
	outcome_idempotency_key = COALESCE(outcome_idempotency_key, $6),
	outcome_event_ids = CASE WHEN outcome_idempotency_key = $6 THEN outcome_event_ids ELSE COALESCE($8, outcome_event_ids) END,
	expected_recall = CASE WHEN outcome_idempotency_key = $6 THEN expected_recall ELSE COALESCE($9, expected_recall) END,
	verification_status = CASE WHEN outcome_idempotency_key = $6 THEN verification_status ELSE $10 END,
	failure_category = CASE WHEN outcome_idempotency_key = $6 THEN failure_category ELSE $11 END,
	updated_at = CASE WHEN outcome_idempotency_key = $6 THEN updated_at ELSE $12 END,
	verified_at = CASE WHEN outcome_idempotency_key = $6 THEN verified_at ELSE $13 END
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND session_id = $4 AND id = $5
	AND (outcome_idempotency_key IS NULL OR outcome_idempotency_key = $6 OR $6 IS NULL)
RETURNING id, session_id, tenant, project, namespace, idempotency_key, outcome_idempotency_key, status, query, context_evidence,
	outcome_event_ids, expected_recall, verification_status, failure_category,
	created_at, updated_at, verified_at
`
	return scanMemorySessionTurn(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.SessionID,
		input.TurnID,
		nullableString(input.OutcomeIdempotencyKey),
		input.Status,
		input.OutcomeEventIDs,
		input.ExpectedRecall,
		nullableString(string(input.VerificationStatus)),
		nullableString(string(input.FailureCategory)),
		input.UpdatedAt,
		nullableTime(input.VerifiedAt),
	))
}

func (r *Repository) listMemorySessionTurns(ctx context.Context, scope memory.Scope, sessionID string) ([]memory.MemorySessionTurn, error) {
	const query = `
SELECT id, session_id, tenant, project, namespace, idempotency_key, outcome_idempotency_key, status, query, context_evidence,
	outcome_event_ids, expected_recall, verification_status, failure_category,
	created_at, updated_at, verified_at
FROM memory_session_turns
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND session_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list memory session turns: %w", err)
	}
	defer rows.Close()
	turns := make([]memory.MemorySessionTurn, 0)
	for rows.Next() {
		turn, err := scanMemorySessionTurn(rows)
		if err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory session turns: %w", err)
	}
	return turns, nil
}

func (r *Repository) listMemorySessionVerifications(ctx context.Context, scope memory.Scope, sessionID string) ([]memory.MemorySessionVerification, error) {
	const query = `
SELECT id, session_id, turn_id, tenant, project, namespace, status, verdict, expected_recall, evidence,
	failure_category, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
FROM memory_session_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND session_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list memory session verifications: %w", err)
	}
	defer rows.Close()
	verifications := make([]memory.MemorySessionVerification, 0)
	for rows.Next() {
		verification, err := scanMemorySessionVerification(rows)
		if err != nil {
			return nil, err
		}
		verifications = append(verifications, verification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory session verifications: %w", err)
	}
	return verifications, nil
}

func (r *Repository) CreateMemoryLoopEvidenceLink(ctx context.Context, link memory.MemoryLoopEvidenceLink) (memory.MemoryLoopEvidenceLink, error) {
	if err := (memory.CreateMemoryLoopEvidenceLinkInput{
		Scope:        link.Scope,
		OwnerKind:    link.OwnerKind,
		OwnerID:      link.OwnerID,
		EvidenceKind: link.EvidenceKind,
		EvidenceID:   link.EvidenceID,
		Metadata:     link.Metadata,
	}).Validate(); err != nil {
		return memory.MemoryLoopEvidenceLink{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(link.Metadata))
	if err != nil {
		return memory.MemoryLoopEvidenceLink{}, fmt.Errorf("marshal memory loop evidence metadata: %w", err)
	}
	const query = `
INSERT INTO memory_loop_evidence_links (
	id, tenant, project, namespace, owner_kind, owner_id, evidence_kind, evidence_id, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, tenant, project, namespace, owner_kind, owner_id, evidence_kind, evidence_id, metadata, created_at
`
	return scanMemoryLoopEvidenceLink(r.db.QueryRow(
		ctx,
		query,
		link.ID,
		link.Scope.Tenant,
		link.Scope.Project,
		link.Scope.Namespace,
		link.OwnerKind,
		link.OwnerID,
		link.EvidenceKind,
		link.EvidenceID,
		metadata,
		link.CreatedAt,
	))
}

func (r *Repository) CreateMemorySessionVerification(ctx context.Context, verification memory.MemorySessionVerification) (memory.MemorySessionVerification, error) {
	if err := verification.Scope.Validate(); err != nil {
		return memory.MemorySessionVerification{}, err
	}
	evidence, err := json.Marshal(normalizeAnyMap(verification.Evidence))
	if err != nil {
		return memory.MemorySessionVerification{}, fmt.Errorf("marshal memory session verification evidence: %w", err)
	}
	const query = `
INSERT INTO memory_session_verifications (
	id, session_id, turn_id, tenant, project, namespace, status, verdict, expected_recall, evidence,
	failure_category, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING id, session_id, turn_id, tenant, project, namespace, status, verdict, expected_recall, evidence,
	failure_category, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
`
	return scanMemorySessionVerification(r.db.QueryRow(
		ctx,
		query,
		verification.ID,
		verification.SessionID,
		nullableString(verification.TurnID),
		verification.Scope.Tenant,
		verification.Scope.Project,
		verification.Scope.Namespace,
		verification.Status,
		verification.Verdict,
		verification.ExpectedRecall,
		evidence,
		nullableString(string(verification.FailureCategory)),
		verification.Attempt,
		nullableString(verification.WorkerID),
		nullableTime(verification.LeaseUntil),
		nullableString(verification.LastError),
		nullableTime(verification.NextAttemptAt),
		verification.CreatedAt,
		verification.UpdatedAt,
		nullableTime(verification.CompletedAt),
	))
}

func (r *Repository) ClaimMemorySessionVerifications(ctx context.Context, input memory.ClaimMemorySessionVerificationsInput) ([]memory.MemorySessionVerification, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH claimed AS (
	SELECT id
	FROM memory_session_verifications
	WHERE tenant = $1
	  AND project = $2
	  AND namespace = $3
	  AND status IN ($9, $10)
	  AND (next_attempt_at IS NULL OR next_attempt_at <= $5)
	  AND (lease_until IS NULL OR lease_until <= $5)
	ORDER BY updated_at ASC, id ASC
	LIMIT $7
	FOR UPDATE SKIP LOCKED
)
UPDATE memory_session_verifications v
SET status = $8,
	worker_id = $4,
	lease_until = $6,
	attempt = v.attempt + 1,
	updated_at = $5
FROM claimed
WHERE v.id = claimed.id
RETURNING v.id, v.session_id, v.turn_id, v.tenant, v.project, v.namespace, v.status, v.verdict, v.expected_recall, v.evidence,
	v.failure_category, v.attempt, v.worker_id, v.lease_until, v.last_error, v.next_attempt_at,
	v.created_at, v.updated_at, v.completed_at
`
	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.WorkerID,
		input.Now,
		input.Now.Add(input.LeaseDuration),
		input.Limit,
		memory.ScopeProofStepStatusRunning,
		memory.ScopeProofStepStatusPending,
		memory.ScopeProofStepStatusFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("claim memory session verifications: %w", err)
	}
	defer rows.Close()
	verifications := make([]memory.MemorySessionVerification, 0)
	for rows.Next() {
		verification, err := scanMemorySessionVerification(rows)
		if err != nil {
			return nil, err
		}
		verifications = append(verifications, verification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed memory session verifications: %w", err)
	}
	return verifications, nil
}

func (r *Repository) CompleteMemorySessionVerification(ctx context.Context, input memory.CompleteMemorySessionVerificationInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	evidence, err := json.Marshal(normalizeAnyMap(input.Evidence))
	if err != nil {
		return fmt.Errorf("marshal memory session verification evidence: %w", err)
	}
	const query = `
UPDATE memory_session_verifications
SET status = $8,
	verdict = $9,
	evidence = $10,
	failure_category = $11,
	worker_id = NULL,
	lease_until = NULL,
	last_error = NULL,
	next_attempt_at = NULL,
	updated_at = $12,
	completed_at = $12
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND session_id = $4 AND id = $5 AND worker_id = $6 AND COALESCE(turn_id, '') = $7
`
	if _, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.SessionID,
		input.VerificationID,
		input.WorkerID,
		input.TurnID,
		input.Status,
		input.Verdict,
		evidence,
		nullableString(string(input.FailureCategory)),
		input.CompletedAt,
	); err != nil {
		return fmt.Errorf("complete memory session verification: %w", err)
	}
	return nil
}

func (r *Repository) RecordMemorySessionVerificationFailure(ctx context.Context, input memory.RecordMemorySessionVerificationFailureInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	completedAt := time.Time{}
	if input.Status == memory.ScopeProofStepStatusExhausted || input.Status == memory.ScopeProofStepStatusManualReview {
		completedAt = input.FailedAt
	}
	const query = `
UPDATE memory_session_verifications
SET status = $8,
	verdict = $9,
	failure_category = $10,
	worker_id = NULL,
	lease_until = NULL,
	last_error = $11,
	next_attempt_at = $12,
	updated_at = $13,
	completed_at = COALESCE($14, completed_at)
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND session_id = $4 AND id = $5 AND worker_id = $6 AND COALESCE(turn_id, '') = $7
`
	if _, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.SessionID,
		input.VerificationID,
		input.WorkerID,
		input.TurnID,
		input.Status,
		nullableString(string(input.Verdict)),
		nullableString(string(input.FailureCategory)),
		input.ErrorMessage,
		nullableTime(input.NextAttemptAt),
		input.FailedAt,
		nullableTime(completedAt),
	); err != nil {
		return fmt.Errorf("record memory session verification failure: %w", err)
	}
	return nil
}

func scanScopeProofRun(scanner provenanceScanner) (memory.ScopeProofRun, error) {
	var run memory.ScopeProofRun
	var checks []string
	var rerunOf sql.NullString
	var linkedSessionID sql.NullString
	var failureCategory sql.NullString
	var summary []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.Status,
		&run.Verdict,
		&checks,
		&run.FixtureMode,
		&run.Actor,
		&run.Reason,
		&rerunOf,
		&linkedSessionID,
		&failureCategory,
		&summary,
		&run.CreatedAt,
		&run.UpdatedAt,
		&startedAt,
		&finishedAt,
	); err != nil {
		return memory.ScopeProofRun{}, fmt.Errorf("scan scope proof run: %w", err)
	}
	run.Checks = scopeProofChecks(checks)
	if rerunOf.Valid {
		run.RerunOf = rerunOf.String
	}
	if linkedSessionID.Valid {
		run.LinkedSessionID = linkedSessionID.String
	}
	if failureCategory.Valid {
		run.FailureCategory = memory.ProofFailureCategory(failureCategory.String)
	}
	run.Summary = map[string]any{}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &run.Summary); err != nil {
			return memory.ScopeProofRun{}, fmt.Errorf("unmarshal scope proof summary: %w", err)
		}
	}
	if startedAt.Valid {
		run.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	return run, nil
}

func scanScopeProofStep(scanner provenanceScanner) (memory.ScopeProofStep, error) {
	var step memory.ScopeProofStep
	var verdict sql.NullString
	var failureCategory sql.NullString
	var evidence []byte
	var workerID sql.NullString
	var leaseUntil sql.NullTime
	var lastError sql.NullString
	var nextAttemptAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&step.ID,
		&step.ProofID,
		&step.Scope.Tenant,
		&step.Scope.Project,
		&step.Scope.Namespace,
		&step.Step,
		&step.Status,
		&verdict,
		&failureCategory,
		&evidence,
		&step.Attempt,
		&workerID,
		&leaseUntil,
		&lastError,
		&nextAttemptAt,
		&step.CreatedAt,
		&step.UpdatedAt,
		&completedAt,
	); err != nil {
		return memory.ScopeProofStep{}, fmt.Errorf("scan scope proof step: %w", err)
	}
	if verdict.Valid {
		step.Verdict = memory.ScopeProofVerdict(verdict.String)
	}
	if failureCategory.Valid {
		step.FailureCategory = memory.ProofFailureCategory(failureCategory.String)
	}
	step.Evidence = map[string]any{}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &step.Evidence); err != nil {
			return memory.ScopeProofStep{}, fmt.Errorf("unmarshal scope proof step evidence: %w", err)
		}
	}
	if workerID.Valid {
		step.WorkerID = workerID.String
	}
	if leaseUntil.Valid {
		step.LeaseUntil = leaseUntil.Time
	}
	if lastError.Valid {
		step.LastError = lastError.String
	}
	if nextAttemptAt.Valid {
		step.NextAttemptAt = nextAttemptAt.Time
	}
	if completedAt.Valid {
		step.CompletedAt = completedAt.Time
	}
	return step, nil
}

func scanMemorySessionRun(scanner provenanceScanner) (memory.MemorySessionRun, error) {
	var session memory.MemorySessionRun
	var actor sql.NullString
	var reason sql.NullString
	var metadata []byte
	var failureCategory sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&session.ID,
		&session.Scope.Tenant,
		&session.Scope.Project,
		&session.Scope.Namespace,
		&session.Status,
		&session.Verdict,
		&actor,
		&reason,
		&metadata,
		&failureCategory,
		&session.CreatedAt,
		&session.UpdatedAt,
		&startedAt,
		&finishedAt,
	); err != nil {
		return memory.MemorySessionRun{}, fmt.Errorf("scan memory session run: %w", err)
	}
	if actor.Valid {
		session.Actor = actor.String
	}
	if reason.Valid {
		session.Reason = reason.String
	}
	session.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &session.Metadata); err != nil {
			return memory.MemorySessionRun{}, fmt.Errorf("unmarshal memory session metadata: %w", err)
		}
	}
	if failureCategory.Valid {
		session.FailureCategory = memory.ProofFailureCategory(failureCategory.String)
	}
	if startedAt.Valid {
		session.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		session.FinishedAt = finishedAt.Time
	}
	return session, nil
}

func scanMemorySessionTurn(scanner provenanceScanner) (memory.MemorySessionTurn, error) {
	var turn memory.MemorySessionTurn
	var contextEvidence []byte
	var idempotencyKey sql.NullString
	var outcomeIdempotencyKey sql.NullString
	var verificationStatus sql.NullString
	var failureCategory sql.NullString
	var verifiedAt sql.NullTime
	if err := scanner.Scan(
		&turn.ID,
		&turn.SessionID,
		&turn.Scope.Tenant,
		&turn.Scope.Project,
		&turn.Scope.Namespace,
		&idempotencyKey,
		&outcomeIdempotencyKey,
		&turn.Status,
		&turn.Query,
		&contextEvidence,
		&turn.OutcomeEventIDs,
		&turn.ExpectedRecall,
		&verificationStatus,
		&failureCategory,
		&turn.CreatedAt,
		&turn.UpdatedAt,
		&verifiedAt,
	); err != nil {
		return memory.MemorySessionTurn{}, fmt.Errorf("scan memory session turn: %w", err)
	}
	turn.ContextEvidence = map[string]any{}
	if len(contextEvidence) > 0 {
		if err := json.Unmarshal(contextEvidence, &turn.ContextEvidence); err != nil {
			return memory.MemorySessionTurn{}, fmt.Errorf("unmarshal memory session turn context evidence: %w", err)
		}
	}
	if idempotencyKey.Valid {
		turn.IdempotencyKey = idempotencyKey.String
	}
	if outcomeIdempotencyKey.Valid {
		turn.OutcomeIdempotencyKey = outcomeIdempotencyKey.String
	}
	if verificationStatus.Valid {
		turn.VerificationStatus = memory.ScopeProofVerdict(verificationStatus.String)
	}
	if failureCategory.Valid {
		turn.FailureCategory = memory.ProofFailureCategory(failureCategory.String)
	}
	if verifiedAt.Valid {
		turn.VerifiedAt = verifiedAt.Time
	}
	return turn, nil
}

func scanMemoryLoopEvidenceLink(scanner provenanceScanner) (memory.MemoryLoopEvidenceLink, error) {
	var link memory.MemoryLoopEvidenceLink
	var metadata []byte
	if err := scanner.Scan(
		&link.ID,
		&link.Scope.Tenant,
		&link.Scope.Project,
		&link.Scope.Namespace,
		&link.OwnerKind,
		&link.OwnerID,
		&link.EvidenceKind,
		&link.EvidenceID,
		&metadata,
		&link.CreatedAt,
	); err != nil {
		return memory.MemoryLoopEvidenceLink{}, fmt.Errorf("scan memory loop evidence link: %w", err)
	}
	link.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &link.Metadata); err != nil {
			return memory.MemoryLoopEvidenceLink{}, fmt.Errorf("unmarshal memory loop evidence metadata: %w", err)
		}
	}
	return link, nil
}

func scanMemorySessionVerification(scanner provenanceScanner) (memory.MemorySessionVerification, error) {
	var verification memory.MemorySessionVerification
	var turnID sql.NullString
	var evidence []byte
	var failureCategory sql.NullString
	var workerID sql.NullString
	var leaseUntil sql.NullTime
	var lastError sql.NullString
	var nextAttemptAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&verification.ID,
		&verification.SessionID,
		&turnID,
		&verification.Scope.Tenant,
		&verification.Scope.Project,
		&verification.Scope.Namespace,
		&verification.Status,
		&verification.Verdict,
		&verification.ExpectedRecall,
		&evidence,
		&failureCategory,
		&verification.Attempt,
		&workerID,
		&leaseUntil,
		&lastError,
		&nextAttemptAt,
		&verification.CreatedAt,
		&verification.UpdatedAt,
		&completedAt,
	); err != nil {
		return memory.MemorySessionVerification{}, fmt.Errorf("scan memory session verification: %w", err)
	}
	if turnID.Valid {
		verification.TurnID = turnID.String
	}
	verification.Evidence = map[string]any{}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &verification.Evidence); err != nil {
			return memory.MemorySessionVerification{}, fmt.Errorf("unmarshal memory session verification evidence: %w", err)
		}
	}
	if failureCategory.Valid {
		verification.FailureCategory = memory.ProofFailureCategory(failureCategory.String)
	}
	if workerID.Valid {
		verification.WorkerID = workerID.String
	}
	if leaseUntil.Valid {
		verification.LeaseUntil = leaseUntil.Time
	}
	if lastError.Valid {
		verification.LastError = lastError.String
	}
	if nextAttemptAt.Valid {
		verification.NextAttemptAt = nextAttemptAt.Time
	}
	if completedAt.Valid {
		verification.CompletedAt = completedAt.Time
	}
	return verification, nil
}

func scopeProofCheckStrings(checks []memory.ScopeProofCheck) []string {
	values := make([]string, 0, len(checks))
	for _, check := range checks {
		values = append(values, string(check))
	}
	return values
}

func scopeProofChecks(values []string) []memory.ScopeProofCheck {
	checks := make([]memory.ScopeProofCheck, 0, len(values))
	for _, value := range values {
		checks = append(checks, memory.ScopeProofCheck(value))
	}
	return checks
}
