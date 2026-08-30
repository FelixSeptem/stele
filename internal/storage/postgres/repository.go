package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/embedding"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type queryRower interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type transactionStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Repository struct {
	db              queryRower
	tx              transactionStarter
	embeddingRouter embedding.Router
}

type governanceRawEventScanner interface {
	Scan(dest ...any) error
}

func NewRepository(db interface {
	queryRower
	transactionStarter
}) *Repository {
	return NewRepositoryWithEmbeddingRouter(db, embedding.Router{})
}

func NewRepositoryWithEmbeddingRouter(db interface {
	queryRower
	transactionStarter
}, router embedding.Router) *Repository {
	return &Repository{
		db:              db,
		tx:              db,
		embeddingRouter: router,
	}
}

func (r *Repository) WriteRawEvent(ctx context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	return writeRawEvent(ctx, r.db, input)
}

func (r *Repository) WriteProvenance(ctx context.Context, record memory.ProvenanceRecord) error {
	return writeProvenance(ctx, r.db, record)
}

func (r *Repository) IngestEvent(ctx context.Context, input memory.IngestEventInput, provenance memory.ProvenanceRecord) (memory.RawEvent, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RawEvent{}, fmt.Errorf("begin ingest transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	event, err := writeRawEvent(ctx, tx, input)
	if err != nil {
		return memory.RawEvent{}, err
	}

	provenance.RawEventID = event.ID
	if err := writeProvenance(ctx, tx, provenance); err != nil {
		return memory.RawEvent{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.RawEvent{}, fmt.Errorf("commit ingest transaction: %w", err)
	}

	return event, nil
}

func (r *Repository) CreateCandidate(ctx context.Context, candidate governance.CandidateMemory, provenance memory.ProvenanceRecord) (governance.CandidateMemory, error) {
	if err := candidate.Validate(); err != nil {
		return governance.CandidateMemory{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("begin create candidate transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := writeCandidate(ctx, tx, candidate)
	if err != nil {
		return governance.CandidateMemory{}, err
	}

	if strings.TrimSpace(provenance.RawEventID) == "" {
		provenance.RawEventID = created.SourceRawEventID
	}
	provenance.CandidateMemoryID = created.ID
	if err := writeProvenance(ctx, tx, provenance); err != nil {
		return governance.CandidateMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("commit create candidate transaction: %w", err)
	}

	return created, nil
}

func (r *Repository) ListCandidatesByRawEvent(ctx context.Context, rawEventID string) ([]governance.CandidateMemory, error) {
	if strings.TrimSpace(rawEventID) == "" {
		return nil, fmt.Errorf("raw event id is required")
	}

	const query = `
SELECT
	id,
	source_raw_event_id,
	tenant,
	project,
	namespace,
	class,
	content,
	confidence,
	importance,
	freshness,
	sensitivity,
	mutability,
	retention_class,
	status,
	created_at,
	updated_at
FROM candidate_memories
WHERE source_raw_event_id = $1
ORDER BY created_at ASC
`

	rows, err := r.db.Query(ctx, query, rawEventID)
	if err != nil {
		return nil, fmt.Errorf("list candidates by raw event: %w", err)
	}
	defer rows.Close()

	candidates := make([]governance.CandidateMemory, 0)
	for rows.Next() {
		candidate, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates by raw event: %w", err)
	}

	return candidates, nil
}

func (r *Repository) ListCandidatesForCompaction(ctx context.Context, scope memory.Scope, cutoff time.Time, limit int) ([]governance.CandidateMemory, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if cutoff.IsZero() {
		return nil, fmt.Errorf("cutoff is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	const query = `
SELECT
	id,
	source_raw_event_id,
	tenant,
	project,
	namespace,
	class,
	content,
	confidence,
	importance,
	freshness,
	sensitivity,
	mutability,
	retention_class,
	status,
	created_at,
	updated_at
FROM candidate_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND status = $4
	AND updated_at <= $5
ORDER BY updated_at ASC
LIMIT $6
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, governance.CandidateStatusPromoted, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list candidates for compaction: %w", err)
	}
	defer rows.Close()

	candidates := make([]governance.CandidateMemory, 0)
	for rows.Next() {
		candidate, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate compaction candidates: %w", err)
	}

	return candidates, nil
}

func (r *Repository) TransitionCandidateStatus(ctx context.Context, transition governance.CandidateStatusTransition, provenance memory.ProvenanceRecord) (governance.CandidateMemory, error) {
	if err := transition.Validate(); err != nil {
		return governance.CandidateMemory{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("begin candidate transition transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
UPDATE candidate_memories
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING
	id,
	source_raw_event_id,
	tenant,
	project,
	namespace,
	class,
	content,
	confidence,
	importance,
	freshness,
	sensitivity,
	mutability,
	retention_class,
	status,
	created_at,
	updated_at
`

	candidate, err := scanCandidate(tx.QueryRow(ctx, query, transition.CandidateID, transition.ToStatus, transition.UpdatedAt))
	if err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("update candidate status: %w", err)
	}

	provenance.CandidateMemoryID = candidate.ID
	if err := writeProvenance(ctx, tx, provenance); err != nil {
		return governance.CandidateMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("commit candidate transition transaction: %w", err)
	}

	return candidate, nil
}

func (r *Repository) ClaimPendingRawEvents(ctx context.Context, input governance.ClaimPendingRawEventsInput) ([]governance.ClaimedRawEvent, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const query = `
WITH claimed AS (
	UPDATE raw_events
	SET
		governance_worker_id = $1,
		governance_claimed_at = $2,
		governance_lease_until = $3,
		governance_attempt = governance_attempt + 1
	WHERE id IN (
		SELECT id
		FROM raw_events
		WHERE governance_processed_at IS NULL
			AND governance_exhausted_at IS NULL
			AND (governance_next_attempt_at IS NULL OR governance_next_attempt_at <= $2)
			AND (governance_lease_until IS NULL OR governance_lease_until <= $2)
		ORDER BY created_at ASC
		LIMIT $4
		FOR UPDATE SKIP LOCKED
	)
	RETURNING
		id,
		tenant,
		project,
		namespace,
		event_type,
		content,
		source_timestamp,
		created_at,
		governance_worker_id,
		governance_claimed_at,
		governance_lease_until,
		governance_attempt
)
SELECT
	id,
	tenant,
	project,
	namespace,
	event_type,
	content,
	source_timestamp,
	created_at,
	governance_worker_id,
	governance_claimed_at,
	governance_lease_until,
	governance_attempt
FROM claimed
ORDER BY created_at ASC
`

	rows, err := r.db.Query(ctx, query, input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("claim pending raw events: %w", err)
	}
	defer rows.Close()

	claims := make([]governance.ClaimedRawEvent, 0)
	for rows.Next() {
		var claim governance.ClaimedRawEvent
		if err := rows.Scan(
			&claim.Event.ID,
			&claim.Event.Scope.Tenant,
			&claim.Event.Scope.Project,
			&claim.Event.Scope.Namespace,
			&claim.Event.EventType,
			&claim.Event.Content,
			&claim.Event.SourceTimestamp,
			&claim.Event.CreatedAt,
			&claim.WorkerID,
			&claim.ClaimedAt,
			&claim.LeaseUntil,
			&claim.Attempt,
		); err != nil {
			return nil, fmt.Errorf("scan claimed raw event: %w", err)
		}

		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed raw events: %w", err)
	}

	return claims, nil
}

func (r *Repository) RecordClaimedRawEventFailure(ctx context.Context, input governance.RecordClaimedRawEventFailureInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	const query = `
UPDATE raw_events
SET
	governance_last_failed_at = $3,
	governance_last_error = $4,
	governance_next_attempt_at = $5,
	governance_exhausted_at = $6,
	governance_worker_id = NULL,
	governance_lease_until = NULL
WHERE id = $1
	AND governance_worker_id = $2
	AND governance_processed_at IS NULL
`

	tag, err := r.db.Exec(
		ctx,
		query,
		input.RawEventID,
		input.WorkerID,
		input.FailedAt,
		input.ErrorMessage,
		nullableTime(input.NextAttemptAt),
		nullableTime(input.ExhaustedAt),
	)
	if err != nil {
		return fmt.Errorf("record claimed raw event failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return governance.ErrClaimOwnershipLost
	}

	return nil
}

func (r *Repository) RenewClaimedRawEventLease(ctx context.Context, input governance.RenewClaimedRawEventLeaseInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	const query = `
UPDATE raw_events
SET
	governance_claimed_at = $3,
	governance_lease_until = $4
WHERE id = $1
	AND governance_worker_id = $2
	AND governance_processed_at IS NULL
	AND governance_exhausted_at IS NULL
`

	tag, err := r.db.Exec(ctx, query, input.RawEventID, input.WorkerID, input.RenewedAt, input.LeaseUntil)
	if err != nil {
		return fmt.Errorf("renew claimed raw event lease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return governance.ErrClaimOwnershipLost
	}

	return nil
}

func (r *Repository) MarkRawEventProcessed(ctx context.Context, input governance.CompleteClaimedRawEventInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	const query = `
UPDATE raw_events
SET
	governance_processed_at = $3,
	governance_lease_until = NULL
WHERE id = $1 AND governance_worker_id = $2
`

	if _, err := r.db.Exec(ctx, query, input.RawEventID, input.WorkerID, input.ProcessedAt); err != nil {
		return fmt.Errorf("mark raw event processed: %w", err)
	}

	return nil
}

func (r *Repository) ReadGovernanceStatus(ctx context.Context, now time.Time) (jobs.GovernanceStatus, error) {
	const query = `
SELECT
	COUNT(*) FILTER (
		WHERE governance_processed_at IS NULL
			AND (governance_lease_until IS NULL OR governance_lease_until <= $1)
	) AS pending_raw_events,
	COUNT(*) FILTER (
		WHERE governance_processed_at IS NULL
			AND governance_lease_until > $1
	) AS leased_raw_events,
	COUNT(*) FILTER (
		WHERE governance_processed_at IS NOT NULL
	) AS processed_raw_events,
	MIN(created_at) FILTER (
		WHERE governance_processed_at IS NULL
			AND (governance_lease_until IS NULL OR governance_lease_until <= $1)
	) AS oldest_pending_created_at
FROM raw_events
`

	var status jobs.GovernanceStatus
	var oldestPending sql.NullTime
	if err := r.db.QueryRow(ctx, query, now).Scan(
		&status.PendingRawEvents,
		&status.LeasedRawEvents,
		&status.ProcessedRawEvents,
		&oldestPending,
	); err != nil {
		return jobs.GovernanceStatus{}, fmt.Errorf("read governance status: %w", err)
	}

	if oldestPending.Valid {
		status.OldestPendingCreatedAt = oldestPending.Time
	}
	status.ObservedAt = now
	return status, nil
}

func (r *Repository) ListGovernanceRawEvents(ctx context.Context, input governance.ListGovernanceRawEventsInput) (governance.GovernanceRawEventPage, error) {
	if err := input.Validate(); err != nil {
		return governance.GovernanceRawEventPage{}, err
	}

	var cursorCreatedAt any
	var cursorRawEventID any
	if strings.TrimSpace(input.Cursor) != "" {
		cursor, err := governance.DecodeGovernanceRawEventCursor(input.Cursor)
		if err != nil {
			return governance.GovernanceRawEventPage{}, err
		}
		cursorCreatedAt = cursor.CreatedAt
		cursorRawEventID = cursor.RawEventID
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	event_type,
	content,
	source_timestamp,
	created_at,
	governance_attempt,
	governance_worker_id,
	governance_claimed_at,
	governance_lease_until,
	governance_last_failed_at,
	governance_last_error,
	governance_next_attempt_at,
	governance_exhausted_at,
	governance_processed_at
FROM raw_events
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND ($4::text = '' OR event_type = $4)
	AND (
		$5::text = ''
		OR ($5 = 'processed' AND governance_processed_at IS NOT NULL)
		OR ($5 = 'exhausted' AND governance_processed_at IS NULL AND governance_exhausted_at IS NOT NULL)
		OR ($5 = 'leased' AND governance_processed_at IS NULL AND governance_exhausted_at IS NULL AND governance_lease_until > $13)
		OR ($5 = 'retry_wait' AND governance_processed_at IS NULL AND governance_exhausted_at IS NULL AND (governance_lease_until IS NULL OR governance_lease_until <= $13) AND governance_next_attempt_at > $13)
		OR ($5 = 'pending' AND governance_processed_at IS NULL AND governance_exhausted_at IS NULL AND (governance_lease_until IS NULL OR governance_lease_until <= $13) AND (governance_next_attempt_at IS NULL OR governance_next_attempt_at <= $13))
	)
	AND ($6::int IS NULL OR governance_attempt >= $6)
	AND ($7::int IS NULL OR governance_attempt <= $7)
	AND ($8::timestamptz IS NULL OR governance_last_failed_at >= $8)
	AND ($9::timestamptz IS NULL OR governance_last_failed_at <= $9)
	AND ($10::timestamptz IS NULL OR governance_next_attempt_at >= $10)
	AND ($11::timestamptz IS NULL OR governance_next_attempt_at <= $11)
	AND (
		$12::timestamptz IS NULL
		OR created_at < $12
		OR (created_at = $12 AND id < $14)
	)
ORDER BY created_at DESC, id DESC
LIMIT $15
`

	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		strings.TrimSpace(input.EventType),
		string(input.State),
		nullableInt(input.AttemptGTE),
		nullableInt(input.AttemptLTE),
		nullableTime(input.FailedFrom),
		nullableTime(input.FailedTo),
		nullableTime(input.NextAttemptFrom),
		nullableTime(input.NextAttemptTo),
		cursorCreatedAt,
		input.Now.UTC(),
		cursorRawEventID,
		input.Limit+1,
	)
	if err != nil {
		return governance.GovernanceRawEventPage{}, fmt.Errorf("list governance raw events: %w", err)
	}
	defer rows.Close()

	items := make([]governance.GovernanceRawEvent, 0, input.Limit+1)
	for rows.Next() {
		resource, _, err := scanGovernanceRawEvent(rows, input.Now.UTC())
		if err != nil {
			return governance.GovernanceRawEventPage{}, err
		}
		items = append(items, resource)
	}
	if err := rows.Err(); err != nil {
		return governance.GovernanceRawEventPage{}, fmt.Errorf("iterate governance raw events: %w", err)
	}

	page := governance.GovernanceRawEventPage{Items: items}
	if len(items) > input.Limit {
		page.Items = items[:input.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = governance.GovernanceRawEventCursor{
			CreatedAt:  last.CreatedAt,
			RawEventID: last.ID,
		}.Encode()
	}

	return page, nil
}

func (r *Repository) ReadGovernanceRawEvent(ctx context.Context, input governance.ReadGovernanceRawEventInput) (governance.GovernanceRawEvent, error) {
	if err := input.Validate(); err != nil {
		return governance.GovernanceRawEvent{}, err
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	event_type,
	content,
	source_timestamp,
	created_at,
	governance_attempt,
	governance_worker_id,
	governance_claimed_at,
	governance_lease_until,
	governance_last_failed_at,
	governance_last_error,
	governance_next_attempt_at,
	governance_exhausted_at,
	governance_processed_at
FROM raw_events
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	resource, _, err := scanGovernanceRawEvent(
		r.db.QueryRow(ctx, query, input.RawEventID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace),
		input.Now.UTC(),
	)
	if err != nil {
		return governance.GovernanceRawEvent{}, fmt.Errorf("read governance raw event: %w", err)
	}

	return resource, nil
}

func (r *Repository) ListGovernanceRecoveryHistory(ctx context.Context, input governance.ListGovernanceRecoveryHistoryInput) ([]governance.GovernanceRecoveryRecord, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const existsQuery = `
SELECT 1
FROM raw_events
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	if err := r.db.QueryRow(ctx, existsQuery, input.RawEventID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace).Scan(new(int)); err != nil {
		return nil, fmt.Errorf("read governance recovery history target: %w", err)
	}

	const query = `
SELECT
	id,
	raw_event_id,
	tenant,
	project,
	namespace,
	action,
	actor,
	reason,
	before_snapshot,
	after_snapshot,
	created_at
FROM governance_recovery_ledger
WHERE raw_event_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
ORDER BY created_at DESC, id DESC
`

	rows, err := r.db.Query(ctx, query, input.RawEventID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list governance recovery history: %w", err)
	}
	defer rows.Close()

	records := make([]governance.GovernanceRecoveryRecord, 0)
	for rows.Next() {
		record, err := scanGovernanceRecoveryRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate governance recovery history: %w", err)
	}

	return records, nil
}

func (r *Repository) ApplyGovernanceRecovery(ctx context.Context, input governance.ApplyGovernanceRecoveryInput) (governance.GovernanceRecoveryOutcome, error) {
	if err := input.Validate(); err != nil {
		return governance.GovernanceRecoveryOutcome{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, fmt.Errorf("begin governance recovery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
SELECT
	id,
	tenant,
	project,
	namespace,
	event_type,
	content,
	source_timestamp,
	created_at,
	governance_attempt,
	governance_worker_id,
	governance_claimed_at,
	governance_lease_until,
	governance_last_failed_at,
	governance_last_error,
	governance_next_attempt_at,
	governance_exhausted_at,
	governance_processed_at
FROM raw_events
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
FOR UPDATE
`

	currentResource, currentSnapshot, err := scanGovernanceRawEvent(
		tx.QueryRow(ctx, selectQuery, input.RawEventID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace),
		input.AppliedAt.UTC(),
	)
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, fmt.Errorf("read governance recovery target: %w", err)
	}

	nextSnapshot, err := governance.ApplyGovernanceRecovery(currentSnapshot, input)
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, err
	}

	const updateQuery = `
UPDATE raw_events
SET
	governance_attempt = $5,
	governance_worker_id = $6,
	governance_claimed_at = $7,
	governance_lease_until = $8,
	governance_last_failed_at = $9,
	governance_last_error = $10,
	governance_next_attempt_at = $11,
	governance_exhausted_at = $12,
	governance_processed_at = $13
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
RETURNING
	id,
	tenant,
	project,
	namespace,
	event_type,
	content,
	source_timestamp,
	created_at,
	governance_attempt,
	governance_worker_id,
	governance_claimed_at,
	governance_lease_until,
	governance_last_failed_at,
	governance_last_error,
	governance_next_attempt_at,
	governance_exhausted_at,
	governance_processed_at
`

	updatedResource, updatedSnapshot, err := scanGovernanceRawEvent(
		tx.QueryRow(
			ctx,
			updateQuery,
			input.RawEventID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			nextSnapshot.Attempt,
			nullableString(nextSnapshot.WorkerID),
			nullableTime(nextSnapshot.ClaimedAt),
			nullableTime(nextSnapshot.LeaseUntil),
			nullableTime(nextSnapshot.LastFailedAt),
			nullableString(nextSnapshot.LastError),
			nullableTime(nextSnapshot.NextAttemptAt),
			nullableTime(nextSnapshot.ExhaustedAt),
			nullableTime(nextSnapshot.ProcessedAt),
		),
		input.AppliedAt.UTC(),
	)
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, fmt.Errorf("update governance recovery target: %w", err)
	}

	beforeSnapshotJSON, err := marshalGovernanceRecoverySnapshot(governance.NewGovernanceRecoverySnapshot(currentSnapshot, input.AppliedAt.UTC()))
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, err
	}
	afterSnapshotJSON, err := marshalGovernanceRecoverySnapshot(governance.NewGovernanceRecoverySnapshot(updatedSnapshot, input.AppliedAt.UTC()))
	if err != nil {
		return governance.GovernanceRecoveryOutcome{}, err
	}

	recovery := governance.GovernanceRecoveryRecord{
		ID:         uuid.NewString(),
		RawEventID: currentResource.ID,
		Scope:      input.Scope,
		Action:     input.Action,
		Actor:      input.Actor,
		Reason:     input.Reason,
		Before:     governance.NewGovernanceRecoverySnapshot(currentSnapshot, input.AppliedAt.UTC()),
		After:      governance.NewGovernanceRecoverySnapshot(updatedSnapshot, input.AppliedAt.UTC()),
		OccurredAt: input.AppliedAt.UTC(),
	}

	const insertLedgerQuery = `
INSERT INTO governance_recovery_ledger (
	id,
	raw_event_id,
	tenant,
	project,
	namespace,
	action,
	actor,
	reason,
	before_snapshot,
	after_snapshot,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`

	if _, err := tx.Exec(
		ctx,
		insertLedgerQuery,
		recovery.ID,
		recovery.RawEventID,
		recovery.Scope.Tenant,
		recovery.Scope.Project,
		recovery.Scope.Namespace,
		string(recovery.Action),
		recovery.Actor,
		recovery.Reason,
		beforeSnapshotJSON,
		afterSnapshotJSON,
		recovery.OccurredAt,
	); err != nil {
		return governance.GovernanceRecoveryOutcome{}, fmt.Errorf("insert governance recovery ledger: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return governance.GovernanceRecoveryOutcome{}, fmt.Errorf("commit governance recovery transaction: %w", err)
	}

	return governance.GovernanceRecoveryOutcome{
		RawEvent: updatedResource,
		Recovery: recovery,
	}, nil
}

func (r *Repository) ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	const query = `
SELECT tenant, project, namespace FROM (
	SELECT DISTINCT tenant, project, namespace
	FROM canonical_memories
	WHERE state = 'active'
	UNION
	SELECT DISTINCT tenant, project, namespace
	FROM candidate_memories
	WHERE status = 'promoted'
) scoped
ORDER BY tenant, project, namespace
LIMIT $1
`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list maintenance scopes: %w", err)
	}
	defer rows.Close()

	scopes := make([]memory.Scope, 0)
	for rows.Next() {
		var scope memory.Scope
		if err := rows.Scan(&scope.Tenant, &scope.Project, &scope.Namespace); err != nil {
			return nil, fmt.Errorf("scan maintenance scope: %w", err)
		}
		scopes = append(scopes, scope.Normalized())
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate maintenance scopes: %w", err)
	}

	return scopes, nil
}

func (r *Repository) ReadCanonicalMemory(ctx context.Context, scope memory.Scope, memoryID string, includeHidden bool) (memory.CanonicalMemory, error) {
	if err := scope.Validate(); err != nil {
		return memory.CanonicalMemory{}, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return memory.CanonicalMemory{}, fmt.Errorf("memory id is required")
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at
FROM canonical_memories
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND ($5 OR state NOT IN ('suppressed', 'forgotten', 'deleted'))
`

	var canonical memory.CanonicalMemory
	if err := r.db.QueryRow(ctx, query, memoryID, scope.Tenant, scope.Project, scope.Namespace, includeHidden).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("read canonical memory: %w", err)
	}

	return canonical, nil
}

func (r *Repository) ReadMemoryProvenance(ctx context.Context, scope memory.Scope, memoryID string) ([]memory.ProvenanceRecord, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return nil, fmt.Errorf("memory id is required")
	}

	const query = `
SELECT
	id,
	raw_event_id,
	candidate_memory_id,
	memory_id,
	tenant,
	project,
	namespace,
	operation,
	request_id,
	actor,
	source_context,
	created_at
FROM provenance_links
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = $4
ORDER BY created_at ASC
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memoryID)
	if err != nil {
		return nil, fmt.Errorf("read memory provenance: %w", err)
	}
	defer rows.Close()

	records := make([]memory.ProvenanceRecord, 0)
	for rows.Next() {
		record, err := scanProvenanceRecord(rows)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory provenance: %w", err)
	}

	return records, nil
}

func (r *Repository) ReadMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string, includeHidden bool) (memory.MemoryHistory, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryHistory{}, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return memory.MemoryHistory{}, fmt.Errorf("memory id is required")
	}

	var history memory.MemoryHistory
	canonical, err := r.ReadCanonicalMemory(ctx, scope, memoryID, includeHidden)
	if err != nil {
		return memory.MemoryHistory{}, err
	}
	history.Memory = canonical

	const versionsQuery = `
SELECT
	id,
	memory_id,
	version,
	state,
	content,
	created_at,
	modified_by
FROM memory_versions
WHERE memory_id = $1
ORDER BY version DESC
`

	versionRows, err := r.db.Query(ctx, versionsQuery, memoryID)
	if err != nil {
		return memory.MemoryHistory{}, fmt.Errorf("read memory versions: %w", err)
	}
	defer versionRows.Close()

	for versionRows.Next() {
		var version memory.MemoryVersion
		if err := versionRows.Scan(
			&version.ID,
			&version.MemoryID,
			&version.Version,
			&version.State,
			&version.Content,
			&version.CreatedAt,
			&version.ModifiedBy,
		); err != nil {
			return memory.MemoryHistory{}, fmt.Errorf("scan memory version: %w", err)
		}

		history.Versions = append(history.Versions, version)
	}
	if err := versionRows.Err(); err != nil {
		return memory.MemoryHistory{}, fmt.Errorf("iterate memory versions: %w", err)
	}

	const provenanceQuery = `
SELECT
	id,
	raw_event_id,
	candidate_memory_id,
	memory_id,
	tenant,
	project,
	namespace,
	operation,
	request_id,
	actor,
	source_context,
	created_at
FROM provenance_links
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = $4
ORDER BY created_at ASC
`

	provenanceRows, err := r.db.Query(ctx, provenanceQuery, scope.Tenant, scope.Project, scope.Namespace, memoryID)
	if err != nil {
		return memory.MemoryHistory{}, fmt.Errorf("read provenance history: %w", err)
	}
	defer provenanceRows.Close()

	for provenanceRows.Next() {
		record, err := scanProvenanceRecord(provenanceRows)
		if err != nil {
			return memory.MemoryHistory{}, err
		}

		history.Provenance = append(history.Provenance, record)
	}
	if err := provenanceRows.Err(); err != nil {
		return memory.MemoryHistory{}, fmt.Errorf("iterate provenance history: %w", err)
	}

	return history, nil
}

func (r *Repository) BeginJobExecution(ctx context.Context, execution jobs.JobExecution) (bool, error) {
	if err := execution.Scope.Validate(); err != nil {
		return false, err
	}
	if strings.TrimSpace(execution.JobName) == "" {
		return false, fmt.Errorf("job name is required")
	}
	if strings.TrimSpace(execution.TriggerSource) == "" {
		return false, fmt.Errorf("trigger source is required")
	}
	if strings.TrimSpace(execution.IdempotencyKey) == "" {
		return false, fmt.Errorf("idempotency key is required")
	}
	if execution.StartedAt.IsZero() {
		return false, fmt.Errorf("started at is required")
	}

	const insertQuery = `
INSERT INTO job_executions (
	job_name,
	tenant,
	project,
	namespace,
	trigger_source,
	idempotency_key,
	status,
	started_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`

	if _, err := r.db.Exec(
		ctx,
		insertQuery,
		execution.JobName,
		execution.Scope.Tenant,
		execution.Scope.Project,
		execution.Scope.Namespace,
		execution.TriggerSource,
		execution.IdempotencyKey,
		jobs.JobExecutionStatusRunning,
		execution.StartedAt,
	); err == nil {
		return true, nil
	} else if !isUniqueViolation(err) {
		return false, fmt.Errorf("begin job execution: %w", err)
	}

	const statusQuery = `
SELECT status, started_at
FROM job_executions
WHERE idempotency_key = $1
`

	var status string
	var startedAt time.Time
	if err := r.db.QueryRow(ctx, statusQuery, execution.IdempotencyKey).Scan(&status, &startedAt); err != nil {
		return false, fmt.Errorf("read existing job execution: %w", err)
	}

	if jobs.JobExecutionStatus(status) == jobs.JobExecutionStatusRunning && time.Since(startedAt) < 15*time.Minute {
		return false, nil
	}
	if jobs.JobExecutionStatus(status) == jobs.JobExecutionStatusCompleted {
		return false, nil
	}

	const retryQuery = `
UPDATE job_executions
SET
	status = $2,
	attempt = attempt + 1,
	processed_count = 0,
	error_message = NULL,
	started_at = $3,
	finished_at = NULL
WHERE idempotency_key = $1
`

	if _, err := r.db.Exec(ctx, retryQuery, execution.IdempotencyKey, jobs.JobExecutionStatusRunning, execution.StartedAt); err != nil {
		return false, fmt.Errorf("retry job execution: %w", err)
	}

	return true, nil
}

func (r *Repository) CompleteJobExecution(ctx context.Context, completion jobs.JobExecutionCompletion) error {
	if strings.TrimSpace(completion.IdempotencyKey) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if completion.FinishedAt.IsZero() {
		return fmt.Errorf("finished at is required")
	}

	const query = `
UPDATE job_executions
SET
	status = $2,
	processed_count = $3,
	error_message = NULL,
	finished_at = $4
WHERE idempotency_key = $1
`

	if _, err := r.db.Exec(ctx, query, completion.IdempotencyKey, jobs.JobExecutionStatusCompleted, completion.ProcessedCount, completion.FinishedAt); err != nil {
		return fmt.Errorf("complete job execution: %w", err)
	}

	return nil
}

func (r *Repository) FailJobExecution(ctx context.Context, failure jobs.JobExecutionFailure) error {
	if strings.TrimSpace(failure.IdempotencyKey) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if failure.FinishedAt.IsZero() {
		return fmt.Errorf("finished at is required")
	}

	const query = `
UPDATE job_executions
SET
	status = $2,
	error_message = $3,
	finished_at = $4
WHERE idempotency_key = $1
`

	if _, err := r.db.Exec(ctx, query, failure.IdempotencyKey, jobs.JobExecutionStatusFailed, failure.ErrorMessage, failure.FinishedAt); err != nil {
		return fmt.Errorf("fail job execution: %w", err)
	}

	return nil
}

func (r *Repository) ListRecentJobExecutions(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}

	const query = `
SELECT
	job_name,
	tenant,
	project,
	namespace,
	trigger_source,
	idempotency_key,
	status,
	attempt,
	processed_count,
	error_message,
	started_at,
	finished_at
FROM job_executions
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
ORDER BY started_at DESC
LIMIT $4
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent job executions: %w", err)
	}
	defer rows.Close()

	records := make([]jobs.JobExecutionRecord, 0)
	for rows.Next() {
		var record jobs.JobExecutionRecord
		var errorMessage sql.NullString
		var finishedAt sql.NullTime
		if err := rows.Scan(
			&record.JobName,
			&record.Scope.Tenant,
			&record.Scope.Project,
			&record.Scope.Namespace,
			&record.TriggerSource,
			&record.IdempotencyKey,
			&record.Status,
			&record.Attempt,
			&record.ProcessedCount,
			&errorMessage,
			&record.StartedAt,
			&finishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job execution: %w", err)
		}

		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}
		if finishedAt.Valid {
			record.FinishedAt = finishedAt.Time
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job executions: %w", err)
	}

	return records, nil
}

func (r *Repository) ListRetentionTargets(ctx context.Context, scope memory.Scope, limit int) ([]governance.RetentionTarget, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	retention_class,
	updated_at
FROM canonical_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND state = $4
ORDER BY updated_at ASC
LIMIT $5
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memory.MemoryStateActive, limit)
	if err != nil {
		return nil, fmt.Errorf("list retention targets: %w", err)
	}
	defer rows.Close()

	targets := make([]governance.RetentionTarget, 0)
	for rows.Next() {
		var target governance.RetentionTarget
		if err := rows.Scan(
			&target.MemoryID,
			&target.Scope.Tenant,
			&target.Scope.Project,
			&target.Scope.Namespace,
			&target.RetentionClass,
			&target.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan retention target: %w", err)
		}

		targets = append(targets, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention targets: %w", err)
	}

	return targets, nil
}

func (r *Repository) DeleteJobExecutionsBefore(ctx context.Context, cutoff time.Time) (int, error) {
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff is required")
	}

	const query = `
DELETE FROM job_executions
WHERE finished_at IS NOT NULL
	AND finished_at < $1
`

	tag, err := r.db.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete job executions before cutoff: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func (r *Repository) GetLatestCanonicalByScopeAndClass(ctx context.Context, scope memory.Scope, class memory.MemoryClass) (memory.CanonicalMemory, bool, error) {
	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at
FROM canonical_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND class = $4
	AND state = $5
ORDER BY updated_at DESC
LIMIT 1
`

	var canonical memory.CanonicalMemory
	err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, class, memory.MemoryStateActive).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return memory.CanonicalMemory{}, false, nil
		}

		return memory.CanonicalMemory{}, false, fmt.Errorf("get latest canonical memory: %w", err)
	}

	return canonical, true, nil
}

func (r *Repository) PromoteCandidate(ctx context.Context, input governance.CanonicalPromotion) (memory.CanonicalMemory, memory.MemoryVersion, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("begin canonical promotion transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	versionNumber, err := nextMemoryVersion(ctx, tx, input.MemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
	}

	if versionNumber == 1 {
		const canonicalInsertQuery = `
INSERT INTO canonical_memories (
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	retention_class,
	content,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, to_tsvector('simple', $8), $9, $10)
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

		var canonical memory.CanonicalMemory
		if err := tx.QueryRow(
			ctx,
			canonicalInsertQuery,
			input.MemoryID,
			input.Candidate.Scope.Tenant,
			input.Candidate.Scope.Project,
			input.Candidate.Scope.Namespace,
			input.Candidate.Class,
			memory.MemoryStateActive,
			input.Candidate.RetentionClass,
			input.Candidate.Content,
			input.CreatedAt,
			input.CreatedAt,
		).Scan(
			&canonical.ID,
			&canonical.Scope.Tenant,
			&canonical.Scope.Project,
			&canonical.Scope.Namespace,
			&canonical.Class,
			&canonical.State,
			&canonical.Content,
			&canonical.CreatedAt,
			&canonical.ModifiedAt,
		); err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("insert canonical memory: %w", err)
		}

		version, err := writeMemoryVersion(ctx, tx, input, versionNumber)
		if err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
		}

		if err := upsertRelationProjection(ctx, tx, canonical, input.CreatedAt); err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
		}

		if err := writeProvenance(ctx, tx, memory.ProvenanceRecord{
			ID:                promotionProvenanceID(input.VersionID),
			Scope:             input.Candidate.Scope,
			RawEventID:        input.Candidate.SourceRawEventID,
			CandidateMemoryID: input.Candidate.ID,
			MemoryID:          input.MemoryID,
			Operation:         "promote_candidate",
			CreatedAt:         input.CreatedAt,
		}); err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("commit canonical promotion transaction: %w", err)
		}

		return canonical, version, nil
	}

	const canonicalUpdateQuery = `
UPDATE canonical_memories
SET state = $2, retention_class = $3, content = $4, search_text = to_tsvector('simple', $4), updated_at = $5
WHERE id = $1
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		canonicalUpdateQuery,
		input.MemoryID,
		memory.MemoryStateActive,
		input.Candidate.RetentionClass,
		input.Candidate.Content,
		input.CreatedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("update canonical memory: %w", err)
	}

	version, err := writeMemoryVersion(ctx, tx, input, versionNumber)
	if err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
	}

	if err := upsertRelationProjection(ctx, tx, canonical, input.CreatedAt); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
	}

	if err := writeProvenance(ctx, tx, memory.ProvenanceRecord{
		ID:                promotionProvenanceID(input.VersionID),
		Scope:             input.Candidate.Scope,
		RawEventID:        input.Candidate.SourceRawEventID,
		CandidateMemoryID: input.Candidate.ID,
		MemoryID:          input.MemoryID,
		Operation:         "promote_candidate",
		CreatedAt:         input.CreatedAt,
	}); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("commit canonical promotion transaction: %w", err)
	}

	return canonical, version, nil
}

func writeMemoryVersion(ctx context.Context, tx pgx.Tx, input governance.CanonicalPromotion, versionNumber int64) (memory.MemoryVersion, error) {
	const versionQuery = `
INSERT INTO memory_versions (
	id,
	memory_id,
	version,
	state,
	content,
	created_at,
	modified_by
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, memory_id, version, state, content, created_at, modified_by
`

	var version memory.MemoryVersion
	if err := tx.QueryRow(
		ctx,
		versionQuery,
		input.VersionID,
		input.MemoryID,
		versionNumber,
		memory.MemoryStateActive,
		input.Candidate.Content,
		input.CreatedAt,
		input.Candidate.ID,
	).Scan(
		&version.ID,
		&version.MemoryID,
		&version.Version,
		&version.State,
		&version.Content,
		&version.CreatedAt,
		&version.ModifiedBy,
	); err != nil {
		return memory.MemoryVersion{}, fmt.Errorf("insert memory version: %w", err)
	}

	return version, nil
}

func promotionProvenanceID(versionID string) string {
	if _, err := uuid.Parse(versionID); err == nil {
		return uuid.NewSHA1(uuid.NameSpaceURL, []byte("stele:promotion-provenance:"+versionID)).String()
	}
	return versionID + "_prov"
}

func (r *Repository) CreateSummaryMemory(ctx context.Context, input governance.SummaryMemoryRecord) (memory.CanonicalMemory, memory.MemoryVersion, error) {
	if err := input.Validate(); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("begin summary memory transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const canonicalQuery = `
INSERT INTO canonical_memories (
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	retention_class,
	content,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, to_tsvector('simple', $8), $9, $10)
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		canonicalQuery,
		input.MemoryID,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		memory.MemoryClassSummary,
		memory.MemoryStateActive,
		policy.RetentionClassDurable,
		input.Content,
		input.CreatedAt,
		input.CreatedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("insert summary canonical memory: %w", err)
	}

	const versionQuery = `
INSERT INTO memory_versions (
	id,
	memory_id,
	version,
	state,
	content,
	created_at,
	modified_by
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, memory_id, version, state, content, created_at, modified_by
`

	var version memory.MemoryVersion
	if err := tx.QueryRow(
		ctx,
		versionQuery,
		input.VersionID,
		input.MemoryID,
		int64(1),
		memory.MemoryStateActive,
		input.Content,
		input.CreatedAt,
		"summary_compactor",
	).Scan(
		&version.ID,
		&version.MemoryID,
		&version.Version,
		&version.State,
		&version.Content,
		&version.CreatedAt,
		&version.ModifiedBy,
	); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("insert summary memory version: %w", err)
	}

	for i, rawEventID := range input.EvidenceRawEventIDs {
		if err := writeProvenance(ctx, tx, memory.ProvenanceRecord{
			ID:         fmt.Sprintf("%s_prov_%d", input.VersionID, i),
			Scope:      input.Scope,
			RawEventID: rawEventID,
			MemoryID:   input.MemoryID,
			Operation:  "create_summary_memory",
			CreatedAt:  input.CreatedAt,
		}); err != nil {
			return memory.CanonicalMemory{}, memory.MemoryVersion{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("commit summary memory transaction: %w", err)
	}

	return canonical, version, nil
}

func (r *Repository) ListCanonicalMemories(ctx context.Context, scope memory.Scope, includeHidden bool) ([]memory.CanonicalMemory, error) {
	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at
FROM canonical_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND ($4 OR state NOT IN ('suppressed', 'forgotten', 'deleted'))
ORDER BY updated_at DESC
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, includeHidden)
	if err != nil {
		return nil, fmt.Errorf("list canonical memories: %w", err)
	}
	defer rows.Close()

	memories := make([]memory.CanonicalMemory, 0)
	for rows.Next() {
		var canonical memory.CanonicalMemory
		if err := rows.Scan(
			&canonical.ID,
			&canonical.Scope.Tenant,
			&canonical.Scope.Project,
			&canonical.Scope.Namespace,
			&canonical.Class,
			&canonical.State,
			&canonical.Content,
			&canonical.CreatedAt,
			&canonical.ModifiedAt,
		); err != nil {
			return nil, fmt.Errorf("scan canonical memory: %w", err)
		}

		memories = append(memories, canonical)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical memories: %w", err)
	}

	return memories, nil
}

func (r *Repository) CreateMemory(ctx context.Context, record memory.ManualCreateMemoryRecord) (memory.CanonicalMemory, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("begin manual create transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	versionNumber, err := nextMemoryVersion(ctx, tx, record.MemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if versionNumber != 1 {
		return memory.CanonicalMemory{}, fmt.Errorf("%w: manual create memory id already exists", memory.ErrManualMutationRejected)
	}

	const canonicalQuery = `
INSERT INTO canonical_memories (
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	retention_class,
	content,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, to_tsvector('simple', $8), $9, $10)
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		canonicalQuery,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Class,
		memory.MemoryStateActive,
		policy.RetentionClassDurable,
		record.Content,
		record.CreatedAt,
		record.CreatedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("insert manual canonical memory: %w", err)
	}

	if _, err := writeManualMemoryVersion(ctx, tx, record.VersionID, record.MemoryID, versionNumber, canonical.State, record.Content, record.CreatedAt, record.Actor); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := upsertRelationProjection(ctx, tx, canonical, record.CreatedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := writeManualProvenance(ctx, tx, record.Scope, record.MemoryID, "manual_create_memory", record.RequestID, record.Actor, record.CreatedAt, map[string]any{
		"reason": record.Reason,
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := r.recordEmbeddingRebuildRequiredTx(ctx, tx, canonical, versionNumber, record.CreatedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("commit manual create transaction: %w", err)
	}

	return canonical, nil
}

func (r *Repository) UpdateMemory(ctx context.Context, record memory.ManualUpdateMemoryRecord) (memory.CanonicalMemory, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("begin manual update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	currentVersion, err := currentMemoryVersion(ctx, tx, record.MemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if currentVersion == 0 {
		return memory.CanonicalMemory{}, pgx.ErrNoRows
	}
	if currentVersion != record.ExpectedVersion {
		return memory.CanonicalMemory{}, memory.ErrManualMutationVersionConflict
	}

	const updateQuery = `
UPDATE canonical_memories
SET content = $5,
	search_text = to_tsvector('simple', $5),
	embedding = NULL,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND state <> 'deleted'
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		updateQuery,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Content,
		record.UpdatedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("update manual canonical memory: %w", err)
	}

	if _, err := writeManualMemoryVersion(ctx, tx, record.VersionID, record.MemoryID, currentVersion+1, canonical.State, record.Content, record.UpdatedAt, record.Actor); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := upsertRelationProjection(ctx, tx, canonical, record.UpdatedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := writeManualProvenance(ctx, tx, record.Scope, record.MemoryID, "manual_update_memory", record.RequestID, record.Actor, record.UpdatedAt, map[string]any{
		"reason": record.Reason,
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := r.recordEmbeddingRebuildRequiredTx(ctx, tx, canonical, currentVersion+1, record.UpdatedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("commit manual update transaction: %w", err)
	}

	return canonical, nil
}

func (r *Repository) MergeMemory(ctx context.Context, record memory.ManualMergeMemoryRecord) (memory.CanonicalMemory, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("begin manual merge transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	target, err := readScopedCanonicalMemory(ctx, tx, record.Scope, record.TargetMemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	source, err := readScopedCanonicalMemory(ctx, tx, record.Scope, record.SourceMemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if target.Class != source.Class {
		return memory.CanonicalMemory{}, fmt.Errorf("%w: merge memories must share the same class", memory.ErrManualMutationRejected)
	}
	if target.State != memory.MemoryStateActive || source.State != memory.MemoryStateActive {
		return memory.CanonicalMemory{}, fmt.Errorf("%w: merge memories must both be active", memory.ErrManualMutationRejected)
	}

	currentVersion, err := currentMemoryVersion(ctx, tx, record.TargetMemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if currentVersion == 0 {
		return memory.CanonicalMemory{}, pgx.ErrNoRows
	}
	if currentVersion != record.ExpectedVersion {
		return memory.CanonicalMemory{}, memory.ErrManualMutationVersionConflict
	}

	const updateTargetQuery = `
UPDATE canonical_memories
SET content = $5,
	search_text = to_tsvector('simple', $5),
	embedding = NULL,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		updateTargetQuery,
		record.TargetMemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Content,
		record.AppliedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("update merge target memory: %w", err)
	}

	if _, err := writeManualMemoryVersion(ctx, tx, record.VersionID, record.TargetMemoryID, currentVersion+1, canonical.State, record.Content, record.AppliedAt, record.Actor); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := upsertRelationProjection(ctx, tx, canonical, record.AppliedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if _, err := suppressScopedCanonicalMemory(ctx, tx, record.Scope, record.SourceMemoryID, record.AppliedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := writeManualProvenance(ctx, tx, record.Scope, record.TargetMemoryID, "manual_merge_memory", record.RequestID, record.Actor, record.AppliedAt, map[string]any{
		"reason":           record.Reason,
		"source_memory_id": record.SourceMemoryID,
		"role":             "target",
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := writeManualProvenance(ctx, tx, record.Scope, record.SourceMemoryID, "manual_merge_memory", record.RequestID, record.Actor, record.AppliedAt, map[string]any{
		"reason":           record.Reason,
		"target_memory_id": record.TargetMemoryID,
		"role":             "source",
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := r.recordEmbeddingRebuildRequiredTx(ctx, tx, canonical, currentVersion+1, record.AppliedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("commit manual merge transaction: %w", err)
	}

	return canonical, nil
}

func (r *Repository) ReclassifyMemory(ctx context.Context, record memory.ManualReclassifyMemoryRecord) (memory.CanonicalMemory, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("begin manual reclassify transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := readScopedCanonicalMemory(ctx, tx, record.Scope, record.MemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if current.State != memory.MemoryStateActive {
		return memory.CanonicalMemory{}, fmt.Errorf("%w: reclassify memory must be active", memory.ErrManualMutationRejected)
	}
	if current.Class == memory.MemoryClassSummary || current.Class == memory.MemoryClassRelation {
		return memory.CanonicalMemory{}, fmt.Errorf("%w: reclassify source class %q is not allowed", memory.ErrManualMutationRejected, current.Class)
	}

	currentVersion, err := currentMemoryVersion(ctx, tx, record.MemoryID)
	if err != nil {
		return memory.CanonicalMemory{}, err
	}
	if currentVersion == 0 {
		return memory.CanonicalMemory{}, pgx.ErrNoRows
	}
	if currentVersion != record.ExpectedVersion {
		return memory.CanonicalMemory{}, memory.ErrManualMutationVersionConflict
	}

	const updateQuery = `
UPDATE canonical_memories
SET class = $5,
	embedding = NULL,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND state <> 'deleted'
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		updateQuery,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.TargetClass,
		record.AppliedAt,
	).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("reclassify canonical memory: %w", err)
	}

	if _, err := writeManualMemoryVersion(ctx, tx, record.VersionID, record.MemoryID, currentVersion+1, canonical.State, canonical.Content, record.AppliedAt, record.Actor); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := writeManualProvenance(ctx, tx, record.Scope, record.MemoryID, "manual_reclassify_memory", record.RequestID, record.Actor, record.AppliedAt, map[string]any{
		"reason":     record.Reason,
		"from_class": current.Class,
		"to_class":   canonical.Class,
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := r.recordEmbeddingRebuildRequiredTx(ctx, tx, canonical, currentVersion+1, record.AppliedAt); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("commit manual reclassify transaction: %w", err)
	}

	return canonical, nil
}

func (r *Repository) RecordEmbeddingRebuildRequired(ctx context.Context, record memory.EmbeddingRebuildRecord) error {
	return recordEmbeddingRebuildRequired(ctx, r.db, record)
}

func (r *Repository) DispatchEmbeddingCutoverWave(ctx context.Context, scope memory.Scope, requestedAt time.Time, limit int) (int, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if requestedAt.IsZero() {
		return 0, fmt.Errorf("requested at is required")
	}
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin embedding cutover dispatch transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	plans, err := listActiveEmbeddingCutoverPlansForDispatch(ctx, tx, scope)
	if err != nil {
		return 0, err
	}

	remaining := limit
	dispatched := 0
	for _, plan := range plans {
		if remaining == 0 {
			break
		}

		waveLimit := plan.WaveSize
		if waveLimit > remaining {
			waveLimit = remaining
		}
		if waveLimit <= 0 {
			continue
		}

		candidates, err := listEmbeddingCutoverWaveCandidates(ctx, tx, plan, waveLimit)
		if err != nil {
			return 0, err
		}
		for _, candidate := range candidates {
			record := memory.EmbeddingRebuildRecord{
				MemoryID:            candidate.MemoryID,
				Scope:               candidate.Scope,
				Class:               candidate.Class,
				SourceVersion:       candidate.CurrentSourceVersion,
				ContentHash:         contentHash(candidate.Content),
				RequestedProvider:   plan.Target.Provider,
				RequestedModel:      plan.Target.Model,
				RequestedDimensions: plan.Target.Dimensions,
				Status:              memory.EmbeddingRebuildStatusPending,
				RequestedAt:         requestedAt.UTC(),
			}
			if candidate.ActiveVectorRevision != "" {
				record.ActiveVectorRevision = candidate.ActiveVectorRevision
			}

			if err := recordEmbeddingRebuildRequired(ctx, tx, record); err != nil {
				return 0, err
			}
			if err := markEmbeddingCutoverItemDispatched(ctx, tx, candidate, requestedAt.UTC()); err != nil {
				return 0, err
			}

			dispatched++
			remaining--
			if remaining == 0 {
				break
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit embedding cutover dispatch transaction: %w", err)
	}

	return dispatched, nil
}

func recordEmbeddingRebuildRequired(ctx context.Context, db queryRower, record memory.EmbeddingRebuildRecord) error {
	const query = `
INSERT INTO embedding_rebuilds (
	memory_id,
	tenant,
	project,
	namespace,
	source_version,
	content_hash,
	requested_provider,
	requested_model,
	requested_dimensions,
	status,
	failure_reason,
	requested_at,
	last_attempted_at,
	active_vector_revision_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (memory_id) DO UPDATE SET
	tenant = EXCLUDED.tenant,
	project = EXCLUDED.project,
	namespace = EXCLUDED.namespace,
	source_version = EXCLUDED.source_version,
	content_hash = EXCLUDED.content_hash,
	requested_provider = EXCLUDED.requested_provider,
	requested_model = EXCLUDED.requested_model,
	requested_dimensions = EXCLUDED.requested_dimensions,
	status = EXCLUDED.status,
	failure_reason = EXCLUDED.failure_reason,
	requested_at = EXCLUDED.requested_at,
	last_attempted_at = EXCLUDED.last_attempted_at,
	active_vector_revision_id = COALESCE(EXCLUDED.active_vector_revision_id, embedding_rebuilds.active_vector_revision_id)
`

	if _, err := db.Exec(
		ctx,
		query,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.SourceVersion,
		record.ContentHash,
		record.RequestedProvider,
		record.RequestedModel,
		record.RequestedDimensions,
		record.Status,
		nullableString(record.FailureReason),
		record.RequestedAt,
		nullableTime(record.LastAttemptedAt),
		nullableString(record.ActiveVectorRevision),
	); err != nil {
		return fmt.Errorf("record embedding rebuild required: %w", err)
	}

	return nil
}

func (r *Repository) AppendVectorRevision(ctx context.Context, revision memory.VectorRevision) error {
	const query = `
INSERT INTO vector_revisions (
	id,
	memory_id,
	tenant,
	project,
	namespace,
	source_version,
	content_hash,
	provider,
	model,
	dimensions,
	embedding,
	status,
	failure_reason,
	superseded_by,
	generated_at,
	activated_at,
	last_rebuild_request_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::vector, $12, $13, $14, $15, $16, $17)
`

	if _, err := r.db.Exec(
		ctx,
		query,
		revision.ID,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		revision.SourceVersion,
		revision.ContentHash,
		revision.Provider,
		revision.Model,
		revision.Dimensions,
		nullableVectorLiteral(revision.Embedding),
		revision.Status,
		nullableString(revision.FailureReason),
		nullableString(revision.SupersededBy),
		revision.GeneratedAt,
		nullableTime(revision.ActivatedAt),
		nullableTime(revision.LastRebuildRequest),
	); err != nil {
		return fmt.Errorf("append vector revision: %w", err)
	}

	return nil
}

func (r *Repository) PromoteVectorRevision(ctx context.Context, revision memory.VectorRevision) error {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin vector promotion transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	currentCanonical, err := readScopedCanonicalMemory(ctx, tx, revision.Scope, revision.MemoryID)
	if err != nil {
		return err
	}
	currentVersion, err := currentMemoryVersion(ctx, tx, revision.MemoryID)
	if err != nil {
		return err
	}
	if currentVersion != revision.SourceVersion || contentHash(currentCanonical.Content) != revision.ContentHash {
		return pgx.ErrNoRows
	}

	const supersedeQuery = `
UPDATE vector_revisions
SET status = $5,
	superseded_by = $6,
	activated_at = $7
WHERE memory_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND status = 'active'
`

	if _, err := tx.Exec(
		ctx,
		supersedeQuery,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		memory.VectorRevisionStatusSuperseded,
		revision.ID,
		revision.ActivatedAt,
	); err != nil {
		return fmt.Errorf("supersede active vector revision: %w", err)
	}

	const activateQuery = `
UPDATE vector_revisions
SET status = $7,
	activated_at = $8
WHERE id = $1
	AND memory_id = $2
	AND tenant = $3
	AND project = $4
	AND namespace = $5
	AND source_version = $6
	AND content_hash = $9
`

	tag, err := tx.Exec(
		ctx,
		activateQuery,
		revision.ID,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		revision.SourceVersion,
		memory.VectorRevisionStatusActive,
		revision.ActivatedAt,
		revision.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("activate vector revision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	const mirrorQuery = `
UPDATE canonical_memories
SET embedding = $5::vector,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	if _, err := tx.Exec(
		ctx,
		mirrorQuery,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		vectorLiteral(revision.Embedding),
		revision.ActivatedAt,
	); err != nil {
		return fmt.Errorf("mirror active vector onto canonical memory: %w", err)
	}

	const rebuildQuery = `
UPDATE embedding_rebuilds
SET
	active_vector_revision_id = $5,
	status = $6,
	failure_reason = NULL
WHERE memory_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	if _, err := tx.Exec(
		ctx,
		rebuildQuery,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		revision.ID,
		memory.EmbeddingRebuildStatusCurrent,
	); err != nil {
		return fmt.Errorf("update embedding rebuild active revision: %w", err)
	}

	if err := syncEmbeddingCutoverPromotion(ctx, tx, revision); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit vector promotion transaction: %w", err)
	}

	return nil
}

func (r *Repository) ListEligibleEmbeddingRebuilds(ctx context.Context, limit int) ([]memory.EmbeddingRebuildRecord, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	const query = `
SELECT
	er.memory_id,
	er.tenant,
	er.project,
	er.namespace,
	er.source_version,
	er.content_hash,
	er.requested_provider,
	er.requested_model,
	er.requested_dimensions,
	er.status,
	er.failure_reason,
	er.requested_at,
	er.last_attempted_at,
	er.active_vector_revision_id
FROM embedding_rebuilds er
JOIN canonical_memories cm
	ON cm.id = er.memory_id
	AND cm.tenant = er.tenant
	AND cm.project = er.project
	AND cm.namespace = er.namespace
WHERE er.status = $1
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
ORDER BY requested_at ASC
LIMIT $2
`

	rows, err := r.db.Query(ctx, query, memory.EmbeddingRebuildStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("list eligible embedding rebuilds: %w", err)
	}
	defer rows.Close()

	records := make([]memory.EmbeddingRebuildRecord, 0)
	for rows.Next() {
		var record memory.EmbeddingRebuildRecord
		var failureReason sql.NullString
		var lastAttemptedAt sql.NullTime
		var activeVectorRevision sql.NullString
		if err := rows.Scan(
			&record.MemoryID,
			&record.Scope.Tenant,
			&record.Scope.Project,
			&record.Scope.Namespace,
			&record.SourceVersion,
			&record.ContentHash,
			&record.RequestedProvider,
			&record.RequestedModel,
			&record.RequestedDimensions,
			&record.Status,
			&failureReason,
			&record.RequestedAt,
			&lastAttemptedAt,
			&activeVectorRevision,
		); err != nil {
			return nil, fmt.Errorf("scan eligible embedding rebuild: %w", err)
		}
		if failureReason.Valid {
			record.FailureReason = failureReason.String
		}
		if lastAttemptedAt.Valid {
			record.LastAttemptedAt = lastAttemptedAt.Time
		}
		if activeVectorRevision.Valid {
			record.ActiveVectorRevision = activeVectorRevision.String
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible embedding rebuilds: %w", err)
	}

	return records, nil
}

func (r *Repository) ListEmbeddingLifecycleCandidates(ctx context.Context, scope memory.Scope, limit int) ([]memory.EmbeddingLifecycleCandidate, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	const query = `
SELECT
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	cm.content,
	COALESCE(mv.version, 0) AS current_version,
	er.status,
	er.source_version,
	er.content_hash,
	er.requested_provider,
	er.requested_model,
	er.requested_dimensions,
	er.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions
FROM canonical_memories cm
LEFT JOIN (
	SELECT memory_id, MAX(version) AS version
	FROM memory_versions
	GROUP BY memory_id
) mv ON mv.memory_id = cm.id
LEFT JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.tenant = $1
	AND cm.project = $2
	AND cm.namespace = $3
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND cm.class IN ('profile', 'episodic', 'procedural', 'summary')
ORDER BY cm.updated_at ASC, cm.id ASC
LIMIT $4
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list embedding lifecycle candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]memory.EmbeddingLifecycleCandidate, 0)
	for rows.Next() {
		var candidate memory.EmbeddingLifecycleCandidate
		var content string
		var rebuildStatus sql.NullString
		var rebuildSourceVersion sql.NullInt64
		var rebuildContentHash sql.NullString
		var requestedProvider sql.NullString
		var requestedModel sql.NullString
		var requestedDimensions sql.NullInt32
		var activeVectorRevision sql.NullString
		var activeProvider sql.NullString
		var activeModel sql.NullString
		var activeDimensions sql.NullInt32
		if err := rows.Scan(
			&candidate.MemoryID,
			&candidate.Scope.Tenant,
			&candidate.Scope.Project,
			&candidate.Scope.Namespace,
			&candidate.Class,
			&content,
			&candidate.CurrentSourceVersion,
			&rebuildStatus,
			&rebuildSourceVersion,
			&rebuildContentHash,
			&requestedProvider,
			&requestedModel,
			&requestedDimensions,
			&activeVectorRevision,
			&activeProvider,
			&activeModel,
			&activeDimensions,
		); err != nil {
			return nil, fmt.Errorf("scan embedding lifecycle candidate: %w", err)
		}

		candidate.CurrentContentHash = contentHash(content)
		if rebuildStatus.Valid {
			candidate.RebuildStatus = memory.EmbeddingRebuildStatus(rebuildStatus.String)
		}
		if rebuildSourceVersion.Valid && rebuildContentHash.Valid &&
			rebuildSourceVersion.Int64 == candidate.CurrentSourceVersion &&
			rebuildContentHash.String == candidate.CurrentContentHash {
			if requestedProvider.Valid {
				candidate.RequestedProvider = requestedProvider.String
			}
			if requestedModel.Valid {
				candidate.RequestedModel = requestedModel.String
			}
			if requestedDimensions.Valid {
				candidate.RequestedDimensions = int(requestedDimensions.Int32)
			}
			if activeVectorRevision.Valid {
				candidate.ActiveVectorRevision = activeVectorRevision.String
			}
			if activeProvider.Valid {
				candidate.ActiveProvider = activeProvider.String
			}
			if activeModel.Valid {
				candidate.ActiveModel = activeModel.String
			}
			if activeDimensions.Valid {
				candidate.ActiveDimensions = int(activeDimensions.Int32)
			}
		} else {
			candidate.RebuildStatus = ""
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding lifecycle candidates: %w", err)
	}

	return candidates, nil
}

func (r *Repository) ListEmbeddingRebuilds(ctx context.Context, input memory.ListEmbeddingRebuildsInput) ([]memory.EmbeddingRebuildView, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const query = `
SELECT
	er.memory_id,
	er.tenant,
	er.project,
	er.namespace,
	cm.class,
	cm.state,
	er.source_version,
	er.content_hash,
	er.requested_provider,
	er.requested_model,
	er.requested_dimensions,
	er.status,
	er.failure_reason,
	er.requested_at,
	er.last_attempted_at,
	er.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions
FROM canonical_memories cm
LEFT JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.tenant = $1
	AND cm.project = $2
	AND cm.namespace = $3
	AND er.memory_id IS NOT NULL
	AND ($4 = '' OR er.status = $4)
	AND ($5 = '' OR er.requested_provider = $5)
	AND ($6 = '' OR er.requested_model = $6)
ORDER BY er.requested_at DESC, er.memory_id ASC
LIMIT $7
`

	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.Status,
		input.RequestedProvider,
		input.RequestedModel,
		input.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list embedding rebuilds: %w", err)
	}
	defer rows.Close()

	items := make([]memory.EmbeddingRebuildView, 0)
	for rows.Next() {
		var item memory.EmbeddingRebuildView
		var failureReason sql.NullString
		var lastAttemptedAt sql.NullTime
		var activeVectorRevision sql.NullString
		var activeProvider sql.NullString
		var activeModel sql.NullString
		var activeDimensions sql.NullInt32
		if err := rows.Scan(
			&item.MemoryID,
			&item.Scope.Tenant,
			&item.Scope.Project,
			&item.Scope.Namespace,
			&item.Class,
			&item.State,
			new(int64),
			new(string),
			&item.RequestedProvider,
			&item.RequestedModel,
			&item.RequestedDimensions,
			&item.Status,
			&failureReason,
			&item.RequestedAt,
			&lastAttemptedAt,
			&activeVectorRevision,
			&activeProvider,
			&activeModel,
			&activeDimensions,
		); err != nil {
			return nil, fmt.Errorf("scan embedding rebuild: %w", err)
		}
		if failureReason.Valid {
			item.FailureReason = failureReason.String
		}
		if lastAttemptedAt.Valid {
			item.LastAttemptedAt = lastAttemptedAt.Time
		}
		if activeVectorRevision.Valid {
			item.ActiveVectorRevision = activeVectorRevision.String
		}
		if activeProvider.Valid {
			item.ActiveProvider = activeProvider.String
		}
		if activeModel.Valid {
			item.ActiveModel = activeModel.String
		}
		if activeDimensions.Valid {
			item.ActiveDimensions = int(activeDimensions.Int32)
		}
		item.Drifted = embedding.DetermineDrift(
			embedding.Target{
				Provider:   item.RequestedProvider,
				Model:      item.RequestedModel,
				Dimensions: item.RequestedDimensions,
			},
			item.ActiveProvider,
			item.ActiveModel,
			item.ActiveDimensions,
		)
		if input.Drifted != nil && item.Drifted != *input.Drifted {
			continue
		}

		items = append(items, item)
		if len(items) >= input.Limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding rebuilds: %w", err)
	}

	return items, nil
}

func (r *Repository) ReadMemoryEmbedding(ctx context.Context, scope memory.Scope, memoryID string) (memory.EmbeddingMemoryInspection, error) {
	if err := scope.Validate(); err != nil {
		return memory.EmbeddingMemoryInspection{}, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return memory.EmbeddingMemoryInspection{}, fmt.Errorf("memory id is required")
	}

	const summaryQuery = `
SELECT
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	cm.state,
	COALESCE(mv.version, 0) AS current_source_version,
	COALESCE(er.content_hash, '') AS current_content_hash,
	er.requested_provider,
	er.requested_model,
	er.requested_dimensions,
	er.status,
	er.failure_reason,
	er.requested_at,
	er.last_attempted_at,
	er.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions
FROM canonical_memories cm
LEFT JOIN (
	SELECT memory_id, MAX(version) AS version
	FROM memory_versions
	GROUP BY memory_id
) mv ON mv.memory_id = cm.id
LEFT JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.tenant = $1
	AND cm.project = $2
	AND cm.namespace = $3
	AND cm.id = $4
`

	var inspection memory.EmbeddingMemoryInspection
	var failureReason sql.NullString
	var requestedProvider sql.NullString
	var requestedModel sql.NullString
	var requestedDimensions sql.NullInt32
	var rebuildStatus sql.NullString
	var requestedAt sql.NullTime
	var lastAttemptedAt sql.NullTime
	var activeVectorRevision sql.NullString
	var activeProvider sql.NullString
	var activeModel sql.NullString
	var activeDimensions sql.NullInt32
	if err := r.db.QueryRow(ctx, summaryQuery, scope.Tenant, scope.Project, scope.Namespace, memoryID).Scan(
		&inspection.Memory.ID,
		&inspection.Memory.Scope.Tenant,
		&inspection.Memory.Scope.Project,
		&inspection.Memory.Scope.Namespace,
		&inspection.Memory.Class,
		&inspection.Memory.State,
		&inspection.Memory.CurrentSourceVersion,
		&inspection.Memory.CurrentContentHash,
		&requestedProvider,
		&requestedModel,
		&requestedDimensions,
		&rebuildStatus,
		&failureReason,
		&requestedAt,
		&lastAttemptedAt,
		&activeVectorRevision,
		&activeProvider,
		&activeModel,
		&activeDimensions,
	); err != nil {
		return memory.EmbeddingMemoryInspection{}, err
	}

	inspection.Rebuild.MemoryID = inspection.Memory.ID
	inspection.Rebuild.Scope = inspection.Memory.Scope
	inspection.Rebuild.Class = inspection.Memory.Class
	inspection.Rebuild.State = inspection.Memory.State
	if requestedProvider.Valid {
		inspection.Rebuild.RequestedProvider = requestedProvider.String
	}
	if requestedModel.Valid {
		inspection.Rebuild.RequestedModel = requestedModel.String
	}
	if requestedDimensions.Valid {
		inspection.Rebuild.RequestedDimensions = int(requestedDimensions.Int32)
	}
	if rebuildStatus.Valid {
		inspection.Rebuild.Status = memory.EmbeddingRebuildStatus(rebuildStatus.String)
	}
	if failureReason.Valid {
		inspection.Rebuild.FailureReason = failureReason.String
	}
	if requestedAt.Valid {
		inspection.Rebuild.RequestedAt = requestedAt.Time
	}
	if lastAttemptedAt.Valid {
		inspection.Rebuild.LastAttemptedAt = lastAttemptedAt.Time
	}
	if activeVectorRevision.Valid {
		inspection.Rebuild.ActiveVectorRevision = activeVectorRevision.String
	}
	if activeProvider.Valid {
		inspection.Rebuild.ActiveProvider = activeProvider.String
	}
	if activeModel.Valid {
		inspection.Rebuild.ActiveModel = activeModel.String
	}
	if activeDimensions.Valid {
		inspection.Rebuild.ActiveDimensions = int(activeDimensions.Int32)
	}
	if inspection.Rebuild.RequestedProvider != "" {
		inspection.Rebuild.Drifted = embedding.DetermineDrift(
			embedding.Target{
				Provider:   inspection.Rebuild.RequestedProvider,
				Model:      inspection.Rebuild.RequestedModel,
				Dimensions: inspection.Rebuild.RequestedDimensions,
			},
			inspection.Rebuild.ActiveProvider,
			inspection.Rebuild.ActiveModel,
			inspection.Rebuild.ActiveDimensions,
		)
	}

	const revisionsQuery = `
SELECT
	id,
	memory_id,
	source_version,
	content_hash,
	provider,
	model,
	dimensions,
	status,
	failure_reason,
	superseded_by,
	generated_at,
	activated_at,
	last_rebuild_request_at
FROM vector_revisions
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = $4
ORDER BY generated_at DESC, id DESC
`

	rows, err := r.db.Query(ctx, revisionsQuery, scope.Tenant, scope.Project, scope.Namespace, memoryID)
	if err != nil {
		return memory.EmbeddingMemoryInspection{}, fmt.Errorf("list memory embedding revisions: %w", err)
	}
	defer rows.Close()

	revisions := make([]memory.EmbeddingVectorRevisionView, 0)
	for rows.Next() {
		var revision memory.EmbeddingVectorRevisionView
		var failureReason sql.NullString
		var supersededBy sql.NullString
		var activatedAt sql.NullTime
		var lastRebuildRequest sql.NullTime
		var ignoredMemoryID string
		if err := rows.Scan(
			&revision.ID,
			&ignoredMemoryID,
			&revision.SourceVersion,
			&revision.ContentHash,
			&revision.Provider,
			&revision.Model,
			&revision.Dimensions,
			&revision.Status,
			&failureReason,
			&supersededBy,
			&revision.GeneratedAt,
			&activatedAt,
			&lastRebuildRequest,
		); err != nil {
			return memory.EmbeddingMemoryInspection{}, fmt.Errorf("scan memory embedding revision: %w", err)
		}
		if failureReason.Valid {
			revision.FailureReason = failureReason.String
		}
		if supersededBy.Valid {
			revision.SupersededBy = supersededBy.String
		}
		if activatedAt.Valid {
			revision.ActivatedAt = activatedAt.Time
		}
		if lastRebuildRequest.Valid {
			revision.LastRebuildRequest = lastRebuildRequest.Time
		}
		revisions = append(revisions, revision)
	}

	if err := rows.Err(); err != nil {
		return memory.EmbeddingMemoryInspection{}, fmt.Errorf("iterate memory embedding revisions: %w", err)
	}

	inspection.Revisions = revisions
	return inspection, nil
}

func (r *Repository) CreateEmbeddingCutoverPlan(ctx context.Context, input memory.CreateEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error) {
	if err := input.Validate(); err != nil {
		return memory.EmbeddingCutoverPlan{}, err
	}

	const query = `
INSERT INTO embedding_cutover_plans (
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING
	id,
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at,
	last_action_by,
	last_action_reason,
	last_action_at,
	activated_at,
	paused_at,
	cancelled_at,
	completed_at
`

	plan, err := scanEmbeddingCutoverPlan(
		r.db.QueryRow(
			ctx,
			query,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			string(memory.EmbeddingCutoverPlanStatusDraft),
			input.Target.Provider,
			input.Target.Model,
			input.Target.Dimensions,
			embeddingCutoverPlanClassStrings(input.Classes),
			input.WaveSize,
			input.Reason,
			input.Actor,
			input.CreatedAt.UTC(),
		),
	)
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("create embedding cutover plan: %w", err)
	}

	return plan, nil
}

func (r *Repository) ListEmbeddingCutoverPlans(ctx context.Context, input memory.ListEmbeddingCutoverPlansInput) ([]memory.EmbeddingCutoverPlan, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const query = `
SELECT
	p.id,
	p.tenant,
	p.project,
	p.namespace,
	p.status,
	p.target_provider,
	p.target_model,
	p.target_dimensions,
	p.class_filters,
	p.wave_size,
	p.reason,
	p.created_by,
	p.created_at,
	p.last_action_by,
	p.last_action_reason,
	p.last_action_at,
	p.activated_at,
	p.paused_at,
	p.cancelled_at,
	p.completed_at,
	COALESCE(stats.total, 0) AS total,
	COALESCE(stats.queued, 0) AS queued,
	COALESCE(stats.rebuilding, 0) AS rebuilding,
	COALESCE(stats.current, 0) AS current,
	COALESCE(stats.failed, 0) AS failed,
	COALESCE(stats.skipped, 0) AS skipped,
	COALESCE(stats.paused, 0) AS paused,
	COALESCE(stats.cancelled, 0) AS cancelled
FROM embedding_cutover_plans p
LEFT JOIN LATERAL (
	SELECT
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE status = 'queued') AS queued,
		COUNT(*) FILTER (WHERE status = 'rebuilding') AS rebuilding,
		COUNT(*) FILTER (WHERE status = 'current') AS current,
		COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		COUNT(*) FILTER (WHERE status = 'skipped') AS skipped,
		COUNT(*) FILTER (WHERE status = 'paused') AS paused,
		COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
	FROM embedding_cutover_items i
	WHERE i.plan_id = p.id
) stats ON true
WHERE p.tenant = $1
	AND p.project = $2
	AND p.namespace = $3
	AND ($4 = '' OR p.status = $4)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $5
`

	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		string(input.Status),
		input.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list embedding cutover plans: %w", err)
	}
	defer rows.Close()

	plans := make([]memory.EmbeddingCutoverPlan, 0)
	for rows.Next() {
		plan, err := scanEmbeddingCutoverPlanWithProgress(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding cutover plans: %w", err)
	}

	return plans, nil
}

func (r *Repository) ReadEmbeddingCutoverPlan(ctx context.Context, input memory.ReadEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error) {
	if err := input.Validate(); err != nil {
		return memory.EmbeddingCutoverPlan{}, err
	}

	plan, err := readEmbeddingCutoverPlan(ctx, r.db, input)
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("read embedding cutover plan: %w", err)
	}

	return plan, nil
}

func (r *Repository) ReadEmbeddingCutoverAdmission(ctx context.Context, input memory.EmbeddingCutoverPreflightInput) (memory.EmbeddingCutoverAdmissionSnapshot, error) {
	if err := input.Validate(); err != nil {
		return memory.EmbeddingCutoverAdmissionSnapshot{}, err
	}

	plan, err := readEmbeddingCutoverPlan(ctx, r.db, memory.ReadEmbeddingCutoverPlanInput{
		Scope:  input.Scope,
		PlanID: input.PlanID,
	})
	if err != nil {
		return memory.EmbeddingCutoverAdmissionSnapshot{}, fmt.Errorf("read embedding cutover admission plan: %w", err)
	}

	conflictingPlan, err := readEmbeddingCutoverAdmissionConflict(ctx, r.db, plan)
	if err != nil {
		return memory.EmbeddingCutoverAdmissionSnapshot{}, err
	}
	breakdown, err := readEmbeddingCutoverAdmissionBreakdown(ctx, r.db, plan)
	if err != nil {
		return memory.EmbeddingCutoverAdmissionSnapshot{}, err
	}

	total := 0
	for _, item := range breakdown {
		total += item.Eligible
	}

	return memory.EmbeddingCutoverAdmissionSnapshot{
		Plan:            plan,
		EligibleTotal:   total,
		ClassBreakdown:  breakdown,
		ConflictingPlan: conflictingPlan,
	}, nil
}

func (r *Repository) ApplyEmbeddingCutoverPlanAction(ctx context.Context, input memory.ApplyEmbeddingCutoverPlanActionInput) (memory.EmbeddingCutoverPlan, error) {
	if err := input.Validate(); err != nil {
		return memory.EmbeddingCutoverPlan{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("begin embedding cutover transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
SELECT
	id,
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at,
	last_action_by,
	last_action_reason,
	last_action_at,
	activated_at,
	paused_at,
	cancelled_at,
	completed_at
FROM embedding_cutover_plans
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
FOR UPDATE
`

	current, err := scanEmbeddingCutoverPlan(tx.QueryRow(ctx, selectQuery, input.PlanID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace))
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("read embedding cutover target: %w", err)
	}

	next, err := memory.ApplyEmbeddingCutoverPlanAction(current, input)
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, err
	}

	const updateQuery = `
UPDATE embedding_cutover_plans
SET
	status = $5,
	last_action_by = $6,
	last_action_reason = $7,
	last_action_at = $8,
	activated_at = $9,
	paused_at = $10,
	cancelled_at = $11,
	completed_at = $12
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
RETURNING
	id,
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at,
	last_action_by,
	last_action_reason,
	last_action_at,
	activated_at,
	paused_at,
	cancelled_at,
	completed_at
`

	updated, err := scanEmbeddingCutoverPlan(
		tx.QueryRow(
			ctx,
			updateQuery,
			input.PlanID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			string(next.Status),
			next.LastActionBy,
			next.LastActionReason,
			nullableTime(next.LastActionAt),
			nullableTime(next.ActivatedAt),
			nullableTime(next.PausedAt),
			nullableTime(next.CancelledAt),
			nullableTime(next.CompletedAt),
		),
	)
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("update embedding cutover target: %w", err)
	}

	if current.Status == memory.EmbeddingCutoverPlanStatusDraft && input.Action == memory.EmbeddingCutoverPlanActionActivate {
		if err := registerEmbeddingCutoverItems(ctx, tx, updated, input.AppliedAt.UTC()); err != nil {
			return memory.EmbeddingCutoverPlan{}, err
		}
	}
	if current.Status == memory.EmbeddingCutoverPlanStatusPaused && input.Action == memory.EmbeddingCutoverPlanActionActivate {
		if err := transitionEmbeddingCutoverItems(ctx, tx, updated.ID, updated.Scope, memory.EmbeddingCutoverItemStatusQueued, input.AppliedAt.UTC(), []memory.EmbeddingCutoverItemStatus{
			memory.EmbeddingCutoverItemStatusPaused,
		}); err != nil {
			return memory.EmbeddingCutoverPlan{}, err
		}
	}
	if input.Action == memory.EmbeddingCutoverPlanActionPause {
		if err := transitionEmbeddingCutoverItems(ctx, tx, updated.ID, updated.Scope, memory.EmbeddingCutoverItemStatusPaused, input.AppliedAt.UTC(), []memory.EmbeddingCutoverItemStatus{
			memory.EmbeddingCutoverItemStatusQueued,
		}); err != nil {
			return memory.EmbeddingCutoverPlan{}, err
		}
	}
	if input.Action == memory.EmbeddingCutoverPlanActionCancel {
		if err := transitionEmbeddingCutoverItems(ctx, tx, updated.ID, updated.Scope, memory.EmbeddingCutoverItemStatusCancelled, input.AppliedAt.UTC(), []memory.EmbeddingCutoverItemStatus{
			memory.EmbeddingCutoverItemStatusQueued,
			memory.EmbeddingCutoverItemStatusPaused,
		}); err != nil {
			return memory.EmbeddingCutoverPlan{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("commit embedding cutover transaction: %w", err)
	}

	return updated, nil
}

func (r *Repository) ListEmbeddingRecoveryHistory(ctx context.Context, input memory.ListEmbeddingRecoveryHistoryInput) ([]memory.EmbeddingRecoveryRecord, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	query := strings.Builder{}
	query.WriteString(`
SELECT
	id,
	memory_id,
	tenant,
	project,
	namespace,
	cutover_plan_id,
	action,
	actor,
	reason,
	before_snapshot,
	after_snapshot,
	created_at
FROM embedding_recovery_ledger
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
`)

	args := []any{input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace}
	position := 4

	if strings.TrimSpace(input.MemoryID) != "" {
		query.WriteString(fmt.Sprintf(" AND memory_id = $%d\n", position))
		args = append(args, strings.TrimSpace(input.MemoryID))
		position++
	}
	if input.Action != "" {
		query.WriteString(fmt.Sprintf(" AND action = $%d\n", position))
		args = append(args, string(input.Action))
		position++
	}
	if strings.TrimSpace(input.Actor) != "" {
		query.WriteString(fmt.Sprintf(" AND actor = $%d\n", position))
		args = append(args, strings.TrimSpace(input.Actor))
		position++
	}
	if strings.TrimSpace(input.CutoverPlanID) != "" {
		query.WriteString(fmt.Sprintf(" AND cutover_plan_id = $%d\n", position))
		args = append(args, strings.TrimSpace(input.CutoverPlanID))
		position++
	}
	if !input.OccurredFrom.IsZero() {
		query.WriteString(fmt.Sprintf(" AND created_at >= $%d\n", position))
		args = append(args, input.OccurredFrom.UTC())
		position++
	}
	if !input.OccurredTo.IsZero() {
		query.WriteString(fmt.Sprintf(" AND created_at <= $%d\n", position))
		args = append(args, input.OccurredTo.UTC())
		position++
	}

	query.WriteString(fmt.Sprintf("ORDER BY created_at DESC, id DESC\nLIMIT $%d", position))
	args = append(args, input.Limit)

	rows, err := r.db.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list embedding recovery history: %w", err)
	}
	defer rows.Close()

	records := make([]memory.EmbeddingRecoveryRecord, 0)
	for rows.Next() {
		record, err := scanEmbeddingRecoveryRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding recovery history: %w", err)
	}

	return records, nil
}

func (r *Repository) ApplyEmbeddingRecovery(ctx context.Context, input memory.ApplyEmbeddingRecoveryInput) (memory.EmbeddingRecoveryOutcome, error) {
	if err := input.Validate(); err != nil {
		return memory.EmbeddingRecoveryOutcome{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, fmt.Errorf("begin embedding recovery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
SELECT
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	cm.state,
	er.requested_provider,
	er.requested_model,
	er.requested_dimensions,
	er.status,
	er.failure_reason,
	er.requested_at,
	er.last_attempted_at,
	er.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions
FROM canonical_memories cm
JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.id = $1
	AND cm.tenant = $2
	AND cm.project = $3
	AND cm.namespace = $4
FOR UPDATE
`

	current, err := scanEmbeddingRebuildView(tx.QueryRow(ctx, selectQuery, input.MemoryID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace))
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, fmt.Errorf("read embedding recovery target: %w", err)
	}

	next, err := memory.ApplyEmbeddingRecovery(current, input)
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, err
	}

	const updateQuery = `
WITH updated AS (
	UPDATE embedding_rebuilds
	SET
		status = $5,
		failure_reason = $6,
		requested_at = $7
	WHERE memory_id = $1
		AND tenant = $2
		AND project = $3
		AND namespace = $4
	RETURNING
		memory_id,
		tenant,
		project,
		namespace,
		requested_provider,
		requested_model,
		requested_dimensions,
		status,
		failure_reason,
		requested_at,
		last_attempted_at,
		active_vector_revision_id
)
SELECT
	updated.memory_id,
	updated.tenant,
	updated.project,
	updated.namespace,
	cm.class,
	cm.state,
	updated.requested_provider,
	updated.requested_model,
	updated.requested_dimensions,
	updated.status,
	updated.failure_reason,
	updated.requested_at,
	updated.last_attempted_at,
	updated.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions
FROM updated
JOIN canonical_memories cm
	ON cm.id = updated.memory_id
	AND cm.tenant = updated.tenant
	AND cm.project = updated.project
	AND cm.namespace = updated.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = updated.active_vector_revision_id
	AND vr.memory_id = updated.memory_id
	AND vr.tenant = updated.tenant
	AND vr.project = updated.project
	AND vr.namespace = updated.namespace
`

	updated, err := scanEmbeddingRebuildView(
		tx.QueryRow(
			ctx,
			updateQuery,
			input.MemoryID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			next.Status,
			nullableString(next.FailureReason),
			next.RequestedAt,
		),
	)
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, fmt.Errorf("update embedding recovery target: %w", err)
	}

	beforeSnapshotJSON, err := marshalEmbeddingRecoverySnapshot(memory.NewEmbeddingRecoverySnapshot(current))
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, err
	}
	afterSnapshotJSON, err := marshalEmbeddingRecoverySnapshot(memory.NewEmbeddingRecoverySnapshot(updated))
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, err
	}

	cutoverPlanID, err := readEmbeddingRecoveryCutoverPlanID(ctx, tx, input.Scope, updated.MemoryID)
	if err != nil {
		return memory.EmbeddingRecoveryOutcome{}, err
	}

	recovery := memory.EmbeddingRecoveryRecord{
		ID:            uuid.NewString(),
		MemoryID:      updated.MemoryID,
		Scope:         input.Scope,
		CutoverPlanID: cutoverPlanID,
		Action:        input.Action,
		Actor:         input.Actor,
		Reason:        input.Reason,
		Before:        memory.NewEmbeddingRecoverySnapshot(current),
		After:         memory.NewEmbeddingRecoverySnapshot(updated),
		OccurredAt:    input.AppliedAt.UTC(),
	}

	const insertLedgerQuery = `
INSERT INTO embedding_recovery_ledger (
	id,
	memory_id,
	tenant,
	project,
	namespace,
	cutover_plan_id,
	action,
	actor,
	reason,
	before_snapshot,
	after_snapshot,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
`

	if _, err := tx.Exec(
		ctx,
		insertLedgerQuery,
		recovery.ID,
		recovery.MemoryID,
		recovery.Scope.Tenant,
		recovery.Scope.Project,
		recovery.Scope.Namespace,
		nullableString(recovery.CutoverPlanID),
		string(recovery.Action),
		recovery.Actor,
		recovery.Reason,
		beforeSnapshotJSON,
		afterSnapshotJSON,
		recovery.OccurredAt,
	); err != nil {
		return memory.EmbeddingRecoveryOutcome{}, fmt.Errorf("insert embedding recovery ledger: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.EmbeddingRecoveryOutcome{}, fmt.Errorf("commit embedding recovery transaction: %w", err)
	}

	return memory.EmbeddingRecoveryOutcome{
		Rebuild:  updated,
		Recovery: recovery,
	}, nil
}

func (r *Repository) ClaimEmbeddingRebuilds(ctx context.Context, scope memory.Scope, limit int, attemptedAt time.Time) ([]memory.EmbeddingRebuildRecord, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}
	if attemptedAt.IsZero() {
		return nil, fmt.Errorf("attempted at is required")
	}

	const query = `
WITH claimed AS (
	UPDATE embedding_rebuilds er
	SET
		status = $5,
		last_attempted_at = $6,
		failure_reason = NULL
	WHERE er.memory_id IN (
		SELECT er2.memory_id
		FROM embedding_rebuilds er2
		JOIN canonical_memories cm
			ON cm.id = er2.memory_id
			AND cm.tenant = er2.tenant
			AND cm.project = er2.project
			AND cm.namespace = er2.namespace
		WHERE er2.tenant = $1
			AND er2.project = $2
			AND er2.namespace = $3
			AND er2.status IN ($4, $7)
			AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
		ORDER BY er2.requested_at ASC
		LIMIT $8
		FOR UPDATE SKIP LOCKED
	)
	RETURNING
		er.memory_id,
		er.tenant,
		er.project,
		er.namespace,
		er.source_version,
		er.content_hash,
		er.requested_provider,
		er.requested_model,
		er.requested_dimensions,
		er.status,
		er.failure_reason,
		er.requested_at,
		er.last_attempted_at,
		er.active_vector_revision_id
)
SELECT
	claimed.memory_id,
	claimed.tenant,
	claimed.project,
	claimed.namespace,
	cm.class,
	cm.content,
	claimed.source_version,
	claimed.content_hash,
	claimed.requested_provider,
	claimed.requested_model,
	claimed.requested_dimensions,
	claimed.status,
	claimed.failure_reason,
	claimed.requested_at,
	claimed.last_attempted_at,
	claimed.active_vector_revision_id
FROM claimed
JOIN canonical_memories cm
	ON cm.id = claimed.memory_id
	AND cm.tenant = claimed.tenant
	AND cm.project = claimed.project
	AND cm.namespace = claimed.namespace
ORDER BY claimed.requested_at ASC
`

	rows, err := r.db.Query(
		ctx,
		query,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		memory.EmbeddingRebuildStatusPending,
		memory.EmbeddingRebuildStatusRebuilding,
		attemptedAt,
		memory.EmbeddingRebuildStatusFailed,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim embedding rebuilds: %w", err)
	}
	defer rows.Close()

	records := make([]memory.EmbeddingRebuildRecord, 0)
	for rows.Next() {
		var record memory.EmbeddingRebuildRecord
		var failureReason sql.NullString
		var lastAttemptedAt sql.NullTime
		var activeVectorRevision sql.NullString
		if err := rows.Scan(
			&record.MemoryID,
			&record.Scope.Tenant,
			&record.Scope.Project,
			&record.Scope.Namespace,
			&record.Class,
			&record.Content,
			&record.SourceVersion,
			&record.ContentHash,
			&record.RequestedProvider,
			&record.RequestedModel,
			&record.RequestedDimensions,
			&record.Status,
			&failureReason,
			&record.RequestedAt,
			&lastAttemptedAt,
			&activeVectorRevision,
		); err != nil {
			return nil, fmt.Errorf("scan claimed embedding rebuild: %w", err)
		}
		if failureReason.Valid {
			record.FailureReason = failureReason.String
		}
		if lastAttemptedAt.Valid {
			record.LastAttemptedAt = lastAttemptedAt.Time
		}
		if activeVectorRevision.Valid {
			record.ActiveVectorRevision = activeVectorRevision.String
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed embedding rebuilds: %w", err)
	}

	return records, nil
}

func (r *Repository) RecordFailedVectorRevision(ctx context.Context, record memory.EmbeddingRebuildRecord, revision memory.VectorRevision) error {
	return r.AppendVectorRevision(ctx, revision)
}

func (r *Repository) RecordEmbeddingRebuildFailure(ctx context.Context, record memory.EmbeddingRebuildRecord, failureCause string, failedAt time.Time) error {
	if strings.TrimSpace(record.MemoryID) == "" {
		return fmt.Errorf("memory id is required")
	}
	if err := record.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(failureCause) == "" {
		return fmt.Errorf("failure cause is required")
	}
	if failedAt.IsZero() {
		return fmt.Errorf("failed at is required")
	}

	const query = `
UPDATE embedding_rebuilds
SET
	status = $5,
	failure_reason = $6,
	last_attempted_at = $7
WHERE memory_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	if _, err := r.db.Exec(
		ctx,
		query,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		memory.EmbeddingRebuildStatusFailed,
		failureCause,
		failedAt,
	); err != nil {
		return fmt.Errorf("record embedding rebuild failure: %w", err)
	}

	if err := syncEmbeddingCutoverFailure(ctx, r.db, record, failureCause, failedAt.UTC()); err != nil {
		return err
	}

	return nil
}

func (r *Repository) ApplyLifecycleAction(ctx context.Context, action governance.LifecycleAction) (memory.CanonicalMemory, error) {
	if err := action.Validate(); err != nil {
		return memory.CanonicalMemory{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("begin lifecycle action transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
UPDATE canonical_memories
SET state = $5,
	content = CASE WHEN $5 = 'deleted' THEN '' ELSE content END,
	metadata = CASE WHEN $5 = 'deleted' THEN '{}'::jsonb ELSE metadata END,
	search_text = CASE WHEN $5 = 'deleted' THEN NULL ELSE search_text END,
	embedding = CASE WHEN $5 = 'deleted' THEN NULL ELSE embedding END,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(ctx, query, action.MemoryID, action.Scope.Tenant, action.Scope.Project, action.Scope.Namespace, action.TargetState(), action.AppliedAt).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("apply lifecycle action: %w", err)
	}

	if action.TargetState() == memory.MemoryStateDeleted {
		if canonical.Class == memory.MemoryClassRelation {
			if err := deleteRelationProjection(ctx, tx, action.MemoryID); err != nil {
				return memory.CanonicalMemory{}, err
			}
		}
		if err := writeDeletionMarker(ctx, tx, action); err != nil {
			return memory.CanonicalMemory{}, err
		}
	}

	if err := writeProvenance(ctx, tx, memory.ProvenanceRecord{
		ID:        uuid.NewString(),
		Scope:     action.Scope,
		MemoryID:  action.MemoryID,
		RequestID: action.RequestID,
		Actor:     action.Actor,
		Operation: lifecycleProvenanceOperation(action.Action),
		CreatedAt: action.AppliedAt,
		SourceContext: map[string]any{
			"reason": lifecycleReason(action),
		},
	}); err != nil {
		return memory.CanonicalMemory{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("commit lifecycle action transaction: %w", err)
	}

	return canonical, nil
}

func (r *Repository) SearchLexical(ctx context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	limit := input.TopK
	if limit <= 0 {
		limit = 10
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at,
	ts_rank_cd(search_text, plainto_tsquery('simple', $4)) AS lexical_score
FROM canonical_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND search_text @@ plainto_tsquery('simple', $4)
	AND ($5::timestamptz IS NULL OR updated_at >= $5)
	AND ($6::timestamptz IS NULL OR updated_at <= $6)
ORDER BY lexical_score DESC, updated_at DESC
LIMIT $7
`

	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Query, nullableTime(input.TimeFrom), nullableTime(input.TimeTo), limit)
	if err != nil {
		return nil, fmt.Errorf("search lexical memories: %w", err)
	}
	defer rows.Close()

	hits := make([]retrieval.ScoredMemory, 0)
	for rows.Next() {
		var hit retrieval.ScoredMemory
		if err := rows.Scan(
			&hit.Memory.ID,
			&hit.Memory.Scope.Tenant,
			&hit.Memory.Scope.Project,
			&hit.Memory.Scope.Namespace,
			&hit.Memory.Class,
			&hit.Memory.State,
			&hit.Memory.Content,
			&hit.Memory.CreatedAt,
			&hit.Memory.ModifiedAt,
			&hit.LexicalScore,
		); err != nil {
			return nil, fmt.Errorf("scan lexical search hit: %w", err)
		}
		hits = append(hits, hit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lexical search hits: %w", err)
	}

	return hits, nil
}

func (r *Repository) SearchSemantic(ctx context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if len(input.QueryEmbedding) == 0 {
		return []retrieval.ScoredMemory{}, nil
	}

	limit := input.TopK
	if limit <= 0 {
		limit = 10
	}

	const query = `
SELECT
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	cm.state,
	cm.content,
	cm.created_at,
	cm.updated_at,
	GREATEST(0, 1 - (vr.embedding <=> $4::vector)) AS semantic_score
FROM canonical_memories cm
JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
	AND er.status = 'current'
	AND er.active_vector_revision_id IS NOT NULL
JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
	AND vr.status = 'active'
	AND vr.source_version = er.source_version
	AND vr.content_hash = er.content_hash
	AND vr.embedding IS NOT NULL
WHERE cm.tenant = $1
	AND cm.project = $2
	AND cm.namespace = $3
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND cm.class IN ('profile', 'episodic', 'procedural', 'summary')
	AND ($5::timestamptz IS NULL OR cm.updated_at >= $5)
	AND ($6::timestamptz IS NULL OR cm.updated_at <= $6)
ORDER BY semantic_score DESC, updated_at DESC
LIMIT $7
`

	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		vectorLiteral(input.QueryEmbedding),
		nullableTime(input.TimeFrom),
		nullableTime(input.TimeTo),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search semantic memories: %w", err)
	}
	defer rows.Close()

	hits := make([]retrieval.ScoredMemory, 0)
	for rows.Next() {
		var hit retrieval.ScoredMemory
		if err := rows.Scan(
			&hit.Memory.ID,
			&hit.Memory.Scope.Tenant,
			&hit.Memory.Scope.Project,
			&hit.Memory.Scope.Namespace,
			&hit.Memory.Class,
			&hit.Memory.State,
			&hit.Memory.Content,
			&hit.Memory.CreatedAt,
			&hit.Memory.ModifiedAt,
			&hit.SemanticScore,
		); err != nil {
			return nil, fmt.Errorf("scan semantic search hit: %w", err)
		}
		hits = append(hits, hit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate semantic search hits: %w", err)
	}

	return hits, nil
}

func (r *Repository) SearchRelations(ctx context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if !input.IncludeRelations {
		return []retrieval.ScoredMemory{}, nil
	}

	limit := input.TopK
	if limit <= 0 {
		limit = 10
	}

	const query = `
SELECT
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	cm.state,
	cm.content,
	cm.created_at,
	cm.updated_at,
	(
		CASE WHEN rp.source_entity ILIKE ('%' || $4 || '%') THEN 0.25 ELSE 0 END +
		CASE WHEN rp.relation_type ILIKE ('%' || $5 || '%') THEN 0.20 ELSE 0 END +
		CASE WHEN rp.target_entity ILIKE ('%' || $6 || '%') THEN 0.25 ELSE 0 END +
		CASE WHEN rp.relation_text ILIKE ('%' || $7 || '%') THEN 0.15 ELSE 0 END +
		ts_rank_cd(rp.search_text, plainto_tsquery('simple', $8))
	) AS relation_score
FROM relation_projections rp
JOIN canonical_memories cm ON cm.id = rp.memory_id
WHERE rp.tenant = $1
	AND rp.project = $2
	AND rp.namespace = $3
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND (
		rp.source_entity ILIKE ('%' || $4 || '%')
		OR rp.relation_type ILIKE ('%' || $5 || '%')
		OR rp.target_entity ILIKE ('%' || $6 || '%')
		OR rp.relation_text ILIKE ('%' || $7 || '%')
		OR rp.search_text @@ plainto_tsquery('simple', $8)
	)
	AND ($9::timestamptz IS NULL OR cm.updated_at >= $9)
	AND ($10::timestamptz IS NULL OR cm.updated_at <= $10)
ORDER BY relation_score DESC, cm.updated_at DESC
LIMIT $11
`

	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.Query,
		input.Query,
		input.Query,
		input.Query,
		input.Query,
		nullableTime(input.TimeFrom),
		nullableTime(input.TimeTo),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search relation memories: %w", err)
	}
	defer rows.Close()

	hits := make([]retrieval.ScoredMemory, 0)
	for rows.Next() {
		var hit retrieval.ScoredMemory
		if err := rows.Scan(
			&hit.Memory.ID,
			&hit.Memory.Scope.Tenant,
			&hit.Memory.Scope.Project,
			&hit.Memory.Scope.Namespace,
			&hit.Memory.Class,
			&hit.Memory.State,
			&hit.Memory.Content,
			&hit.Memory.CreatedAt,
			&hit.Memory.ModifiedAt,
			&hit.RelationScore,
		); err != nil {
			return nil, fmt.Errorf("scan relation search hit: %w", err)
		}
		hits = append(hits, hit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relation search hits: %w", err)
	}

	return hits, nil
}

func (r *Repository) ListCitations(ctx context.Context, scope memory.Scope, memoryIDs []string) (map[string][]retrieval.Citation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if len(memoryIDs) == 0 {
		return map[string][]retrieval.Citation{}, nil
	}

	const query = `
SELECT memory_id, raw_event_id, operation
FROM provenance_links
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = ANY($4)
ORDER BY created_at ASC
`

	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memoryIDs)
	if err != nil {
		return nil, fmt.Errorf("list citations: %w", err)
	}
	defer rows.Close()

	citations := make(map[string][]retrieval.Citation)
	for rows.Next() {
		var memoryID string
		var rawEventID sql.NullString
		var operation string
		if err := rows.Scan(&memoryID, &rawEventID, &operation); err != nil {
			return nil, fmt.Errorf("scan citation: %w", err)
		}

		citation := retrieval.Citation{
			MemoryID:  memoryID,
			Operation: operation,
		}
		if rawEventID.Valid {
			citation.RawEventID = rawEventID.String
		}

		citations[memoryID] = append(citations[memoryID], citation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate citations: %w", err)
	}

	return citations, nil
}

func nextMemoryVersion(ctx context.Context, tx pgx.Tx, memoryID string) (int64, error) {
	const query = `
SELECT COALESCE(MAX(version), 0)
FROM memory_versions
WHERE memory_id = $1
`

	var current int64
	if err := tx.QueryRow(ctx, query, memoryID).Scan(&current); err != nil {
		return 0, fmt.Errorf("select next memory version: %w", err)
	}

	return current + 1, nil
}

func currentMemoryVersion(ctx context.Context, tx pgx.Tx, memoryID string) (int64, error) {
	nextVersion, err := nextMemoryVersion(ctx, tx, memoryID)
	if err != nil {
		return 0, err
	}

	return nextVersion - 1, nil
}

func writeDeletionMarker(ctx context.Context, tx pgx.Tx, action governance.LifecycleAction) error {
	const query = `
INSERT INTO deletion_markers (
	memory_id,
	tenant,
	project,
	namespace,
	reason,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6)
`

	if _, err := tx.Exec(
		ctx,
		query,
		action.MemoryID,
		action.Scope.Tenant,
		action.Scope.Project,
		action.Scope.Namespace,
		lifecycleReason(action),
		action.AppliedAt,
	); err != nil {
		return fmt.Errorf("insert deletion marker: %w", err)
	}

	return nil
}

func writeManualMemoryVersion(ctx context.Context, tx pgx.Tx, versionID string, memoryID string, versionNumber int64, state memory.MemoryState, content string, createdAt time.Time, modifiedBy string) (memory.MemoryVersion, error) {
	const versionQuery = `
INSERT INTO memory_versions (
	id,
	memory_id,
	version,
	state,
	content,
	created_at,
	modified_by
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, memory_id, version, state, content, created_at, modified_by
`

	var version memory.MemoryVersion
	if err := tx.QueryRow(
		ctx,
		versionQuery,
		versionID,
		memoryID,
		versionNumber,
		state,
		content,
		createdAt,
		modifiedBy,
	).Scan(
		&version.ID,
		&version.MemoryID,
		&version.Version,
		&version.State,
		&version.Content,
		&version.CreatedAt,
		&version.ModifiedBy,
	); err != nil {
		return memory.MemoryVersion{}, fmt.Errorf("insert manual memory version: %w", err)
	}

	return version, nil
}

func writeManualProvenance(ctx context.Context, tx pgx.Tx, scope memory.Scope, memoryID, operation, requestID, actor string, createdAt time.Time, sourceContext map[string]any) error {
	return writeProvenance(ctx, tx, memory.ProvenanceRecord{
		ID:            uuid.NewString(),
		Scope:         scope,
		MemoryID:      memoryID,
		RequestID:     requestID,
		Actor:         actor,
		Operation:     operation,
		CreatedAt:     createdAt,
		SourceContext: sourceContext,
	})
}

func readScopedCanonicalMemory(ctx context.Context, tx pgx.Tx, scope memory.Scope, memoryID string) (memory.CanonicalMemory, error) {
	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at
FROM canonical_memories
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(ctx, query, memoryID, scope.Tenant, scope.Project, scope.Namespace).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("read scoped canonical memory: %w", err)
	}

	return canonical, nil
}

func suppressScopedCanonicalMemory(ctx context.Context, tx pgx.Tx, scope memory.Scope, memoryID string, updatedAt time.Time) (memory.CanonicalMemory, error) {
	const query = `
UPDATE canonical_memories
SET state = $5,
	embedding = NULL,
	updated_at = $6
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND state <> 'deleted'
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(ctx, query, memoryID, scope.Tenant, scope.Project, scope.Namespace, memory.MemoryStateSuppressed, updatedAt).Scan(
		&canonical.ID,
		&canonical.Scope.Tenant,
		&canonical.Scope.Project,
		&canonical.Scope.Namespace,
		&canonical.Class,
		&canonical.State,
		&canonical.Content,
		&canonical.CreatedAt,
		&canonical.ModifiedAt,
	); err != nil {
		return memory.CanonicalMemory{}, fmt.Errorf("suppress scoped canonical memory: %w", err)
	}

	return canonical, nil
}

func (r *Repository) recordEmbeddingRebuildRequiredTx(ctx context.Context, tx pgx.Tx, canonical memory.CanonicalMemory, sourceVersion int64, requestedAt time.Time) error {
	target, ok, err := r.resolveEmbeddingTarget(canonical.Class)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	record := memory.EmbeddingRebuildRecord{
		MemoryID:            canonical.ID,
		Scope:               canonical.Scope,
		SourceVersion:       sourceVersion,
		ContentHash:         contentHash(canonical.Content),
		RequestedProvider:   target.Provider,
		RequestedModel:      target.Model,
		RequestedDimensions: target.Dimensions,
		Status:              memory.EmbeddingRebuildStatusPending,
		RequestedAt:         requestedAt,
	}

	if err := recordEmbeddingRebuildRequired(ctx, tx, record); err != nil {
		return err
	}

	return nil
}

func (r *Repository) resolveEmbeddingTarget(class memory.MemoryClass) (embedding.Target, bool, error) {
	if r.embeddingRouter.Default == (embedding.Target{}) && len(r.embeddingRouter.ByClass) == 0 {
		return embedding.Target{}, false, nil
	}

	target, err := r.embeddingRouter.ResolveTarget(string(class))
	if err != nil {
		return embedding.Target{}, false, fmt.Errorf("resolve embedding target: %w", err)
	}

	return target, true, nil
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func upsertRelationProjection(ctx context.Context, tx pgx.Tx, canonical memory.CanonicalMemory, updatedAt time.Time) error {
	if canonical.Class != memory.MemoryClassRelation {
		return nil
	}

	sourceEntity, relationType, targetEntity := parseRelationContent(canonical.Content)
	const query = `
INSERT INTO relation_projections (
	memory_id,
	tenant,
	project,
	namespace,
	source_entity,
	relation_type,
	target_entity,
	relation_text,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, to_tsvector('simple', $8), $9, $10)
ON CONFLICT (memory_id) DO UPDATE SET
	tenant = EXCLUDED.tenant,
	project = EXCLUDED.project,
	namespace = EXCLUDED.namespace,
	source_entity = EXCLUDED.source_entity,
	relation_type = EXCLUDED.relation_type,
	target_entity = EXCLUDED.target_entity,
	relation_text = EXCLUDED.relation_text,
	search_text = EXCLUDED.search_text,
	updated_at = EXCLUDED.updated_at
`

	if _, err := tx.Exec(
		ctx,
		query,
		canonical.ID,
		canonical.Scope.Tenant,
		canonical.Scope.Project,
		canonical.Scope.Namespace,
		sourceEntity,
		relationType,
		targetEntity,
		canonical.Content,
		canonical.CreatedAt,
		updatedAt,
	); err != nil {
		return fmt.Errorf("upsert relation projection: %w", err)
	}

	return nil
}

func deleteRelationProjection(ctx context.Context, tx pgx.Tx, memoryID string) error {
	const query = `
DELETE FROM relation_projections
WHERE memory_id = $1
`

	if _, err := tx.Exec(ctx, query, memoryID); err != nil {
		return fmt.Errorf("delete relation projection: %w", err)
	}

	return nil
}

func parseRelationContent(content string) (string, string, string) {
	var sourceEntity string
	var relationType string
	var targetEntity string

	for _, part := range strings.Fields(content) {
		switch {
		case strings.HasPrefix(part, "entity:"):
			sourceEntity = strings.TrimPrefix(part, "entity:")
		case strings.HasPrefix(part, "relation:"):
			relationType = strings.TrimPrefix(part, "relation:")
		case strings.HasPrefix(part, "target:"):
			targetEntity = strings.TrimPrefix(part, "target:")
		}
	}

	return sourceEntity, relationType, targetEntity
}

func writeRawEvent(ctx context.Context, db queryRower, input memory.IngestEventInput) (memory.RawEvent, error) {
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return memory.RawEvent{}, fmt.Errorf("marshal metadata: %w", err)
	}

	const query = `
INSERT INTO raw_events (
	tenant,
	project,
	namespace,
	event_type,
	content,
	metadata,
	source_timestamp
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, tenant, project, namespace, event_type, content, source_timestamp, created_at
`

	var event memory.RawEvent
	if err := db.QueryRow(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.EventType,
		input.Content,
		metadata,
		input.SourceTimestamp,
	).Scan(
		&event.ID,
		&event.Scope.Tenant,
		&event.Scope.Project,
		&event.Scope.Namespace,
		&event.EventType,
		&event.Content,
		&event.SourceTimestamp,
		&event.CreatedAt,
	); err != nil {
		return memory.RawEvent{}, fmt.Errorf("insert raw event: %w", err)
	}

	event.Metadata = input.Metadata
	return event, nil
}

func writeCandidate(ctx context.Context, db queryRower, candidate governance.CandidateMemory) (governance.CandidateMemory, error) {
	const query = `
INSERT INTO candidate_memories (
	id,
	source_raw_event_id,
	tenant,
	project,
	namespace,
	class,
	content,
	confidence,
	importance,
	freshness,
	sensitivity,
	mutability,
	retention_class,
	status,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING
	id,
	source_raw_event_id,
	tenant,
	project,
	namespace,
	class,
	content,
	confidence,
	importance,
	freshness,
	sensitivity,
	mutability,
	retention_class,
	status,
	created_at,
	updated_at
`

	created, err := scanCandidate(db.QueryRow(
		ctx,
		query,
		candidate.ID,
		candidate.SourceRawEventID,
		candidate.Scope.Tenant,
		candidate.Scope.Project,
		candidate.Scope.Namespace,
		candidate.Class,
		candidate.Content,
		candidate.Confidence,
		candidate.Importance,
		candidate.Freshness,
		candidate.Sensitivity,
		candidate.Mutability,
		candidate.RetentionClass,
		candidate.Status,
		candidate.CreatedAt,
		candidate.UpdatedAt,
	))
	if err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("insert candidate memory: %w", err)
	}

	return created, nil
}

func writeProvenance(ctx context.Context, db queryRower, record memory.ProvenanceRecord) error {
	const query = `
INSERT INTO provenance_links (
	id,
	raw_event_id,
	candidate_memory_id,
	memory_id,
	tenant,
	project,
	namespace,
	operation,
	request_id,
	actor,
	source_context,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
`

	sourceContext, err := json.Marshal(normalizeSourceContext(record.SourceContext))
	if err != nil {
		return fmt.Errorf("marshal provenance source context: %w", err)
	}

	if _, err := db.Exec(
		ctx,
		query,
		record.ID,
		nullableString(record.RawEventID),
		nullableString(record.CandidateMemoryID),
		nullableString(record.MemoryID),
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Operation,
		nullableString(record.RequestID),
		nullableString(record.Actor),
		sourceContext,
		record.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert provenance: %w", err)
	}

	return nil
}

type candidateScanner interface {
	Scan(dest ...any) error
}

func scanCandidate(scanner candidateScanner) (governance.CandidateMemory, error) {
	var candidate governance.CandidateMemory
	if err := scanner.Scan(
		&candidate.ID,
		&candidate.SourceRawEventID,
		&candidate.Scope.Tenant,
		&candidate.Scope.Project,
		&candidate.Scope.Namespace,
		&candidate.Class,
		&candidate.Content,
		&candidate.Confidence,
		&candidate.Importance,
		&candidate.Freshness,
		&candidate.Sensitivity,
		&candidate.Mutability,
		&candidate.RetentionClass,
		&candidate.Status,
		&candidate.CreatedAt,
		&candidate.UpdatedAt,
	); err != nil {
		return governance.CandidateMemory{}, fmt.Errorf("scan candidate memory: %w", err)
	}

	return candidate, nil
}

func scanGovernanceRawEvent(scanner governanceRawEventScanner, now time.Time) (governance.GovernanceRawEvent, governance.RawEventGovernanceSnapshot, error) {
	var event memory.RawEvent
	var snapshot governance.RawEventGovernanceSnapshot
	var sourceTimestamp sql.NullTime
	var workerID sql.NullString
	var claimedAt sql.NullTime
	var leaseUntil sql.NullTime
	var lastFailedAt sql.NullTime
	var lastError sql.NullString
	var nextAttemptAt sql.NullTime
	var exhaustedAt sql.NullTime
	var processedAt sql.NullTime

	if err := scanner.Scan(
		&event.ID,
		&event.Scope.Tenant,
		&event.Scope.Project,
		&event.Scope.Namespace,
		&event.EventType,
		&event.Content,
		&sourceTimestamp,
		&event.CreatedAt,
		&snapshot.Attempt,
		&workerID,
		&claimedAt,
		&leaseUntil,
		&lastFailedAt,
		&lastError,
		&nextAttemptAt,
		&exhaustedAt,
		&processedAt,
	); err != nil {
		return governance.GovernanceRawEvent{}, governance.RawEventGovernanceSnapshot{}, fmt.Errorf("scan governance raw event: %w", err)
	}

	if sourceTimestamp.Valid {
		event.SourceTimestamp = sourceTimestamp.Time
	}
	if workerID.Valid {
		snapshot.WorkerID = workerID.String
	}
	if claimedAt.Valid {
		snapshot.ClaimedAt = claimedAt.Time
	}
	if leaseUntil.Valid {
		snapshot.LeaseUntil = leaseUntil.Time
	}
	if lastFailedAt.Valid {
		snapshot.LastFailedAt = lastFailedAt.Time
	}
	if lastError.Valid {
		snapshot.LastError = lastError.String
	}
	if nextAttemptAt.Valid {
		snapshot.NextAttemptAt = nextAttemptAt.Time
	}
	if exhaustedAt.Valid {
		snapshot.ExhaustedAt = exhaustedAt.Time
	}
	if processedAt.Valid {
		snapshot.ProcessedAt = processedAt.Time
	}

	return governance.NewGovernanceRawEvent(event, snapshot, now.UTC()), snapshot, nil
}

func scanGovernanceRecoveryRecord(scanner provenanceScanner) (governance.GovernanceRecoveryRecord, error) {
	var record governance.GovernanceRecoveryRecord
	var beforeSnapshot []byte
	var afterSnapshot []byte
	if err := scanner.Scan(
		&record.ID,
		&record.RawEventID,
		&record.Scope.Tenant,
		&record.Scope.Project,
		&record.Scope.Namespace,
		&record.Action,
		&record.Actor,
		&record.Reason,
		&beforeSnapshot,
		&afterSnapshot,
		&record.OccurredAt,
	); err != nil {
		return governance.GovernanceRecoveryRecord{}, fmt.Errorf("scan governance recovery record: %w", err)
	}

	if err := json.Unmarshal(beforeSnapshot, &record.Before); err != nil {
		return governance.GovernanceRecoveryRecord{}, fmt.Errorf("unmarshal governance recovery before snapshot: %w", err)
	}
	if err := json.Unmarshal(afterSnapshot, &record.After); err != nil {
		return governance.GovernanceRecoveryRecord{}, fmt.Errorf("unmarshal governance recovery after snapshot: %w", err)
	}

	return record, nil
}

func readEmbeddingCutoverPlan(ctx context.Context, db queryRower, input memory.ReadEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error) {
	const planQuery = `
SELECT
	id,
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at,
	last_action_by,
	last_action_reason,
	last_action_at,
	activated_at,
	paused_at,
	cancelled_at,
	completed_at
FROM embedding_cutover_plans
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	plan, err := scanEmbeddingCutoverPlan(db.QueryRow(ctx, planQuery, input.PlanID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace))
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, err
	}

	const itemsQuery = `
SELECT
	plan_id,
	memory_id,
	tenant,
	project,
	namespace,
	class,
	status,
	failure_reason,
	active_vector_revision_id,
	active_provider,
	active_model,
	active_dimensions,
	requested_at,
	last_attempted_at,
	updated_at
FROM embedding_cutover_items
WHERE plan_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
ORDER BY updated_at ASC, memory_id ASC
`

	rows, err := db.Query(ctx, itemsQuery, input.PlanID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace)
	if err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("list embedding cutover items: %w", err)
	}
	defer rows.Close()

	items := make([]memory.EmbeddingCutoverItem, 0)
	for rows.Next() {
		item, err := scanEmbeddingCutoverItem(rows)
		if err != nil {
			return memory.EmbeddingCutoverPlan{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("iterate embedding cutover items: %w", err)
	}

	plan.Items = items
	plan.Progress = calculateEmbeddingCutoverProgress(items)
	return plan, nil
}

func registerEmbeddingCutoverItems(ctx context.Context, db queryRower, plan memory.EmbeddingCutoverPlan, appliedAt time.Time) error {
	const query = `
INSERT INTO embedding_cutover_items (
	plan_id,
	memory_id,
	tenant,
	project,
	namespace,
	class,
	status,
	failure_reason,
	active_vector_revision_id,
	active_provider,
	active_model,
	active_dimensions,
	requested_at,
	last_attempted_at,
	created_at,
	updated_at
)
SELECT
	$1,
	cm.id,
	cm.tenant,
	cm.project,
	cm.namespace,
	cm.class,
	CASE
		WHEN vr.provider = $5
			AND vr.model = $6
			AND vr.dimensions = $7
			AND er.status = $8
		THEN $9
		WHEN er.requested_provider = $5
			AND er.requested_model = $6
			AND er.requested_dimensions = $7
			AND er.status = $10
		THEN $11
		WHEN er.requested_provider = $5
			AND er.requested_model = $6
			AND er.requested_dimensions = $7
			AND er.status = $12
		THEN $13
		WHEN er.requested_provider = $5
			AND er.requested_model = $6
			AND er.requested_dimensions = $7
			AND er.status = $14
		THEN $15
		ELSE $16
	END,
	CASE
		WHEN er.requested_provider = $5
			AND er.requested_model = $6
			AND er.requested_dimensions = $7
			AND er.status = $14
		THEN er.failure_reason
		ELSE NULL
	END,
	er.active_vector_revision_id,
	vr.provider,
	vr.model,
	vr.dimensions,
	er.requested_at,
	er.last_attempted_at,
	$17,
	$17
FROM canonical_memories cm
LEFT JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.tenant = $2
	AND cm.project = $3
	AND cm.namespace = $4
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND cm.class = ANY($18)
ON CONFLICT (plan_id, memory_id) DO NOTHING
`

	if _, err := db.Exec(
		ctx,
		query,
		plan.ID,
		plan.Scope.Tenant,
		plan.Scope.Project,
		plan.Scope.Namespace,
		plan.Target.Provider,
		plan.Target.Model,
		plan.Target.Dimensions,
		memory.EmbeddingRebuildStatusCurrent,
		memory.EmbeddingCutoverItemStatusCurrent,
		memory.EmbeddingRebuildStatusPending,
		memory.EmbeddingCutoverItemStatusQueued,
		memory.EmbeddingRebuildStatusRebuilding,
		memory.EmbeddingCutoverItemStatusRebuilding,
		memory.EmbeddingRebuildStatusFailed,
		memory.EmbeddingCutoverItemStatusFailed,
		memory.EmbeddingCutoverItemStatusQueued,
		appliedAt.UTC(),
		embeddingCutoverEligibleClassStrings(plan.Classes),
	); err != nil {
		return fmt.Errorf("register embedding cutover items: %w", err)
	}

	return nil
}

func readEmbeddingCutoverAdmissionConflict(ctx context.Context, db queryRower, plan memory.EmbeddingCutoverPlan) (*memory.EmbeddingCutoverPlanSummary, error) {
	const query = `
SELECT id, status
FROM embedding_cutover_plans
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND id <> $4
	AND status = ANY($5)
ORDER BY created_at ASC
LIMIT 1
`

	rows, err := db.Query(
		ctx,
		query,
		plan.Scope.Tenant,
		plan.Scope.Project,
		plan.Scope.Namespace,
		plan.ID,
		[]string{string(memory.EmbeddingCutoverPlanStatusActive), string(memory.EmbeddingCutoverPlanStatusPaused)},
	)
	if err != nil {
		return nil, fmt.Errorf("read embedding cutover admission conflict: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate embedding cutover admission conflict: %w", err)
		}
		return nil, nil
	}

	var summary memory.EmbeddingCutoverPlanSummary
	if err := rows.Scan(&summary.ID, &summary.Status); err != nil {
		return nil, fmt.Errorf("scan embedding cutover admission conflict: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding cutover admission conflict: %w", err)
	}
	return &summary, nil
}

func readEmbeddingCutoverAdmissionBreakdown(ctx context.Context, db queryRower, plan memory.EmbeddingCutoverPlan) ([]memory.EmbeddingCutoverClassBreakdown, error) {
	const query = `
SELECT
	cm.class,
	COUNT(*)::integer AS eligible,
	COUNT(*) FILTER (
		WHERE er.active_vector_revision_id IS NOT NULL
			AND (
				vr.provider IS DISTINCT FROM $4
				OR vr.model IS DISTINCT FROM $5
				OR vr.dimensions IS DISTINCT FROM $6
			)
	)::integer AS drifted,
	COUNT(*) FILTER (
		WHERE er.active_vector_revision_id IS NULL
	)::integer AS missing_active_vector,
	0::integer AS missing_route
FROM canonical_memories cm
LEFT JOIN embedding_rebuilds er
	ON er.memory_id = cm.id
	AND er.tenant = cm.tenant
	AND er.project = cm.project
	AND er.namespace = cm.namespace
LEFT JOIN vector_revisions vr
	ON vr.id = er.active_vector_revision_id
	AND vr.memory_id = cm.id
	AND vr.tenant = cm.tenant
	AND vr.project = cm.project
	AND vr.namespace = cm.namespace
WHERE cm.tenant = $1
	AND cm.project = $2
	AND cm.namespace = $3
	AND cm.state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND cm.class = ANY($7)
GROUP BY cm.class
ORDER BY cm.class ASC
`

	rows, err := db.Query(
		ctx,
		query,
		plan.Scope.Tenant,
		plan.Scope.Project,
		plan.Scope.Namespace,
		plan.Target.Provider,
		plan.Target.Model,
		plan.Target.Dimensions,
		embeddingCutoverEligibleClassStrings(plan.Classes),
	)
	if err != nil {
		return nil, fmt.Errorf("read embedding cutover admission breakdown: %w", err)
	}
	defer rows.Close()

	breakdown := make([]memory.EmbeddingCutoverClassBreakdown, 0)
	for rows.Next() {
		var item memory.EmbeddingCutoverClassBreakdown
		if err := rows.Scan(&item.Class, &item.Eligible, &item.Drifted, &item.MissingActiveVector, &item.MissingRoute); err != nil {
			return nil, fmt.Errorf("scan embedding cutover admission breakdown: %w", err)
		}
		breakdown = append(breakdown, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding cutover admission breakdown: %w", err)
	}
	return breakdown, nil
}

type embeddingCutoverDispatchCandidate struct {
	PlanID               string
	MemoryID             string
	Scope                memory.Scope
	Class                memory.MemoryClass
	Status               memory.EmbeddingCutoverItemStatus
	ActiveVectorRevision string
	CurrentSourceVersion int64
	Content              string
}

func listActiveEmbeddingCutoverPlansForDispatch(ctx context.Context, db queryRower, scope memory.Scope) ([]memory.EmbeddingCutoverPlan, error) {
	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	status,
	target_provider,
	target_model,
	target_dimensions,
	class_filters,
	wave_size,
	reason,
	created_by,
	created_at,
	last_action_by,
	last_action_reason,
	last_action_at,
	activated_at,
	paused_at,
	cancelled_at,
	completed_at
FROM embedding_cutover_plans
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND status = $4
ORDER BY activated_at ASC NULLS FIRST, created_at ASC, id ASC
FOR UPDATE
`

	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memory.EmbeddingCutoverPlanStatusActive)
	if err != nil {
		return nil, fmt.Errorf("list active embedding cutover plans: %w", err)
	}
	defer rows.Close()

	plans := make([]memory.EmbeddingCutoverPlan, 0)
	for rows.Next() {
		plan, err := scanEmbeddingCutoverPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active embedding cutover plans: %w", err)
	}

	return plans, nil
}

func listEmbeddingCutoverWaveCandidates(ctx context.Context, db queryRower, plan memory.EmbeddingCutoverPlan, limit int) ([]embeddingCutoverDispatchCandidate, error) {
	const query = `
SELECT
	eci.plan_id,
	eci.memory_id,
	eci.tenant,
	eci.project,
	eci.namespace,
	eci.class,
	eci.status,
	eci.failure_reason,
	eci.active_vector_revision_id,
	eci.active_provider,
	eci.active_model,
	eci.active_dimensions,
	eci.requested_at,
	eci.last_attempted_at,
	eci.updated_at,
	COALESCE(mv.version, 0) AS current_source_version,
	cm.content
FROM embedding_cutover_items eci
JOIN canonical_memories cm
	ON cm.id = eci.memory_id
	AND cm.tenant = eci.tenant
	AND cm.project = eci.project
	AND cm.namespace = eci.namespace
LEFT JOIN (
	SELECT memory_id, MAX(version) AS version
	FROM memory_versions
	GROUP BY memory_id
) mv ON mv.memory_id = cm.id
WHERE eci.plan_id = $1
	AND eci.tenant = $2
	AND eci.project = $3
	AND eci.namespace = $4
	AND eci.status = $5
ORDER BY eci.updated_at ASC, eci.memory_id ASC
LIMIT $6
FOR UPDATE OF eci SKIP LOCKED
`

	rows, err := db.Query(ctx, query, plan.ID, plan.Scope.Tenant, plan.Scope.Project, plan.Scope.Namespace, memory.EmbeddingCutoverItemStatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("list embedding cutover wave candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]embeddingCutoverDispatchCandidate, 0)
	for rows.Next() {
		var candidate embeddingCutoverDispatchCandidate
		var failureReason sql.NullString
		var activeVectorRevision sql.NullString
		var activeProvider sql.NullString
		var activeModel sql.NullString
		var activeDimensions sql.NullInt32
		var requestedAt sql.NullTime
		var lastAttemptedAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(
			&candidate.PlanID,
			&candidate.MemoryID,
			&candidate.Scope.Tenant,
			&candidate.Scope.Project,
			&candidate.Scope.Namespace,
			&candidate.Class,
			&candidate.Status,
			&failureReason,
			&activeVectorRevision,
			&activeProvider,
			&activeModel,
			&activeDimensions,
			&requestedAt,
			&lastAttemptedAt,
			&updatedAt,
			&candidate.CurrentSourceVersion,
			&candidate.Content,
		); err != nil {
			return nil, fmt.Errorf("scan embedding cutover wave candidate: %w", err)
		}
		if activeVectorRevision.Valid {
			candidate.ActiveVectorRevision = activeVectorRevision.String
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedding cutover wave candidates: %w", err)
	}

	return candidates, nil
}

func markEmbeddingCutoverItemDispatched(ctx context.Context, db queryRower, candidate embeddingCutoverDispatchCandidate, requestedAt time.Time) error {
	const query = `
UPDATE embedding_cutover_items
SET
	status = $6,
	failure_reason = NULL,
	requested_at = $7,
	updated_at = $8
WHERE plan_id = $1
	AND memory_id = $2
	AND tenant = $3
	AND project = $4
	AND namespace = $5
`

	if _, err := db.Exec(
		ctx,
		query,
		candidate.PlanID,
		candidate.MemoryID,
		candidate.Scope.Tenant,
		candidate.Scope.Project,
		candidate.Scope.Namespace,
		memory.EmbeddingCutoverItemStatusRebuilding,
		requestedAt,
		requestedAt,
	); err != nil {
		return fmt.Errorf("mark embedding cutover item dispatched: %w", err)
	}

	return nil
}

func transitionEmbeddingCutoverItems(ctx context.Context, db queryRower, planID string, scope memory.Scope, nextStatus memory.EmbeddingCutoverItemStatus, updatedAt time.Time, currentStatuses []memory.EmbeddingCutoverItemStatus) error {
	if len(currentStatuses) == 0 {
		return nil
	}

	const query = `
UPDATE embedding_cutover_items
SET
	status = $5,
	updated_at = $6
WHERE plan_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND status = ANY($7)
`

	if _, err := db.Exec(
		ctx,
		query,
		planID,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		nextStatus,
		updatedAt,
		embeddingCutoverItemStatusStrings(currentStatuses),
	); err != nil {
		return fmt.Errorf("transition embedding cutover items: %w", err)
	}

	return nil
}

func syncEmbeddingCutoverPromotion(ctx context.Context, db queryRower, revision memory.VectorRevision) error {
	const query = `
UPDATE embedding_cutover_items eci
SET
	status = $8,
	failure_reason = NULL,
	active_vector_revision_id = $9,
	active_provider = $10,
	active_model = $11,
	active_dimensions = $12,
	last_attempted_at = $13,
	updated_at = $14
FROM embedding_cutover_plans ecp
WHERE eci.plan_id = ecp.id
	AND eci.memory_id = $1
	AND eci.tenant = $2
	AND eci.project = $3
	AND eci.namespace = $4
	AND ecp.target_provider = $5
	AND ecp.target_model = $6
	AND ecp.target_dimensions = $7
	AND eci.status IN ($15, $16)
`

	if _, err := db.Exec(
		ctx,
		query,
		revision.MemoryID,
		revision.Scope.Tenant,
		revision.Scope.Project,
		revision.Scope.Namespace,
		revision.Provider,
		revision.Model,
		revision.Dimensions,
		memory.EmbeddingCutoverItemStatusCurrent,
		revision.ID,
		revision.Provider,
		revision.Model,
		revision.Dimensions,
		revision.ActivatedAt,
		revision.ActivatedAt,
		memory.EmbeddingCutoverItemStatusRebuilding,
		memory.EmbeddingCutoverItemStatusFailed,
	); err != nil {
		return fmt.Errorf("sync embedding cutover promotion: %w", err)
	}

	return nil
}

func syncEmbeddingCutoverFailure(ctx context.Context, db queryRower, record memory.EmbeddingRebuildRecord, failureCause string, failedAt time.Time) error {
	const query = `
UPDATE embedding_cutover_items eci
SET
	status = $5,
	failure_reason = $6,
	last_attempted_at = $7,
	updated_at = $8
FROM embedding_cutover_plans ecp
WHERE eci.plan_id = ecp.id
	AND eci.memory_id = $1
	AND eci.tenant = $2
	AND eci.project = $3
	AND eci.namespace = $4
	AND ecp.target_provider = $9
	AND ecp.target_model = $10
	AND ecp.target_dimensions = $11
	AND eci.status = $12
`

	if _, err := db.Exec(
		ctx,
		query,
		record.MemoryID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		memory.EmbeddingCutoverItemStatusFailed,
		failureCause,
		failedAt,
		failedAt,
		record.RequestedProvider,
		record.RequestedModel,
		record.RequestedDimensions,
		memory.EmbeddingCutoverItemStatusRebuilding,
	); err != nil {
		return fmt.Errorf("sync embedding cutover failure: %w", err)
	}

	return nil
}

func readEmbeddingRecoveryCutoverPlanID(ctx context.Context, db queryRower, scope memory.Scope, memoryID string) (string, error) {
	const query = `
SELECT ecp.id
FROM embedding_cutover_items eci
JOIN embedding_cutover_plans ecp
	ON ecp.id = eci.plan_id
	AND ecp.tenant = eci.tenant
	AND ecp.project = eci.project
	AND ecp.namespace = eci.namespace
WHERE eci.memory_id = $1
	AND eci.tenant = $2
	AND eci.project = $3
	AND eci.namespace = $4
	AND ecp.status IN ($5, $6)
ORDER BY ecp.created_at DESC, ecp.id DESC
LIMIT 1
`

	var planID string
	err := db.QueryRow(
		ctx,
		query,
		memoryID,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		string(memory.EmbeddingCutoverPlanStatusActive),
		string(memory.EmbeddingCutoverPlanStatusPaused),
	).Scan(&planID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("read embedding recovery cutover context: %w", err)
	}

	return planID, nil
}

func scanEmbeddingCutoverPlan(scanner provenanceScanner) (memory.EmbeddingCutoverPlan, error) {
	var plan memory.EmbeddingCutoverPlan
	var classFilters []string
	var lastActionBy sql.NullString
	var lastActionReason sql.NullString
	var lastActionAt sql.NullTime
	var activatedAt sql.NullTime
	var pausedAt sql.NullTime
	var cancelledAt sql.NullTime
	var completedAt sql.NullTime

	if err := scanner.Scan(
		&plan.ID,
		&plan.Scope.Tenant,
		&plan.Scope.Project,
		&plan.Scope.Namespace,
		&plan.Status,
		&plan.Target.Provider,
		&plan.Target.Model,
		&plan.Target.Dimensions,
		&classFilters,
		&plan.WaveSize,
		&plan.Reason,
		&plan.CreatedBy,
		&plan.CreatedAt,
		&lastActionBy,
		&lastActionReason,
		&lastActionAt,
		&activatedAt,
		&pausedAt,
		&cancelledAt,
		&completedAt,
	); err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("scan embedding cutover plan: %w", err)
	}

	plan.Classes = embeddingCutoverPlanClasses(classFilters)
	if lastActionBy.Valid {
		plan.LastActionBy = lastActionBy.String
	}
	if lastActionReason.Valid {
		plan.LastActionReason = lastActionReason.String
	}
	if lastActionAt.Valid {
		plan.LastActionAt = lastActionAt.Time
	}
	if activatedAt.Valid {
		plan.ActivatedAt = activatedAt.Time
	}
	if pausedAt.Valid {
		plan.PausedAt = pausedAt.Time
	}
	if cancelledAt.Valid {
		plan.CancelledAt = cancelledAt.Time
	}
	if completedAt.Valid {
		plan.CompletedAt = completedAt.Time
	}

	return plan, nil
}

func scanEmbeddingCutoverPlanWithProgress(scanner provenanceScanner) (memory.EmbeddingCutoverPlan, error) {
	var plan memory.EmbeddingCutoverPlan
	var classFilters []string
	var lastActionBy sql.NullString
	var lastActionReason sql.NullString
	var lastActionAt sql.NullTime
	var activatedAt sql.NullTime
	var pausedAt sql.NullTime
	var cancelledAt sql.NullTime
	var completedAt sql.NullTime

	if err := scanner.Scan(
		&plan.ID,
		&plan.Scope.Tenant,
		&plan.Scope.Project,
		&plan.Scope.Namespace,
		&plan.Status,
		&plan.Target.Provider,
		&plan.Target.Model,
		&plan.Target.Dimensions,
		&classFilters,
		&plan.WaveSize,
		&plan.Reason,
		&plan.CreatedBy,
		&plan.CreatedAt,
		&lastActionBy,
		&lastActionReason,
		&lastActionAt,
		&activatedAt,
		&pausedAt,
		&cancelledAt,
		&completedAt,
		&plan.Progress.Total,
		&plan.Progress.Queued,
		&plan.Progress.Rebuilding,
		&plan.Progress.Current,
		&plan.Progress.Failed,
		&plan.Progress.Skipped,
		&plan.Progress.Paused,
		&plan.Progress.Cancelled,
	); err != nil {
		return memory.EmbeddingCutoverPlan{}, fmt.Errorf("scan embedding cutover plan with progress: %w", err)
	}

	plan.Classes = embeddingCutoverPlanClasses(classFilters)
	if lastActionBy.Valid {
		plan.LastActionBy = lastActionBy.String
	}
	if lastActionReason.Valid {
		plan.LastActionReason = lastActionReason.String
	}
	if lastActionAt.Valid {
		plan.LastActionAt = lastActionAt.Time
	}
	if activatedAt.Valid {
		plan.ActivatedAt = activatedAt.Time
	}
	if pausedAt.Valid {
		plan.PausedAt = pausedAt.Time
	}
	if cancelledAt.Valid {
		plan.CancelledAt = cancelledAt.Time
	}
	if completedAt.Valid {
		plan.CompletedAt = completedAt.Time
	}

	return plan, nil
}

func scanEmbeddingCutoverItem(scanner provenanceScanner) (memory.EmbeddingCutoverItem, error) {
	var item memory.EmbeddingCutoverItem
	var failureReason sql.NullString
	var activeVectorRevision sql.NullString
	var activeProvider sql.NullString
	var activeModel sql.NullString
	var activeDimensions sql.NullInt32
	var requestedAt sql.NullTime
	var lastAttemptedAt sql.NullTime

	if err := scanner.Scan(
		&item.PlanID,
		&item.MemoryID,
		&item.Scope.Tenant,
		&item.Scope.Project,
		&item.Scope.Namespace,
		&item.Class,
		&item.Status,
		&failureReason,
		&activeVectorRevision,
		&activeProvider,
		&activeModel,
		&activeDimensions,
		&requestedAt,
		&lastAttemptedAt,
		&item.UpdatedAt,
	); err != nil {
		return memory.EmbeddingCutoverItem{}, fmt.Errorf("scan embedding cutover item: %w", err)
	}

	if failureReason.Valid {
		item.FailureReason = failureReason.String
	}
	if activeVectorRevision.Valid {
		item.ActiveVectorRevision = activeVectorRevision.String
	}
	if activeProvider.Valid {
		item.ActiveProvider = activeProvider.String
	}
	if activeModel.Valid {
		item.ActiveModel = activeModel.String
	}
	if activeDimensions.Valid {
		item.ActiveDimensions = int(activeDimensions.Int32)
	}
	if requestedAt.Valid {
		item.RequestedAt = requestedAt.Time
	}
	if lastAttemptedAt.Valid {
		item.LastAttemptedAt = lastAttemptedAt.Time
	}

	return item, nil
}

func scanEmbeddingRecoveryRecord(scanner provenanceScanner) (memory.EmbeddingRecoveryRecord, error) {
	var record memory.EmbeddingRecoveryRecord
	var cutoverPlanID sql.NullString
	var beforeSnapshot []byte
	var afterSnapshot []byte
	if err := scanner.Scan(
		&record.ID,
		&record.MemoryID,
		&record.Scope.Tenant,
		&record.Scope.Project,
		&record.Scope.Namespace,
		&cutoverPlanID,
		&record.Action,
		&record.Actor,
		&record.Reason,
		&beforeSnapshot,
		&afterSnapshot,
		&record.OccurredAt,
	); err != nil {
		return memory.EmbeddingRecoveryRecord{}, fmt.Errorf("scan embedding recovery record: %w", err)
	}

	if cutoverPlanID.Valid {
		record.CutoverPlanID = cutoverPlanID.String
	}
	if err := json.Unmarshal(beforeSnapshot, &record.Before); err != nil {
		return memory.EmbeddingRecoveryRecord{}, fmt.Errorf("unmarshal embedding recovery before snapshot: %w", err)
	}
	if err := json.Unmarshal(afterSnapshot, &record.After); err != nil {
		return memory.EmbeddingRecoveryRecord{}, fmt.Errorf("unmarshal embedding recovery after snapshot: %w", err)
	}

	return record, nil
}

func scanEmbeddingRebuildView(scanner provenanceScanner) (memory.EmbeddingRebuildView, error) {
	var rebuild memory.EmbeddingRebuildView
	var failureReason sql.NullString
	var lastAttemptedAt sql.NullTime
	var activeVectorRevision sql.NullString
	var activeProvider sql.NullString
	var activeModel sql.NullString
	var activeDimensions sql.NullInt32

	if err := scanner.Scan(
		&rebuild.MemoryID,
		&rebuild.Scope.Tenant,
		&rebuild.Scope.Project,
		&rebuild.Scope.Namespace,
		&rebuild.Class,
		&rebuild.State,
		&rebuild.RequestedProvider,
		&rebuild.RequestedModel,
		&rebuild.RequestedDimensions,
		&rebuild.Status,
		&failureReason,
		&rebuild.RequestedAt,
		&lastAttemptedAt,
		&activeVectorRevision,
		&activeProvider,
		&activeModel,
		&activeDimensions,
	); err != nil {
		return memory.EmbeddingRebuildView{}, fmt.Errorf("scan embedding rebuild: %w", err)
	}

	if failureReason.Valid {
		rebuild.FailureReason = failureReason.String
	}
	if lastAttemptedAt.Valid {
		rebuild.LastAttemptedAt = lastAttemptedAt.Time
	}
	if activeVectorRevision.Valid {
		rebuild.ActiveVectorRevision = activeVectorRevision.String
	}
	if activeProvider.Valid {
		rebuild.ActiveProvider = activeProvider.String
	}
	if activeModel.Valid {
		rebuild.ActiveModel = activeModel.String
	}
	if activeDimensions.Valid {
		rebuild.ActiveDimensions = int(activeDimensions.Int32)
	}
	rebuild.Drifted = embedding.DetermineDrift(
		embedding.Target{
			Provider:   rebuild.RequestedProvider,
			Model:      rebuild.RequestedModel,
			Dimensions: rebuild.RequestedDimensions,
		},
		rebuild.ActiveProvider,
		rebuild.ActiveModel,
		rebuild.ActiveDimensions,
	)

	return rebuild, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return value
}

func lifecycleReason(action governance.LifecycleAction) string {
	if strings.TrimSpace(action.Reason) != "" {
		return action.Reason
	}

	return action.Content
}

func lifecycleProvenanceOperation(action policy.ForgettingAction) string {
	switch action {
	case policy.ForgettingActionSuppress:
		return "suppress_memory"
	case policy.ForgettingActionExpire:
		return "expire_memory"
	case policy.ForgettingActionDelete:
		return "delete_memory"
	default:
		return "lifecycle_memory"
	}
}

func normalizeSourceContext(sourceContext map[string]any) map[string]any {
	if sourceContext == nil {
		return map[string]any{}
	}

	return sourceContext
}

type provenanceScanner interface {
	Scan(dest ...any) error
}

func scanProvenanceRecord(scanner provenanceScanner) (memory.ProvenanceRecord, error) {
	var record memory.ProvenanceRecord
	var rawEventID sql.NullString
	var candidateMemoryID sql.NullString
	var linkedMemoryID sql.NullString
	var requestID sql.NullString
	var actor sql.NullString
	var sourceContext []byte

	if err := scanner.Scan(
		&record.ID,
		&rawEventID,
		&candidateMemoryID,
		&linkedMemoryID,
		&record.Scope.Tenant,
		&record.Scope.Project,
		&record.Scope.Namespace,
		&record.Operation,
		&requestID,
		&actor,
		&sourceContext,
		&record.CreatedAt,
	); err != nil {
		return memory.ProvenanceRecord{}, fmt.Errorf("scan provenance record: %w", err)
	}

	if rawEventID.Valid {
		record.RawEventID = rawEventID.String
	}
	if candidateMemoryID.Valid {
		record.CandidateMemoryID = candidateMemoryID.String
	}
	if linkedMemoryID.Valid {
		record.MemoryID = linkedMemoryID.String
	}
	if requestID.Valid {
		record.RequestID = requestID.String
	}
	if actor.Valid {
		record.Actor = actor.String
	}

	record.SourceContext = map[string]any{}
	if len(sourceContext) > 0 {
		if err := json.Unmarshal(sourceContext, &record.SourceContext); err != nil {
			return memory.ProvenanceRecord{}, fmt.Errorf("unmarshal provenance source context: %w", err)
		}
	}

	return record, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}

	return value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}

	return *value
}

func embeddingCutoverPlanClassStrings(classes []memory.MemoryClass) []string {
	values := make([]string, 0, len(classes))
	for _, class := range classes {
		values = append(values, string(class))
	}
	if len(values) == 0 {
		return []string{}
	}

	return values
}

func embeddingCutoverPlanClasses(values []string) []memory.MemoryClass {
	classes := make([]memory.MemoryClass, 0, len(values))
	for _, value := range values {
		classes = append(classes, memory.MemoryClass(value))
	}

	return classes
}

func embeddingCutoverEligibleClassStrings(classes []memory.MemoryClass) []string {
	if len(classes) == 0 {
		return []string{
			string(memory.MemoryClassProfile),
			string(memory.MemoryClassEpisodic),
			string(memory.MemoryClassProcedural),
			string(memory.MemoryClassSummary),
		}
	}

	return embeddingCutoverPlanClassStrings(classes)
}

func embeddingCutoverItemStatusStrings(statuses []memory.EmbeddingCutoverItemStatus) []string {
	values := make([]string, 0, len(statuses))
	for _, status := range statuses {
		values = append(values, string(status))
	}

	return values
}

func calculateEmbeddingCutoverProgress(items []memory.EmbeddingCutoverItem) memory.EmbeddingCutoverProgress {
	progress := memory.EmbeddingCutoverProgress{Total: len(items)}
	for _, item := range items {
		switch item.Status {
		case memory.EmbeddingCutoverItemStatusQueued:
			progress.Queued++
		case memory.EmbeddingCutoverItemStatusRebuilding:
			progress.Rebuilding++
		case memory.EmbeddingCutoverItemStatusCurrent:
			progress.Current++
		case memory.EmbeddingCutoverItemStatusFailed:
			progress.Failed++
		case memory.EmbeddingCutoverItemStatusSkipped:
			progress.Skipped++
		case memory.EmbeddingCutoverItemStatusPaused:
			progress.Paused++
		case memory.EmbeddingCutoverItemStatusCancelled:
			progress.Cancelled++
		}
	}

	return progress
}

func marshalGovernanceRecoverySnapshot(snapshot governance.GovernanceRecoverySnapshot) ([]byte, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal governance recovery snapshot: %w", err)
	}

	return payload, nil
}

func marshalEmbeddingRecoverySnapshot(snapshot memory.EmbeddingRecoverySnapshot) ([]byte, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding recovery snapshot: %w", err)
	}

	return payload, nil
}

func vectorLiteral(values []float32) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}

	return "[" + strings.Join(parts, ",") + "]"
}

func nullableVectorLiteral(values []float32) any {
	if len(values) == 0 {
		return nil
	}

	return vectorLiteral(values)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505"
}
