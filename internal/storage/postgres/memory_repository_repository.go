package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	memoryRepositoryImportWorkMaxAttempts = 5
	memoryRepositoryImportWorkRetention   = 30 * 24 * time.Hour
)

// RegisterMemoryRepository persists only a scoped source identity and opaque
// display labels. It never receives a filesystem path, archive, or transport.
func (r *Repository) RegisterMemoryRepository(ctx context.Context, record memory.MemoryRepositoryRecord) (memory.MemoryRepositoryRecord, error) {
	if strings.TrimSpace(record.ID) == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if err := record.Validate(); err != nil {
		return memory.MemoryRepositoryRecord{}, err
	}
	const query = `
INSERT INTO memory_repositories (id,tenant,project,namespace,name,source_url,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id,tenant,project,namespace,name,source_url,generation,created_at`
	return scanMemoryRepository(r.db.QueryRow(ctx, query, record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Name, record.SourceURL, record.CreatedAt.UTC()))
}

// ReadMemoryRepository never performs an unscoped repository lookup: callers
// must already hold an exact scope and a foreign id is indistinguishable from
// an absent resource to the storage boundary.
func (r *Repository) ReadMemoryRepository(ctx context.Context, scope memory.Scope, repositoryID string) (memory.MemoryRepositoryRecord, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryRepositoryRecord{}, err
	}
	if strings.TrimSpace(repositoryID) == "" {
		return memory.MemoryRepositoryRecord{}, fmt.Errorf("memory repository id is required")
	}
	const query = `
SELECT id,tenant,project,namespace,name,source_url,generation,created_at
FROM memory_repositories
WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4`
	return scanMemoryRepository(r.db.QueryRow(ctx, query, repositoryID, scope.Tenant, scope.Project, scope.Namespace))
}

func scanMemoryRepository(row interface{ Scan(...any) error }) (memory.MemoryRepositoryRecord, error) {
	var record memory.MemoryRepositoryRecord
	if err := row.Scan(&record.ID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace, &record.Name, &record.SourceURL, &record.Generation, &record.CreatedAt); err != nil {
		return memory.MemoryRepositoryRecord{}, err
	}
	return record, nil
}

// SaveMemoryRepositoryPreview persists an immutable reviewed plan. The JSON
// payload is evidence only; callers must still recheck expiry and scope before
// accepting an import.
func (r *Repository) SaveMemoryRepositoryPreview(ctx context.Context, preview memory.MemoryRepositoryPreview) (memory.MemoryRepositoryPreview, error) {
	if strings.TrimSpace(preview.ID) == "" {
		preview.ID = uuid.NewString()
	}
	if preview.CreatedAt.IsZero() {
		preview.CreatedAt = time.Now().UTC()
	}
	if err := preview.Scope.Validate(); err != nil || strings.TrimSpace(preview.RepositoryID) == "" || preview.Generation < 0 || !memoryRepositoryDigest(preview.SnapshotDigest) || !memoryRepositoryDigest(preview.PlanDigest) || !memoryRepositoryDigest(preview.WarningDigest) || preview.ParserVersion != memory.MemoryRepositoryParserVersion || preview.ExpiresAt.IsZero() {
		return memory.MemoryRepositoryPreview{}, fmt.Errorf("memory repository preview is invalid")
	}
	payload, err := json.Marshal(preview)
	if err != nil {
		return memory.MemoryRepositoryPreview{}, fmt.Errorf("marshal memory repository preview: %w", err)
	}
	const query = `
INSERT INTO memory_repository_previews (id,repository_id,tenant,project,namespace,generation,snapshot_digest,plan_digest,warning_digest,parser_version,payload,expires_at,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
RETURNING id,repository_id,tenant,project,namespace,generation,snapshot_digest,plan_digest,warning_digest,parser_version,payload,expires_at,created_at`
	return scanMemoryRepositoryPreview(r.db.QueryRow(ctx, query, preview.ID, preview.RepositoryID, preview.Scope.Tenant, preview.Scope.Project, preview.Scope.Namespace, preview.Generation, preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, preview.ParserVersion, payload, preview.ExpiresAt.UTC(), preview.CreatedAt.UTC()))
}

func scanMemoryRepositoryPreview(row interface{ Scan(...any) error }) (memory.MemoryRepositoryPreview, error) {
	var preview memory.MemoryRepositoryPreview
	var payload []byte
	if err := row.Scan(&preview.ID, &preview.RepositoryID, &preview.Scope.Tenant, &preview.Scope.Project, &preview.Scope.Namespace, &preview.Generation, &preview.SnapshotDigest, &preview.PlanDigest, &preview.WarningDigest, &preview.ParserVersion, &payload, &preview.ExpiresAt, &preview.CreatedAt); err != nil {
		return memory.MemoryRepositoryPreview{}, err
	}
	if len(payload) > 0 {
		var evidence struct {
			Revision        string
			Actor           string
			Reason          string
			EffectiveSource memory.SourceMetadata
			Entries         []memory.MemoryRepositoryPreviewEntry
			Navigation      []memory.MemoryRepositoryNavigation
			Diagnostics     []memory.MemoryRepositoryDiagnostic
		}
		if err := json.Unmarshal(payload, &evidence); err != nil {
			return memory.MemoryRepositoryPreview{}, fmt.Errorf("decode memory repository preview: %w", err)
		}
		preview.Revision, preview.Actor, preview.Reason = evidence.Revision, evidence.Actor, evidence.Reason
		preview.EffectiveSource, preview.Entries, preview.Navigation, preview.Diagnostics = evidence.EffectiveSource, evidence.Entries, evidence.Navigation, evidence.Diagnostics
	}
	return preview, nil
}

// ReadMemoryRepositoryPreview is deliberately exact-scope. It cannot become a
// cross-scope payload-disclosure oracle even though previews contain source
// evidence that is more sensitive than ordinary registration labels.
func (r *Repository) ReadMemoryRepositoryPreview(ctx context.Context, scope memory.Scope, repositoryID, previewID string) (memory.MemoryRepositoryPreview, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryRepositoryPreview{}, err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(previewID) == "" {
		return memory.MemoryRepositoryPreview{}, fmt.Errorf("memory repository and preview id are required")
	}
	const query = `
SELECT id,repository_id,tenant,project,namespace,generation,snapshot_digest,plan_digest,warning_digest,parser_version,payload,expires_at,created_at
FROM memory_repository_previews
WHERE id=$1 AND repository_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5`
	return scanMemoryRepositoryPreview(r.db.QueryRow(ctx, query, previewID, repositoryID, scope.Tenant, scope.Project, scope.Namespace))
}

// AcceptMemoryRepositoryImport persists the reviewed plan before returning an
// accepted import. It never creates canonical memory: the worker later turns
// each durable entry into a governed remember intent.
func (r *Repository) AcceptMemoryRepositoryImport(ctx context.Context, request memory.MemoryRepositoryImportRequest, preview memory.MemoryRepositoryPreview) (memory.MemoryRepositoryImportRecord, bool, error) {
	if err := request.Validate(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, err
	}
	if preview.RepositoryID != request.RepositoryID || preview.Scope.Normalized() != request.Scope.Normalized() || preview.ID != request.PreviewID || preview.Generation != request.ExpectedGeneration || preview.PlanDigest != request.PlanDigest || preview.WarningDigest != request.WarningDigest {
		return memory.MemoryRepositoryImportRecord{}, false, memory.ErrMemoryRepositoryIdempotencyConflict
	}
	fingerprint := repositoryImportFingerprint(request)
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("begin memory repository import transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const replayQuery = `
SELECT id,generation,request_fingerprint,status,created_at,completed_at
FROM memory_repository_imports
WHERE repository_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND idempotency_key=$5
FOR UPDATE`
	var replay memory.MemoryRepositoryImportRecord
	var completed *time.Time
	err = tx.QueryRow(ctx, replayQuery, request.RepositoryID, request.Scope.Tenant, request.Scope.Project, request.Scope.Namespace, request.IdempotencyKey).Scan(&replay.ID, &replay.Generation, &replay.RequestFingerprint, &replay.Status, &replay.CreatedAt, &completed)
	if err == nil {
		if replay.RequestFingerprint != fingerprint {
			return memory.MemoryRepositoryImportRecord{}, false, memory.ErrMemoryRepositoryIdempotencyConflict
		}
		replay.RepositoryID, replay.Scope, replay.PreviewID, replay.IdempotencyKey = request.RepositoryID, request.Scope.Normalized(), request.PreviewID, request.IdempotencyKey
		if completed != nil {
			replay.CompletedAt = *completed
		}
		if err := tx.Commit(ctx); err != nil {
			return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("commit import replay: %w", err)
		}
		return replay, true, nil
	}
	if err != pgx.ErrNoRows {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("read memory repository import replay: %w", err)
	}

	var generation int64
	const lockQuery = `SELECT generation FROM memory_repositories WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 FOR UPDATE`
	if err := tx.QueryRow(ctx, lockQuery, request.RepositoryID, request.Scope.Tenant, request.Scope.Project, request.Scope.Namespace).Scan(&generation); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("lock memory repository: %w", err)
	}
	if generation != request.ExpectedGeneration {
		return memory.MemoryRepositoryImportRecord{}, false, memory.ErrMemoryRepositoryStaleGeneration
	}
	const revisionQuery = `
INSERT INTO memory_repository_revisions (repository_id,tenant,project,namespace,claimed_revision,snapshot_digest)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (repository_id,claimed_revision) DO NOTHING
RETURNING snapshot_digest`
	var recordedDigest string
	err = tx.QueryRow(ctx, revisionQuery, request.RepositoryID, request.Scope.Tenant, request.Scope.Project, request.Scope.Namespace, preview.Revision, preview.SnapshotDigest).Scan(&recordedDigest)
	if err == pgx.ErrNoRows {
		const existingRevision = `SELECT snapshot_digest FROM memory_repository_revisions WHERE repository_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND claimed_revision=$5`
		if err := tx.QueryRow(ctx, existingRevision, request.RepositoryID, request.Scope.Tenant, request.Scope.Project, request.Scope.Namespace, preview.Revision).Scan(&recordedDigest); err != nil {
			return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("read memory repository revision: %w", err)
		}
	}
	if err != nil && err != pgx.ErrNoRows {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("bind memory repository revision: %w", err)
	}
	if recordedDigest != preview.SnapshotDigest {
		return memory.MemoryRepositoryImportRecord{}, false, memory.ErrMemoryRepositoryRevisionConflict
	}

	planPayload, err := json.Marshal(preview)
	if err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("marshal memory repository import plan: %w", err)
	}
	result := memory.MemoryRepositoryImportRecord{ID: uuid.NewString(), RepositoryID: request.RepositoryID, Scope: request.Scope.Normalized(), PreviewID: request.PreviewID, Generation: generation + 1, IdempotencyKey: request.IdempotencyKey, RequestFingerprint: fingerprint, Status: memory.MemoryRepositoryImportQueued, CreatedAt: time.Now().UTC()}
	const importQuery = `
INSERT INTO memory_repository_imports (id,repository_id,tenant,project,namespace,preview_id,generation,idempotency_key,request_fingerprint,snapshot_digest,plan_digest,warning_digest,plan_payload,status,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`
	if _, err := tx.Exec(ctx, importQuery, result.ID, result.RepositoryID, result.Scope.Tenant, result.Scope.Project, result.Scope.Namespace, result.PreviewID, result.Generation, result.IdempotencyKey, result.RequestFingerprint, preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, planPayload, string(result.Status), result.CreatedAt); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("persist memory repository import: %w", err)
	}
	for _, entry := range preview.Entries {
		status := memory.MemoryRepositoryEntryQueued
		const tombstoneQuery = `
INSERT INTO memory_repository_entry_tombstones (repository_id,tenant,project,namespace,entry_key)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (repository_id,entry_key) DO NOTHING`
		tag, err := tx.Exec(ctx, tombstoneQuery, result.RepositoryID, result.Scope.Tenant, result.Scope.Project, result.Scope.Namespace, entry.EntryKey)
		if err != nil {
			return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("persist memory repository entry identity: %w", err)
		}
		if tag.RowsAffected() == 0 {
			status = memory.MemoryRepositoryEntryDuplicate
		}
		entryPayload, err := json.Marshal(entry)
		if err != nil {
			return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("marshal memory repository import entry: %w", err)
		}
		const entryQuery = `INSERT INTO memory_repository_import_entries (import_id,entry_key,dispatch_status,payload) VALUES ($1,$2,$3,$4)`
		if _, err := tx.Exec(ctx, entryQuery, result.ID, entry.EntryKey, string(status), entryPayload); err != nil {
			return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("persist memory repository import entry: %w", err)
		}
		if status == memory.MemoryRepositoryEntryDuplicate {
			const duplicateAttemptQuery = `
INSERT INTO memory_repository_import_attempts (id,import_id,entry_key,sequence,outcome)
VALUES ($1,$2,$3,1,$4)`
			if _, err := tx.Exec(ctx, duplicateAttemptQuery, uuid.NewString(), result.ID, entry.EntryKey, string(status)); err != nil {
				return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("record duplicate memory repository import entry: %w", err)
			}
		}
		result.Entries = append(result.Entries, memory.MemoryRepositoryImportEntry{EntryKey: entry.EntryKey, DispatchStatus: status, Payload: entry})
	}
	workInput := workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: result.Scope, Kind: workqueue.WorkKindMemoryRepositoryImport, Watermark: preview.PlanDigest, Idempotency: result.ID, Reference: result.RepositoryID + "|" + result.ID},
		MaxAttempts:      memoryRepositoryImportWorkMaxAttempts,
		Now:              result.CreatedAt,
		DetailExpiresAt:  result.CreatedAt.Add(memoryRepositoryImportWorkRetention),
	}
	if err := workInput.Validate(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("build memory repository import work: %w", err)
	}
	const workQuery = `
INSERT INTO derived_work_items (
    work_key, kind, tenant, project, namespace, watermark, idempotency_key,
    reference, state, max_attempts, next_attempt_at, detail_expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'queued',$9,$10,$11)
ON CONFLICT (work_key) DO UPDATE SET updated_at = GREATEST(derived_work_items.updated_at, EXCLUDED.updated_at)`
	if _, err := tx.Exec(ctx, workQuery, workInput.WorkKey(), string(workInput.Kind), result.Scope.Tenant, result.Scope.Project, result.Scope.Namespace, workInput.Watermark, workInput.Idempotency, workInput.Reference, workInput.MaxAttempts, workInput.Now, workInput.DetailExpiresAt); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("persist memory repository import work: %w", err)
	}
	const generationQuery = `UPDATE memory_repositories SET generation=$5 WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4`
	if _, err := tx.Exec(ctx, generationQuery, result.RepositoryID, result.Scope.Tenant, result.Scope.Project, result.Scope.Namespace, result.Generation); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("advance memory repository generation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.MemoryRepositoryImportRecord{}, false, fmt.Errorf("commit memory repository import: %w", err)
	}
	return result, false, nil
}

func repositoryImportFingerprint(request memory.MemoryRepositoryImportRequest) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{request.RepositoryID, request.PreviewID, request.PlanDigest, request.WarningDigest, fmt.Sprintf("%d", request.ExpectedGeneration)}, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

// ReadMemoryRepositoryImport exposes only durable dispatch bookkeeping. The
// content-bearing preview and per-entry plan payloads remain retained evidence,
// not a status-response channel.
func (r *Repository) ReadMemoryRepositoryImport(ctx context.Context, scope memory.Scope, repositoryID, importID string) (memory.MemoryRepositoryImportRecord, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" {
		return memory.MemoryRepositoryImportRecord{}, fmt.Errorf("memory repository and import id are required")
	}
	const importQuery = `
SELECT id,repository_id,tenant,project,namespace,preview_id,generation,idempotency_key,request_fingerprint,status,created_at,completed_at
FROM memory_repository_imports
WHERE id=$1 AND repository_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5`
	var record memory.MemoryRepositoryImportRecord
	var completed *time.Time
	if err := r.db.QueryRow(ctx, importQuery, importID, repositoryID, scope.Tenant, scope.Project, scope.Namespace).Scan(
		&record.ID, &record.RepositoryID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace,
		&record.PreviewID, &record.Generation, &record.IdempotencyKey, &record.RequestFingerprint, &record.Status,
		&record.CreatedAt, &completed,
	); err != nil {
		return memory.MemoryRepositoryImportRecord{}, err
	}
	if completed != nil {
		record.CompletedAt = *completed
	}
	const entriesQuery = `
SELECT entry_key,dispatch_status,intent_id
FROM memory_repository_import_entries
WHERE import_id=$1
ORDER BY entry_key
LIMIT 1000`
	rows, err := r.db.Query(ctx, entriesQuery, record.ID)
	if err != nil {
		return memory.MemoryRepositoryImportRecord{}, fmt.Errorf("read memory repository import entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry memory.MemoryRepositoryImportEntry
		var intentID sql.NullString
		if err := rows.Scan(&entry.EntryKey, &entry.DispatchStatus, &intentID); err != nil {
			return memory.MemoryRepositoryImportRecord{}, err
		}
		if intentID.Valid {
			entry.IntentID = intentID.String
		}
		record.Entries = append(record.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, err
	}
	return record, nil
}

// ReadMemoryRepositoryImportPlan is a worker-only exact-scope read of the
// immutable accepted plan. Ordinary inspection deliberately never uses this
// method, because the plan contains retained source evidence.
func (r *Repository) ReadMemoryRepositoryImportPlan(ctx context.Context, scope memory.Scope, repositoryID, importID string) (memory.MemoryRepositoryImportRecord, memory.MemoryRepositoryPreview, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, fmt.Errorf("memory repository and import id are required")
	}
	const query = `
SELECT id,repository_id,tenant,project,namespace,preview_id,generation,idempotency_key,request_fingerprint,status,created_at,completed_at,plan_payload
FROM memory_repository_imports
WHERE id=$1 AND repository_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5`
	var record memory.MemoryRepositoryImportRecord
	var completed *time.Time
	var payload []byte
	if err := r.db.QueryRow(ctx, query, importID, repositoryID, scope.Tenant, scope.Project, scope.Namespace).Scan(
		&record.ID, &record.RepositoryID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace,
		&record.PreviewID, &record.Generation, &record.IdempotencyKey, &record.RequestFingerprint, &record.Status,
		&record.CreatedAt, &completed, &payload,
	); err != nil {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, err
	}
	if completed != nil {
		record.CompletedAt = *completed
	}
	var preview memory.MemoryRepositoryPreview
	if err := json.Unmarshal(payload, &preview); err != nil {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, fmt.Errorf("decode memory repository import plan: %w", err)
	}
	if preview.Scope.Normalized() != scope.Normalized() || preview.RepositoryID != repositoryID || preview.ID != record.PreviewID || !memoryRepositoryDigest(preview.PlanDigest) || preview.ParserVersion != memory.MemoryRepositoryParserVersion {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, fmt.Errorf("memory repository import plan is invalid")
	}
	const entriesQuery = `
SELECT entry_key,dispatch_status,intent_id
FROM memory_repository_import_entries
WHERE import_id=$1
ORDER BY entry_key
LIMIT 1000`
	rows, err := r.db.Query(ctx, entriesQuery, record.ID)
	if err != nil {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, fmt.Errorf("read memory repository import plan entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry memory.MemoryRepositoryImportEntry
		var intentID sql.NullString
		if err := rows.Scan(&entry.EntryKey, &entry.DispatchStatus, &intentID); err != nil {
			return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, err
		}
		if intentID.Valid {
			entry.IntentID = intentID.String
		}
		record.Entries = append(record.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return memory.MemoryRepositoryImportRecord{}, memory.MemoryRepositoryPreview{}, err
	}
	return record, preview, nil
}

func (r *Repository) MarkMemoryRepositoryImportRunning(ctx context.Context, scope memory.Scope, repositoryID, importID string) error {
	if err := scope.Validate(); err != nil || strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" {
		return fmt.Errorf("memory repository import running transition is invalid")
	}
	const query = `UPDATE memory_repository_imports SET status='running' WHERE id=$1 AND repository_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5 AND status='queued'`
	if _, err := r.db.Exec(ctx, query, importID, repositoryID, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		return fmt.Errorf("mark memory repository import running: %w", err)
	}
	return nil
}

// RecordMemoryRepositoryImportEntryAttempt appends a bounded worker outcome
// before updating its current status. Both statements bind the repository and
// exact scope so an import id cannot cross an isolation boundary.
func (r *Repository) RecordMemoryRepositoryImportEntryAttempt(ctx context.Context, scope memory.Scope, repositoryID, importID string, entry memory.MemoryRepositoryImportEntry) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" || !memoryRepositoryDigest(entry.EntryKey) || !entry.DispatchStatus.Valid() {
		return fmt.Errorf("memory repository import entry attempt is invalid")
	}
	const attemptQuery = `
INSERT INTO memory_repository_import_attempts (id,import_id,entry_key,sequence,outcome,intent_id)
SELECT $1,i.id,$3,COALESCE((SELECT MAX(a.sequence)+1 FROM memory_repository_import_attempts a WHERE a.import_id=i.id AND a.entry_key=$3),1),$4,NULLIF($5,'')::uuid
FROM memory_repository_imports i
WHERE i.id=$2 AND i.repository_id=$6 AND i.tenant=$7 AND i.project=$8 AND i.namespace=$9`
	tag, err := r.db.Exec(ctx, attemptQuery, uuid.NewString(), importID, entry.EntryKey, string(entry.DispatchStatus), entry.IntentID, repositoryID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return fmt.Errorf("append memory repository import attempt: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("memory repository import is unavailable in scope")
	}
	const entryQuery = `
UPDATE memory_repository_import_entries e
SET dispatch_status=$1,intent_id=NULLIF($2,'')::uuid
FROM memory_repository_imports i
WHERE e.import_id=i.id AND i.id=$3 AND e.entry_key=$4 AND i.repository_id=$5
  AND i.tenant=$6 AND i.project=$7 AND i.namespace=$8`
	tag, err = r.db.Exec(ctx, entryQuery, string(entry.DispatchStatus), entry.IntentID, importID, entry.EntryKey, repositoryID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return fmt.Errorf("update memory repository import entry: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("memory repository import entry is unavailable in scope")
	}
	return nil
}

// CompleteMemoryRepositoryImport marks dispatch complete only when every
// entry has a terminal outcome. Completion is distinct from later candidate
// activation and retains no source payload in its public record.
func (r *Repository) CompleteMemoryRepositoryImport(ctx context.Context, scope memory.Scope, repositoryID, importID string, completedAt time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" || completedAt.IsZero() {
		return fmt.Errorf("memory repository import completion is invalid")
	}
	const query = `
UPDATE memory_repository_imports i
SET status='completed',completed_at=$1
WHERE i.id=$2 AND i.repository_id=$3 AND i.tenant=$4 AND i.project=$5 AND i.namespace=$6
  AND NOT EXISTS (
    SELECT 1 FROM memory_repository_import_entries e
    WHERE e.import_id=i.id AND e.dispatch_status='queued'
  )`
	tag, err := r.db.Exec(ctx, query, completedAt.UTC(), importID, repositoryID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return fmt.Errorf("complete memory repository import: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("memory repository import cannot be completed")
	}
	return nil
}

// PruneExpiredMemoryRepositoryPreviews removes only unaccepted preview
// evidence in bounded batches. Accepted imports retain their immutable plan
// payload and deduplication history for recovery.
func (r *Repository) PruneExpiredMemoryRepositoryPreviews(ctx context.Context, before time.Time, limit int) (int, error) {
	if before.IsZero() || limit <= 0 || limit > 1000 {
		return 0, fmt.Errorf("memory repository preview prune input is invalid")
	}
	const query = `
WITH expired AS (
    SELECT p.id
    FROM memory_repository_previews p
    WHERE p.expires_at <= $1
      AND NOT EXISTS (SELECT 1 FROM memory_repository_imports i WHERE i.preview_id=p.id)
    ORDER BY p.expires_at ASC, p.id ASC
    LIMIT $2
)
DELETE FROM memory_repository_previews p
USING expired e
WHERE p.id=e.id`
	tag, err := r.db.Exec(ctx, query, before.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("prune expired memory repository previews: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ReadMemoryRepositoryManifest returns a latest-import watermark. Canonical
// identity mapping is intentionally omitted until the import worker records
// provenance that can be joined with current lifecycle/disclosure policy; an
// empty set is safer than exposing retained source text or stale targets.
func (r *Repository) ReadMemoryRepositoryManifest(ctx context.Context, scope memory.Scope, repositoryID string) (memory.MemoryRepositoryManifest, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryRepositoryManifest{}, err
	}
	if strings.TrimSpace(repositoryID) == "" {
		return memory.MemoryRepositoryManifest{}, fmt.Errorf("memory repository id is required")
	}
	const query = `
SELECT generation,status
FROM memory_repository_imports
WHERE repository_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4
ORDER BY generation DESC
LIMIT 1`
	manifest := memory.MemoryRepositoryManifest{RepositoryID: repositoryID, Items: []memory.MemoryRepositoryManifestItem{}}
	if err := r.db.QueryRow(ctx, query, repositoryID, scope.Tenant, scope.Project, scope.Namespace).Scan(&manifest.Generation, &manifest.ProcessingStatus); err != nil {
		return memory.MemoryRepositoryManifest{}, err
	}
	const itemsQuery = `
SELECT c.id::text,v.version,e.payload->>'Path'
FROM memory_repository_imports i
JOIN memory_repository_import_entries e ON e.import_id=i.id
LEFT JOIN LATERAL (
    SELECT prior.intent_id
    FROM memory_repository_imports prior_import
    JOIN memory_repository_import_entries prior ON prior.import_id=prior_import.id
    WHERE prior_import.repository_id=i.repository_id
      AND prior_import.tenant=i.tenant AND prior_import.project=i.project AND prior_import.namespace=i.namespace
      AND prior_import.generation < i.generation
      AND prior.entry_key=e.entry_key AND prior.intent_id IS NOT NULL
    ORDER BY prior_import.generation DESC
    LIMIT 1
) prior ON true
JOIN LATERAL (
    SELECT t.outcome_reference FROM memory_intent_transitions t
    WHERE t.intent_id::text=COALESCE(e.intent_id,prior.intent_id)::text AND t.tenant=i.tenant AND t.project=i.project AND t.namespace=i.namespace
      AND t.to_status='active' AND t.outcome_reference IS NOT NULL
    ORDER BY t.sequence DESC LIMIT 1
) outcome ON true
JOIN canonical_memories c ON c.id::text=outcome.outcome_reference AND c.tenant=i.tenant AND c.project=i.project AND c.namespace=i.namespace AND c.state='active'
JOIN LATERAL (
    SELECT version FROM memory_versions WHERE memory_id=c.id AND state='active' ORDER BY version DESC LIMIT 1
) v ON true
LEFT JOIN LATERAL (
    SELECT n.ordinality::int AS navigation_order
    FROM jsonb_array_elements(i.plan_payload->'Navigation') WITH ORDINALITY n(value, ordinality)
    WHERE n.value->>'Path'='MEMORY.md'
      AND n.value->>'Target'=e.payload->>'Path'
    ORDER BY n.ordinality ASC
    LIMIT 1
) nav ON true
WHERE i.repository_id=$1 AND i.tenant=$2 AND i.project=$3 AND i.namespace=$4 AND i.generation=$5
  AND e.dispatch_status IN ('submitted','duplicate')
	  AND c.source_metadata->>'version'='source-metadata-v1'
	  AND c.source_metadata->>'classification' IN ('low','medium')
ORDER BY nav.navigation_order NULLS LAST, e.entry_key,c.id
LIMIT 100`
	rows, err := r.db.Query(ctx, itemsQuery, repositoryID, scope.Tenant, scope.Project, scope.Namespace, manifest.Generation)
	if err != nil {
		return memory.MemoryRepositoryManifest{}, fmt.Errorf("read memory repository manifest items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item memory.MemoryRepositoryManifestItem
		if err := rows.Scan(&item.MemoryID, &item.Version, &item.SourcePath); err != nil {
			return memory.MemoryRepositoryManifest{}, err
		}
		manifest.Items = append(manifest.Items, item)
	}
	if err := rows.Err(); err != nil {
		return memory.MemoryRepositoryManifest{}, err
	}
	return manifest, nil
}

func memoryRepositoryDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
