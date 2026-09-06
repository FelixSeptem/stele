package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListContextProjectionCandidates(ctx context.Context, scope memory.Scope, kind memory.ContextProjectionKind, limit int) ([]memory.ContextProjectionCandidate, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if !kind.Valid() {
		return nil, fmt.Errorf("invalid projection kind %q", kind)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("projection candidate limit must be greater than zero")
	}
	if kind == memory.ContextProjectionKindArchivalHistory {
		return r.listArchivalProjectionCandidates(ctx, scope, limit)
	}
	const query = `
SELECT
    latest.id,
    canonical.id,
    latest.version,
    canonical.class,
    latest.state,
    latest.content,
    latest.created_at
FROM canonical_memories canonical
JOIN LATERAL (
    SELECT id, version, state, content, created_at
    FROM memory_versions
    WHERE memory_id = canonical.id
    ORDER BY version DESC
    LIMIT 1
) latest ON true
WHERE canonical.tenant = $1
  AND canonical.project = $2
  AND canonical.namespace = $3
  AND canonical.state = 'active'
  AND latest.state = 'active'
ORDER BY canonical.updated_at DESC, canonical.id ASC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list canonical projection candidates: %w", err)
	}
	defer rows.Close()
	items := make([]memory.ContextProjectionCandidate, 0)
	for rows.Next() {
		var versionID, memoryID, content string
		var version int64
		var class memory.MemoryClass
		var state memory.MemoryState
		var observedAt time.Time
		if err := rows.Scan(&versionID, &memoryID, &version, &class, &state, &content, &observedAt); err != nil {
			return nil, fmt.Errorf("scan canonical projection candidate: %w", err)
		}
		if state != memory.MemoryStateActive {
			continue
		}
		items = append(items, memory.ContextProjectionCandidate{
			Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: versionID, MemoryID: memoryID, Version: version, Scope: scope},
			Class:  class, State: state, Content: content, Confidence: 1, ObservedAt: observedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical projection candidates: %w", err)
	}
	return items, nil
}

func (r *Repository) listArchivalProjectionCandidates(ctx context.Context, scope memory.Scope, limit int) ([]memory.ContextProjectionCandidate, error) {
	const query = `
SELECT id, content, COALESCE(source_timestamp, created_at)
FROM raw_events
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY COALESCE(source_timestamp, created_at) DESC, id ASC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list archival projection candidates: %w", err)
	}
	defer rows.Close()
	items := make([]memory.ContextProjectionCandidate, 0)
	for rows.Next() {
		var eventID, content string
		var observedAt time.Time
		if err := rows.Scan(&eventID, &content, &observedAt); err != nil {
			return nil, fmt.Errorf("scan archival projection candidate: %w", err)
		}
		items = append(items, memory.ContextProjectionCandidate{
			Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceRawEvent, ID: eventID, Scope: scope},
			Class:  memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: content, ObservedAt: observedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate archival projection candidates: %w", err)
	}
	return items, nil
}

func (r *Repository) CreateContextProjection(ctx context.Context, projection memory.ContextProjection) (memory.ContextProjection, error) {
	if err := projection.Validate(); err != nil {
		return memory.ContextProjection{}, err
	}
	projection.Scope = projection.Scope.Normalized()
	projectionID, err := uuid.Parse(projection.ID)
	if err != nil {
		return memory.ContextProjection{}, fmt.Errorf("projection id must be a UUID: %w", err)
	}
	watermark, err := json.Marshal(projection.SourceWatermark)
	if err != nil {
		return memory.ContextProjection{}, fmt.Errorf("marshal projection watermark: %w", err)
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.ContextProjection{}, fmt.Errorf("begin create context projection: %w", err)
	}
	defer tx.Rollback(ctx)
	var existingID uuid.UUID
	var existingVersion int64
	err = tx.QueryRow(ctx, `SELECT id, version FROM context_projections WHERE tenant=$1 AND project=$2 AND namespace=$3 AND kind=$4 AND policy_version=$5 AND renderer_version=$6 AND source_watermark_hash=$7`, projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace, string(projection.Kind), projection.PolicyVersion, projection.RendererVersion, projection.SourceWatermarkHash()).Scan(&existingID, &existingVersion)
	if err == nil {
		projection.ID = existingID.String()
		projection.Version = existingVersion
		return projection, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return memory.ContextProjection{}, fmt.Errorf("check existing context projection: %w", err)
	}
	command, err := tx.Exec(ctx, `
INSERT INTO context_projections
 (id, tenant, project, namespace, kind, version, schema_version, policy_version, renderer_version, source_watermark, source_watermark_hash, status, created_at, updated_at, superseded_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,COALESCE(NULLIF($13::timestamptz,'0001-01-01T00:00:00Z'::timestamptz), now()),COALESCE(NULLIF($14::timestamptz,'0001-01-01T00:00:00Z'::timestamptz), now()),NULLIF($15::timestamptz,'0001-01-01T00:00:00Z'::timestamptz))
ON CONFLICT (tenant, project, namespace, kind, policy_version, renderer_version, source_watermark_hash)
DO NOTHING`, projectionID, projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace,
		string(projection.Kind), projection.Version, projection.SchemaVersion, projection.PolicyVersion, projection.RendererVersion,
		watermark, projection.SourceWatermarkHash(), string(projection.Status), projection.CreatedAt, projection.UpdatedAt, projection.SupersededAt)
	if err != nil {
		return memory.ContextProjection{}, fmt.Errorf("insert context projection: %w", err)
	}
	if command.RowsAffected() == 0 {
		if err := tx.QueryRow(ctx, `SELECT id, version FROM context_projections WHERE tenant=$1 AND project=$2 AND namespace=$3 AND kind=$4 AND policy_version=$5 AND renderer_version=$6 AND source_watermark_hash=$7`, projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace, string(projection.Kind), projection.PolicyVersion, projection.RendererVersion, projection.SourceWatermarkHash()).Scan(&existingID, &existingVersion); err != nil {
			return memory.ContextProjection{}, fmt.Errorf("read idempotent context projection: %w", err)
		}
		projection.ID = existingID.String()
		projection.Version = existingVersion
		if err := tx.Commit(ctx); err != nil {
			return memory.ContextProjection{}, fmt.Errorf("commit idempotent context projection: %w", err)
		}
		return projection, nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE context_projections
SET status = 'superseded', updated_at = now(), superseded_at = now()
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND kind = $4
  AND status = 'active' AND id <> $5`, projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace, string(projection.Kind), projectionID); err != nil {
		return memory.ContextProjection{}, fmt.Errorf("supersede prior context projections: %w", err)
	}
	for _, item := range projection.Items {
		sourceID, err := uuid.Parse(item.Source.ID)
		if err != nil {
			return memory.ContextProjection{}, fmt.Errorf("projection source id must be a UUID: %w", err)
		}
		citation, err := json.Marshal(item.Citation)
		if err != nil {
			return memory.ContextProjection{}, fmt.Errorf("marshal projection citation: %w", err)
		}
		var memoryID any
		if strings.TrimSpace(item.Source.MemoryID) != "" {
			parsed, parseErr := uuid.Parse(item.Source.MemoryID)
			if parseErr != nil {
				return memory.ContextProjection{}, fmt.Errorf("projection memory id must be a UUID: %w", parseErr)
			}
			memoryID = parsed
		}
		_, err = tx.Exec(ctx, `
INSERT INTO context_projection_items
 (id, projection_id, tenant, project, namespace, source_kind, source_id, source_version, memory_id, class, lifecycle_state, rendered_text, sort_key, citation)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, uuidOrNew(item.ID), projectionID,
			projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace, string(item.Source.Kind), sourceID,
			nullableVersion(item.Source.Version), memoryID, string(item.Class), string(item.LifecycleState), item.Text, item.SortKey, citation)
		if err != nil {
			return memory.ContextProjection{}, fmt.Errorf("insert context projection item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.ContextProjection{}, fmt.Errorf("commit context projection: %w", err)
	}
	return projection, nil
}

func (r *Repository) ReadLatestContextProjection(ctx context.Context, scope memory.Scope, kind memory.ContextProjectionKind) (memory.ContextProjection, error) {
	if err := scope.Validate(); err != nil {
		return memory.ContextProjection{}, err
	}
	row := r.db.QueryRow(ctx, `SELECT id, tenant, project, namespace, kind, version, schema_version, policy_version, renderer_version, source_watermark, status, created_at, updated_at, superseded_at FROM context_projections WHERE tenant=$1 AND project=$2 AND namespace=$3 AND kind=$4 AND status='active' ORDER BY version DESC LIMIT 1`, scope.Tenant, scope.Project, scope.Namespace, string(kind))
	projection, err := scanContextProjection(row)
	if err != nil {
		return memory.ContextProjection{}, err
	}
	items, err := r.readContextProjectionItems(ctx, projection)
	if err != nil {
		return memory.ContextProjection{}, err
	}
	projection.Items = items
	return projection, nil
}

func (r *Repository) ReadContextProjection(ctx context.Context, scope memory.Scope, projectionID string) (memory.ContextProjection, error) {
	if err := scope.Validate(); err != nil {
		return memory.ContextProjection{}, err
	}
	id, err := uuid.Parse(projectionID)
	if err != nil {
		return memory.ContextProjection{}, fmt.Errorf("projection id must be a UUID: %w", err)
	}
	row := r.db.QueryRow(ctx, `SELECT id, tenant, project, namespace, kind, version, schema_version, policy_version, renderer_version, source_watermark, status, created_at, updated_at, superseded_at FROM context_projections WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4`, id, scope.Tenant, scope.Project, scope.Namespace)
	projection, err := scanContextProjection(row)
	if err != nil {
		return memory.ContextProjection{}, err
	}
	projection.Items, err = r.readContextProjectionItems(ctx, projection)
	return projection, err
}

func (r *Repository) readContextProjectionItems(ctx context.Context, projection memory.ContextProjection) ([]memory.ContextProjectionItem, error) {
	id, err := uuid.Parse(projection.ID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT id, source_kind, source_id, COALESCE(source_version, 0), COALESCE(memory_id::text, ''), class, lifecycle_state, rendered_text, sort_key, citation FROM context_projection_items WHERE projection_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 ORDER BY sort_key ASC, source_kind ASC, source_id ASC, id ASC`, id, projection.Scope.Tenant, projection.Scope.Project, projection.Scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("query context projection items: %w", err)
	}
	defer rows.Close()
	items := make([]memory.ContextProjectionItem, 0)
	for rows.Next() {
		var itemID, sourceKind, sourceID, class, state, text, sortKey string
		var sourceVersion int64
		var memoryID string
		var citationBytes []byte
		if err := rows.Scan(&itemID, &sourceKind, &sourceID, &sourceVersion, &memoryID, &class, &state, &text, &sortKey, &citationBytes); err != nil {
			return nil, fmt.Errorf("scan context projection item: %w", err)
		}
		item := memory.ContextProjectionItem{ID: itemID, Class: memory.MemoryClass(class), LifecycleState: memory.MemoryState(state), Text: text, SortKey: sortKey}
		item.Source = memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceKind(sourceKind), ID: sourceID, Scope: projection.Scope}
		item.Source.Version = sourceVersion
		item.Source.MemoryID = memoryID
		if len(citationBytes) > 0 {
			if err := json.Unmarshal(citationBytes, &item.Citation); err != nil {
				return nil, fmt.Errorf("decode projection citation: %w", err)
			}
		}
		if item.LifecycleState != memory.MemoryStateActive {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate context projection items: %w", err)
	}
	memory.SortContextProjectionItems(items)
	return items, nil
}

type contextProjectionScanner interface{ Scan(...any) error }

func scanContextProjection(row contextProjectionScanner) (memory.ContextProjection, error) {
	var id, tenant, project, namespace, kind, schemaVersion, policyVersion, rendererVersion, status string
	var version int64
	var watermarkBytes []byte
	var createdAt, updatedAt time.Time
	var supersededAt *time.Time
	if err := row.Scan(&id, &tenant, &project, &namespace, &kind, &version, &schemaVersion, &policyVersion, &rendererVersion, &watermarkBytes, &status, &createdAt, &updatedAt, &supersededAt); err != nil {
		return memory.ContextProjection{}, fmt.Errorf("scan context projection: %w", err)
	}
	projection := memory.ContextProjection{ID: id, Scope: memory.Scope{Tenant: tenant, Project: project, Namespace: namespace}, Kind: memory.ContextProjectionKind(kind), Version: version, SchemaVersion: schemaVersion, PolicyVersion: policyVersion, RendererVersion: rendererVersion, Status: memory.ContextProjectionStatus(status)}
	if len(watermarkBytes) > 0 {
		if err := json.Unmarshal(watermarkBytes, &projection.SourceWatermark); err != nil {
			return memory.ContextProjection{}, fmt.Errorf("decode context projection watermark: %w", err)
		}
	}
	projection.CreatedAt = createdAt
	projection.UpdatedAt = updatedAt
	if supersededAt != nil {
		projection.SupersededAt = *supersededAt
	}
	return projection, nil
}

func uuidOrNew(value string) uuid.UUID {
	if parsed, err := uuid.Parse(value); err == nil {
		return parsed
	}
	return uuid.New()
}

func nullableVersion(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
