package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

// ProviderSyncSource exposes existing PostgreSQL raw-event and canonical-memory
// projections through the transport-neutral provider synchronization contract.
// It never writes canonical state and applies the exact runtime scope to every
// query.
type ProviderSyncSource struct {
	Repository *Repository
}

// RetentionFloor reports the oldest replay sequence still present in the
// source projection. raw_events is currently retained as a complete durable
// projection, so the floor is one while rows exist and zero for an empty
// scope. If retention pruning is introduced, this method is the single place
// that exposes the persisted floor to the transport-neutral synchronizer.
func (s ProviderSyncSource) RetentionFloor(ctx context.Context, binding provider.RuntimeBinding) (int64, error) {
	if s.Repository == nil || s.Repository.db == nil {
		return 0, fmt.Errorf("synchronization repository is not configured")
	}
	var exists bool
	if err := s.Repository.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM raw_events WHERE tenant=$1 AND project=$2 AND namespace=$3)`, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace).Scan(&exists); err != nil {
		return 0, fmt.Errorf("check synchronization retention: %w", err)
	}
	if !exists {
		return 0, nil
	}
	return 1, nil
}

func (r *Repository) SaveSyncCursor(ctx context.Context, binding provider.RuntimeBinding, cursor provider.SyncCursor) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("synchronization repository is not configured")
	}
	if err := cursor.Validate(time.Now().UTC()); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `INSERT INTO provider_sync_cursors (binding_id, tenant, project, namespace, agent_id, session_id, provider_instance_id, contract_version, snapshot_watermark, acknowledged_sequence, issued_at, updated_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),$12) ON CONFLICT (binding_id) DO UPDATE SET snapshot_watermark=EXCLUDED.snapshot_watermark, acknowledged_sequence=EXCLUDED.acknowledged_sequence, contract_version=EXCLUDED.contract_version, issued_at=EXCLUDED.issued_at, updated_at=now(), expires_at=EXCLUDED.expires_at`, binding.BindingID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ProviderInstanceID, cursor.ContractVersion, cursor.SnapshotWatermark, cursor.Sequence, cursor.IssuedAt, cursor.IssuedAt.Add(30*24*time.Hour))
	if err != nil {
		return fmt.Errorf("save synchronization cursor: %w", err)
	}
	return nil
}

func (r *Repository) LoadSyncCursor(ctx context.Context, binding provider.RuntimeBinding) (provider.SyncCursor, error) {
	if r == nil || r.db == nil {
		return provider.SyncCursor{}, fmt.Errorf("synchronization repository is not configured")
	}
	var cursor provider.SyncCursor
	var tenant, project, namespace, agentID, sessionID, providerInstanceID string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, `SELECT tenant, project, namespace, agent_id, session_id, provider_instance_id, contract_version, snapshot_watermark, acknowledged_sequence, issued_at, expires_at FROM provider_sync_cursors WHERE binding_id=$1`, binding.BindingID).Scan(&tenant, &project, &namespace, &agentID, &sessionID, &providerInstanceID, &cursor.ContractVersion, &cursor.SnapshotWatermark, &cursor.Sequence, &cursor.IssuedAt, &expiresAt)
	if err != nil {
		return provider.SyncCursor{}, fmt.Errorf("load synchronization cursor: %w", err)
	}
	if tenant != binding.Scope.Tenant || project != binding.Scope.Project || namespace != binding.Scope.Namespace || agentID != binding.AgentID || sessionID != binding.SessionID || providerInstanceID != binding.ProviderInstanceID {
		return provider.SyncCursor{}, fmt.Errorf("synchronization cursor scope mismatch")
	}
	cursor.BindingID = binding.BindingID
	cursor.ScopeHash = provider.ScopeHash(binding.Scope)
	if !expiresAt.After(time.Now().UTC()) {
		return provider.SyncCursor{}, fmt.Errorf("synchronization cursor expired")
	}
	return cursor, nil
}

func (s ProviderSyncSource) Snapshot(ctx context.Context, binding provider.RuntimeBinding, maxBytes int) (provider.SyncSnapshot, error) {
	if s.Repository == nil || s.Repository.db == nil {
		return provider.SyncSnapshot{}, fmt.Errorf("synchronization repository is not configured")
	}
	items := make([]json.RawMessage, 0)
	memories, err := s.Repository.ListCanonicalMemories(ctx, binding.Scope, false)
	if err != nil {
		return provider.SyncSnapshot{}, fmt.Errorf("load synchronization snapshot: %w", err)
	}
	for _, item := range memories {
		payload, err := json.Marshal(struct {
			Kind   string                 `json:"kind"`
			Memory memory.CanonicalMemory `json:"memory"`
		}{Kind: provider.SyncEventCanonicalMemory, Memory: item})
		if err != nil {
			return provider.SyncSnapshot{}, fmt.Errorf("marshal synchronization snapshot: %w", err)
		}
		items = append(items, payload)
	}
	b, err := json.Marshal(items)
	if err != nil {
		return provider.SyncSnapshot{}, err
	}
	if len(b) > maxBytes {
		return provider.SyncSnapshot{}, fmt.Errorf("snapshot exceeds configured limit")
	}
	now := time.Now().UTC()
	return provider.SyncSnapshot{SnapshotID: "snapshot_" + provider.ScopeHash(binding.Scope)[:16], Watermark: now.Format(time.RFC3339Nano), Items: items}, nil
}

func (s ProviderSyncSource) Events(ctx context.Context, binding provider.RuntimeBinding, after int64, max int) ([]provider.SyncEvent, error) {
	if s.Repository == nil || s.Repository.db == nil {
		return nil, fmt.Errorf("synchronization repository is not configured")
	}
	if max <= 0 {
		return nil, fmt.Errorf("synchronization batch size is invalid")
	}
	rows, err := s.Repository.db.Query(ctx, `
WITH ordered AS (
    SELECT id::text, event_type, content, metadata, created_at,
           row_number() OVER (ORDER BY created_at, id) AS sequence
    FROM raw_events
    WHERE tenant=$1 AND project=$2 AND namespace=$3
)
SELECT id, event_type, content, metadata, created_at, sequence
FROM ordered
WHERE sequence > $4
ORDER BY sequence
LIMIT $5`, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, after, max)
	if err != nil {
		return nil, fmt.Errorf("list synchronization events: %w", err)
	}
	defer rows.Close()
	events := make([]provider.SyncEvent, 0, max)
	for rows.Next() {
		var id, eventType, content string
		var metadata []byte
		var createdAt time.Time
		var sequence int64
		if err := rows.Scan(&id, &eventType, &content, &metadata, &createdAt, &sequence); err != nil {
			return nil, fmt.Errorf("scan synchronization event: %w", err)
		}
		payload, err := json.Marshal(struct {
			EventType string          `json:"event_type"`
			Content   string          `json:"content"`
			Metadata  json.RawMessage `json:"metadata"`
		}{EventType: eventType, Content: content, Metadata: json.RawMessage(metadata)})
		if err != nil {
			return nil, fmt.Errorf("marshal synchronization event: %w", err)
		}
		events = append(events, provider.SyncEvent{Sequence: sequence, ReplayID: id, Kind: provider.SyncEventRawEvent, SchemaVersion: provider.SyncContractVersion, SourceWatermark: createdAt.UTC().Format(time.RFC3339Nano), Payload: payload, Available: true})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate synchronization events: %w", err)
	}
	return events, nil
}

var _ provider.SyncSource = ProviderSyncSource{}
var _ provider.SyncCursorStore = (*Repository)(nil)
