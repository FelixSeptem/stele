package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"

	"github.com/FelixSeptem/stele/internal/memory"
)

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type transactionStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Repository struct {
	db queryRower
	tx transactionStarter
}

func NewRepository(db interface {
	queryRower
	transactionStarter
}) *Repository {
	return &Repository{db: db, tx: db}
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

func writeProvenance(ctx context.Context, db queryRower, record memory.ProvenanceRecord) error {
	const query = `
INSERT INTO provenance_links (
	id,
	raw_event_id,
	tenant,
	project,
	namespace,
	operation,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
`

	if _, err := db.Exec(
		ctx,
		query,
		record.ID,
		record.RawEventID,
		record.Scope.Tenant,
		record.Scope.Project,
		record.Scope.Namespace,
		record.Operation,
		record.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert provenance: %w", err)
	}

	return nil
}
