package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/FelixSeptem/stele/internal/memory"
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
	content,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, to_tsvector('simple', $7), $8, $9)
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
			ID:                input.VersionID + "_prov",
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
SET state = $2, content = $3, search_text = to_tsvector('simple', $3), updated_at = $4
WHERE id = $1
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(
		ctx,
		canonicalUpdateQuery,
		input.MemoryID,
		memory.MemoryStateActive,
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
		ID:                input.VersionID + "_prov",
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
	content,
	search_text,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, to_tsvector('simple', $7), $8, $9)
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
SET state = $2,
	content = CASE WHEN $2 = 'deleted' THEN '' ELSE content END,
	metadata = CASE WHEN $2 = 'deleted' THEN '{}'::jsonb ELSE metadata END,
	search_text = CASE WHEN $2 = 'deleted' THEN NULL ELSE search_text END,
	embedding = CASE WHEN $2 = 'deleted' THEN NULL ELSE embedding END,
	updated_at = $3
WHERE id = $1
RETURNING id, tenant, project, namespace, class, state, content, created_at, updated_at
`

	var canonical memory.CanonicalMemory
	if err := tx.QueryRow(ctx, query, action.MemoryID, action.TargetState(), action.AppliedAt).Scan(
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
	id,
	tenant,
	project,
	namespace,
	class,
	state,
	content,
	created_at,
	updated_at,
	GREATEST(0, 1 - (embedding <=> $4::vector)) AS semantic_score
FROM canonical_memories
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND state NOT IN ('suppressed', 'forgotten', 'deleted')
	AND embedding IS NOT NULL
	AND class IN ('profile', 'episodic', 'procedural', 'summary')
	AND ($5::timestamptz IS NULL OR updated_at >= $5)
	AND ($6::timestamptz IS NULL OR updated_at <= $6)
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
		action.Content,
		action.AppliedAt,
	); err != nil {
		return fmt.Errorf("insert deletion marker: %w", err)
	}

	return nil
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
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`

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

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}

	return value
}

func vectorLiteral(values []float32) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}

	return "[" + strings.Join(parts, ",") + "]"
}
