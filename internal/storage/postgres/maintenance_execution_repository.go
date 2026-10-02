package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

const schedulerRunDetailRetention = 30 * 24 * time.Hour

type schedulerRunCursor struct {
	ObservedAt time.Time `json:"observed_at"`
	RunKey     string    `json:"run_key"`
}

func encodeSchedulerRunCursor(cursor schedulerRunCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSchedulerRunCursor(value string) (schedulerRunCursor, error) {
	if strings.TrimSpace(value) == "" {
		return schedulerRunCursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return schedulerRunCursor{}, fmt.Errorf("invalid scheduler history cursor")
	}
	var cursor schedulerRunCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.RunKey == "" || cursor.ObservedAt.IsZero() {
		return schedulerRunCursor{}, fmt.Errorf("invalid scheduler history cursor")
	}
	return cursor, nil
}

func (r *Repository) RecordSchedulerRunAttempt(ctx context.Context, attempt jobs.SchedulerRunAttempt) error {
	if err := attempt.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(attempt.RunKey) == "" || strings.TrimSpace(attempt.JobClass) == "" || attempt.Attempt < 1 || attempt.ObservedAt.IsZero() {
		return fmt.Errorf("scheduler run attempt identity and time are required")
	}
	if !attempt.State.Valid() || !attempt.Recovery.Valid() {
		return fmt.Errorf("scheduler run attempt category is invalid")
	}
	if attempt.Disposition != "" && !attempt.Disposition.Valid() {
		return fmt.Errorf("scheduler run disposition %q is invalid", attempt.Disposition)
	}
	terminal := attempt.Disposition
	retryExhausted := attempt.State == jobs.SchedulerRunExhausted
	const summaryQuery = `
INSERT INTO scheduler_run_summaries (run_key, job_class, tenant, project, namespace, cadence_window, state, attempt_count, terminal_disposition, recovery, checkpoint, source_watermark, retry_exhausted, observed_at, finished_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (run_key) DO UPDATE SET state=CASE WHEN scheduler_run_summaries.state IN ('completed','exhausted','cancelled') AND EXCLUDED.state='duplicate' THEN scheduler_run_summaries.state ELSE EXCLUDED.state END, attempt_count=GREATEST(scheduler_run_summaries.attempt_count, EXCLUDED.attempt_count), terminal_disposition=CASE WHEN scheduler_run_summaries.state IN ('completed','exhausted','cancelled') AND EXCLUDED.state='duplicate' THEN scheduler_run_summaries.terminal_disposition ELSE COALESCE(EXCLUDED.terminal_disposition, scheduler_run_summaries.terminal_disposition) END, recovery=EXCLUDED.recovery, checkpoint=COALESCE(NULLIF(EXCLUDED.checkpoint,''), scheduler_run_summaries.checkpoint), source_watermark=COALESCE(NULLIF(EXCLUDED.source_watermark,''), scheduler_run_summaries.source_watermark), retry_exhausted=scheduler_run_summaries.retry_exhausted OR EXCLUDED.retry_exhausted, observed_at=GREATEST(scheduler_run_summaries.observed_at, EXCLUDED.observed_at), finished_at=COALESCE(EXCLUDED.finished_at, scheduler_run_summaries.finished_at)`
	if _, err := r.db.Exec(ctx, summaryQuery, attempt.RunKey, attempt.JobClass, attempt.Scope.Tenant, attempt.Scope.Project, attempt.Scope.Namespace, attempt.CadenceWindow, attempt.State, attempt.Attempt, nullableDisposition(terminal), attempt.Recovery, attempt.Checkpoint, attempt.SourceWatermark, retryExhausted, attempt.ObservedAt, nullableTime(attempt.FinishedAt)); err != nil {
		return fmt.Errorf("record scheduler run summary: %w", err)
	}
	const attemptQuery = `INSERT INTO scheduler_run_attempts (run_key, attempt, state, disposition, worker_id, lease_until, checkpoint, source_watermark, retry_at, recovery, error_category, observed_at, finished_at, detail_expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	if _, err := r.db.Exec(ctx, attemptQuery, attempt.RunKey, attempt.Attempt, attempt.State, nullableDisposition(terminal), nullableSchedulerString(attempt.WorkerID), nullableTime(attempt.LeaseUntil), attempt.Checkpoint, attempt.SourceWatermark, nullableTime(attempt.RetryAt), attempt.Recovery, nullableSchedulerString(attempt.ErrorCategory), attempt.ObservedAt, nullableTime(attempt.FinishedAt), attempt.ObservedAt.Add(schedulerRunDetailRetention)); err != nil {
		return fmt.Errorf("record scheduler run attempt: %w", err)
	}
	return nil
}

func (r *Repository) RecordSchedulerRunSummary(ctx context.Context, summary jobs.SchedulerRunSummary) error {
	if err := summary.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(summary.RunKey) == "" || strings.TrimSpace(summary.JobClass) == "" || summary.ObservedAt.IsZero() || !summary.State.Valid() || !summary.Recovery.Valid() {
		return fmt.Errorf("scheduler run summary is invalid")
	}
	const query = `INSERT INTO scheduler_run_summaries (run_key, job_class, tenant, project, namespace, cadence_window, state, attempt_count, terminal_disposition, recovery, checkpoint, source_watermark, freshness, slo, retry_exhausted, cleanup_state, observed_at, finished_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) ON CONFLICT (run_key) DO UPDATE SET state=EXCLUDED.state, attempt_count=GREATEST(scheduler_run_summaries.attempt_count, EXCLUDED.attempt_count), terminal_disposition=COALESCE(EXCLUDED.terminal_disposition, scheduler_run_summaries.terminal_disposition), recovery=EXCLUDED.recovery, checkpoint=COALESCE(NULLIF(EXCLUDED.checkpoint,''), scheduler_run_summaries.checkpoint), source_watermark=COALESCE(NULLIF(EXCLUDED.source_watermark,''), scheduler_run_summaries.source_watermark), freshness=EXCLUDED.freshness, slo=EXCLUDED.slo, retry_exhausted=EXCLUDED.retry_exhausted, cleanup_state=EXCLUDED.cleanup_state, observed_at=GREATEST(scheduler_run_summaries.observed_at, EXCLUDED.observed_at), finished_at=COALESCE(EXCLUDED.finished_at, scheduler_run_summaries.finished_at)`
	if _, err := r.db.Exec(ctx, query, summary.RunKey, summary.JobClass, summary.Scope.Tenant, summary.Scope.Project, summary.Scope.Namespace, summary.CadenceWindow, summary.State, summary.AttemptCount, nullableDisposition(summary.Terminal), summary.Recovery, summary.Checkpoint, summary.SourceWatermark, boundedHistoryLabel(summary.Freshness), boundedHistoryLabel(summary.SLO), summary.RetryExhausted, boundedHistoryLabel(summary.CleanupState), summary.ObservedAt, nullableTime(summary.FinishedAt)); err != nil {
		return fmt.Errorf("record scheduler run summary: %w", err)
	}
	return nil
}

func (r *Repository) ListSchedulerRunHistory(ctx context.Context, input jobs.SchedulerRunHistoryQuery) (jobs.SchedulerRunHistoryPage, error) {
	if err := input.Scope.Validate(); err != nil {
		return jobs.SchedulerRunHistoryPage{}, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	cursor, err := decodeSchedulerRunCursor(input.Cursor)
	if err != nil {
		return jobs.SchedulerRunHistoryPage{}, err
	}
	if input.State != "" && !input.State.Valid() {
		return jobs.SchedulerRunHistoryPage{}, fmt.Errorf("invalid scheduler run state")
	}
	if input.Recovery != "" && !input.Recovery.Valid() {
		return jobs.SchedulerRunHistoryPage{}, fmt.Errorf("invalid scheduler recovery")
	}
	const query = `SELECT run_key, job_class, tenant, project, namespace, cadence_window, state, attempt_count, terminal_disposition, recovery, COALESCE(checkpoint,''), COALESCE(source_watermark,''), freshness, slo, retry_exhausted, cleanup_state, observed_at, finished_at FROM scheduler_run_summaries WHERE tenant=$1 AND project=$2 AND namespace=$3 AND ($4='' OR job_class=$4) AND ($5='' OR state=$5) AND ($6='' OR recovery=$6) AND ($7::timestamptz IS NULL OR observed_at >= $7) AND ($8::timestamptz IS NULL OR observed_at <= $8) AND ($9::timestamptz IS NULL OR (observed_at,run_key) < ($9,$10)) ORDER BY observed_at DESC, run_key DESC LIMIT $11`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, strings.TrimSpace(input.JobClass), input.State, input.Recovery, nullableTime(input.ObservedFrom), nullableTime(input.ObservedTo), nullableTime(cursor.ObservedAt), cursor.RunKey, limit)
	if err != nil {
		return jobs.SchedulerRunHistoryPage{}, fmt.Errorf("list scheduler run history: %w", err)
	}
	defer rows.Close()
	page := jobs.SchedulerRunHistoryPage{Runs: make([]jobs.SchedulerRunSummary, 0, limit)}
	for rows.Next() {
		var run jobs.SchedulerRunSummary
		var terminal sql.NullString
		var finished sql.NullTime
		if err := rows.Scan(&run.RunKey, &run.JobClass, &run.Scope.Tenant, &run.Scope.Project, &run.Scope.Namespace, &run.CadenceWindow, &run.State, &run.AttemptCount, &terminal, &run.Recovery, &run.Checkpoint, &run.SourceWatermark, &run.Freshness, &run.SLO, &run.RetryExhausted, &run.CleanupState, &run.ObservedAt, &finished); err != nil {
			return jobs.SchedulerRunHistoryPage{}, fmt.Errorf("scan scheduler run history: %w", err)
		}
		if terminal.Valid {
			run.Terminal = jobs.MaintenanceExecutionDisposition(terminal.String)
		}
		if finished.Valid {
			run.FinishedAt = finished.Time
		}
		page.Runs = append(page.Runs, run)
	}
	if err := rows.Err(); err != nil {
		return jobs.SchedulerRunHistoryPage{}, err
	}
	if len(page.Runs) == limit {
		last := page.Runs[len(page.Runs)-1]
		page.NextCursor = encodeSchedulerRunCursor(schedulerRunCursor{ObservedAt: last.ObservedAt, RunKey: last.RunKey})
	}
	return page, nil
}

func (r *Repository) ReadSchedulerRunHistory(ctx context.Context, scope memory.Scope, runKey string) (jobs.SchedulerRunSummary, []jobs.SchedulerRunAttempt, error) {
	if err := scope.Validate(); err != nil {
		return jobs.SchedulerRunSummary{}, nil, err
	}
	var summary jobs.SchedulerRunSummary
	var terminal sql.NullString
	var finished sql.NullTime
	const summaryQuery = `SELECT run_key, job_class, tenant, project, namespace, cadence_window, state, attempt_count, terminal_disposition, recovery, COALESCE(checkpoint,''), COALESCE(source_watermark,''), freshness, slo, retry_exhausted, cleanup_state, observed_at, finished_at FROM scheduler_run_summaries WHERE run_key=$1 AND tenant=$2 AND project=$3 AND namespace=$4`
	if err := r.db.QueryRow(ctx, summaryQuery, strings.TrimSpace(runKey), scope.Tenant, scope.Project, scope.Namespace).Scan(&summary.RunKey, &summary.JobClass, &summary.Scope.Tenant, &summary.Scope.Project, &summary.Scope.Namespace, &summary.CadenceWindow, &summary.State, &summary.AttemptCount, &terminal, &summary.Recovery, &summary.Checkpoint, &summary.SourceWatermark, &summary.Freshness, &summary.SLO, &summary.RetryExhausted, &summary.CleanupState, &summary.ObservedAt, &finished); err != nil {
		return jobs.SchedulerRunSummary{}, nil, err
	}
	if terminal.Valid {
		summary.Terminal = jobs.MaintenanceExecutionDisposition(terminal.String)
	}
	if finished.Valid {
		summary.FinishedAt = finished.Time
	}
	const attemptsQuery = `SELECT run_key, attempt, state, disposition, COALESCE(worker_id,''), lease_until, COALESCE(checkpoint,''), COALESCE(source_watermark,''), retry_at, recovery, COALESCE(error_category,''), observed_at, finished_at, detail_expires_at FROM scheduler_run_attempts WHERE run_key=$1 ORDER BY attempt ASC, observed_at ASC`
	rows, err := r.db.Query(ctx, attemptsQuery, summary.RunKey)
	if err != nil {
		return jobs.SchedulerRunSummary{}, nil, err
	}
	defer rows.Close()
	attempts := make([]jobs.SchedulerRunAttempt, 0, summary.AttemptCount)
	for rows.Next() {
		var attempt jobs.SchedulerRunAttempt
		var disposition sql.NullString
		var lease, retry, finished, expires sql.NullTime
		if err := rows.Scan(&attempt.RunKey, &attempt.Attempt, &attempt.State, &disposition, &attempt.WorkerID, &lease, &attempt.Checkpoint, &attempt.SourceWatermark, &retry, &attempt.Recovery, &attempt.ErrorCategory, &attempt.ObservedAt, &finished, &expires); err != nil {
			return jobs.SchedulerRunSummary{}, nil, err
		}
		attempt.JobClass, attempt.Scope, attempt.CadenceWindow = summary.JobClass, summary.Scope, summary.CadenceWindow
		if disposition.Valid {
			attempt.Disposition = jobs.MaintenanceExecutionDisposition(disposition.String)
		}
		if lease.Valid {
			attempt.LeaseUntil = lease.Time
		}
		if retry.Valid {
			attempt.RetryAt = retry.Time
		}
		if finished.Valid {
			attempt.FinishedAt = finished.Time
		}
		if expires.Valid {
			attempt.DetailExpiresAt = expires.Time
		}
		attempts = append(attempts, attempt)
	}
	return summary, attempts, rows.Err()
}

func (r *Repository) PruneSchedulerRunAttemptDetails(ctx context.Context, before time.Time, limit int) (int, error) {
	if before.IsZero() {
		return 0, fmt.Errorf("scheduler retention cutoff is required")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const query = `DELETE FROM scheduler_run_attempts WHERE id IN (SELECT id FROM scheduler_run_attempts WHERE detail_expires_at IS NOT NULL AND detail_expires_at <= $1 ORDER BY detail_expires_at ASC LIMIT $2)`
	tag, err := r.db.Exec(ctx, query, before, limit)
	if err != nil {
		return 0, fmt.Errorf("prune scheduler run attempt details: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *Repository) CancelSchedulerRun(ctx context.Context, scope memory.Scope, runKey string, now time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(runKey) == "" || now.IsZero() {
		return fmt.Errorf("scheduler cancellation identity and time are required")
	}
	const query = `WITH updated AS (UPDATE scheduler_run_summaries SET state='cancelled', terminal_disposition='cancelled', recovery='cancelled', attempt_count=attempt_count+1, observed_at=$5, finished_at=$5 WHERE run_key=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND state IN ('pending','retrying','failed') AND NOT EXISTS (SELECT 1 FROM scheduler_run_attempts a WHERE a.run_key=scheduler_run_summaries.run_key AND a.lease_until IS NOT NULL AND a.lease_until > $5) RETURNING run_key, attempt_count) INSERT INTO scheduler_run_attempts (run_key, attempt, state, disposition, recovery, observed_at, finished_at, detail_expires_at) SELECT run_key, attempt_count, 'cancelled', 'cancelled', 'cancelled', $5, $5, $5 + interval '30 days' FROM updated`
	tag, err := r.db.Exec(ctx, query, strings.TrimSpace(runKey), scope.Tenant, scope.Project, scope.Namespace, now)
	if err != nil {
		return fmt.Errorf("cancel scheduler run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scheduler run cancellation conflict")
	}
	return nil
}

func (r *Repository) RecoverSchedulerRun(ctx context.Context, scope memory.Scope, runKey string, now time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(runKey) == "" || now.IsZero() {
		return fmt.Errorf("scheduler recovery identity and time are required")
	}
	const query = `WITH updated AS (UPDATE scheduler_run_summaries SET state='pending', terminal_disposition=NULL, recovery='manual_review', retry_exhausted=false, attempt_count=attempt_count+1, observed_at=$5, finished_at=NULL WHERE run_key=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND state IN ('failed','exhausted','cancelled') AND NOT EXISTS (SELECT 1 FROM scheduler_run_attempts a WHERE a.run_key=scheduler_run_summaries.run_key AND a.lease_until IS NOT NULL AND a.lease_until > $5) RETURNING run_key, attempt_count) INSERT INTO scheduler_run_attempts (run_key, attempt, state, recovery, observed_at, detail_expires_at) SELECT run_key, attempt_count, 'recovered', 'manual_review', $5, $5 + interval '30 days' FROM updated`
	tag, err := r.db.Exec(ctx, query, strings.TrimSpace(runKey), scope.Tenant, scope.Project, scope.Namespace, now)
	if err != nil {
		return fmt.Errorf("recover scheduler run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scheduler run recovery conflict")
	}
	return nil
}

func nullableDisposition(value jobs.MaintenanceExecutionDisposition) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableSchedulerString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func boundedHistoryLabel(value string) string {
	switch value {
	case "fresh", "stale", "divergent", "missing", "foreign_scope", "lifecycle_hidden", "within_budget", "over_budget", "retained", "pruned", "unknown":
		return value
	default:
		return "unknown"
	}
}

func (r *Repository) ReadOwnedMaintenanceExecution(ctx context.Context, identity jobs.MaintenanceIdentity, workerID string) (jobs.MaintenanceExecutionState, error) {
	if err := identity.Scope.Validate(); err != nil {
		return jobs.MaintenanceExecutionState{}, err
	}
	const query = `SELECT attempt, lease_until, next_attempt_at, COALESCE(checkpoint,''), COALESCE(source_watermark,''), COALESCE(disposition,''), COALESCE(processed_count,0), COALESCE(error_category,''), started_at, finished_at FROM job_executions WHERE idempotency_key=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND worker_id=$5 ORDER BY started_at DESC LIMIT 1`
	var state jobs.MaintenanceExecutionState
	state.Identity, state.WorkerID = identity, workerID
	var next, finished sql.NullTime
	err := r.db.QueryRow(ctx, query, identity.Key(), identity.Scope.Tenant, identity.Scope.Project, identity.Scope.Namespace, workerID).Scan(&state.Attempt, &state.LeaseUntil, &next, &state.Checkpoint, &state.SourceWatermark, &state.Disposition, &state.ProcessedCount, &state.ErrorCategory, &state.StartedAt, &finished)
	if err == pgx.ErrNoRows {
		return jobs.MaintenanceExecutionState{}, err
	}
	if err != nil {
		return jobs.MaintenanceExecutionState{}, fmt.Errorf("read maintenance execution: %w", err)
	}
	if next.Valid {
		state.NextAttemptAt = next.Time
	}
	if finished.Valid {
		state.FinishedAt = finished.Time
	}
	return state, nil
}

func (r *Repository) AcquireMaintenanceLease(ctx context.Context, input jobs.MaintenanceLeaseInput) (bool, error) {
	if err := validateMaintenanceLeaseInput(input); err != nil {
		return false, err
	}
	const query = `
INSERT INTO job_executions (job_name, tenant, project, namespace, trigger_source, idempotency_key, status, attempt, worker_id, lease_until, checkpoint, source_watermark, started_at)
VALUES ($1, $2, $3, $4, 'scheduler', $5, 'running', $6, $7, $8, $9, $10, $11)
ON CONFLICT (idempotency_key) DO UPDATE SET worker_id = EXCLUDED.worker_id, lease_until = EXCLUDED.lease_until, status = 'running', attempt = GREATEST(job_executions.attempt, EXCLUDED.attempt), checkpoint = COALESCE(NULLIF(EXCLUDED.checkpoint, ''), job_executions.checkpoint), source_watermark = COALESCE(NULLIF(EXCLUDED.source_watermark, ''), job_executions.source_watermark), started_at = EXCLUDED.started_at
 WHERE job_executions.status <> 'completed' AND (job_executions.lease_until IS NULL OR job_executions.lease_until <= $12) AND (job_executions.next_attempt_at IS NULL OR job_executions.next_attempt_at <= $12)
RETURNING true`
	var acquired bool
	err := r.db.QueryRow(ctx, query, input.Identity.JobClass, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.Identity.Key(), input.Attempt, strings.TrimSpace(input.WorkerID), input.LeaseUntil, input.Checkpoint, input.Watermark, input.Identity.WindowStart, input.Now).Scan(&acquired)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("acquire maintenance lease: %w", err)
	}
	return acquired, nil
}

func (r *Repository) RenewMaintenanceLease(ctx context.Context, input jobs.MaintenanceLeaseInput) error {
	if err := validateMaintenanceLeaseInput(input); err != nil {
		return err
	}
	const query = `UPDATE job_executions SET lease_until=$2, checkpoint=$3, source_watermark=$4 WHERE idempotency_key=$1 AND tenant=$5 AND project=$6 AND namespace=$7 AND worker_id=$8 AND status='running' AND lease_until > $9`
	tag, err := r.db.Exec(ctx, query, input.Identity.Key(), input.LeaseUntil, input.Checkpoint, input.Watermark, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.WorkerID, input.Now)
	if err != nil {
		return fmt.Errorf("renew maintenance lease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("maintenance lease conflict")
	}
	return nil
}

func (r *Repository) ReclaimMaintenanceLease(ctx context.Context, input jobs.MaintenanceLeaseInput) (bool, error) {
	if err := validateMaintenanceLeaseInput(input); err != nil {
		return false, err
	}
	const query = `UPDATE job_executions SET worker_id=$2, lease_until=$3, status='running', attempt=attempt+1, started_at=$4 WHERE idempotency_key=$1 AND tenant=$5 AND project=$6 AND namespace=$7 AND status='running' AND lease_until IS NOT NULL AND lease_until <= $8 RETURNING true`
	var reclaimed bool
	err := r.db.QueryRow(ctx, query, input.Identity.Key(), input.WorkerID, input.LeaseUntil, input.Now, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.Now).Scan(&reclaimed)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reclaim maintenance lease: %w", err)
	}
	return reclaimed, nil
}

func (r *Repository) CompleteMaintenanceExecution(ctx context.Context, input jobs.MaintenanceCompletion) error {
	if err := input.Identity.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.WorkerID) == "" || input.FinishedAt.IsZero() {
		return fmt.Errorf("maintenance completion worker and time are required")
	}
	if input.ProcessedCount < 0 {
		return fmt.Errorf("maintenance processed count cannot be negative")
	}
	const query = `UPDATE job_executions SET status='completed', disposition=$2, processed_count=$3, error_category=$4, finished_at=$5, lease_until=NULL WHERE idempotency_key=$1 AND tenant=$6 AND project=$7 AND namespace=$8 AND worker_id=$9 AND status='running'`
	tag, err := r.db.Exec(ctx, query, input.Identity.Key(), input.Disposition, input.ProcessedCount, input.ErrorCategory, input.FinishedAt, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.WorkerID)
	if err != nil {
		return fmt.Errorf("complete maintenance execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("maintenance completion conflict")
	}
	return nil
}

func (r *Repository) FailMaintenanceExecution(ctx context.Context, input jobs.MaintenanceFailure) error {
	if err := input.Identity.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.WorkerID) == "" || input.FailedAt.IsZero() {
		return fmt.Errorf("maintenance failure worker and time are required")
	}
	disposition := input.Disposition
	if disposition == "" {
		const query = `UPDATE job_executions SET status='failed', disposition='failed', error_category=$2, next_attempt_at=$3, checkpoint=$4, source_watermark=$5, finished_at=$6, lease_until=NULL WHERE idempotency_key=$1 AND tenant=$7 AND project=$8 AND namespace=$9 AND worker_id=$10 AND status='running'`
		if _, err := r.db.Exec(ctx, query, input.Identity.Key(), input.ErrorCategory, nullableTime(input.NextAttemptAt), input.Checkpoint, input.Watermark, input.FailedAt, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.WorkerID); err != nil {
			return fmt.Errorf("fail maintenance execution: %w", err)
		}
		return nil
	}
	if !disposition.Valid() {
		return fmt.Errorf("maintenance failure disposition %q is invalid", disposition)
	}
	const query = `UPDATE job_executions SET status='failed', disposition=$2, error_category=$3, next_attempt_at=$4, checkpoint=$5, source_watermark=$6, finished_at=$7, lease_until=NULL WHERE idempotency_key=$1 AND tenant=$8 AND project=$9 AND namespace=$10 AND worker_id=$11 AND status='running'`
	if _, err := r.db.Exec(ctx, query, input.Identity.Key(), disposition, input.ErrorCategory, nullableTime(input.NextAttemptAt), input.Checkpoint, input.Watermark, input.FailedAt, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.WorkerID); err != nil {
		return fmt.Errorf("fail maintenance execution: %w", err)
	}
	return nil
}

func (r *Repository) ListMaintenanceExecutionHistory(ctx context.Context, scope memory.Scope, limit int, cursor *jobs.MaintenanceHistoryCursor) (jobs.MaintenanceHistoryPage, error) {
	if err := scope.Validate(); err != nil {
		return jobs.MaintenanceHistoryPage{}, err
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	const query = `SELECT job_name, tenant, project, namespace, trigger_source, idempotency_key, status, attempt, processed_count, error_message, started_at, finished_at FROM job_executions WHERE tenant=$1 AND project=$2 AND namespace=$3 AND ($4::timestamptz IS NULL OR (started_at,idempotency_key) < ($4,$5)) ORDER BY started_at DESC, idempotency_key DESC LIMIT $6`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, cursorTime(cursor), cursorID(cursor), limit)
	if err != nil {
		return jobs.MaintenanceHistoryPage{}, fmt.Errorf("list maintenance execution history: %w", err)
	}
	defer rows.Close()
	page := jobs.MaintenanceHistoryPage{Records: make([]jobs.JobExecutionRecord, 0, limit)}
	for rows.Next() {
		var record jobs.JobExecutionRecord
		var errorMessage sql.NullString
		var finishedAt sql.NullTime
		if err := rows.Scan(&record.JobName, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace, &record.TriggerSource, &record.IdempotencyKey, &record.Status, &record.Attempt, &record.ProcessedCount, &errorMessage, &record.StartedAt, &finishedAt); err != nil {
			return jobs.MaintenanceHistoryPage{}, fmt.Errorf("scan maintenance execution history: %w", err)
		}
		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}
		if finishedAt.Valid {
			record.FinishedAt = finishedAt.Time
		}
		page.Records = append(page.Records, record)
		if len(page.Records) == limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return jobs.MaintenanceHistoryPage{}, err
	}
	if len(page.Records) == limit && len(page.Records) > 0 {
		last := page.Records[len(page.Records)-1]
		page.NextCursor = &jobs.MaintenanceHistoryCursor{StartedAt: last.StartedAt, ID: last.IdempotencyKey}
	}
	return page, nil
}

func cursorTime(cursor *jobs.MaintenanceHistoryCursor) any {
	if cursor == nil || cursor.StartedAt.IsZero() {
		return nil
	}
	return cursor.StartedAt
}
func cursorID(cursor *jobs.MaintenanceHistoryCursor) any {
	if cursor == nil {
		return nil
	}
	return cursor.ID
}

func validateMaintenanceLeaseInput(input jobs.MaintenanceLeaseInput) error {
	if err := input.Identity.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.WorkerID) == "" || input.Now.IsZero() || input.LeaseUntil.IsZero() {
		return fmt.Errorf("maintenance lease worker and times are required")
	}
	if input.Attempt < 1 {
		return fmt.Errorf("maintenance attempt must be positive")
	}
	return nil
}
