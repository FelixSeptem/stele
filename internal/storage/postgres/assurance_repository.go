package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateHealthEvaluation(ctx context.Context, evaluation assurance.HealthEvaluation) (assurance.HealthEvaluation, error) {
	if err := evaluation.Validate(); err != nil {
		return assurance.HealthEvaluation{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return assurance.HealthEvaluation{}, fmt.Errorf("begin health evaluation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const evaluationQuery = `
INSERT INTO assurance_health_evaluations (
	id, tenant, project, namespace, status, severity, reason, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, tenant, project, namespace, status, severity, reason, created_at
`
	created, err := scanHealthEvaluation(tx.QueryRow(
		ctx,
		evaluationQuery,
		evaluation.ID,
		evaluation.Scope.Tenant,
		evaluation.Scope.Project,
		evaluation.Scope.Namespace,
		evaluation.Status,
		evaluation.Severity,
		evaluation.Reason,
		evaluation.CreatedAt,
	))
	if err != nil {
		return assurance.HealthEvaluation{}, err
	}

	created.Components = make([]assurance.HealthComponentSummary, 0, len(evaluation.Components))
	for _, component := range evaluation.Components {
		if component.EvaluationID == "" {
			component.EvaluationID = created.ID
		}
		if component.Scope == (memory.Scope{}) {
			component.Scope = created.Scope
		}
		inserted, err := createHealthComponent(ctx, tx, component)
		if err != nil {
			return assurance.HealthEvaluation{}, err
		}
		created.Components = append(created.Components, inserted)
	}
	if err := tx.Commit(ctx); err != nil {
		return assurance.HealthEvaluation{}, fmt.Errorf("commit health evaluation transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) ReadHealthEvaluation(ctx context.Context, input assurance.ReadHealthEvaluationInput) (assurance.HealthEvaluation, error) {
	if err := input.Validate(); err != nil {
		return assurance.HealthEvaluation{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, severity, reason, created_at
FROM assurance_health_evaluations
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	evaluation, err := scanHealthEvaluation(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EvaluationID))
	if err != nil {
		return assurance.HealthEvaluation{}, err
	}
	components, err := r.listHealthComponents(ctx, input.Scope, input.EvaluationID)
	if err != nil {
		return assurance.HealthEvaluation{}, err
	}
	evaluation.Components = components
	return evaluation, nil
}

func (r *Repository) ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]assurance.HealthEvaluation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, severity, reason, created_at
FROM assurance_health_evaluations
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY created_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list health evaluations: %w", err)
	}
	defer rows.Close()
	evaluations := make([]assurance.HealthEvaluation, 0)
	for rows.Next() {
		evaluation, err := scanHealthEvaluation(rows)
		if err != nil {
			return nil, err
		}
		evaluations = append(evaluations, evaluation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health evaluations: %w", err)
	}
	return evaluations, nil
}

func (r *Repository) CreateIncident(ctx context.Context, incident assurance.Incident) (assurance.Incident, error) {
	if err := incident.Validate(); err != nil {
		return assurance.Incident{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(incident.Metadata))
	if err != nil {
		return assurance.Incident{}, fmt.Errorf("marshal incident metadata: %w", err)
	}
	const query = `
INSERT INTO assurance_incidents (
	id, tenant, project, namespace, status, severity, component, reason, deduplication_key,
	opened_at, updated_at, resolved_at, latest_evaluation_id, runbook_hints, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, tenant, project, namespace, status, severity, component, reason, deduplication_key,
	opened_at, updated_at, resolved_at, latest_evaluation_id, runbook_hints, metadata
`
	return scanIncident(r.db.QueryRow(
		ctx,
		query,
		incident.ID,
		incident.Scope.Tenant,
		incident.Scope.Project,
		incident.Scope.Namespace,
		incident.Status,
		incident.Severity,
		incident.Component,
		incident.Reason,
		incident.DeduplicationKey,
		incident.OpenedAt,
		incident.UpdatedAt,
		nullableTime(incident.ResolvedAt),
		nullableString(incident.LatestEvaluationID),
		runbookHintStrings(incident.RunbookHints),
		metadata,
	))
}

func (r *Repository) ReadIncident(ctx context.Context, input assurance.ReadIncidentInput) (assurance.Incident, error) {
	if err := input.Validate(); err != nil {
		return assurance.Incident{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, severity, component, reason, deduplication_key,
	opened_at, updated_at, resolved_at, latest_evaluation_id, runbook_hints, metadata
FROM assurance_incidents
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanIncident(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.IncidentID))
}

func (r *Repository) ListIncidents(ctx context.Context, input assurance.ListIncidentsInput) ([]assurance.Incident, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, tenant, project, namespace, status, severity, component, reason, deduplication_key,
	opened_at, updated_at, resolved_at, latest_evaluation_id, runbook_hints, metadata
FROM assurance_incidents
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR status = $4)
ORDER BY updated_at DESC, id ASC
LIMIT $5
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, string(input.Status), limit)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()
	incidents := make([]assurance.Incident, 0)
	for rows.Next() {
		incident, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}
	return incidents, nil
}

func (r *Repository) TransitionIncident(ctx context.Context, transition assurance.IncidentTransition) (assurance.Incident, error) {
	if err := transition.Validate(); err != nil {
		return assurance.Incident{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return assurance.Incident{}, fmt.Errorf("begin incident transition transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertTransition = `
INSERT INTO assurance_incident_transitions (
	id, incident_id, tenant, project, namespace, from_status, to_status, action, actor, reason, occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, incident_id, tenant, project, namespace, from_status, to_status, action, actor, reason, occurred_at
`
	if _, err := scanIncidentTransition(tx.QueryRow(
		ctx,
		insertTransition,
		transition.ID,
		transition.IncidentID,
		transition.Scope.Tenant,
		transition.Scope.Project,
		transition.Scope.Namespace,
		transition.FromStatus,
		transition.ToStatus,
		transition.Action,
		transition.Actor,
		transition.Reason,
		transition.OccurredAt,
	)); err != nil {
		return assurance.Incident{}, err
	}

	resolvedAt := timeForResolvedStatus(transition.ToStatus, transition.OccurredAt)
	const updateIncident = `
UPDATE assurance_incidents
SET status = $1, updated_at = $2, resolved_at = COALESCE($3, resolved_at)
WHERE id = $4 AND tenant = $5 AND project = $6 AND namespace = $7
RETURNING id, tenant, project, namespace, status, severity, component, reason, deduplication_key,
	opened_at, updated_at, resolved_at, latest_evaluation_id, runbook_hints, metadata
`
	updated, err := scanIncident(tx.QueryRow(
		ctx,
		updateIncident,
		transition.ToStatus,
		transition.OccurredAt,
		nullableTime(resolvedAt),
		transition.IncidentID,
		transition.Scope.Tenant,
		transition.Scope.Project,
		transition.Scope.Namespace,
	))
	if err != nil {
		return assurance.Incident{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return assurance.Incident{}, fmt.Errorf("commit incident transition transaction: %w", err)
	}
	return updated, nil
}

func (r *Repository) CreateAlertCandidate(ctx context.Context, candidate assurance.AlertCandidate) (assurance.AlertCandidate, error) {
	if err := candidate.Validate(); err != nil {
		return assurance.AlertCandidate{}, err
	}
	payload, err := json.Marshal(normalizeAnyMap(candidate.Payload))
	if err != nil {
		return assurance.AlertCandidate{}, fmt.Errorf("marshal alert payload: %w", err)
	}
	const query = `
INSERT INTO assurance_alert_candidates (
	id, tenant, project, namespace, incident_id, evaluation_id, severity, component, reason,
	deduplication_key, delivery_policy, payload, created_at, next_attempt_at, suppressed_until
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, tenant, project, namespace, incident_id, evaluation_id, severity, component, reason,
	deduplication_key, delivery_policy, payload, created_at, next_attempt_at, suppressed_until
`
	return scanAlertCandidate(r.db.QueryRow(
		ctx,
		query,
		candidate.ID,
		candidate.Scope.Tenant,
		candidate.Scope.Project,
		candidate.Scope.Namespace,
		nullableString(candidate.IncidentID),
		nullableString(candidate.EvaluationID),
		candidate.Severity,
		candidate.Component,
		candidate.Reason,
		candidate.DeduplicationKey,
		candidate.DeliveryPolicy,
		payload,
		candidate.CreatedAt,
		nullableTime(candidate.NextAttemptAt),
		nullableTime(candidate.SuppressedUntil),
	))
}

func (r *Repository) ReadAlertCandidate(ctx context.Context, input assurance.ReadAlertCandidateInput) (assurance.AlertCandidate, error) {
	if err := input.Validate(); err != nil {
		return assurance.AlertCandidate{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, incident_id, evaluation_id, severity, component, reason,
	deduplication_key, delivery_policy, payload, created_at, next_attempt_at, suppressed_until
FROM assurance_alert_candidates
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanAlertCandidate(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.AlertCandidateID))
}

func (r *Repository) ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]assurance.AlertCandidate, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, incident_id, evaluation_id, severity, component, reason,
	deduplication_key, delivery_policy, payload, created_at, next_attempt_at, suppressed_until
FROM assurance_alert_candidates
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY created_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list alert candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]assurance.AlertCandidate, 0)
	for rows.Next() {
		candidate, err := scanAlertCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert candidates: %w", err)
	}
	return candidates, nil
}

func (r *Repository) CreateAlertDeliveryAttempt(ctx context.Context, attempt assurance.AlertDeliveryAttempt) (assurance.AlertDeliveryAttempt, error) {
	if err := attempt.Validate(); err != nil {
		return assurance.AlertDeliveryAttempt{}, err
	}
	const query = `
INSERT INTO assurance_alert_delivery_attempts (
	id, alert_candidate_id, tenant, project, namespace, adapter_kind, result, failure_category,
	attempt, worker_id, lease_until, next_attempt_at, payload_hash, attempted_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, alert_candidate_id, tenant, project, namespace, adapter_kind, result, failure_category,
	attempt, worker_id, lease_until, next_attempt_at, payload_hash, attempted_at, completed_at
`
	return scanAlertDeliveryAttempt(r.db.QueryRow(
		ctx,
		query,
		attempt.ID,
		attempt.AlertCandidateID,
		attempt.Scope.Tenant,
		attempt.Scope.Project,
		attempt.Scope.Namespace,
		attempt.Adapter,
		attempt.Result,
		nullableString(attempt.FailureCategory),
		attempt.Attempt,
		nullableString(attempt.WorkerID),
		nullableTime(attempt.LeaseUntil),
		nullableTime(attempt.NextAttemptAt),
		nullableString(attempt.PayloadHash),
		attempt.AttemptedAt,
		nullableTime(attempt.CompletedAt),
	))
}

func (r *Repository) ListAlertDeliveryAttempts(ctx context.Context, input assurance.ListAlertDeliveryAttemptsInput) ([]assurance.AlertDeliveryAttempt, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, alert_candidate_id, tenant, project, namespace, adapter_kind, result, failure_category,
	attempt, worker_id, lease_until, next_attempt_at, payload_hash, attempted_at, completed_at
FROM assurance_alert_delivery_attempts
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND alert_candidate_id = $4
ORDER BY attempted_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.AlertCandidateID)
	if err != nil {
		return nil, fmt.Errorf("list alert delivery attempts: %w", err)
	}
	defer rows.Close()
	attempts := make([]assurance.AlertDeliveryAttempt, 0)
	for rows.Next() {
		attempt, err := scanAlertDeliveryAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert delivery attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) ClaimAlertCandidatesForDelivery(ctx context.Context, input assurance.ClaimAlertCandidatesForDeliveryInput) ([]assurance.AlertDeliveryClaim, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH attempt_state AS (
	SELECT
		alert_candidate_id,
		COALESCE(MAX(attempt), 0) AS max_attempt,
		BOOL_OR(result IN ('success', 'disabled', 'skipped')) AS completed,
		BOOL_OR(lease_until IS NOT NULL AND lease_until > $5 AND completed_at IS NULL) AS leased
	FROM assurance_alert_delivery_attempts
	WHERE tenant = $1
		AND project = $2
		AND namespace = $3
	GROUP BY alert_candidate_id
),
eligible AS (
	SELECT
		c.id,
		COALESCE(s.max_attempt, 0) + 1 AS next_attempt
	FROM assurance_alert_candidates c
	LEFT JOIN attempt_state s
		ON s.alert_candidate_id = c.id
	WHERE c.tenant = $1
		AND c.project = $2
		AND c.namespace = $3
		AND (c.next_attempt_at IS NULL OR c.next_attempt_at <= $5)
		AND (c.suppressed_until IS NULL OR c.suppressed_until <= $5)
		AND COALESCE(s.completed, false) = false
		AND COALESCE(s.leased, false) = false
		AND COALESCE(s.max_attempt, 0) < $8
	ORDER BY c.created_at ASC, c.id ASC
	LIMIT $7
	FOR UPDATE OF c SKIP LOCKED
),
claimed AS (
	UPDATE assurance_alert_candidates c
	SET next_attempt_at = $6
	FROM eligible
	WHERE c.id = eligible.id
		AND c.tenant = $1
		AND c.project = $2
		AND c.namespace = $3
	RETURNING c.id, c.tenant, c.project, c.namespace, c.incident_id, c.evaluation_id,
		c.severity, c.component, c.reason, c.deduplication_key, c.delivery_policy, c.payload,
		c.created_at, c.next_attempt_at, c.suppressed_until, eligible.next_attempt
)
SELECT id, tenant, project, namespace, incident_id, evaluation_id, severity, component, reason,
	deduplication_key, delivery_policy, payload, created_at, next_attempt_at, suppressed_until,
	next_attempt AS attempt, $4::text AS worker_id, $5::timestamptz AS claimed_at, $6::timestamptz AS lease_until
FROM claimed
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.Limit, input.MaxAttempts)
	if err != nil {
		return nil, fmt.Errorf("claim alert candidates for delivery: %w", err)
	}
	defer rows.Close()
	claims := make([]assurance.AlertDeliveryClaim, 0)
	for rows.Next() {
		claim, err := scanAlertDeliveryClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert delivery claims: %w", err)
	}
	return claims, nil
}

func (r *Repository) CreateConformanceProfile(ctx context.Context, profile assurance.ConformanceProfile) (assurance.ConformanceProfile, error) {
	if err := profile.Validate(); err != nil {
		return assurance.ConformanceProfile{}, err
	}
	expectedEvidence, err := json.Marshal(profile.ExpectedEvidence)
	if err != nil {
		return assurance.ConformanceProfile{}, fmt.Errorf("marshal expected evidence: %w", err)
	}
	const query = `
INSERT INTO assurance_conformance_profiles (
	id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
`
	return scanConformanceProfile(r.db.QueryRow(
		ctx,
		query,
		profile.ID,
		profile.Scope.Tenant,
		profile.Scope.Project,
		profile.Scope.Namespace,
		profile.Status,
		expectedEvidence,
		profile.Actor,
		profile.Reason,
		profile.CreatedAt,
		profile.UpdatedAt,
		nil,
	))
}

func (r *Repository) ReadConformanceProfile(ctx context.Context, input assurance.ReadConformanceProfileInput) (assurance.ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return assurance.ConformanceProfile{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
FROM assurance_conformance_profiles
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanConformanceProfile(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.ProfileID))
}

func (r *Repository) ListConformanceProfiles(ctx context.Context, input assurance.ListConformanceProfilesInput) ([]assurance.ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
FROM assurance_conformance_profiles
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR status = $4)
ORDER BY updated_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, string(input.Status))
	if err != nil {
		return nil, fmt.Errorf("list conformance profiles: %w", err)
	}
	defer rows.Close()
	profiles := make([]assurance.ConformanceProfile, 0)
	for rows.Next() {
		profile, err := scanConformanceProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conformance profiles: %w", err)
	}
	return profiles, nil
}

func (r *Repository) UpdateConformanceProfile(ctx context.Context, input assurance.UpdateConformanceProfileInput) (assurance.ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return assurance.ConformanceProfile{}, err
	}
	expectedEvidence, err := json.Marshal(input.ExpectedEvidence)
	if err != nil {
		return assurance.ConformanceProfile{}, fmt.Errorf("marshal expected evidence: %w", err)
	}
	const query = `
UPDATE assurance_conformance_profiles
SET expected_evidence = $5, actor = $6, reason = $7, updated_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
`
	return scanConformanceProfile(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ProfileID,
		expectedEvidence,
		input.Actor,
		input.Reason,
		input.UpdatedAt,
	))
}

func (r *Repository) DisableConformanceProfile(ctx context.Context, input assurance.DisableConformanceProfileInput) (assurance.ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return assurance.ConformanceProfile{}, err
	}
	const query = `
UPDATE assurance_conformance_profiles
SET status = $5, actor = $6, reason = $7, updated_at = $8, disabled_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, expected_evidence, actor, reason, created_at, updated_at, disabled_at
`
	return scanConformanceProfile(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ProfileID,
		assurance.ConformanceProfileStatusDisabled,
		input.Actor,
		input.Reason,
		input.DisabledAt,
	))
}

func (r *Repository) InspectConformanceEvidence(ctx context.Context, input assurance.ConformanceEvidenceInspectionInput) ([]assurance.ConformanceEvidenceObservation, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	observations := make([]assurance.ConformanceEvidenceObservation, 0, len(input.ExpectedEvidence))
	for _, expected := range input.ExpectedEvidence {
		observation, err := r.inspectConformanceEvidenceKind(ctx, input.Scope, expected.Kind)
		if err != nil {
			return nil, err
		}
		observation.Kind = expected.Kind
		observations = append(observations, observation)
	}
	return observations, nil
}

func (r *Repository) inspectConformanceEvidenceKind(ctx context.Context, scope memory.Scope, kind assurance.ExpectedEvidenceKind) (assurance.ConformanceEvidenceObservation, error) {
	switch kind {
	case assurance.ExpectedEvidenceSession:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), false, false, false
FROM memory_session_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceContext:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), bool_or(context_evidence = '{}'::jsonb), false, false
FROM memory_session_turns
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND context_evidence <> '{}'::jsonb
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceOutcome:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), false, false, false
FROM memory_session_turns
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND cardinality(outcome_event_ids) > 0
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceVerification:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), bool_or(evidence = '{}'::jsonb), bool_or(verdict IN ('failed', 'manual_review')), false
FROM memory_session_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceUsefulnessFeedback:
		const query = `
SELECT COUNT(*)::bigint, MAX(uf.created_at), bool_or(ufs.subject_id IS NULL AND ufs.opaque_token IS NOT NULL), false, false
FROM usefulness_feedback uf
LEFT JOIN usefulness_feedback_subjects ufs
	ON ufs.feedback_id = uf.id
	AND ufs.tenant = uf.tenant
	AND ufs.project = uf.project
	AND ufs.namespace = uf.namespace
WHERE uf.tenant = $1 AND uf.project = $2 AND uf.namespace = $3
	AND uf.superseded_at IS NULL
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceTaskEvaluation:
		const query = `
SELECT COUNT(*)::bigint, MAX(te.updated_at),
	bool_or(tel.evidence_id IS NULL AND tel.opaque_token IS NOT NULL),
	bool_or(te.verdict = 'failed'),
	false
FROM task_evaluations te
LEFT JOIN task_evidence_links tel
	ON tel.task_evaluation_id = te.id
	AND tel.tenant = te.tenant
	AND tel.project = te.project
	AND tel.namespace = te.namespace
WHERE te.tenant = $1 AND te.project = $2 AND te.namespace = $3
	AND te.superseded_at IS NULL
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceProof:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), false, bool_or(verdict IN ('failed', 'manual_review')), false
FROM scope_proof_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceRepair:
		const query = `
SELECT COUNT(*)::bigint, MAX(updated_at), false, bool_or(status IN ('failed', 'manual_review') OR verification_status = 'failed'), false
FROM repair_plans
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceRankingRollout:
		const query = `
SELECT COUNT(*)::bigint, MAX(created_at), false, bool_or(threshold_status = 'blocked'), bool_or(hidden_evidence_count > 0)
FROM ranking_rollout_dry_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	case assurance.ExpectedEvidenceWorkflow:
		const query = `
SELECT
	COUNT(DISTINCT wr.id) FILTER (WHERE wr.status = 'completed')::bigint,
	GREATEST(MAX(wr.updated_at), MAX(wsr.observed_at), MAX(wgd.created_at)),
	false,
	COALESCE(bool_or(wgd.status = 'open' AND wgd.readiness_impact IN ('degraded', 'blocked')), false),
	COALESCE(bool_or(wgd.category = 'hidden'), false)
FROM integration_workflow_runs wr
LEFT JOIN integration_workflow_step_records wsr
	ON wsr.run_id = wr.id
	AND wsr.tenant = wr.tenant
	AND wsr.project = wr.project
	AND wsr.namespace = wr.namespace
LEFT JOIN integration_workflow_gap_diagnostics wgd
	ON wgd.run_id = wr.id
	AND wgd.tenant = wr.tenant
	AND wgd.project = wr.project
	AND wgd.namespace = wr.namespace
WHERE wr.tenant = $1 AND wr.project = $2 AND wr.namespace = $3
`
		return scanConformanceEvidenceObservation(kind, r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace))
	default:
		return assurance.ConformanceEvidenceObservation{Kind: kind, OutOfScope: true}, nil
	}
}

func (r *Repository) CreateConformanceRun(ctx context.Context, run assurance.ConformanceRun) (assurance.ConformanceRun, error) {
	if err := run.Validate(); err != nil {
		return assurance.ConformanceRun{}, err
	}
	evidenceCounts, err := json.Marshal(normalizeAnyMap(run.EvidenceCounts))
	if err != nil {
		return assurance.ConformanceRun{}, fmt.Errorf("marshal conformance evidence counts: %w", err)
	}
	const query = `
INSERT INTO assurance_conformance_runs (
	id, profile_id, tenant, project, namespace, result, evidence_counts, started_at, finished_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, profile_id, tenant, project, namespace, result, evidence_counts, started_at, finished_at, created_at
`
	return scanConformanceRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.ProfileID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.Result,
		evidenceCounts,
		run.StartedAt,
		nullableTime(run.FinishedAt),
		run.CreatedAt,
	))
}

func (r *Repository) ReadConformanceRun(ctx context.Context, input assurance.ReadConformanceRunInput) (assurance.ConformanceRun, error) {
	if err := input.Validate(); err != nil {
		return assurance.ConformanceRun{}, err
	}
	const query = `
SELECT id, profile_id, tenant, project, namespace, result, evidence_counts, started_at, finished_at, created_at
FROM assurance_conformance_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanConformanceRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID))
}

func (r *Repository) ListConformanceRuns(ctx context.Context, input assurance.ListConformanceRunsInput) ([]assurance.ConformanceRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, profile_id, tenant, project, namespace, result, evidence_counts, started_at, finished_at, created_at
FROM assurance_conformance_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR profile_id = $4)
ORDER BY created_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("list conformance runs: %w", err)
	}
	defer rows.Close()
	runs := make([]assurance.ConformanceRun, 0)
	for rows.Next() {
		run, err := scanConformanceRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conformance runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) CreateMissingEvidenceDiagnostic(ctx context.Context, diagnostic assurance.MissingEvidenceDiagnostic) (assurance.MissingEvidenceDiagnostic, error) {
	if err := diagnostic.Validate(); err != nil {
		return assurance.MissingEvidenceDiagnostic{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(diagnostic.Metadata))
	if err != nil {
		return assurance.MissingEvidenceDiagnostic{}, fmt.Errorf("marshal missing evidence metadata: %w", err)
	}
	const query = `
INSERT INTO assurance_missing_evidence_diagnostics (
	id, conformance_run_id, tenant, project, namespace, evidence_kind, category, readiness_impact, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, conformance_run_id, tenant, project, namespace, evidence_kind, category, readiness_impact, metadata, created_at
`
	return scanMissingEvidenceDiagnostic(r.db.QueryRow(
		ctx,
		query,
		diagnostic.ID,
		diagnostic.ConformanceRunID,
		diagnostic.Scope.Tenant,
		diagnostic.Scope.Project,
		diagnostic.Scope.Namespace,
		diagnostic.EvidenceKind,
		diagnostic.Category,
		diagnostic.ReadinessImpact,
		metadata,
		diagnostic.CreatedAt,
	))
}

func (r *Repository) CreateOperationalProof(ctx context.Context, proof assurance.OperationalProof) (assurance.OperationalProof, error) {
	if err := proof.Validate(); err != nil {
		return assurance.OperationalProof{}, err
	}
	evidence, err := json.Marshal(normalizeAnyMap(proof.Evidence))
	if err != nil {
		return assurance.OperationalProof{}, fmt.Errorf("marshal operational proof evidence: %w", err)
	}
	const query = `
INSERT INTO assurance_operational_proofs (
	id, tenant, project, namespace, target, status, severity, reason, observed_at, fresh_through, evidence, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, tenant, project, namespace, target, status, severity, reason, observed_at, fresh_through, evidence, created_at
`
	return scanOperationalProof(r.db.QueryRow(
		ctx,
		query,
		proof.ID,
		proof.Scope.Tenant,
		proof.Scope.Project,
		proof.Scope.Namespace,
		proof.Target,
		proof.Status,
		proof.Severity,
		proof.Reason,
		proof.ObservedAt,
		nullableTime(proof.FreshThrough),
		evidence,
		proof.CreatedAt,
	))
}

func (r *Repository) CreateRetentionRun(ctx context.Context, run assurance.RetentionRun) (assurance.RetentionRun, error) {
	if err := run.Validate(); err != nil {
		return assurance.RetentionRun{}, err
	}
	const query = `
INSERT INTO assurance_retention_runs (
	id, tenant, project, namespace, record_category, cutoff, deleted_count, status, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, tenant, project, namespace, record_category, cutoff, deleted_count, status, started_at, finished_at
`
	return scanRetentionRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.RecordCategory,
		run.Cutoff,
		run.DeletedCount,
		run.Status,
		run.StartedAt,
		nullableTime(run.FinishedAt),
	))
}

func (r *Repository) ReadRetentionRun(ctx context.Context, input assurance.ReadRetentionRunInput) (assurance.RetentionRun, error) {
	if err := input.Validate(); err != nil {
		return assurance.RetentionRun{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, record_category, cutoff, deleted_count, status, started_at, finished_at
FROM assurance_retention_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanRetentionRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID))
}

func (r *Repository) ListRetentionRuns(ctx context.Context, input assurance.ListRetentionRunsInput) ([]assurance.RetentionRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, record_category, cutoff, deleted_count, status, started_at, finished_at
FROM assurance_retention_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR record_category = $4)
ORDER BY started_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, string(input.RecordCategory))
	if err != nil {
		return nil, fmt.Errorf("list retention runs: %w", err)
	}
	defer rows.Close()
	runs := make([]assurance.RetentionRun, 0)
	for rows.Next() {
		run, err := scanRetentionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) ListOperationalProofs(ctx context.Context, scope memory.Scope) ([]assurance.OperationalProof, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, target, status, severity, reason, observed_at, fresh_through, evidence, created_at
FROM assurance_operational_proofs
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY observed_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list operational proofs: %w", err)
	}
	defer rows.Close()
	proofs := make([]assurance.OperationalProof, 0)
	for rows.Next() {
		proof, err := scanOperationalProof(rows)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational proofs: %w", err)
	}
	return proofs, nil
}

func (r *Repository) ReadOperationalProof(ctx context.Context, input assurance.ReadOperationalProofInput) (assurance.OperationalProof, error) {
	if err := input.Validate(); err != nil {
		return assurance.OperationalProof{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, target, status, severity, reason, observed_at, fresh_through, evidence, created_at
FROM assurance_operational_proofs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanOperationalProof(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.ProofID))
}

func (r *Repository) CreateReadinessReport(ctx context.Context, report assurance.ReadinessReport) (assurance.ReadinessReport, error) {
	if err := report.Validate(); err != nil {
		return assurance.ReadinessReport{}, err
	}
	componentSummary, err := json.Marshal(normalizeAnyMap(report.ComponentSummary))
	if err != nil {
		return assurance.ReadinessReport{}, fmt.Errorf("marshal readiness component summary: %w", err)
	}
	const query = `
INSERT INTO assurance_readiness_reports (
	id, tenant, project, namespace, status, health_evaluation_id, conformance_run_id,
	component_summary, recommended_actions, generated_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, tenant, project, namespace, status, health_evaluation_id, conformance_run_id,
	component_summary, recommended_actions, generated_at, created_at
`
	return scanReadinessReport(r.db.QueryRow(
		ctx,
		query,
		report.ID,
		report.Scope.Tenant,
		report.Scope.Project,
		report.Scope.Namespace,
		report.Status,
		nullableString(report.HealthEvaluationID),
		nullableString(report.ConformanceRunID),
		componentSummary,
		runbookHintStrings(report.RecommendedActions),
		report.GeneratedAt,
		report.CreatedAt,
	))
}

func (r *Repository) ListReadinessReports(ctx context.Context, scope memory.Scope) ([]assurance.ReadinessReport, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, health_evaluation_id, conformance_run_id,
	component_summary, recommended_actions, generated_at, created_at
FROM assurance_readiness_reports
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY generated_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list readiness reports: %w", err)
	}
	defer rows.Close()
	reports := make([]assurance.ReadinessReport, 0)
	for rows.Next() {
		report, err := scanReadinessReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate readiness reports: %w", err)
	}
	return reports, nil
}

func (r *Repository) ReadReadinessReport(ctx context.Context, input assurance.ReadReadinessReportInput) (assurance.ReadinessReport, error) {
	if err := input.Validate(); err != nil {
		return assurance.ReadinessReport{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, health_evaluation_id, conformance_run_id,
	component_summary, recommended_actions, generated_at, created_at
FROM assurance_readiness_reports
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanReadinessReport(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.ReportID))
}

func (r *Repository) CreateRecoveryVerification(ctx context.Context, verification assurance.RecoveryVerification) (assurance.RecoveryVerification, error) {
	if err := verification.Validate(); err != nil {
		return assurance.RecoveryVerification{}, err
	}
	linkedEvidence, err := json.Marshal(normalizeAnyMap(verification.LinkedEvidence))
	if err != nil {
		return assurance.RecoveryVerification{}, fmt.Errorf("marshal recovery linked evidence: %w", err)
	}
	const query = `
INSERT INTO assurance_recovery_verifications (
	id, tenant, project, namespace, target_kind, target_id, status, checked_surfaces,
	result_category, linked_evidence, actor, reason, created_at, verified_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, tenant, project, namespace, target_kind, target_id, status, checked_surfaces,
	result_category, linked_evidence, actor, reason, created_at, verified_at
`
	return scanRecoveryVerification(r.db.QueryRow(
		ctx,
		query,
		verification.ID,
		verification.Scope.Tenant,
		verification.Scope.Project,
		verification.Scope.Namespace,
		verification.Target,
		verification.TargetID,
		verification.Status,
		verification.CheckedSurfaces,
		verification.ResultCategory,
		linkedEvidence,
		verification.Actor,
		verification.Reason,
		verification.CreatedAt,
		nullableTime(verification.VerifiedAt),
	))
}

func (r *Repository) ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]assurance.RecoveryVerification, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, tenant, project, namespace, target_kind, target_id, status, checked_surfaces,
	result_category, linked_evidence, actor, reason, created_at, verified_at
FROM assurance_recovery_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY created_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list recovery verifications: %w", err)
	}
	defer rows.Close()
	records := make([]assurance.RecoveryVerification, 0)
	for rows.Next() {
		record, err := scanRecoveryVerification(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovery verifications: %w", err)
	}
	return records, nil
}

func (r *Repository) ReadRecoveryVerification(ctx context.Context, input assurance.ReadRecoveryVerificationInput) (assurance.RecoveryVerification, error) {
	if err := input.Validate(); err != nil {
		return assurance.RecoveryVerification{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, target_kind, target_id, status, checked_surfaces,
	result_category, linked_evidence, actor, reason, created_at, verified_at
FROM assurance_recovery_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanRecoveryVerification(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RecordID))
}

func createHealthComponent(ctx context.Context, db queryRower, component assurance.HealthComponentSummary) (assurance.HealthComponentSummary, error) {
	if err := component.Validate(); err != nil {
		return assurance.HealthComponentSummary{}, err
	}
	evidence, err := json.Marshal(normalizeAnyMap(component.Evidence))
	if err != nil {
		return assurance.HealthComponentSummary{}, fmt.Errorf("marshal health component evidence: %w", err)
	}
	const query = `
INSERT INTO assurance_health_components (
	id, evaluation_id, tenant, project, namespace, component, status, severity, reason, observed_at, fresh_through, evidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, evaluation_id, tenant, project, namespace, component, status, severity, reason, observed_at, fresh_through, evidence
`
	return scanHealthComponent(db.QueryRow(
		ctx,
		query,
		component.ID,
		component.EvaluationID,
		component.Scope.Tenant,
		component.Scope.Project,
		component.Scope.Namespace,
		component.Component,
		component.Status,
		component.Severity,
		component.Reason,
		component.ObservedAt,
		nullableTime(component.FreshThrough),
		evidence,
	))
}

func (r *Repository) listHealthComponents(ctx context.Context, scope memory.Scope, evaluationID string) ([]assurance.HealthComponentSummary, error) {
	const query = `
SELECT id, evaluation_id, tenant, project, namespace, component, status, severity, reason, observed_at, fresh_through, evidence
FROM assurance_health_components
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND evaluation_id = $4
ORDER BY observed_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, evaluationID)
	if err != nil {
		return nil, fmt.Errorf("list health components: %w", err)
	}
	defer rows.Close()
	components := make([]assurance.HealthComponentSummary, 0)
	for rows.Next() {
		component, err := scanHealthComponent(rows)
		if err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health components: %w", err)
	}
	return components, nil
}

func scanHealthEvaluation(scanner provenanceScanner) (assurance.HealthEvaluation, error) {
	var evaluation assurance.HealthEvaluation
	if err := scanner.Scan(
		&evaluation.ID,
		&evaluation.Scope.Tenant,
		&evaluation.Scope.Project,
		&evaluation.Scope.Namespace,
		&evaluation.Status,
		&evaluation.Severity,
		&evaluation.Reason,
		&evaluation.CreatedAt,
	); err != nil {
		return assurance.HealthEvaluation{}, fmt.Errorf("scan health evaluation: %w", err)
	}
	return evaluation, nil
}

func scanHealthComponent(scanner provenanceScanner) (assurance.HealthComponentSummary, error) {
	var component assurance.HealthComponentSummary
	var freshThrough sql.NullTime
	var evidence []byte
	if err := scanner.Scan(
		&component.ID,
		&component.EvaluationID,
		&component.Scope.Tenant,
		&component.Scope.Project,
		&component.Scope.Namespace,
		&component.Component,
		&component.Status,
		&component.Severity,
		&component.Reason,
		&component.ObservedAt,
		&freshThrough,
		&evidence,
	); err != nil {
		return assurance.HealthComponentSummary{}, fmt.Errorf("scan health component: %w", err)
	}
	if freshThrough.Valid {
		component.FreshThrough = freshThrough.Time
	}
	component.Evidence = map[string]any{}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &component.Evidence); err != nil {
			return assurance.HealthComponentSummary{}, fmt.Errorf("unmarshal health component evidence: %w", err)
		}
	}
	return component, nil
}

func scanIncident(scanner provenanceScanner) (assurance.Incident, error) {
	var incident assurance.Incident
	var resolvedAt sql.NullTime
	var latestEvaluationID sql.NullString
	var hints []string
	var metadata []byte
	if err := scanner.Scan(
		&incident.ID,
		&incident.Scope.Tenant,
		&incident.Scope.Project,
		&incident.Scope.Namespace,
		&incident.Status,
		&incident.Severity,
		&incident.Component,
		&incident.Reason,
		&incident.DeduplicationKey,
		&incident.OpenedAt,
		&incident.UpdatedAt,
		&resolvedAt,
		&latestEvaluationID,
		&hints,
		&metadata,
	); err != nil {
		return assurance.Incident{}, fmt.Errorf("scan incident: %w", err)
	}
	if resolvedAt.Valid {
		incident.ResolvedAt = resolvedAt.Time
	}
	if latestEvaluationID.Valid {
		incident.LatestEvaluationID = latestEvaluationID.String
	}
	incident.RunbookHints = runbookHints(hints)
	incident.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &incident.Metadata); err != nil {
			return assurance.Incident{}, fmt.Errorf("unmarshal incident metadata: %w", err)
		}
	}
	return incident, nil
}

func scanIncidentTransition(scanner provenanceScanner) (assurance.IncidentTransition, error) {
	var transition assurance.IncidentTransition
	var fromStatus sql.NullString
	if err := scanner.Scan(
		&transition.ID,
		&transition.IncidentID,
		&transition.Scope.Tenant,
		&transition.Scope.Project,
		&transition.Scope.Namespace,
		&fromStatus,
		&transition.ToStatus,
		&transition.Action,
		&transition.Actor,
		&transition.Reason,
		&transition.OccurredAt,
	); err != nil {
		return assurance.IncidentTransition{}, fmt.Errorf("scan incident transition: %w", err)
	}
	if fromStatus.Valid {
		transition.FromStatus = assurance.IncidentStatus(fromStatus.String)
	}
	return transition, nil
}

func scanAlertCandidate(scanner provenanceScanner) (assurance.AlertCandidate, error) {
	var candidate assurance.AlertCandidate
	var incidentID sql.NullString
	var evaluationID sql.NullString
	var payload []byte
	var nextAttemptAt sql.NullTime
	var suppressedUntil sql.NullTime
	if err := scanner.Scan(
		&candidate.ID,
		&candidate.Scope.Tenant,
		&candidate.Scope.Project,
		&candidate.Scope.Namespace,
		&incidentID,
		&evaluationID,
		&candidate.Severity,
		&candidate.Component,
		&candidate.Reason,
		&candidate.DeduplicationKey,
		&candidate.DeliveryPolicy,
		&payload,
		&candidate.CreatedAt,
		&nextAttemptAt,
		&suppressedUntil,
	); err != nil {
		return assurance.AlertCandidate{}, fmt.Errorf("scan alert candidate: %w", err)
	}
	if incidentID.Valid {
		candidate.IncidentID = incidentID.String
	}
	if evaluationID.Valid {
		candidate.EvaluationID = evaluationID.String
	}
	if nextAttemptAt.Valid {
		candidate.NextAttemptAt = nextAttemptAt.Time
	}
	if suppressedUntil.Valid {
		candidate.SuppressedUntil = suppressedUntil.Time
	}
	candidate.Payload = map[string]any{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &candidate.Payload); err != nil {
			return assurance.AlertCandidate{}, fmt.Errorf("unmarshal alert payload: %w", err)
		}
	}
	return candidate, nil
}

func scanAlertDeliveryAttempt(scanner provenanceScanner) (assurance.AlertDeliveryAttempt, error) {
	var attempt assurance.AlertDeliveryAttempt
	var failureCategory sql.NullString
	var workerID sql.NullString
	var leaseUntil sql.NullTime
	var nextAttemptAt sql.NullTime
	var payloadHash sql.NullString
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&attempt.ID,
		&attempt.AlertCandidateID,
		&attempt.Scope.Tenant,
		&attempt.Scope.Project,
		&attempt.Scope.Namespace,
		&attempt.Adapter,
		&attempt.Result,
		&failureCategory,
		&attempt.Attempt,
		&workerID,
		&leaseUntil,
		&nextAttemptAt,
		&payloadHash,
		&attempt.AttemptedAt,
		&completedAt,
	); err != nil {
		return assurance.AlertDeliveryAttempt{}, fmt.Errorf("scan alert delivery attempt: %w", err)
	}
	if failureCategory.Valid {
		attempt.FailureCategory = failureCategory.String
	}
	if workerID.Valid {
		attempt.WorkerID = workerID.String
	}
	if leaseUntil.Valid {
		attempt.LeaseUntil = leaseUntil.Time
	}
	if nextAttemptAt.Valid {
		attempt.NextAttemptAt = nextAttemptAt.Time
	}
	if payloadHash.Valid {
		attempt.PayloadHash = payloadHash.String
	}
	if completedAt.Valid {
		attempt.CompletedAt = completedAt.Time
	}
	return attempt, nil
}

func scanAlertDeliveryClaim(scanner provenanceScanner) (assurance.AlertDeliveryClaim, error) {
	var claim assurance.AlertDeliveryClaim
	var incidentID sql.NullString
	var evaluationID sql.NullString
	var payload []byte
	var nextAttemptAt sql.NullTime
	var suppressedUntil sql.NullTime
	if err := scanner.Scan(
		&claim.Candidate.ID,
		&claim.Candidate.Scope.Tenant,
		&claim.Candidate.Scope.Project,
		&claim.Candidate.Scope.Namespace,
		&incidentID,
		&evaluationID,
		&claim.Candidate.Severity,
		&claim.Candidate.Component,
		&claim.Candidate.Reason,
		&claim.Candidate.DeduplicationKey,
		&claim.Candidate.DeliveryPolicy,
		&payload,
		&claim.Candidate.CreatedAt,
		&nextAttemptAt,
		&suppressedUntil,
		&claim.Attempt,
		&claim.WorkerID,
		&claim.ClaimedAt,
		&claim.LeaseUntil,
	); err != nil {
		return assurance.AlertDeliveryClaim{}, fmt.Errorf("scan alert delivery claim: %w", err)
	}
	if incidentID.Valid {
		claim.Candidate.IncidentID = incidentID.String
	}
	if evaluationID.Valid {
		claim.Candidate.EvaluationID = evaluationID.String
	}
	if nextAttemptAt.Valid {
		claim.Candidate.NextAttemptAt = nextAttemptAt.Time
	}
	if suppressedUntil.Valid {
		claim.Candidate.SuppressedUntil = suppressedUntil.Time
	}
	claim.Candidate.Payload = map[string]any{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &claim.Candidate.Payload); err != nil {
			return assurance.AlertDeliveryClaim{}, fmt.Errorf("unmarshal alert payload: %w", err)
		}
	}
	return claim, nil
}

func scanConformanceProfile(scanner provenanceScanner) (assurance.ConformanceProfile, error) {
	var profile assurance.ConformanceProfile
	var expectedEvidence []byte
	var disabledAt sql.NullTime
	if err := scanner.Scan(
		&profile.ID,
		&profile.Scope.Tenant,
		&profile.Scope.Project,
		&profile.Scope.Namespace,
		&profile.Status,
		&expectedEvidence,
		&profile.Actor,
		&profile.Reason,
		&profile.CreatedAt,
		&profile.UpdatedAt,
		&disabledAt,
	); err != nil {
		return assurance.ConformanceProfile{}, fmt.Errorf("scan conformance profile: %w", err)
	}
	if len(expectedEvidence) > 0 {
		if err := json.Unmarshal(expectedEvidence, &profile.ExpectedEvidence); err != nil {
			return assurance.ConformanceProfile{}, fmt.Errorf("unmarshal expected evidence: %w", err)
		}
	}
	if disabledAt.Valid {
		profile.DisabledAt = disabledAt.Time
	}
	return profile, nil
}

func scanConformanceRun(scanner provenanceScanner) (assurance.ConformanceRun, error) {
	var run assurance.ConformanceRun
	var evidenceCounts []byte
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.ProfileID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.Result,
		&evidenceCounts,
		&run.StartedAt,
		&finishedAt,
		&run.CreatedAt,
	); err != nil {
		return assurance.ConformanceRun{}, fmt.Errorf("scan conformance run: %w", err)
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	run.EvidenceCounts = map[string]any{}
	if len(evidenceCounts) > 0 {
		if err := json.Unmarshal(evidenceCounts, &run.EvidenceCounts); err != nil {
			return assurance.ConformanceRun{}, fmt.Errorf("unmarshal conformance evidence counts: %w", err)
		}
	}
	return run, nil
}

func scanConformanceEvidenceObservation(kind assurance.ExpectedEvidenceKind, scanner provenanceScanner) (assurance.ConformanceEvidenceObservation, error) {
	var count int64
	var freshestAt sql.NullTime
	var opaqueOnly sql.NullBool
	var contradictory sql.NullBool
	var hidden sql.NullBool
	if err := scanner.Scan(&count, &freshestAt, &opaqueOnly, &contradictory, &hidden); err != nil {
		return assurance.ConformanceEvidenceObservation{}, fmt.Errorf("scan conformance evidence observation: %w", err)
	}
	observation := assurance.ConformanceEvidenceObservation{
		Kind:          kind,
		Count:         int(count),
		OpaqueOnly:    opaqueOnly.Valid && opaqueOnly.Bool,
		Contradictory: contradictory.Valid && contradictory.Bool,
		Hidden:        hidden.Valid && hidden.Bool,
	}
	if freshestAt.Valid {
		observation.FreshestAt = freshestAt.Time
	}
	return observation, nil
}

func scanMissingEvidenceDiagnostic(scanner provenanceScanner) (assurance.MissingEvidenceDiagnostic, error) {
	var diagnostic assurance.MissingEvidenceDiagnostic
	var metadata []byte
	if err := scanner.Scan(
		&diagnostic.ID,
		&diagnostic.ConformanceRunID,
		&diagnostic.Scope.Tenant,
		&diagnostic.Scope.Project,
		&diagnostic.Scope.Namespace,
		&diagnostic.EvidenceKind,
		&diagnostic.Category,
		&diagnostic.ReadinessImpact,
		&metadata,
		&diagnostic.CreatedAt,
	); err != nil {
		return assurance.MissingEvidenceDiagnostic{}, fmt.Errorf("scan missing evidence diagnostic: %w", err)
	}
	diagnostic.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &diagnostic.Metadata); err != nil {
			return assurance.MissingEvidenceDiagnostic{}, fmt.Errorf("unmarshal missing evidence metadata: %w", err)
		}
	}
	return diagnostic, nil
}

func scanOperationalProof(scanner provenanceScanner) (assurance.OperationalProof, error) {
	var proof assurance.OperationalProof
	var freshThrough sql.NullTime
	var evidence []byte
	if err := scanner.Scan(
		&proof.ID,
		&proof.Scope.Tenant,
		&proof.Scope.Project,
		&proof.Scope.Namespace,
		&proof.Target,
		&proof.Status,
		&proof.Severity,
		&proof.Reason,
		&proof.ObservedAt,
		&freshThrough,
		&evidence,
		&proof.CreatedAt,
	); err != nil {
		return assurance.OperationalProof{}, fmt.Errorf("scan operational proof: %w", err)
	}
	if freshThrough.Valid {
		proof.FreshThrough = freshThrough.Time
	}
	proof.Evidence = map[string]any{}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &proof.Evidence); err != nil {
			return assurance.OperationalProof{}, fmt.Errorf("unmarshal operational proof evidence: %w", err)
		}
	}
	return proof, nil
}

func scanReadinessReport(scanner provenanceScanner) (assurance.ReadinessReport, error) {
	var report assurance.ReadinessReport
	var healthEvaluationID sql.NullString
	var conformanceRunID sql.NullString
	var componentSummary []byte
	var recommendedActions []string
	if err := scanner.Scan(
		&report.ID,
		&report.Scope.Tenant,
		&report.Scope.Project,
		&report.Scope.Namespace,
		&report.Status,
		&healthEvaluationID,
		&conformanceRunID,
		&componentSummary,
		&recommendedActions,
		&report.GeneratedAt,
		&report.CreatedAt,
	); err != nil {
		return assurance.ReadinessReport{}, fmt.Errorf("scan readiness report: %w", err)
	}
	if healthEvaluationID.Valid {
		report.HealthEvaluationID = healthEvaluationID.String
	}
	if conformanceRunID.Valid {
		report.ConformanceRunID = conformanceRunID.String
	}
	report.ComponentSummary = map[string]any{}
	if len(componentSummary) > 0 {
		if err := json.Unmarshal(componentSummary, &report.ComponentSummary); err != nil {
			return assurance.ReadinessReport{}, fmt.Errorf("unmarshal readiness component summary: %w", err)
		}
	}
	report.RecommendedActions = runbookHints(recommendedActions)
	return report, nil
}

func scanRecoveryVerification(scanner provenanceScanner) (assurance.RecoveryVerification, error) {
	var verification assurance.RecoveryVerification
	var linkedEvidence []byte
	var verifiedAt sql.NullTime
	if err := scanner.Scan(
		&verification.ID,
		&verification.Scope.Tenant,
		&verification.Scope.Project,
		&verification.Scope.Namespace,
		&verification.Target,
		&verification.TargetID,
		&verification.Status,
		&verification.CheckedSurfaces,
		&verification.ResultCategory,
		&linkedEvidence,
		&verification.Actor,
		&verification.Reason,
		&verification.CreatedAt,
		&verifiedAt,
	); err != nil {
		return assurance.RecoveryVerification{}, fmt.Errorf("scan recovery verification: %w", err)
	}
	if verifiedAt.Valid {
		verification.VerifiedAt = verifiedAt.Time
	}
	verification.LinkedEvidence = map[string]any{}
	if len(linkedEvidence) > 0 {
		if err := json.Unmarshal(linkedEvidence, &verification.LinkedEvidence); err != nil {
			return assurance.RecoveryVerification{}, fmt.Errorf("unmarshal recovery linked evidence: %w", err)
		}
	}
	return verification, nil
}

func scanRetentionRun(scanner provenanceScanner) (assurance.RetentionRun, error) {
	var run assurance.RetentionRun
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.RecordCategory,
		&run.Cutoff,
		&run.DeletedCount,
		&run.Status,
		&run.StartedAt,
		&finishedAt,
	); err != nil {
		return assurance.RetentionRun{}, fmt.Errorf("scan retention run: %w", err)
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	return run, nil
}

func runbookHintStrings(hints []assurance.RunbookHintCategory) []string {
	values := make([]string, 0, len(hints))
	for _, hint := range hints {
		values = append(values, string(hint))
	}
	return values
}

func runbookHints(values []string) []assurance.RunbookHintCategory {
	hints := make([]assurance.RunbookHintCategory, 0, len(values))
	for _, value := range values {
		hints = append(hints, assurance.RunbookHintCategory(value))
	}
	return hints
}

func timeForResolvedStatus(status assurance.IncidentStatus, at time.Time) time.Time {
	if status != assurance.IncidentStatusResolved {
		return time.Time{}
	}
	return at
}
