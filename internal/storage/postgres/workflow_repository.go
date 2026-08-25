package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ReadWorkflowHealth(ctx context.Context, scope memory.Scope, observedAt time.Time) (assurance.WorkflowHealthSnapshot, error) {
	if err := scope.Validate(); err != nil {
		return assurance.WorkflowHealthSnapshot{}, err
	}
	if observedAt.IsZero() {
		return assurance.WorkflowHealthSnapshot{}, fmt.Errorf("workflow health observed at is required")
	}
	const query = `
SELECT
	COUNT(DISTINCT wr.id) FILTER (WHERE wr.status = 'completed')::bigint,
	COUNT(DISTINCT wr.id) FILTER (WHERE wr.status IN ('running', 'blocked', 'abandoned'))::bigint,
	COUNT(DISTINCT wr.id) FILTER (WHERE wr.status = 'expired' OR (wr.status = 'running' AND wr.expires_at IS NOT NULL AND wr.expires_at < $4))::bigint,
	COUNT(DISTINCT wgd.id) FILTER (WHERE wgd.status = 'open' AND wgd.readiness_impact IN ('degraded', 'blocked'))::bigint,
	GREATEST(MAX(wr.updated_at), MAX(wsr.observed_at), MAX(wgd.created_at))
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
	var completed, incomplete, stale, blocking int64
	var latest sql.NullTime
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, observedAt.UTC()).Scan(&completed, &incomplete, &stale, &blocking, &latest); err != nil {
		return assurance.WorkflowHealthSnapshot{}, fmt.Errorf("read workflow health: %w", err)
	}
	snapshot := assurance.WorkflowHealthSnapshot{
		Scope:               scope.Normalized(),
		CompletedRuns:       int(completed),
		IncompleteRuns:      int(incomplete),
		StaleRuns:           int(stale),
		BlockingDiagnostics: int(blocking),
		Status:              assurance.HealthStatusHealthy,
		Reason:              assurance.ReasonWorkflowGap,
	}
	if latest.Valid {
		snapshot.LatestObservedAt = latest.Time
	}
	switch {
	case snapshot.CompletedRuns == 0 && snapshot.IncompleteRuns == 0 && snapshot.StaleRuns == 0:
		snapshot.Status = assurance.HealthStatusUnknown
	case snapshot.BlockingDiagnostics > 0 || snapshot.StaleRuns > 0:
		snapshot.Status = assurance.HealthStatusUnhealthy
	case snapshot.IncompleteRuns > 0:
		snapshot.Status = assurance.HealthStatusDegraded
	}
	return snapshot, nil
}

// VerifyWorkflowEvidence is the single production boundary for workflow links.
// It reads only durable evidence metadata and never copies source payloads into a workflow.
func (r *Repository) VerifyWorkflowEvidence(ctx context.Context, input workflow.EvidenceVerificationInput) (workflow.EvidenceVerificationResult, error) {
	if err := input.Scope.Validate(); err != nil {
		return workflow.EvidenceVerificationResult{}, err
	}
	if !input.Kind.Valid() || input.Kind == workflow.EvidenceKindOpaque || input.TargetID == "" {
		return workflow.EvidenceVerificationResult{}, fmt.Errorf("valid internal workflow evidence kind and target id are required")
	}
	query, err := workflowEvidenceVerificationQuery(input.Kind)
	if err != nil {
		return workflow.EvidenceVerificationResult{}, err
	}
	var result workflow.EvidenceVerificationResult
	if err := r.db.QueryRow(ctx, query, input.TargetID).Scan(
		&result.Scope.Tenant,
		&result.Scope.Project,
		&result.Scope.Namespace,
		&result.Hidden,
		&result.HasSubject,
		&result.HasSufficientEvidence,
		&result.Contradictory,
	); err != nil {
		if err == pgx.ErrNoRows {
			return workflow.EvidenceVerificationResult{}, nil
		}
		return workflow.EvidenceVerificationResult{}, fmt.Errorf("verify workflow evidence: %w", err)
	}
	result.Exists = true
	return result, nil
}

func workflowEvidenceVerificationQuery(kind workflow.EvidenceKind) (string, error) {
	const ordinary = `SELECT tenant, project, namespace, false, true, true, false FROM %s WHERE id::text = $1 LIMIT 1`
	switch kind {
	case workflow.EvidenceKindSession:
		return fmt.Sprintf(ordinary, "memory_session_runs"), nil
	case workflow.EvidenceKindTurn, workflow.EvidenceKindContext:
		return fmt.Sprintf(ordinary, "memory_session_turns"), nil
	case workflow.EvidenceKindOutcome:
		return fmt.Sprintf(ordinary, "raw_events"), nil
	case workflow.EvidenceKindVerification:
		return fmt.Sprintf(ordinary, "memory_session_verifications"), nil
	case workflow.EvidenceKindProof:
		return fmt.Sprintf(ordinary, "scope_proof_runs"), nil
	case workflow.EvidenceKindQualityFinding:
		return fmt.Sprintf(ordinary, "quality_evaluation_findings"), nil
	case workflow.EvidenceKindRepairPlan:
		return fmt.Sprintf(ordinary, "repair_plans"), nil
	case workflow.EvidenceKindRankingRollout:
		return fmt.Sprintf(ordinary, "ranking_rollout_policies"), nil
	case workflow.EvidenceKindConformanceRun:
		return fmt.Sprintf(ordinary, "assurance_conformance_runs"), nil
	case workflow.EvidenceKindReadinessReport:
		return fmt.Sprintf(ordinary, "assurance_readiness_reports"), nil
	case workflow.EvidenceKindIncident:
		return fmt.Sprintf(ordinary, "assurance_incidents"), nil
	case workflow.EvidenceKindRecoveryVerification:
		return fmt.Sprintf(ordinary, "assurance_recovery_verifications"), nil
	case workflow.EvidenceKindUsefulnessFeedback:
		return `
SELECT f.tenant, f.project, f.namespace,
       f.superseded_at IS NOT NULL,
       EXISTS (SELECT 1 FROM usefulness_feedback_subjects s WHERE s.feedback_id = f.id AND s.tenant = f.tenant AND s.project = f.project AND s.namespace = f.namespace),
       true, false
FROM usefulness_feedback f WHERE f.id = $1 LIMIT 1`, nil
	case workflow.EvidenceKindTaskEvaluation:
		return `
SELECT e.tenant, e.project, e.namespace,
       e.superseded_at IS NOT NULL OR e.correction_state <> 'active',
       true,
       EXISTS (SELECT 1 FROM task_evidence_links l WHERE l.task_evaluation_id = e.id AND l.tenant = e.tenant AND l.project = e.project AND l.namespace = e.namespace AND l.evidence_id IS NOT NULL),
       false
FROM task_evaluations e WHERE e.id::text = $1 LIMIT 1`, nil
	default:
		return "", fmt.Errorf("workflow evidence kind %q is not verifiable", kind)
	}
}

func (r *Repository) DeleteWorkflowHistoryBefore(ctx context.Context, scope memory.Scope, cutoff time.Time) (int, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("workflow history cutoff is required")
	}
	const deleteRuns = `
DELETE FROM integration_workflow_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND status IN ('completed', 'abandoned', 'expired')
	AND updated_at < $4
`
	result, err := r.db.Exec(ctx, deleteRuns, scope.Tenant, scope.Project, scope.Namespace, cutoff.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete workflow history: %w", err)
	}
	return int(result.RowsAffected()), nil
}

func (r *Repository) CreateWorkflowTemplate(ctx context.Context, template workflow.WorkflowTemplate) (workflow.WorkflowTemplate, error) {
	if err := template.Validate(); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("begin workflow template transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := insertWorkflowTemplate(ctx, tx, template)
	if err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	created.Steps = make([]workflow.TemplateStep, 0, len(template.Steps))
	for _, step := range template.Steps {
		if step.TemplateID == "" {
			step.TemplateID = created.ID
		}
		if step.Scope == (memory.Scope{}) {
			step.Scope = created.Scope
		}
		inserted, err := insertWorkflowTemplateStep(ctx, tx, step)
		if err != nil {
			return workflow.WorkflowTemplate{}, err
		}
		created.Steps = append(created.Steps, inserted)
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("commit workflow template transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) ReadWorkflowTemplate(ctx context.Context, input workflow.ReadTemplateInput) (workflow.WorkflowTemplate, error) {
	if err := input.Validate(); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
FROM integration_workflow_templates
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	template, err := scanWorkflowTemplate(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.TemplateID))
	if err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	steps, err := r.listWorkflowTemplateSteps(ctx, input.Scope, input.TemplateID)
	if err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	template.Steps = steps
	return template, nil
}

func (r *Repository) ListWorkflowTemplates(ctx context.Context, input workflow.ListTemplatesInput) ([]workflow.WorkflowTemplate, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
FROM integration_workflow_templates
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR status = $4)
ORDER BY updated_at DESC, id ASC
LIMIT $5
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, string(input.Status), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow templates: %w", err)
	}
	defer rows.Close()
	templates := make([]workflow.WorkflowTemplate, 0)
	for rows.Next() {
		template, err := scanWorkflowTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow templates: %w", err)
	}
	return templates, nil
}

func (r *Repository) UpdateWorkflowTemplate(ctx context.Context, input workflow.UpdateTemplateInput) (workflow.WorkflowTemplate, error) {
	if err := input.Validate(); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(input.Metadata))
	if err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("marshal workflow template metadata: %w", err)
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("begin workflow template update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQuery = `
UPDATE integration_workflow_templates
SET integration_kind = $5, completion_policy = $6, actor = $7, reason = $8, metadata = $9, updated_at = $10
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
`
	updated, err := scanWorkflowTemplate(tx.QueryRow(
		ctx,
		updateQuery,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.TemplateID,
		input.IntegrationKind,
		input.CompletionPolicy,
		input.Actor,
		input.Reason,
		metadata,
		input.UpdatedAt,
	))
	if err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	const deleteSteps = `
DELETE FROM integration_workflow_template_steps
WHERE template_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
`
	if _, err := tx.Exec(ctx, deleteSteps, input.TemplateID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace); err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("delete workflow template steps: %w", err)
	}
	updated.Steps = make([]workflow.TemplateStep, 0, len(input.Steps))
	for _, step := range input.Steps {
		if step.TemplateID == "" {
			step.TemplateID = input.TemplateID
		}
		if step.Scope == (memory.Scope{}) {
			step.Scope = input.Scope
		}
		inserted, err := insertWorkflowTemplateStep(ctx, tx, step)
		if err != nil {
			return workflow.WorkflowTemplate{}, err
		}
		updated.Steps = append(updated.Steps, inserted)
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("commit workflow template update transaction: %w", err)
	}
	return updated, nil
}

func (r *Repository) DisableWorkflowTemplate(ctx context.Context, input workflow.DisableTemplateInput) (workflow.WorkflowTemplate, error) {
	if err := input.Validate(); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	const query = `
UPDATE integration_workflow_templates
SET status = $5, actor = $6, reason = $7, updated_at = $8, disabled_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
`
	return scanWorkflowTemplate(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.TemplateID,
		workflow.TemplateStatusDisabled,
		input.Actor,
		input.Reason,
		input.DisabledAt,
	))
}

func (r *Repository) StartWorkflowRun(ctx context.Context, run workflow.WorkflowRun) (workflow.WorkflowRun, error) {
	if err := run.Validate(); err != nil {
		return workflow.WorkflowRun{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(run.Metadata))
	if err != nil {
		return workflow.WorkflowRun{}, fmt.Errorf("marshal workflow run metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_runs (
	id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (tenant, project, namespace, template_id, idempotency_key) DO UPDATE
SET updated_at = integration_workflow_runs.updated_at
RETURNING id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
`
	return scanWorkflowRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.TemplateID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.Status,
		run.IntegrationKind,
		run.IdempotencyKey,
		run.Actor,
		run.Reason,
		metadata,
		run.CreatedAt,
		run.UpdatedAt,
		run.StartedAt,
		nullableTime(run.CompletedAt),
		nullableTime(run.ExpiresAt),
	))
}

func (r *Repository) ReadWorkflowRun(ctx context.Context, input workflow.ReadRunInput) (workflow.WorkflowRun, error) {
	if err := input.Validate(); err != nil {
		return workflow.WorkflowRun{}, err
	}
	const query = `
SELECT id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
FROM integration_workflow_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanWorkflowRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID))
}

func (r *Repository) ListWorkflowRuns(ctx context.Context, input workflow.ListRunsInput) ([]workflow.WorkflowRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
FROM integration_workflow_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND ($4::text = '' OR template_id = $4)
	AND ($5::text = '' OR status = $5)
ORDER BY updated_at DESC, id ASC
LIMIT $6
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.TemplateID, string(input.Status), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()
	runs := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) TransitionWorkflowRun(ctx context.Context, input workflow.TransitionRunInput) (workflow.WorkflowRun, error) {
	if err := input.Validate(); err != nil {
		return workflow.WorkflowRun{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workflow.WorkflowRun{}, fmt.Errorf("begin workflow run transition transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertTransition = `
INSERT INTO integration_workflow_transitions (
	id, run_id, tenant, project, namespace, from_status, to_status, actor, reason, occurred_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, run_id, tenant, project, namespace, from_status, to_status, actor, reason, occurred_at
`
	if _, err := scanWorkflowTransition(tx.QueryRow(
		ctx,
		insertTransition,
		input.Transition.ID,
		input.Transition.RunID,
		input.Transition.Scope.Tenant,
		input.Transition.Scope.Project,
		input.Transition.Scope.Namespace,
		input.Transition.FromStatus,
		input.Transition.ToStatus,
		input.Transition.Actor,
		input.Transition.Reason,
		input.Transition.OccurredAt,
	)); err != nil {
		return workflow.WorkflowRun{}, err
	}

	const updateRun = `
UPDATE integration_workflow_runs
SET status = $1, updated_at = $2, completed_at = COALESCE($3, completed_at)
WHERE id = $4 AND tenant = $5 AND project = $6 AND namespace = $7
RETURNING id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
`
	completedAt := completedAtForWorkflowStatus(input.Transition.ToStatus, input.UpdatedAt)
	updated, err := scanWorkflowRun(tx.QueryRow(
		ctx,
		updateRun,
		input.Transition.ToStatus,
		input.UpdatedAt,
		nullableTime(completedAt),
		input.Transition.RunID,
		input.Transition.Scope.Tenant,
		input.Transition.Scope.Project,
		input.Transition.Scope.Namespace,
	))
	if err != nil {
		return workflow.WorkflowRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.WorkflowRun{}, fmt.Errorf("commit workflow run transition transaction: %w", err)
	}
	return updated, nil
}

func (r *Repository) RecordWorkflowStep(ctx context.Context, record workflow.WorkflowStepRecord) (workflow.WorkflowStepRecord, error) {
	if err := record.Validate(); err != nil {
		return workflow.WorkflowStepRecord{}, err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return workflow.WorkflowStepRecord{}, fmt.Errorf("begin workflow step transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := insertWorkflowStepRecord(ctx, tx, record)
	if err != nil {
		return workflow.WorkflowStepRecord{}, err
	}
	created.EvidenceLinks = make([]workflow.EvidenceLink, 0, len(record.EvidenceLinks))
	for _, link := range record.EvidenceLinks {
		if link.RunID == "" {
			link.RunID = record.RunID
		}
		if link.StepRecordID == "" {
			link.StepRecordID = record.ID
		}
		if link.Scope == (memory.Scope{}) {
			link.Scope = record.Scope
		}
		inserted, err := insertWorkflowEvidenceLink(ctx, tx, link)
		if err != nil {
			return workflow.WorkflowStepRecord{}, err
		}
		created.EvidenceLinks = append(created.EvidenceLinks, inserted)
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.WorkflowStepRecord{}, fmt.Errorf("commit workflow step transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) ListWorkflowStepRecords(ctx context.Context, input workflow.ListStepRecordsInput) ([]workflow.WorkflowStepRecord, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, run_id, tenant, project, namespace, kind, status, result, actor, reason,
	metadata, observed_at, created_at
FROM integration_workflow_step_records
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND run_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID)
	if err != nil {
		return nil, fmt.Errorf("list workflow step records: %w", err)
	}
	defer rows.Close()
	records := make([]workflow.WorkflowStepRecord, 0)
	for rows.Next() {
		record, err := scanWorkflowStepRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow step records: %w", err)
	}
	return records, nil
}

func (r *Repository) ListWorkflowEvidenceLinks(ctx context.Context, input workflow.ListEvidenceLinksInput) ([]workflow.EvidenceLink, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, run_id, step_record_id, tenant, project, namespace, kind, status, source,
	target_id, opaque_token, metadata, created_at, superseded_at
FROM integration_workflow_evidence_links
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND run_id = $4
	AND ($5::text = '' OR status = $5)
ORDER BY created_at DESC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID, string(input.Status))
	if err != nil {
		return nil, fmt.Errorf("list workflow evidence links: %w", err)
	}
	defer rows.Close()
	links := make([]workflow.EvidenceLink, 0)
	for rows.Next() {
		link, err := scanWorkflowEvidenceLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow evidence links: %w", err)
	}
	return links, nil
}

func (r *Repository) RecordWorkflowGapDiagnostic(ctx context.Context, diagnostic workflow.GapDiagnostic) (workflow.GapDiagnostic, error) {
	if err := diagnostic.Validate(); err != nil {
		return workflow.GapDiagnostic{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(diagnostic.Metadata))
	if err != nil {
		return workflow.GapDiagnostic{}, fmt.Errorf("marshal workflow diagnostic metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_gap_diagnostics (
	id, run_id, step_record_id, evidence_link_id, tenant, project, namespace, step_kind,
	evidence_kind, category, readiness_impact, status, metadata, created_at, resolved_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, run_id, step_record_id, evidence_link_id, tenant, project, namespace, step_kind,
	evidence_kind, category, readiness_impact, status, metadata, created_at, resolved_at
`
	return scanWorkflowGapDiagnostic(r.db.QueryRow(
		ctx,
		query,
		diagnostic.ID,
		diagnostic.RunID,
		nullableString(diagnostic.StepRecordID),
		nullableString(diagnostic.EvidenceLinkID),
		diagnostic.Scope.Tenant,
		diagnostic.Scope.Project,
		diagnostic.Scope.Namespace,
		diagnostic.StepKind,
		diagnostic.EvidenceKind,
		diagnostic.Category,
		diagnostic.ReadinessImpact,
		nullableString(diagnostic.Status),
		metadata,
		diagnostic.CreatedAt,
		nullableTime(diagnostic.ResolvedAt),
	))
}

func (r *Repository) ListWorkflowGapDiagnostics(ctx context.Context, input workflow.ListDiagnosticsInput) ([]workflow.GapDiagnostic, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, run_id, step_record_id, evidence_link_id, tenant, project, namespace,
	step_kind, evidence_kind, category, readiness_impact, status, metadata, created_at, resolved_at
FROM integration_workflow_gap_diagnostics
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND run_id = $4
	AND ($5::text = '' OR category = $5)
ORDER BY created_at DESC, id ASC
LIMIT $6
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID, string(input.Category), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow gap diagnostics: %w", err)
	}
	defer rows.Close()
	diagnostics := make([]workflow.GapDiagnostic, 0)
	for rows.Next() {
		diagnostic, err := scanWorkflowGapDiagnostic(rows)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow gap diagnostics: %w", err)
	}
	return diagnostics, nil
}

func (r *Repository) RecordWorkflowNextAction(ctx context.Context, action workflow.NextAction) (workflow.NextAction, error) {
	if err := action.Validate(); err != nil {
		return workflow.NextAction{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(action.Metadata))
	if err != nil {
		return workflow.NextAction{}, fmt.Errorf("marshal workflow next action metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_next_actions (
	id, run_id, tenant, project, namespace, category, step_kind, evidence_kind,
	route_category, status, metadata, created_at, resolved_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, run_id, tenant, project, namespace, category, step_kind, evidence_kind,
	route_category, status, metadata, created_at, resolved_at
`
	return scanWorkflowNextAction(r.db.QueryRow(
		ctx,
		query,
		action.ID,
		action.RunID,
		action.Scope.Tenant,
		action.Scope.Project,
		action.Scope.Namespace,
		action.Category,
		action.StepKind,
		action.EvidenceKind,
		action.RouteCategory,
		action.Status,
		metadata,
		action.CreatedAt,
		nullableTime(action.ResolvedAt),
	))
}

func (r *Repository) ResolveWorkflowNextActions(ctx context.Context, scope memory.Scope, runID string, resolvedAt time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if runID == "" || resolvedAt.IsZero() {
		return fmt.Errorf("workflow run id and action resolution time are required")
	}
	const query = `
UPDATE integration_workflow_next_actions
SET status = $5, resolved_at = $6
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND run_id = $4 AND status = 'open'
`
	if _, err := r.db.Exec(ctx, query, scope.Tenant, scope.Project, scope.Namespace, runID, workflow.NextActionStatusSatisfied, resolvedAt.UTC()); err != nil {
		return fmt.Errorf("resolve workflow next actions: %w", err)
	}
	return nil
}

func (r *Repository) ListWorkflowNextActions(ctx context.Context, input workflow.ListNextActionsInput) ([]workflow.NextAction, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, run_id, tenant, project, namespace, category, step_kind, evidence_kind,
	route_category, status, metadata, created_at, resolved_at
FROM integration_workflow_next_actions
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND run_id = $4
	AND ($5::text = '' OR status = $5)
ORDER BY created_at DESC, id ASC
LIMIT $6
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RunID, string(input.Status), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow next actions: %w", err)
	}
	defer rows.Close()
	actions := make([]workflow.NextAction, 0)
	for rows.Next() {
		action, err := scanWorkflowNextAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow next actions: %w", err)
	}
	return actions, nil
}

func (r *Repository) SupersedeWorkflowEvidenceLink(ctx context.Context, input workflow.SupersedeEvidenceLinkInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE integration_workflow_evidence_links
SET status = $5, superseded_by_actor = $6, superseded_by_reason = $7, superseded_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	result, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.LinkID,
		workflow.EvidenceLinkStatusSuperseded,
		input.Actor,
		input.Reason,
		input.SupersededAt,
	)
	if err != nil {
		return fmt.Errorf("supersede workflow evidence link: %w", err)
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) FindWorkflowRetentionEligibleHistory(ctx context.Context, input workflow.FindRetentionEligibleHistoryInput) ([]workflow.WorkflowRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, template_id, tenant, project, namespace, status, integration_kind, idempotency_key,
	actor, reason, metadata, created_at, updated_at, started_at, completed_at, expires_at
FROM integration_workflow_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
	AND status IN ('completed', 'abandoned', 'expired')
	AND updated_at <= $5
ORDER BY updated_at ASC, id ASC
LIMIT $6
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Cutoff, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("find workflow retention eligible history: %w", err)
	}
	defer rows.Close()
	runs := make([]workflow.WorkflowRun, 0)
	for rows.Next() {
		run, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow retention eligible history: %w", err)
	}
	return runs, nil
}

func (r *Repository) CreateWorkflowRetentionRun(ctx context.Context, run workflow.WorkflowRetentionRun) (workflow.WorkflowRetentionRun, error) {
	if err := run.Validate(); err != nil {
		return workflow.WorkflowRetentionRun{}, err
	}
	const query = `
INSERT INTO integration_workflow_retention_runs (
	id, tenant, project, namespace, record_category, cutoff, deleted_count, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant, project, namespace, record_category, cutoff, deleted_count, started_at, finished_at
`
	return scanWorkflowRetentionRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.RecordCategory,
		run.Cutoff,
		run.DeletedCount,
		run.StartedAt,
		nullableTime(run.FinishedAt),
	))
}

func insertWorkflowTemplate(ctx context.Context, db queryRower, template workflow.WorkflowTemplate) (workflow.WorkflowTemplate, error) {
	metadata, err := json.Marshal(normalizeAnyMap(template.Metadata))
	if err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("marshal workflow template metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_templates (
	id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, tenant, project, namespace, status, integration_kind, completion_policy,
	actor, reason, metadata, created_at, updated_at, disabled_at
`
	return scanWorkflowTemplate(db.QueryRow(
		ctx,
		query,
		template.ID,
		template.Scope.Tenant,
		template.Scope.Project,
		template.Scope.Namespace,
		template.Status,
		template.IntegrationKind,
		template.CompletionPolicy,
		template.Actor,
		template.Reason,
		metadata,
		template.CreatedAt,
		template.UpdatedAt,
		nullableTime(template.DisabledAt),
	))
}

func insertWorkflowTemplateStep(ctx context.Context, db queryRower, step workflow.TemplateStep) (workflow.TemplateStep, error) {
	if err := step.Validate(); err != nil {
		return workflow.TemplateStep{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(step.Metadata))
	if err != nil {
		return workflow.TemplateStep{}, fmt.Errorf("marshal workflow template step metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_template_steps (
	id, template_id, tenant, project, namespace, kind, requirement, allowed_evidence,
	minimum_count, requires_internal, freshness_window_ns, completion_window_ns,
	position, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, template_id, tenant, project, namespace, kind, requirement, allowed_evidence,
	minimum_count, requires_internal, freshness_window_ns, completion_window_ns,
	position, metadata, created_at
`
	return scanWorkflowTemplateStep(db.QueryRow(
		ctx,
		query,
		step.ID,
		step.TemplateID,
		step.Scope.Tenant,
		step.Scope.Project,
		step.Scope.Namespace,
		step.Kind,
		step.Requirement,
		evidenceKindStrings(step.AllowedEvidence),
		step.MinimumCount,
		step.RequiresInternal,
		int64(step.FreshnessWindow),
		int64(step.CompletionWindow),
		step.Position,
		metadata,
		step.CreatedAt,
	))
}

func (r *Repository) listWorkflowTemplateSteps(ctx context.Context, scope memory.Scope, templateID string) ([]workflow.TemplateStep, error) {
	const query = `
SELECT id, template_id, tenant, project, namespace, kind, requirement, allowed_evidence,
	minimum_count, requires_internal, freshness_window_ns, completion_window_ns,
	position, metadata, created_at
FROM integration_workflow_template_steps
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND template_id = $4
ORDER BY position ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, templateID)
	if err != nil {
		return nil, fmt.Errorf("list workflow template steps: %w", err)
	}
	defer rows.Close()
	steps := make([]workflow.TemplateStep, 0)
	for rows.Next() {
		step, err := scanWorkflowTemplateStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow template steps: %w", err)
	}
	return steps, nil
}

func insertWorkflowStepRecord(ctx context.Context, db queryRower, record workflow.WorkflowStepRecord) (workflow.WorkflowStepRecord, error) {
	metadata, err := json.Marshal(normalizeAnyMap(record.Metadata))
	if err != nil {
		return workflow.WorkflowStepRecord{}, fmt.Errorf("marshal workflow step metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_step_records (
	id, run_id, tenant, project, namespace, kind, status, result, actor, reason,
	metadata, observed_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, run_id, tenant, project, namespace, kind, status, result, actor, reason,
	metadata, observed_at, created_at
`
	return scanWorkflowStepRecord(db.QueryRow(
		ctx,
		query,
		record.ID,
		record.RunID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Kind,
		record.Status,
		record.Result,
		record.Actor,
		record.Reason,
		metadata,
		record.ObservedAt,
		record.CreatedAt,
	))
}

func insertWorkflowEvidenceLink(ctx context.Context, db queryRower, link workflow.EvidenceLink) (workflow.EvidenceLink, error) {
	if err := link.Validate(); err != nil {
		return workflow.EvidenceLink{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(link.Metadata))
	if err != nil {
		return workflow.EvidenceLink{}, fmt.Errorf("marshal workflow evidence link metadata: %w", err)
	}
	const query = `
INSERT INTO integration_workflow_evidence_links (
	id, run_id, step_record_id, tenant, project, namespace, kind, status, source,
	target_id, opaque_token, metadata, created_at, superseded_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, run_id, step_record_id, tenant, project, namespace, kind, status, source,
	target_id, opaque_token, metadata, created_at, superseded_at
`
	return scanWorkflowEvidenceLink(db.QueryRow(
		ctx,
		query,
		link.ID,
		link.RunID,
		nullableString(link.StepRecordID),
		link.Scope.Tenant,
		link.Scope.Project,
		link.Scope.Namespace,
		link.Kind,
		link.Status,
		link.Source,
		nullableString(link.TargetID),
		nullableString(link.OpaqueToken),
		metadata,
		link.CreatedAt,
		nullableTime(link.SupersededAt),
	))
}

func scanWorkflowTemplate(scanner provenanceScanner) (workflow.WorkflowTemplate, error) {
	var template workflow.WorkflowTemplate
	var metadata []byte
	var disabledAt sql.NullTime
	if err := scanner.Scan(
		&template.ID,
		&template.Scope.Tenant,
		&template.Scope.Project,
		&template.Scope.Namespace,
		&template.Status,
		&template.IntegrationKind,
		&template.CompletionPolicy,
		&template.Actor,
		&template.Reason,
		&metadata,
		&template.CreatedAt,
		&template.UpdatedAt,
		&disabledAt,
	); err != nil {
		return workflow.WorkflowTemplate{}, fmt.Errorf("scan workflow template: %w", err)
	}
	if disabledAt.Valid {
		template.DisabledAt = disabledAt.Time
	}
	template.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &template.Metadata); err != nil {
			return workflow.WorkflowTemplate{}, fmt.Errorf("unmarshal workflow template metadata: %w", err)
		}
	}
	return template, nil
}

func scanWorkflowTemplateStep(scanner provenanceScanner) (workflow.TemplateStep, error) {
	var step workflow.TemplateStep
	var allowed []string
	var freshnessWindowNS int64
	var completionWindowNS int64
	var metadata []byte
	if err := scanner.Scan(
		&step.ID,
		&step.TemplateID,
		&step.Scope.Tenant,
		&step.Scope.Project,
		&step.Scope.Namespace,
		&step.Kind,
		&step.Requirement,
		&allowed,
		&step.MinimumCount,
		&step.RequiresInternal,
		&freshnessWindowNS,
		&completionWindowNS,
		&step.Position,
		&metadata,
		&step.CreatedAt,
	); err != nil {
		return workflow.TemplateStep{}, fmt.Errorf("scan workflow template step: %w", err)
	}
	step.AllowedEvidence = evidenceKinds(allowed)
	step.FreshnessWindow = time.Duration(freshnessWindowNS)
	step.CompletionWindow = time.Duration(completionWindowNS)
	step.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &step.Metadata); err != nil {
			return workflow.TemplateStep{}, fmt.Errorf("unmarshal workflow template step metadata: %w", err)
		}
	}
	return step, nil
}

func scanWorkflowRun(scanner provenanceScanner) (workflow.WorkflowRun, error) {
	var run workflow.WorkflowRun
	var metadata []byte
	var completedAt sql.NullTime
	var expiresAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.TemplateID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.Status,
		&run.IntegrationKind,
		&run.IdempotencyKey,
		&run.Actor,
		&run.Reason,
		&metadata,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.StartedAt,
		&completedAt,
		&expiresAt,
	); err != nil {
		return workflow.WorkflowRun{}, fmt.Errorf("scan workflow run: %w", err)
	}
	if completedAt.Valid {
		run.CompletedAt = completedAt.Time
	}
	if expiresAt.Valid {
		run.ExpiresAt = expiresAt.Time
	}
	run.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &run.Metadata); err != nil {
			return workflow.WorkflowRun{}, fmt.Errorf("unmarshal workflow run metadata: %w", err)
		}
	}
	return run, nil
}

func scanWorkflowStepRecord(scanner provenanceScanner) (workflow.WorkflowStepRecord, error) {
	var record workflow.WorkflowStepRecord
	var metadata []byte
	if err := scanner.Scan(
		&record.ID,
		&record.RunID,
		&record.Scope.Tenant,
		&record.Scope.Project,
		&record.Scope.Namespace,
		&record.Kind,
		&record.Status,
		&record.Result,
		&record.Actor,
		&record.Reason,
		&metadata,
		&record.ObservedAt,
		&record.CreatedAt,
	); err != nil {
		return workflow.WorkflowStepRecord{}, fmt.Errorf("scan workflow step record: %w", err)
	}
	record.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &record.Metadata); err != nil {
			return workflow.WorkflowStepRecord{}, fmt.Errorf("unmarshal workflow step metadata: %w", err)
		}
	}
	return record, nil
}

func scanWorkflowEvidenceLink(scanner provenanceScanner) (workflow.EvidenceLink, error) {
	var link workflow.EvidenceLink
	var stepRecordID sql.NullString
	var targetID sql.NullString
	var opaqueToken sql.NullString
	var metadata []byte
	var supersededAt sql.NullTime
	if err := scanner.Scan(
		&link.ID,
		&link.RunID,
		&stepRecordID,
		&link.Scope.Tenant,
		&link.Scope.Project,
		&link.Scope.Namespace,
		&link.Kind,
		&link.Status,
		&link.Source,
		&targetID,
		&opaqueToken,
		&metadata,
		&link.CreatedAt,
		&supersededAt,
	); err != nil {
		return workflow.EvidenceLink{}, fmt.Errorf("scan workflow evidence link: %w", err)
	}
	if stepRecordID.Valid {
		link.StepRecordID = stepRecordID.String
	}
	if targetID.Valid {
		link.TargetID = targetID.String
	}
	if opaqueToken.Valid {
		link.OpaqueToken = opaqueToken.String
	}
	if supersededAt.Valid {
		link.SupersededAt = supersededAt.Time
	}
	link.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &link.Metadata); err != nil {
			return workflow.EvidenceLink{}, fmt.Errorf("unmarshal workflow evidence link metadata: %w", err)
		}
	}
	return link, nil
}

func scanWorkflowGapDiagnostic(scanner provenanceScanner) (workflow.GapDiagnostic, error) {
	var diagnostic workflow.GapDiagnostic
	var stepRecordID sql.NullString
	var evidenceLinkID sql.NullString
	var status sql.NullString
	var metadata []byte
	var resolvedAt sql.NullTime
	if err := scanner.Scan(
		&diagnostic.ID,
		&diagnostic.RunID,
		&stepRecordID,
		&evidenceLinkID,
		&diagnostic.Scope.Tenant,
		&diagnostic.Scope.Project,
		&diagnostic.Scope.Namespace,
		&diagnostic.StepKind,
		&diagnostic.EvidenceKind,
		&diagnostic.Category,
		&diagnostic.ReadinessImpact,
		&status,
		&metadata,
		&diagnostic.CreatedAt,
		&resolvedAt,
	); err != nil {
		return workflow.GapDiagnostic{}, fmt.Errorf("scan workflow gap diagnostic: %w", err)
	}
	if stepRecordID.Valid {
		diagnostic.StepRecordID = stepRecordID.String
	}
	if evidenceLinkID.Valid {
		diagnostic.EvidenceLinkID = evidenceLinkID.String
	}
	if status.Valid {
		diagnostic.Status = status.String
	}
	if resolvedAt.Valid {
		diagnostic.ResolvedAt = resolvedAt.Time
	}
	diagnostic.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &diagnostic.Metadata); err != nil {
			return workflow.GapDiagnostic{}, fmt.Errorf("unmarshal workflow gap diagnostic metadata: %w", err)
		}
	}
	return diagnostic, nil
}

func scanWorkflowNextAction(scanner provenanceScanner) (workflow.NextAction, error) {
	var action workflow.NextAction
	var metadata []byte
	var resolvedAt sql.NullTime
	if err := scanner.Scan(
		&action.ID,
		&action.RunID,
		&action.Scope.Tenant,
		&action.Scope.Project,
		&action.Scope.Namespace,
		&action.Category,
		&action.StepKind,
		&action.EvidenceKind,
		&action.RouteCategory,
		&action.Status,
		&metadata,
		&action.CreatedAt,
		&resolvedAt,
	); err != nil {
		return workflow.NextAction{}, fmt.Errorf("scan workflow next action: %w", err)
	}
	if resolvedAt.Valid {
		action.ResolvedAt = resolvedAt.Time
	}
	action.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &action.Metadata); err != nil {
			return workflow.NextAction{}, fmt.Errorf("unmarshal workflow next action metadata: %w", err)
		}
	}
	return action, nil
}

func scanWorkflowTransition(scanner provenanceScanner) (workflow.WorkflowTransition, error) {
	var transition workflow.WorkflowTransition
	var fromStatus sql.NullString
	if err := scanner.Scan(
		&transition.ID,
		&transition.RunID,
		&transition.Scope.Tenant,
		&transition.Scope.Project,
		&transition.Scope.Namespace,
		&fromStatus,
		&transition.ToStatus,
		&transition.Actor,
		&transition.Reason,
		&transition.OccurredAt,
	); err != nil {
		return workflow.WorkflowTransition{}, fmt.Errorf("scan workflow transition: %w", err)
	}
	if fromStatus.Valid {
		transition.FromStatus = workflow.RunStatus(fromStatus.String)
	}
	return transition, nil
}

func scanWorkflowRetentionRun(scanner provenanceScanner) (workflow.WorkflowRetentionRun, error) {
	var run workflow.WorkflowRetentionRun
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.RecordCategory,
		&run.Cutoff,
		&run.DeletedCount,
		&run.StartedAt,
		&finishedAt,
	); err != nil {
		return workflow.WorkflowRetentionRun{}, fmt.Errorf("scan workflow retention run: %w", err)
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	return run, nil
}

func evidenceKindStrings(kinds []workflow.EvidenceKind) []string {
	values := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, string(kind))
	}
	return values
}

func evidenceKinds(values []string) []workflow.EvidenceKind {
	kinds := make([]workflow.EvidenceKind, 0, len(values))
	for _, value := range values {
		kinds = append(kinds, workflow.EvidenceKind(value))
	}
	return kinds
}

func completedAtForWorkflowStatus(status workflow.RunStatus, at time.Time) time.Time {
	if status != workflow.RunStatusCompleted {
		return time.Time{}
	}
	return at
}
