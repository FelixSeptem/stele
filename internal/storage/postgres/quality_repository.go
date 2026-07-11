package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func (r *Repository) CreateQualityEvaluationRun(ctx context.Context, run memory.QualityEvaluationRun) (memory.QualityEvaluationRun, error) {
	if err := run.Scope.Validate(); err != nil {
		return memory.QualityEvaluationRun{}, err
	}
	const query = `
INSERT INTO quality_evaluation_runs (
	id, tenant, project, namespace, status, checks, actor, reason,
	created_at, updated_at, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, tenant, project, namespace, status, checks, actor, reason, created_at, updated_at, started_at, finished_at
`
	return scanQualityEvaluationRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.Status,
		qualityEvaluationCheckStrings(run.Checks),
		nullableString(run.Actor),
		nullableString(run.Reason),
		run.CreatedAt,
		run.UpdatedAt,
		nullableTime(run.StartedAt),
		nullableTime(run.FinishedAt),
	))
}

func (r *Repository) ReadQualityEvaluationRun(ctx context.Context, input memory.ReadQualityEvaluationRunInput) (memory.QualityEvaluationRun, error) {
	if err := input.Validate(); err != nil {
		return memory.QualityEvaluationRun{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, status, checks, actor, reason, created_at, updated_at, started_at, finished_at
FROM quality_evaluation_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	return scanQualityEvaluationRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EvaluationRunID))
}

func (r *Repository) ListQualityEvaluationFindings(ctx context.Context, input memory.ListQualityEvaluationFindingsInput) ([]memory.QualityEvaluationFinding, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
SELECT id, evaluation_run_id, tenant, project, namespace, code, severity, component, category, message, suggested_action_category, metadata, evidence, created_at
FROM quality_evaluation_findings
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND evaluation_run_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EvaluationRunID)
	if err != nil {
		return nil, fmt.Errorf("list quality evaluation findings: %w", err)
	}
	defer rows.Close()
	findings := make([]memory.QualityEvaluationFinding, 0)
	for rows.Next() {
		finding, err := scanQualityEvaluationFinding(rows)
		if err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quality evaluation findings: %w", err)
	}
	return findings, nil
}

func (r *Repository) CreateQualityEvaluationFinding(ctx context.Context, finding memory.QualityEvaluationFinding) (memory.QualityEvaluationFinding, error) {
	if err := finding.Scope.Validate(); err != nil {
		return memory.QualityEvaluationFinding{}, err
	}
	metadata, err := json.Marshal(normalizeStringMap(finding.Metadata))
	if err != nil {
		return memory.QualityEvaluationFinding{}, fmt.Errorf("marshal quality finding metadata: %w", err)
	}
	evidence, err := json.Marshal(normalizeAnyMap(finding.Evidence))
	if err != nil {
		return memory.QualityEvaluationFinding{}, fmt.Errorf("marshal quality finding evidence: %w", err)
	}
	const query = `
INSERT INTO quality_evaluation_findings (
	id, evaluation_run_id, tenant, project, namespace, code, severity, component, category,
	message, suggested_action_category, metadata, evidence, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, evaluation_run_id, tenant, project, namespace, code, severity, component, category, message, suggested_action_category, metadata, evidence, created_at
`
	return scanQualityEvaluationFinding(r.db.QueryRow(
		ctx,
		query,
		finding.ID,
		finding.EvaluationRunID,
		finding.Scope.Tenant,
		finding.Scope.Project,
		finding.Scope.Namespace,
		finding.Code,
		finding.Severity,
		finding.Component,
		finding.Category,
		nullableString(finding.Message),
		nullableString(string(finding.SuggestedActionCategory)),
		metadata,
		evidence,
		finding.CreatedAt,
	))
}

func (r *Repository) CreateRepairPlan(ctx context.Context, plan memory.RepairPlan) (memory.RepairPlan, error) {
	if err := plan.Scope.Validate(); err != nil {
		return memory.RepairPlan{}, err
	}
	const query = `
INSERT INTO repair_plans (
	id, tenant, project, namespace, evaluation_run_id, baseline_run_id, verification_run_id,
	status, verification_status, dry_run, actor, reason, created_at, updated_at, approved_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id, tenant, project, namespace, evaluation_run_id, baseline_run_id, verification_run_id,
	status, verification_status, dry_run, actor, reason, created_at, updated_at, approved_at, completed_at
`
	return scanRepairPlan(r.db.QueryRow(
		ctx,
		query,
		plan.ID,
		plan.Scope.Tenant,
		plan.Scope.Project,
		plan.Scope.Namespace,
		plan.EvaluationRunID,
		nullableString(plan.BaselineRunID),
		nullableString(plan.VerificationRunID),
		plan.Status,
		nullableString(string(plan.VerificationStatus)),
		plan.DryRun,
		plan.Actor,
		plan.Reason,
		plan.CreatedAt,
		plan.UpdatedAt,
		nullableTime(plan.ApprovedAt),
		nullableTime(plan.CompletedAt),
	))
}

func (r *Repository) ReadRepairPlan(ctx context.Context, input memory.ReadRepairPlanInput) (memory.RepairPlan, error) {
	if err := input.Validate(); err != nil {
		return memory.RepairPlan{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, evaluation_run_id, baseline_run_id, verification_run_id,
	status, verification_status, dry_run, actor, reason, created_at, updated_at, approved_at, completed_at
FROM repair_plans
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	plan, err := scanRepairPlan(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RepairPlanID))
	if err != nil {
		return memory.RepairPlan{}, err
	}
	actions, err := r.listRepairActionsByPlan(ctx, input.Scope, input.RepairPlanID)
	if err != nil {
		return memory.RepairPlan{}, err
	}
	plan.Actions = actions
	return plan, nil
}

func (r *Repository) ApproveRepairPlan(ctx context.Context, input memory.ApproveRepairPlanInput) (memory.RepairPlan, error) {
	if err := input.Validate(); err != nil {
		return memory.RepairPlan{}, err
	}
	const query = `
UPDATE repair_plans
SET status = $5, actor = $6, reason = $7, approved_at = $8, updated_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4 AND status = $9
RETURNING id, tenant, project, namespace, evaluation_run_id, baseline_run_id, verification_run_id,
	status, verification_status, dry_run, actor, reason, created_at, updated_at, approved_at, completed_at
`
	plan, err := scanRepairPlan(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.RepairPlanID,
		memory.RepairPlanStatusApproved,
		input.Actor,
		input.Reason,
		input.ApprovedAt,
		memory.RepairPlanStatusDraft,
	))
	if err != nil {
		return memory.RepairPlan{}, err
	}
	actions, err := r.listRepairActionsByPlan(ctx, input.Scope, input.RepairPlanID)
	if err != nil {
		return memory.RepairPlan{}, err
	}
	plan.Actions = actions
	return plan, nil
}

func (r *Repository) UpdateRepairPlanVerification(ctx context.Context, input memory.UpdateRepairPlanVerificationInput) (memory.RepairPlan, error) {
	if err := input.Scope.Validate(); err != nil {
		return memory.RepairPlan{}, err
	}
	const query = `
UPDATE repair_plans
SET verification_run_id = $5, verification_status = $6, updated_at = $7
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, evaluation_run_id, baseline_run_id, verification_run_id,
	status, verification_status, dry_run, actor, reason, created_at, updated_at, approved_at, completed_at
`
	plan, err := scanRepairPlan(r.db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.RepairPlanID,
		input.VerificationRunID,
		input.VerificationStatus,
		input.UpdatedAt,
	))
	if err != nil {
		return memory.RepairPlan{}, err
	}
	actions, err := r.listRepairActionsByPlan(ctx, input.Scope, input.RepairPlanID)
	if err != nil {
		return memory.RepairPlan{}, err
	}
	plan.Actions = actions
	return plan, nil
}

func (r *Repository) CreateRepairAction(ctx context.Context, action memory.RepairAction) (memory.RepairAction, error) {
	if err := action.Scope.Validate(); err != nil {
		return memory.RepairAction{}, err
	}
	const query = `
INSERT INTO repair_actions (
	id, plan_id, evaluation_run_id, finding_id, tenant, project, namespace, category, status,
	target_kind, target_id, reason_code, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
RETURNING id, plan_id, evaluation_run_id, finding_id, tenant, project, namespace, category, status,
	target_kind, target_id, reason_code, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
`
	return scanRepairAction(r.db.QueryRow(
		ctx,
		query,
		action.ID,
		action.PlanID,
		action.EvaluationRunID,
		nullableString(action.FindingID),
		action.Scope.Tenant,
		action.Scope.Project,
		action.Scope.Namespace,
		action.Category,
		action.Status,
		nullableString(action.TargetKind),
		nullableString(action.TargetID),
		nullableString(string(action.ReasonCode)),
		action.Attempt,
		nullableString(action.WorkerID),
		nullableTime(action.LeaseUntil),
		nullableString(action.LastError),
		nullableTime(action.NextAttemptAt),
		action.CreatedAt,
		action.UpdatedAt,
		nullableTime(action.CompletedAt),
	))
}

func (r *Repository) listRepairActionsByPlan(ctx context.Context, scope memory.Scope, planID string) ([]memory.RepairAction, error) {
	const query = `
SELECT id, plan_id, evaluation_run_id, finding_id, tenant, project, namespace, category, status,
	target_kind, target_id, reason_code, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
FROM repair_actions
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND plan_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, planID)
	if err != nil {
		return nil, fmt.Errorf("list repair actions by plan: %w", err)
	}
	defer rows.Close()
	actions := make([]memory.RepairAction, 0)
	for rows.Next() {
		action, err := scanRepairAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repair actions by plan: %w", err)
	}
	return actions, nil
}

func (r *Repository) ClaimRepairActions(ctx context.Context, input memory.ClaimRepairActionsInput) ([]memory.RepairAction, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH claimed AS (
	UPDATE repair_actions
	SET
		status = $8,
		worker_id = $4,
		lease_until = $6,
		attempt = attempt + 1,
		updated_at = $5
	WHERE id IN (
		SELECT ra.id
		FROM repair_actions ra
		JOIN repair_plans rp ON rp.id = ra.plan_id
		WHERE ra.tenant = $1
			AND ra.project = $2
			AND ra.namespace = $3
			AND ra.status IN ($9, $10)
			AND (ra.next_attempt_at IS NULL OR ra.next_attempt_at <= $5)
			AND (ra.lease_until IS NULL OR ra.lease_until <= $5)
			AND rp.status = $11
			AND rp.dry_run = false
		ORDER BY ra.updated_at ASC, ra.id ASC
		LIMIT $7
		FOR UPDATE SKIP LOCKED
	)
	RETURNING id, plan_id, evaluation_run_id, finding_id, tenant, project, namespace, category, status,
		target_kind, target_id, reason_code, attempt, worker_id, lease_until, last_error, next_attempt_at,
		created_at, updated_at, completed_at
)
SELECT id, plan_id, evaluation_run_id, finding_id, tenant, project, namespace, category, status,
	target_kind, target_id, reason_code, attempt, worker_id, lease_until, last_error, next_attempt_at,
	created_at, updated_at, completed_at
FROM claimed
ORDER BY updated_at ASC, id ASC
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
		memory.RepairActionStatusRunning,
		memory.RepairActionStatusPending,
		memory.RepairActionStatusFailed,
		memory.RepairPlanStatusApproved,
	)
	if err != nil {
		return nil, fmt.Errorf("claim repair actions: %w", err)
	}
	defer rows.Close()
	actions := make([]memory.RepairAction, 0)
	for rows.Next() {
		action, err := scanRepairAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed repair actions: %w", err)
	}
	return actions, nil
}

func (r *Repository) RecordRepairActionFailure(ctx context.Context, input memory.RecordRepairActionFailureInput) error {
	if err := input.Scope.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE repair_actions
SET
	status = $6,
	last_error = $7,
	next_attempt_at = $8,
	lease_until = NULL,
	worker_id = NULL,
	updated_at = $5
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4 AND worker_id = $9 AND status = $10
`
	status := memory.RepairActionStatusFailed
	if input.Exhausted {
		status = memory.RepairActionStatusExhausted
	}
	tag, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ActionID,
		input.FailedAt,
		status,
		input.ErrorMessage,
		nullableTime(input.NextAttemptAt),
		input.WorkerID,
		memory.RepairActionStatusRunning,
	)
	if err != nil {
		return fmt.Errorf("record repair action failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("repair action ownership lost")
	}
	return nil
}

func (r *Repository) CompleteRepairAction(ctx context.Context, input memory.CompleteRepairActionInput) error {
	if err := input.Scope.Validate(); err != nil {
		return err
	}
	status := input.Status
	if status == "" {
		status = memory.RepairActionStatusCompleted
	}
	const query = `
UPDATE repair_actions
SET
	status = $6,
	completed_at = $5,
	lease_until = NULL,
	worker_id = NULL,
	updated_at = $5
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4 AND worker_id = $7 AND status = $8
`
	tag, err := r.db.Exec(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.ActionID,
		input.CompletedAt,
		status,
		input.WorkerID,
		memory.RepairActionStatusRunning,
	)
	if err != nil {
		return fmt.Errorf("complete repair action: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("repair action ownership lost")
	}
	return nil
}

func (r *Repository) ReadAdmissionPressureSnapshot(ctx context.Context, scope memory.Scope, operation memory.AdmissionPressureOperation, observedAt time.Time) (memory.AdmissionPressureSnapshot, error) {
	if err := scope.Validate(); err != nil {
		return memory.AdmissionPressureSnapshot{}, err
	}
	const query = `
SELECT
	COUNT(*) FILTER (WHERE governance_processed_at IS NULL AND governance_exhausted_at IS NULL) AS pending_governance,
	COUNT(*) FILTER (WHERE governance_processed_at IS NULL AND governance_lease_until > $4) AS leased_governance
FROM raw_events
WHERE tenant = $1 AND project = $2 AND namespace = $3
`
	var snapshot memory.AdmissionPressureSnapshot
	snapshot.IntentWritable = true
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, observedAt.UTC()).Scan(&snapshot.PendingGovernance, &snapshot.LeasedGovernance); err != nil {
		return memory.AdmissionPressureSnapshot{}, fmt.Errorf("read admission pressure snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *Repository) ReadQualityDiagnostics(ctx context.Context, input memory.ReadQualityDiagnosticsInput) (memory.QualityDiagnostics, error) {
	if err := input.Validate(); err != nil {
		return memory.QualityDiagnostics{}, err
	}
	diagnostics := memory.QualityDiagnostics{
		Scope:             input.Scope.Normalized(),
		EvaluationStatus:  map[string]int64{},
		FindingCategories: map[string]int64{},
		RepairStatus:      map[string]int64{},
		ObservedAt:        time.Now().UTC(),
	}
	if err := readQualityCountMap(ctx, r.db, `
SELECT status, COUNT(*)
FROM quality_evaluation_runs
WHERE tenant = $1 AND project = $2 AND namespace = $3
GROUP BY status
`, input.Scope, diagnostics.EvaluationStatus); err != nil {
		return memory.QualityDiagnostics{}, fmt.Errorf("read quality evaluation diagnostics: %w", err)
	}
	if err := readQualityCountMap(ctx, r.db, `
SELECT category, COUNT(*)
FROM quality_evaluation_findings
WHERE tenant = $1 AND project = $2 AND namespace = $3
GROUP BY category
`, input.Scope, diagnostics.FindingCategories); err != nil {
		return memory.QualityDiagnostics{}, fmt.Errorf("read quality finding diagnostics: %w", err)
	}
	if err := readQualityCountMap(ctx, r.db, `
SELECT status, COUNT(*)
FROM repair_actions
WHERE tenant = $1 AND project = $2 AND namespace = $3
GROUP BY status
`, input.Scope, diagnostics.RepairStatus); err != nil {
		return memory.QualityDiagnostics{}, fmt.Errorf("read repair diagnostics: %w", err)
	}
	return diagnostics, nil
}

func readQualityCountMap(ctx context.Context, db queryRower, query string, scope memory.Scope, target map[string]int64) error {
	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return err
		}
		target[key] = count
	}
	return rows.Err()
}

func scanQualityEvaluationRun(scanner provenanceScanner) (memory.QualityEvaluationRun, error) {
	var run memory.QualityEvaluationRun
	var checks []string
	var actor sql.NullString
	var reason sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.Status,
		&checks,
		&actor,
		&reason,
		&run.CreatedAt,
		&run.UpdatedAt,
		&startedAt,
		&finishedAt,
	); err != nil {
		return memory.QualityEvaluationRun{}, fmt.Errorf("scan quality evaluation run: %w", err)
	}
	run.Checks = qualityEvaluationChecks(checks)
	if actor.Valid {
		run.Actor = actor.String
	}
	if reason.Valid {
		run.Reason = reason.String
	}
	if startedAt.Valid {
		run.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	return run, nil
}

func scanQualityEvaluationFinding(scanner provenanceScanner) (memory.QualityEvaluationFinding, error) {
	var finding memory.QualityEvaluationFinding
	var message sql.NullString
	var suggested sql.NullString
	var metadata []byte
	var evidence []byte
	if err := scanner.Scan(
		&finding.ID,
		&finding.EvaluationRunID,
		&finding.Scope.Tenant,
		&finding.Scope.Project,
		&finding.Scope.Namespace,
		&finding.Code,
		&finding.Severity,
		&finding.Component,
		&finding.Category,
		&message,
		&suggested,
		&metadata,
		&evidence,
		&finding.CreatedAt,
	); err != nil {
		return memory.QualityEvaluationFinding{}, fmt.Errorf("scan quality evaluation finding: %w", err)
	}
	if message.Valid {
		finding.Message = message.String
	}
	if suggested.Valid {
		finding.SuggestedActionCategory = memory.RepairActionCategory(suggested.String)
	}
	finding.Metadata = map[string]string{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &finding.Metadata); err != nil {
			return memory.QualityEvaluationFinding{}, fmt.Errorf("unmarshal quality finding metadata: %w", err)
		}
	}
	finding.Evidence = map[string]any{}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &finding.Evidence); err != nil {
			return memory.QualityEvaluationFinding{}, fmt.Errorf("unmarshal quality finding evidence: %w", err)
		}
	}
	return finding, nil
}

func scanRepairPlan(scanner provenanceScanner) (memory.RepairPlan, error) {
	var plan memory.RepairPlan
	var baselineRunID sql.NullString
	var verificationRunID sql.NullString
	var verificationStatus sql.NullString
	var approvedAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&plan.ID,
		&plan.Scope.Tenant,
		&plan.Scope.Project,
		&plan.Scope.Namespace,
		&plan.EvaluationRunID,
		&baselineRunID,
		&verificationRunID,
		&plan.Status,
		&verificationStatus,
		&plan.DryRun,
		&plan.Actor,
		&plan.Reason,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&approvedAt,
		&completedAt,
	); err != nil {
		return memory.RepairPlan{}, fmt.Errorf("scan repair plan: %w", err)
	}
	if baselineRunID.Valid {
		plan.BaselineRunID = baselineRunID.String
	}
	if verificationRunID.Valid {
		plan.VerificationRunID = verificationRunID.String
	}
	if verificationStatus.Valid {
		plan.VerificationStatus = memory.RepairVerificationStatus(verificationStatus.String)
	}
	if approvedAt.Valid {
		plan.ApprovedAt = approvedAt.Time
	}
	if completedAt.Valid {
		plan.CompletedAt = completedAt.Time
	}
	return plan, nil
}

func scanRepairAction(scanner provenanceScanner) (memory.RepairAction, error) {
	var action memory.RepairAction
	var findingID sql.NullString
	var targetKind sql.NullString
	var targetID sql.NullString
	var reasonCode sql.NullString
	var workerID sql.NullString
	var leaseUntil sql.NullTime
	var lastError sql.NullString
	var nextAttemptAt sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&action.ID,
		&action.PlanID,
		&action.EvaluationRunID,
		&findingID,
		&action.Scope.Tenant,
		&action.Scope.Project,
		&action.Scope.Namespace,
		&action.Category,
		&action.Status,
		&targetKind,
		&targetID,
		&reasonCode,
		&action.Attempt,
		&workerID,
		&leaseUntil,
		&lastError,
		&nextAttemptAt,
		&action.CreatedAt,
		&action.UpdatedAt,
		&completedAt,
	); err != nil {
		return memory.RepairAction{}, fmt.Errorf("scan repair action: %w", err)
	}
	if findingID.Valid {
		action.FindingID = findingID.String
	}
	if targetKind.Valid {
		action.TargetKind = targetKind.String
	}
	if targetID.Valid {
		action.TargetID = targetID.String
	}
	if reasonCode.Valid {
		action.ReasonCode = memory.QualityFindingCode(reasonCode.String)
	}
	if workerID.Valid {
		action.WorkerID = workerID.String
	}
	if leaseUntil.Valid {
		action.LeaseUntil = leaseUntil.Time
	}
	if lastError.Valid {
		action.LastError = lastError.String
	}
	if nextAttemptAt.Valid {
		action.NextAttemptAt = nextAttemptAt.Time
	}
	if completedAt.Valid {
		action.CompletedAt = completedAt.Time
	}
	return action, nil
}

func qualityEvaluationCheckStrings(checks []memory.QualityEvaluationCheck) []string {
	values := make([]string, 0, len(checks))
	for _, check := range checks {
		values = append(values, string(check))
	}
	return values
}

func qualityEvaluationChecks(values []string) []memory.QualityEvaluationCheck {
	checks := make([]memory.QualityEvaluationCheck, 0, len(values))
	for _, value := range values {
		checks = append(checks, memory.QualityEvaluationCheck(value))
	}
	return checks
}

func normalizeStringMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	return input
}

func normalizeAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return input
}
