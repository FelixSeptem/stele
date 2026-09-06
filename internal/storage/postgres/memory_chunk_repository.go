package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// ChunkAdjacentOptions bounds evidence expansion. SessionID and UserID, when
// supplied, are assertions rather than filters: a mismatch fails closed.
// ChunkAdjacentOptions is retained as an alias for callers that historically
// imported the storage package; the neutral contract lives in memory.
type ChunkAdjacentOptions = memory.ChunkAdjacentOptions

// MemoryChunkWatermark is the durable, non-content identity used by an
// operator or a rebuild process to decide whether a source has already been
// materialized under a particular policy and renderer.
type MemoryChunkWatermark struct {
	DerivationID        string
	Source              memory.ChunkSourceReference
	PolicyVersion       string
	RendererVersion     string
	CounterVersion      string
	SourceWatermarkHash string
	CreatedAt           time.Time
}

// CreateMemoryChunk records a single chunk through the same append-only,
// idempotent derivation boundary as batch materialization. counterVersion is
// part of that identity because a changed counter may change bounded output.
func (r *Repository) CreateMemoryChunk(ctx context.Context, chunk memory.MemoryChunk, counterVersion string) (memory.MemoryChunk, error) {
	chunks, err := r.CreateMemoryChunks(ctx, []memory.MemoryChunk{chunk}, counterVersion)
	if err != nil {
		return memory.MemoryChunk{}, err
	}
	return chunks[0], nil
}

// CreateMemoryChunks persists chunks as immutable items. Replaying an
// identical source/policy/renderer/ordinal converges; it never updates an
// earlier derivation or item.
func (r *Repository) CreateMemoryChunks(ctx context.Context, chunks []memory.MemoryChunk, counterVersion string) ([]memory.MemoryChunk, error) {
	if len(chunks) == 0 {
		return []memory.MemoryChunk{}, nil
	}
	if strings.TrimSpace(counterVersion) == "" {
		return nil, fmt.Errorf("chunk counter version is required")
	}
	for _, chunk := range chunks {
		if err := chunk.Validate(); err != nil {
			return nil, fmt.Errorf("validate memory chunk: %w", err)
		}
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin create memory chunks transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i := range chunks {
		chunk := &chunks[i]
		if chunk.CreatedAt.IsZero() {
			chunk.CreatedAt = time.Now().UTC()
		}
		derivationID, watermarkHash, contentHash, watermark, err := memoryChunkDerivationIdentity(*chunk, counterVersion)
		if err != nil {
			return nil, err
		}
		if err := verifyMemoryChunkSource(ctx, tx, *chunk); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO memory_chunk_derivations
 (id,tenant,project,namespace,source_kind,source_id,source_version,parent_memory_id,source_session_id,source_user_id,source_watermark,source_watermark_hash,source_content_hash,policy_version,renderer_version,counter_version,lifecycle_state,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,$13,$14,$15,$16,'active',$17)
ON CONFLICT (tenant,project,namespace,source_kind,source_id,source_version,policy_version,renderer_version,counter_version,source_content_hash) DO NOTHING`,
			derivationID, chunk.Scope.Tenant, chunk.Scope.Project, chunk.Scope.Namespace,
			string(chunk.Source.Kind), chunk.Source.ID, chunk.Source.Version, chunk.Source.MemoryID,
			chunk.Source.SessionID, chunk.Source.UserID, watermark, watermarkHash, contentHash,
			chunk.PolicyVersion, chunk.RendererVersion, counterVersion, chunk.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert memory chunk derivation: %w", err)
		}

		itemContentHash := memoryChunkContentHash(chunk.Content)
		if _, err := tx.Exec(ctx, `
INSERT INTO memory_chunk_items
 (id,derivation_id,tenant,project,namespace,source_session_id,source_user_id,ordinal,class,lifecycle_state,content,source_start,source_end,character_count,token_count,content_hash,created_at)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,'active',$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT (derivation_id, ordinal) DO NOTHING`,
			chunk.ID, derivationID, chunk.Scope.Tenant, chunk.Scope.Project, chunk.Scope.Namespace,
			chunk.Source.SessionID, chunk.Source.UserID, chunk.Ordinal, string(chunk.Class), chunk.Content,
			chunk.SourceRange.Start, chunk.SourceRange.End, chunk.CharacterCount, chunk.TokenCount, itemContentHash, chunk.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert memory chunk item: %w", err)
		}
		// ON CONFLICT converges retries to the first immutable item. Return that
		// durable identity to callers instead of the retry's in-memory value.
		var persistedID string
		if err := tx.QueryRow(ctx, `SELECT id FROM memory_chunk_items WHERE derivation_id=$1 AND ordinal=$2`, derivationID, chunk.Ordinal).Scan(&persistedID); err != nil {
			return nil, fmt.Errorf("read idempotent memory chunk item: %w", err)
		}
		chunk.ID = persistedID
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create memory chunks: %w", err)
	}
	return chunks, nil
}

func verifyMemoryChunkSource(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, chunk memory.MemoryChunk) error {
	switch chunk.Source.Kind {
	case memory.ChunkSourceKindRawEvent:
		var session string
		err := q.QueryRow(ctx, `SELECT COALESCE(memory_session_id,'') FROM raw_events WHERE id::text=$1 AND tenant=$2 AND project=$3 AND namespace=$4`, chunk.Source.ID, chunk.Scope.Tenant, chunk.Scope.Project, chunk.Scope.Namespace).Scan(&session)
		if err != nil {
			return fmt.Errorf("verify memory chunk raw source: %w", err)
		}
		if chunk.Source.SessionID != "" && session != chunk.Source.SessionID {
			return fmt.Errorf("memory chunk source session mismatch")
		}
	case memory.ChunkSourceKindCanonicalVersion:
		var state, memoryState string
		err := q.QueryRow(ctx, `SELECT mv.state, cm.state FROM memory_versions mv JOIN canonical_memories cm ON cm.id=mv.memory_id WHERE mv.id::text=$1 AND mv.memory_id::text=$2 AND cm.tenant=$3 AND cm.project=$4 AND cm.namespace=$5`, chunk.Source.ID, chunk.Source.MemoryID, chunk.Scope.Tenant, chunk.Scope.Project, chunk.Scope.Namespace).Scan(&state, &memoryState)
		if err != nil {
			return fmt.Errorf("verify memory chunk canonical source: %w", err)
		}
		if state != string(memory.MemoryStateActive) || memoryState != string(memory.MemoryStateActive) {
			return fmt.Errorf("memory chunk canonical source is not visible")
		}
	default:
		return fmt.Errorf("invalid memory chunk source kind %q", chunk.Source.Kind)
	}
	return nil
}

// ReadMemoryChunk reads only a chunk whose parent source remains visible in
// precisely the requested scope. In particular, stale chunk lifecycle
// snapshots never bypass a later canonical lifecycle transition.
func (r *Repository) ReadMemoryChunk(ctx context.Context, scope memory.Scope, chunkID string) (memory.MemoryChunk, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryChunk{}, err
	}
	if strings.TrimSpace(chunkID) == "" {
		return memory.MemoryChunk{}, fmt.Errorf("memory chunk id is required")
	}
	return scanVisibleMemoryChunk(r.db.QueryRow(ctx, visibleMemoryChunkSelect+` WHERE i.id=$1 AND i.tenant=$2 AND i.project=$3 AND i.namespace=$4 AND `+visibleChunkSourcePredicate, chunkID, scope.Tenant, scope.Project, scope.Namespace))
}

// ListMemoryChunksBySource returns ordinary visible chunks in exact scope.
func (r *Repository) ListMemoryChunksBySource(ctx context.Context, scope memory.Scope, source memory.ChunkSourceReference) ([]memory.MemoryChunk, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if source.Scope.Normalized() != scope.Normalized() || source.Kind == "" || strings.TrimSpace(source.ID) == "" || source.Version <= 0 {
		return nil, fmt.Errorf("chunk source must be exact scoped and versioned")
	}
	rows, err := r.db.Query(ctx, visibleMemoryChunkSelect+` WHERE d.tenant=$1 AND d.project=$2 AND d.namespace=$3 AND d.source_kind=$4 AND d.source_id=$5 AND d.source_version=$6 AND `+visibleChunkSourcePredicate+` ORDER BY i.ordinal ASC, i.id ASC`, scope.Tenant, scope.Project, scope.Namespace, string(source.Kind), source.ID, source.Version)
	if err != nil {
		return nil, fmt.Errorf("list memory chunks by source: %w", err)
	}
	defer rows.Close()
	return scanVisibleMemoryChunkRows(rows)
}

// ReadMemoryChunkParent proves that the linked source is still visible before
// exposing the source identity to callers.
func (r *Repository) ReadMemoryChunkParent(ctx context.Context, scope memory.Scope, chunkID string) (memory.ChunkSourceReference, error) {
	chunk, err := r.ReadMemoryChunk(ctx, scope, chunkID)
	if err != nil {
		return memory.ChunkSourceReference{}, fmt.Errorf("read visible memory chunk parent: %w", err)
	}
	return chunk.Source, nil
}

// ListAdjacentMemoryChunks returns a bounded local window from the same
// derivation only. It validates session/user assertions before any content is
// returned and applies budgets while preserving ordinal order.
func (r *Repository) ListAdjacentMemoryChunks(ctx context.Context, scope memory.Scope, chunkID string, options ChunkAdjacentOptions) ([]memory.MemoryChunk, error) {
	target, err := r.ReadMemoryChunk(ctx, scope, chunkID)
	if err != nil {
		return nil, err
	}
	if options.SessionID != "" && options.SessionID != target.Source.SessionID {
		return nil, fmt.Errorf("adjacent chunk session does not match source")
	}
	if options.UserID != "" && options.UserID != target.Source.UserID {
		return nil, fmt.Errorf("adjacent chunk user does not match source")
	}
	before, after := options.Before, options.After
	if before < 0 || after < 0 {
		return nil, fmt.Errorf("adjacent chunk window cannot be negative")
	}
	if before == 0 && after == 0 {
		return []memory.MemoryChunk{}, nil
	}
	derivationID, err := r.visibleMemoryChunkDerivationID(ctx, scope, chunkID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, visibleMemoryChunkSelect+` WHERE d.id=$1 AND i.tenant=$2 AND i.project=$3 AND i.namespace=$4 AND i.id <> $5 AND i.ordinal >= $6 AND i.ordinal <= $7 AND `+visibleChunkSourcePredicate+` ORDER BY i.ordinal ASC, i.id ASC`, derivationID, scope.Tenant, scope.Project, scope.Namespace, chunkID, target.Ordinal-before, target.Ordinal+after)
	if err != nil {
		return nil, fmt.Errorf("list adjacent memory chunks: %w", err)
	}
	defer rows.Close()
	items, err := scanVisibleMemoryChunkRows(rows)
	if err != nil {
		return nil, err
	}
	maxCharacters, maxTokens := options.MaxCharacters, options.MaxTokens
	if maxCharacters <= 0 {
		maxCharacters = 4096
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	selected := make([]memory.MemoryChunk, 0, len(items))
	characters, tokens := 0, 0
	for _, item := range items {
		if characters+item.CharacterCount > maxCharacters || tokens+item.TokenCount > maxTokens {
			continue
		}
		selected = append(selected, item)
		characters += item.CharacterCount
		tokens += item.TokenCount
	}
	return selected, nil
}

func (r *Repository) ListMemoryChunkWatermarks(ctx context.Context, scope memory.Scope, source memory.ChunkSourceReference) ([]MemoryChunkWatermark, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if source.Scope.Normalized() != scope.Normalized() || !source.Kind.Valid() || strings.TrimSpace(source.ID) == "" || source.Version <= 0 {
		return nil, fmt.Errorf("chunk source must be exact scoped and valid")
	}
	rows, err := r.db.Query(ctx, `SELECT id,source_kind,source_id,source_version,COALESCE(parent_memory_id,''),COALESCE(source_session_id,''),COALESCE(source_user_id,''),policy_version,renderer_version,counter_version,source_watermark_hash,created_at FROM memory_chunk_derivations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND source_kind=$4 AND source_id=$5 AND source_version=$6 ORDER BY created_at ASC,id ASC`, scope.Tenant, scope.Project, scope.Namespace, string(source.Kind), source.ID, source.Version)
	if err != nil {
		return nil, fmt.Errorf("list memory chunk watermarks: %w", err)
	}
	defer rows.Close()
	result := make([]MemoryChunkWatermark, 0)
	for rows.Next() {
		var item MemoryChunkWatermark
		var kind string
		if err := rows.Scan(&item.DerivationID, &kind, &item.Source.ID, &item.Source.Version, &item.Source.MemoryID, &item.Source.SessionID, &item.Source.UserID, &item.PolicyVersion, &item.RendererVersion, &item.CounterVersion, &item.SourceWatermarkHash, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory chunk watermark: %w", err)
		}
		item.Source.Kind = memory.ChunkSourceKind(kind)
		item.Source.Scope = scope
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory chunk watermarks: %w", err)
	}
	return result, nil
}

func (r *Repository) visibleMemoryChunkDerivationID(ctx context.Context, scope memory.Scope, chunkID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT d.id FROM memory_chunk_items i JOIN memory_chunk_derivations d ON d.id=i.derivation_id WHERE i.id=$1 AND i.tenant=$2 AND i.project=$3 AND i.namespace=$4 AND `+visibleChunkSourcePredicate, chunkID, scope.Tenant, scope.Project, scope.Namespace).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("read visible memory chunk derivation: %w", err)
	}
	return id, nil
}

const visibleMemoryChunkSelect = `
SELECT i.id,i.tenant,i.project,i.namespace,d.source_kind,d.source_id,d.source_version,COALESCE(d.parent_memory_id,''),COALESCE(i.source_session_id,''),COALESCE(i.source_user_id,''),i.class,i.ordinal,i.content,i.source_start,i.source_end,i.character_count,i.token_count,i.lifecycle_state,d.policy_version,d.renderer_version,i.created_at
FROM memory_chunk_items i
JOIN memory_chunk_derivations d ON d.id=i.derivation_id`

// Both clauses retain lifecycle safety after materialization. Raw events have
// no mutable lifecycle in the base schema; canonical chunks must prove both
// canonical and version state remain active.
const visibleChunkSourcePredicate = `(
 (d.source_kind='raw_event' AND EXISTS (SELECT 1 FROM raw_events re WHERE re.id::text=d.source_id AND re.tenant=d.tenant AND re.project=d.project AND re.namespace=d.namespace))
 OR
 (d.source_kind='canonical_version' AND EXISTS (SELECT 1 FROM memory_versions mv JOIN canonical_memories cm ON cm.id=mv.memory_id WHERE mv.id::text=d.source_id AND mv.memory_id::text=d.parent_memory_id AND cm.tenant=d.tenant AND cm.project=d.project AND cm.namespace=d.namespace AND cm.state='active' AND mv.state='active'))
)`

type memoryChunkScanner interface{ Scan(...any) error }

func scanVisibleMemoryChunk(row memoryChunkScanner) (memory.MemoryChunk, error) {
	var chunk memory.MemoryChunk
	var kind, class, lifecycle string
	if err := row.Scan(&chunk.ID, &chunk.Scope.Tenant, &chunk.Scope.Project, &chunk.Scope.Namespace, &kind, &chunk.Source.ID, &chunk.Source.Version, &chunk.Source.MemoryID, &chunk.Source.SessionID, &chunk.Source.UserID, &class, &chunk.Ordinal, &chunk.Content, &chunk.SourceRange.Start, &chunk.SourceRange.End, &chunk.CharacterCount, &chunk.TokenCount, &lifecycle, &chunk.PolicyVersion, &chunk.RendererVersion, &chunk.CreatedAt); err != nil {
		return memory.MemoryChunk{}, err
	}
	chunk.Source.Kind = memory.ChunkSourceKind(kind)
	chunk.Source.Scope = chunk.Scope
	chunk.Source.LifecycleState = memory.MemoryStateActive
	chunk.Class = memory.MemoryClass(class)
	chunk.LifecycleState = memory.MemoryState(lifecycle)
	return chunk, nil
}

func scanVisibleMemoryChunkRows(rows pgx.Rows) ([]memory.MemoryChunk, error) {
	items := make([]memory.MemoryChunk, 0)
	for rows.Next() {
		item, err := scanVisibleMemoryChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("scan memory chunk: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory chunks: %w", err)
	}
	return items, nil
}

func memoryChunkDerivationIdentity(chunk memory.MemoryChunk, counter string) (id, watermarkHash, contentHash string, watermark []byte, err error) {
	identity := map[string]any{"source_kind": chunk.Source.Kind, "source_id": chunk.Source.ID, "source_version": chunk.Source.Version, "memory_id": chunk.Source.MemoryID, "session_id": chunk.Source.SessionID, "user_id": chunk.Source.UserID, "policy_version": chunk.PolicyVersion, "renderer_version": chunk.RendererVersion, "counter_version": counter}
	watermark, err = json.Marshal(identity)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("marshal memory chunk watermark: %w", err)
	}
	watermarkHash = memoryChunkContentHash(string(watermark))
	// An immutable source version is authoritative. This fingerprint identifies
	// the derivation without deriving a second source payload store.
	contentHash = watermarkHash
	id = "chunk-derivation-" + watermarkHash
	return id, watermarkHash, contentHash, watermark, nil
}

func memoryChunkContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

var _ interface {
	CreateMemoryChunk(context.Context, memory.MemoryChunk, string) (memory.MemoryChunk, error)
	ReadMemoryChunk(context.Context, memory.Scope, string) (memory.MemoryChunk, error)
} = (*Repository)(nil)
