package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) AcquireMaintenanceLease(ctx context.Context, input jobs.MaintenanceLeaseInput) (bool, error) {
	if err := validateMaintenanceLeaseInput(input); err != nil {
		return false, err
	}
	const query = `
INSERT INTO job_executions (job_name, tenant, project, namespace, trigger_source, idempotency_key, status, attempt, worker_id, lease_until, checkpoint, source_watermark, started_at)
VALUES ($1, $2, $3, $4, 'scheduler', $5, 'running', $6, $7, $8, $9, $10, $11)
ON CONFLICT (idempotency_key) DO UPDATE SET worker_id = EXCLUDED.worker_id, lease_until = EXCLUDED.lease_until, status = 'running', attempt = EXCLUDED.attempt, checkpoint = EXCLUDED.checkpoint, source_watermark = EXCLUDED.source_watermark, started_at = EXCLUDED.started_at
WHERE job_executions.status <> 'completed' AND (job_executions.lease_until IS NULL OR job_executions.lease_until <= $12)
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
	const query = `UPDATE job_executions SET status='failed', disposition='failed', error_category=$2, next_attempt_at=$3, checkpoint=$4, source_watermark=$5, finished_at=$6, lease_until=NULL WHERE idempotency_key=$1 AND tenant=$7 AND project=$8 AND namespace=$9 AND worker_id=$10 AND status='running'`
	if _, err := r.db.Exec(ctx, query, input.Identity.Key(), input.ErrorCategory, input.NextAttemptAt, input.Checkpoint, input.Watermark, input.FailedAt, input.Identity.Scope.Tenant, input.Identity.Scope.Project, input.Identity.Scope.Namespace, input.WorkerID); err != nil {
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
