package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func (r *Repository) ListDerivedWork(ctx context.Context, input workqueue.ListInput) (workqueue.WorkPage, error) {
	if err := input.Validate(); err != nil {
		return workqueue.WorkPage{}, err
	}
	cursor, err := workqueue.DecodeCursor(input.Cursor)
	if err != nil {
		return workqueue.WorkPage{}, err
	}
	const query = `
SELECT id, work_key, kind, tenant, project, namespace, watermark,
    idempotency_key, reference, state, attempt_count, max_attempts,
    lease_owner, lease_until, next_attempt_at, failure_category,
    loss_disposition, created_at, updated_at, terminal_at, detail_expires_at
FROM derived_work_items
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND ($4::timestamptz IS NULL OR (created_at, id) < ($4, $5))
ORDER BY created_at DESC, id DESC
LIMIT $6`
	var cursorAt any
	var cursorID any
	if !cursor.CreatedAt.IsZero() {
		cursorAt, cursorID = cursor.CreatedAt, cursor.ID
	}
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, cursorAt, cursorID, input.Limit+1)
	if err != nil {
		return workqueue.WorkPage{}, fmt.Errorf("list derived work: %w", err)
	}
	defer rows.Close()
	items := make([]workqueue.DerivedWorkItem, 0, input.Limit+1)
	for rows.Next() {
		item, err := scanDerivedWorkItem(rows)
		if err != nil {
			return workqueue.WorkPage{}, fmt.Errorf("scan derived work detail: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return workqueue.WorkPage{}, fmt.Errorf("iterate derived work detail: %w", err)
	}
	page := workqueue.WorkPage{Items: items}
	if len(items) > input.Limit {
		page.Items = items[:input.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = (workqueue.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}).Encode()
		if err != nil {
			return workqueue.WorkPage{}, err
		}
	}
	return page, nil
}

func (r *Repository) RetainDerivedWorkDetails(ctx context.Context, input workqueue.RetentionInput) (int64, error) {
	if err := input.Validate(); err != nil {
		return 0, err
	}
	const query = `
WITH expired AS (
    SELECT id FROM derived_work_attempts
    WHERE tenant = $1 AND project = $2 AND namespace = $3
      AND detail_expires_at IS NOT NULL AND detail_expires_at <= $4
    ORDER BY detail_expires_at ASC, id ASC
    LIMIT $5
)
DELETE FROM derived_work_attempts a USING expired
WHERE a.id = expired.id`
	tag, err := r.db.Exec(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Before, input.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("retain derived work details: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) ReadDerivedWorkStatus(ctx context.Context, scope memory.Scope, observedAt time.Time) (workqueue.Status, error) {
	if err := scope.Validate(); err != nil {
		return workqueue.Status{}, err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	const query = `
SELECT
    COUNT(*) FILTER (WHERE state = 'queued'),
    COUNT(*) FILTER (WHERE state IN ('claimed','running')),
    COUNT(*) FILTER (WHERE state = 'retry'),
    COUNT(*) FILTER (WHERE state = 'completed'),
    COUNT(*) FILTER (WHERE state = 'exhausted'),
    COUNT(*) FILTER (WHERE state = 'cancelled'),
    COUNT(*) FILTER (WHERE state = 'dropped'),
    MIN(created_at) FILTER (WHERE state IN ('queued','retry','claimed','running'))
FROM derived_work_items
WHERE tenant = $1 AND project = $2 AND namespace = $3`
	var status workqueue.Status
	var oldest sql.NullTime
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace).Scan(&status.Queued, &status.Running, &status.Retry, &status.Completed, &status.Exhausted, &status.Cancelled, &status.Dropped, &oldest); err != nil {
		return workqueue.Status{}, fmt.Errorf("read derived work status: %w", err)
	}
	status.Mode = workqueue.QueueModePostgresDurable
	status.Depth = status.Queued + status.Running + status.Retry
	status.ObservedAt = observedAt.UTC()
	if oldest.Valid {
		status.OldestPendingAt = oldest.Time
	}
	return status, nil
}

func (r *Repository) EnqueueDerivedWork(ctx context.Context, input workqueue.EnqueueInput) (workqueue.DerivedWorkItem, error) {
	if err := input.Validate(); err != nil {
		return workqueue.DerivedWorkItem{}, err
	}
	const query = `
INSERT INTO derived_work_items (
    work_key, kind, tenant, project, namespace, watermark, idempotency_key,
    reference, state, max_attempts, next_attempt_at, detail_expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'queued',$9,$10,$11)
ON CONFLICT (work_key) DO UPDATE SET updated_at = GREATEST(derived_work_items.updated_at, EXCLUDED.updated_at)
RETURNING id, work_key, kind, tenant, project, namespace, watermark,
    idempotency_key, reference, state, attempt_count, max_attempts,
    lease_owner, lease_until, next_attempt_at, failure_category,
    loss_disposition, created_at, updated_at, terminal_at, detail_expires_at`
	item, err := scanDerivedWorkItem(r.db.QueryRow(ctx, query,
		input.WorkKey(), string(input.Kind), input.Scope.Tenant, input.Scope.Project,
		input.Scope.Namespace, input.Watermark, input.Idempotency, input.Reference,
		input.MaxAttempts, input.Now, input.DetailExpiresAt))
	if err != nil {
		return workqueue.DerivedWorkItem{}, fmt.Errorf("enqueue derived work: %w", err)
	}
	return item, nil
}

func (r *Repository) RenewDerivedWorkLease(ctx context.Context, input workqueue.RenewLeaseInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE derived_work_items
SET lease_until = $7, updated_at = $6
WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
  AND lease_owner = $5 AND state IN ('claimed','running')`
	// The owner is bound separately from the renewal timestamps so the CAS
	// predicate cannot accidentally renew another worker's lease.
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.RenewedAt, input.LeaseUntil)
	if err != nil {
		return fmt.Errorf("renew derived work lease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return workqueue.ErrLeaseOwnershipLost
	}
	return nil
}

func (r *Repository) ClaimDerivedWork(ctx context.Context, input workqueue.ClaimInput) ([]workqueue.DerivedWorkItem, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `
WITH claimed AS (
    UPDATE derived_work_items
    SET state = 'running', lease_owner = $4, lease_until = $6,
        attempt_count = attempt_count + 1, updated_at = $5
    WHERE id IN (
        SELECT id FROM derived_work_items
        WHERE tenant = $1 AND project = $2 AND namespace = $3
          AND state IN ('queued','retry','claimed','running')
          AND ($8::text IS NULL OR kind = $8)
          AND next_attempt_at <= $5
          AND (lease_until IS NULL OR lease_until <= $5)
        ORDER BY created_at ASC, id ASC
        LIMIT $7 FOR UPDATE SKIP LOCKED
    )
    RETURNING id, work_key, kind, tenant, project, namespace, watermark,
      idempotency_key, reference, state, attempt_count, max_attempts,
      lease_owner, lease_until, next_attempt_at, failure_category,
      loss_disposition, created_at, updated_at, terminal_at, detail_expires_at
)
SELECT * FROM claimed ORDER BY created_at ASC, id ASC`
	var kind any
	if input.Kind != nil {
		kind = string(*input.Kind)
	}
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.Limit, kind)
	if err != nil {
		return nil, fmt.Errorf("claim derived work: %w", err)
	}
	defer rows.Close()
	items := make([]workqueue.DerivedWorkItem, 0, input.Limit)
	for rows.Next() {
		item, err := scanDerivedWorkItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan claimed derived work: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed derived work: %w", err)
	}
	return items, nil
}

func (r *Repository) CheckpointDerivedWork(ctx context.Context, input workqueue.CheckpointInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
WITH advanced AS (
    UPDATE derived_work_items SET updated_at = $9
    WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
      AND lease_owner = $5 AND state IN ('claimed','running')
      AND NOT EXISTS (
        SELECT 1 FROM derived_work_checkpoints c
        WHERE c.work_id = derived_work_items.id AND c.processed_offset > $7
      )
    RETURNING id
)
INSERT INTO derived_work_checkpoints (
    work_id, tenant, project, namespace, checkpoint_seq, processed_offset,
    source_watermark, committed_at
)
SELECT id, $2, $3, $4, $6, $7, $8, $9 FROM advanced`
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.Sequence, input.ProcessedOffset, input.SourceWatermark, input.CommittedAt)
	if err != nil {
		return fmt.Errorf("checkpoint derived work: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return workqueue.ErrLeaseOwnershipLost
	}
	return nil
}

func (r *Repository) CompleteDerivedWork(ctx context.Context, input workqueue.TerminalInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
WITH completed AS (
    UPDATE derived_work_items
    SET state = 'completed', lease_owner = NULL, lease_until = NULL,
        terminal_at = $6, updated_at = $6
    WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
      AND lease_owner = $5 AND state IN ('claimed','running')
    RETURNING id
)
INSERT INTO derived_work_terminal_summaries (
    work_id, tenant, project, namespace, state, disposition, checkpoint_seq,
    evidence_reference, recorded_at
)
SELECT id, $2, $3, $4, 'completed', $7, NULLIF($8, 0), NULLIF($9, ''), $6 FROM completed
ON CONFLICT (work_id) DO NOTHING`
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.CompletedAt, input.Disposition, input.CheckpointSequence, input.EvidenceReference)
	if err != nil {
		return fmt.Errorf("complete derived work: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return workqueue.ErrLeaseOwnershipLost
	}
	return nil
}

func (r *Repository) RetryDerivedWork(ctx context.Context, input workqueue.RetryInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE derived_work_items
SET state = CASE WHEN attempt_count >= max_attempts THEN 'exhausted' ELSE 'retry' END,
    failure_category = $6, next_attempt_at = $7, lease_owner = NULL,
    lease_until = NULL, terminal_at = CASE WHEN attempt_count >= max_attempts THEN $8 ELSE terminal_at END,
    updated_at = $8
WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
  AND lease_owner = $5 AND state IN ('claimed','running')`
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.WorkerID, input.FailureCategory, input.RetryAt, input.FailedAt)
	if err != nil {
		return fmt.Errorf("retry derived work: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return workqueue.ErrLeaseOwnershipLost
	}
	return nil
}

func (r *Repository) CancelDerivedWork(ctx context.Context, input workqueue.CancelInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE derived_work_items SET state = 'cancelled', terminal_at = $5,
    updated_at = $5, lease_owner = NULL, lease_until = NULL
WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
  AND state IN ('queued','retry')`
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.CancelledAt)
	if err != nil {
		return fmt.Errorf("cancel derived work: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("derived work cannot be cancelled")
	}
	return nil
}

func (r *Repository) RecoverDerivedWork(ctx context.Context, input workqueue.RecoverInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
UPDATE derived_work_items
SET state = 'queued', attempt_count = 0, failure_category = NULL,
    next_attempt_at = $5, terminal_at = NULL, updated_at = $5
WHERE id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
  AND state IN ('exhausted','cancelled')`
	tag, err := r.db.Exec(ctx, query, input.WorkID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.RecoveredAt)
	if err != nil {
		return fmt.Errorf("recover derived work: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("derived work is not recoverable")
	}
	return nil
}

func scanDerivedWorkItem(scanner interface{ Scan(...any) error }) (workqueue.DerivedWorkItem, error) {
	var item workqueue.DerivedWorkItem
	var leaseOwner, failureCategory sql.NullString
	var leaseUntil, terminalAt sql.NullTime
	var kind, state, loss string
	if err := scanner.Scan(&item.ID, &item.WorkKey, &kind, &item.Scope.Tenant, &item.Scope.Project, &item.Scope.Namespace, &item.Watermark, &item.Idempotency, &item.Reference, &state, &item.AttemptCount, &item.MaxAttempts, &leaseOwner, &leaseUntil, &item.NextAttemptAt, &failureCategory, &loss, &item.CreatedAt, &item.UpdatedAt, &terminalAt, &item.DetailExpiresAt); err != nil {
		return workqueue.DerivedWorkItem{}, err
	}
	item.Kind = workqueue.WorkKind(kind)
	item.State = workqueue.WorkState(state)
	item.LossDisposition = workqueue.LossDisposition(loss)
	item.LeaseOwner = leaseOwner.String
	item.FailureCategory = failureCategory.String
	if leaseUntil.Valid {
		item.LeaseUntil = leaseUntil.Time
	}
	if terminalAt.Valid {
		item.TerminalAt = terminalAt.Time
	}
	return item, nil
}

func derivedWorkScopeConditions(start int) string {
	return strings.Join([]string{fmt.Sprintf("tenant = $%d", start), fmt.Sprintf("project = $%d", start+1), fmt.Sprintf("namespace = $%d", start+2)}, " AND ")
}
