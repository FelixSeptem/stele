package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	SyncContractVersion      = "sync-v1"
	SyncTransportOpenAPI     = "openapi_pull"
	SyncEventRawEvent        = "raw_event"
	SyncEventCanonicalMemory = "canonical_memory"
	SyncEventLifecycle       = "lifecycle"
	SyncEventComplete        = "sync_complete"
	SyncStatusSnapshot       = "snapshot"
	SyncStatusDelta          = "delta"
	SyncStatusComplete       = "complete"
	SyncStatusResyncRequired = "resync_required"
)

type SyncRequest struct {
	SchemaVersion    string `json:"schema_version"`
	Cursor           string `json:"cursor,omitempty"`
	MaxEvents        int    `json:"max_events,omitempty"`
	MaxSnapshotBytes int    `json:"max_snapshot_bytes,omitempty"`
}

func (r SyncRequest) Validate(c SynchronizationCapabilities) error {
	if !boundedToken(r.SchemaVersion, MaxSchemaVersionBytes) || r.SchemaVersion != c.ContractVersion {
		return fmt.Errorf("unsupported synchronization schema version")
	}
	if len(r.Cursor) > c.MaxCursorBytes {
		return fmt.Errorf("synchronization cursor exceeds limit")
	}
	if r.MaxEvents < 0 || r.MaxEvents > c.MaxBatchEvents || r.MaxSnapshotBytes < 0 || r.MaxSnapshotBytes > c.MaxSnapshotBytes {
		return fmt.Errorf("synchronization limits exceed capability")
	}
	return nil
}

type SyncCursor struct {
	ContractVersion      string    `json:"contract_version"`
	BindingID            string    `json:"binding_id"`
	ScopeHash            string    `json:"scope_hash"`
	SnapshotWatermark    string    `json:"snapshot_watermark"`
	SnapshotContinuation string    `json:"snapshot_continuation,omitempty"`
	Sequence             int64     `json:"sequence"`
	IssuedAt             time.Time `json:"issued_at"`
}

// SyncRetentionSource optionally reports the oldest replay sequence that is
// still available for an exact binding. A cursor before that floor cannot be
// resumed safely and must trigger a full snapshot.
type SyncRetentionSource interface {
	RetentionFloor(context.Context, RuntimeBinding) (int64, error)
}

func (c SyncCursor) Validate(now time.Time) error {
	if c.ContractVersion != SyncContractVersion || !boundedToken(c.BindingID, maxRuntimeBindingLength) || !boundedToken(c.ScopeHash, 128) || (c.SnapshotContinuation != "" && !boundedToken(c.SnapshotContinuation, MaxSyncCursorBytes)) || c.Sequence < 0 || c.IssuedAt.IsZero() {
		return fmt.Errorf("synchronization cursor is invalid")
	}
	if c.IssuedAt.After(now.Add(time.Minute)) {
		return fmt.Errorf("synchronization cursor is from the future")
	}
	return nil
}

func EncodeSyncCursor(c SyncCursor) (string, error) {
	if err := c.Validate(time.Now().UTC()); err != nil {
		return "", err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal synchronization cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeSyncCursor(value string) (SyncCursor, error) {
	if strings.TrimSpace(value) == "" || len(value) > MaxSyncCursorBytes {
		return SyncCursor{}, fmt.Errorf("synchronization cursor is required")
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return SyncCursor{}, fmt.Errorf("decode synchronization cursor: %w", err)
	}
	var c SyncCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return SyncCursor{}, fmt.Errorf("decode synchronization cursor: %w", err)
	}
	if err := c.Validate(time.Now().UTC()); err != nil {
		return SyncCursor{}, err
	}
	return c, nil
}

func ScopeHash(scope memory.Scope) string {
	b, _ := json.Marshal(scope.Normalized())
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

type SyncEvent struct {
	Sequence        int64           `json:"sequence"`
	ReplayID        string          `json:"replay_id"`
	Kind            string          `json:"kind"`
	SchemaVersion   string          `json:"schema_version"`
	SourceWatermark string          `json:"source_watermark"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Available       bool            `json:"available"`
}

func (e SyncEvent) Validate() error {
	if e.Sequence <= 0 || !boundedToken(e.ReplayID, 128) || !boundedToken(e.Kind, MaxOperationNameBytes) || !boundedToken(e.SchemaVersion, MaxSchemaVersionBytes) || !boundedToken(e.SourceWatermark, 128) || len(e.Payload) > 1<<20 {
		return fmt.Errorf("synchronization event is invalid")
	}
	if e.Kind != SyncEventComplete && !e.Available && len(e.Payload) != 0 {
		return fmt.Errorf("unavailable event must not contain payload")
	}
	return nil
}

type SyncSnapshot struct {
	SnapshotID   string            `json:"snapshot_id"`
	Watermark    string            `json:"watermark"`
	Items        []json.RawMessage `json:"items,omitempty"`
	Continuation string            `json:"continuation,omitempty"`
}

type SyncResponse struct {
	SchemaVersion string        `json:"schema_version"`
	Status        string        `json:"status"`
	SyncID        string        `json:"sync_id"`
	Snapshot      *SyncSnapshot `json:"snapshot,omitempty"`
	Events        []SyncEvent   `json:"events,omitempty"`
	Cursor        string        `json:"cursor,omitempty"`
	NextCursor    string        `json:"next_cursor,omitempty"`
	SyncComplete  bool          `json:"sync_complete"`
	Reason        string        `json:"reason,omitempty"`
}

func (r SyncResponse) Validate(c SynchronizationCapabilities) error {
	if r.SchemaVersion != c.ContractVersion || !boundedToken(r.Status, 32) || !boundedToken(r.SyncID, 128) || len(r.Events) > c.MaxBatchEvents {
		return fmt.Errorf("synchronization response is invalid")
	}
	if r.Status == SyncStatusResyncRequired && r.SyncComplete {
		return fmt.Errorf("resynchronization cannot be complete")
	}
	for _, event := range r.Events {
		if err := event.Validate(); err != nil {
			return err
		}
	}
	if r.Snapshot != nil && !boundedToken(r.Snapshot.SnapshotID, 128) {
		return fmt.Errorf("synchronization snapshot is invalid")
	}
	return nil
}

// SyncSource supplies one exact-scope snapshot and its ordered durable event
// stream. Implementations may use PostgreSQL projections or deterministic test
// fixtures; the synchronizer owns cursor and recovery semantics.
type SyncSource interface {
	Snapshot(context.Context, RuntimeBinding, int) (SyncSnapshot, error)
	Events(context.Context, RuntimeBinding, int64, int) ([]SyncEvent, error)
}

// SyncSnapshotContinuationSource is implemented by sources that can page a
// snapshot without changing its watermark. Sources that do not need paging
// may leave this optional interface out.
type SyncSnapshotContinuationSource interface {
	SnapshotContinuation(context.Context, RuntimeBinding, string, int) (SyncSnapshot, error)
}

type SyncCursorStore interface {
	SaveSyncCursor(context.Context, RuntimeBinding, SyncCursor) error
	LoadSyncCursor(context.Context, RuntimeBinding) (SyncCursor, error)
}

type Synchronizer struct {
	Source       SyncSource
	CursorStore  SyncCursorStore
	Capabilities SynchronizationCapabilities
	Now          func() time.Time
}

func (s Synchronizer) Synchronize(ctx context.Context, binding RuntimeBinding, request SyncRequest) (SyncResponse, error) {
	caps := s.Capabilities
	if !caps.Enabled {
		return SyncResponse{}, fmt.Errorf("synchronization is not configured")
	}
	if err := request.Validate(caps); err != nil {
		return SyncResponse{}, err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	maxEvents := request.MaxEvents
	if maxEvents == 0 {
		maxEvents = caps.MaxBatchEvents
	}
	maxSnapshot := request.MaxSnapshotBytes
	if maxSnapshot == 0 {
		maxSnapshot = caps.MaxSnapshotBytes
	}
	if strings.TrimSpace(request.Cursor) == "" {
		snapshot, err := s.Source.Snapshot(ctx, binding, maxSnapshot)
		if err != nil {
			return SyncResponse{}, err
		}
		if snapshot.Watermark == "" {
			snapshot.Watermark = now.Format(time.RFC3339Nano)
		}
		cursor, err := EncodeSyncCursor(SyncCursor{ContractVersion: caps.ContractVersion, BindingID: binding.BindingID, ScopeHash: ScopeHash(binding.Scope), SnapshotWatermark: snapshot.Watermark, SnapshotContinuation: snapshot.Continuation, Sequence: 0, IssuedAt: now})
		if err != nil {
			return SyncResponse{}, err
		}
		if s.CursorStore != nil {
			decoded, decodeErr := DecodeSyncCursor(cursor)
			if decodeErr != nil {
				return SyncResponse{}, decodeErr
			}
			if err := s.CursorStore.SaveSyncCursor(ctx, binding, decoded); err != nil {
				return SyncResponse{}, err
			}
		}
		response := SyncResponse{SchemaVersion: caps.ContractVersion, Status: SyncStatusSnapshot, SyncID: "sync_" + ScopeHash(binding.Scope)[:16], Snapshot: &snapshot, Cursor: cursor, NextCursor: cursor}
		if snapshot.Continuation == "" {
			response.SyncComplete = true
			response.Status = SyncStatusComplete
		}
		return response, response.Validate(caps)
	}
	cursor, err := DecodeSyncCursor(request.Cursor)
	if err != nil {
		return SyncResponse{}, fmt.Errorf("resync required: %w", err)
	}
	if cursor.BindingID != binding.BindingID || cursor.ScopeHash != ScopeHash(binding.Scope) {
		return SyncResponse{}, fmt.Errorf("synchronization scope mismatch")
	}
	if s.CursorStore != nil {
		stored, loadErr := s.CursorStore.LoadSyncCursor(ctx, binding)
		if loadErr != nil || stored.Sequence != cursor.Sequence || stored.SnapshotWatermark != cursor.SnapshotWatermark {
			return SyncResponse{}, fmt.Errorf("resync required: synchronization cursor is not current")
		}
	}
	if retention, ok := s.Source.(SyncRetentionSource); ok {
		floor, floorErr := retention.RetentionFloor(ctx, binding)
		if floorErr != nil {
			return SyncResponse{}, fmt.Errorf("synchronization retention check: %w", floorErr)
		}
		if floor > 0 && cursor.Sequence < floor-1 {
			return SyncResponse{}, fmt.Errorf("resync required: synchronization cursor is outside retention")
		}
	}
	if cursor.IssuedAt.Before(now.Add(-time.Duration(caps.RetentionWindowHours) * time.Hour)) {
		return SyncResponse{}, fmt.Errorf("resync required: synchronization cursor expired")
	}
	if cursor.SnapshotContinuation != "" {
		pager, ok := s.Source.(SyncSnapshotContinuationSource)
		if !ok {
			return SyncResponse{}, fmt.Errorf("resync required: snapshot continuation is unsupported")
		}
		snapshot, pageErr := pager.SnapshotContinuation(ctx, binding, cursor.SnapshotContinuation, maxSnapshot)
		if pageErr != nil {
			return SyncResponse{}, pageErr
		}
		next := cursor
		next.SnapshotContinuation = snapshot.Continuation
		next.IssuedAt = now
		nextCursor, encodeErr := EncodeSyncCursor(next)
		if encodeErr != nil {
			return SyncResponse{}, encodeErr
		}
		if s.CursorStore != nil {
			if saveErr := s.CursorStore.SaveSyncCursor(ctx, binding, next); saveErr != nil {
				return SyncResponse{}, saveErr
			}
		}
		response := SyncResponse{SchemaVersion: caps.ContractVersion, Status: SyncStatusSnapshot, SyncID: "sync_" + ScopeHash(binding.Scope)[:16], Snapshot: &snapshot, Cursor: request.Cursor, NextCursor: nextCursor, SyncComplete: snapshot.Continuation == ""}
		if response.SyncComplete {
			response.Status = SyncStatusComplete
		}
		return response, response.Validate(caps)
	}
	events, err := s.Source.Events(ctx, binding, cursor.Sequence, maxEvents)
	if err != nil {
		return SyncResponse{}, err
	}
	next := cursor
	if len(events) > 0 {
		next.Sequence = events[len(events)-1].Sequence
		next.IssuedAt = now
	}
	nextCursor, err := EncodeSyncCursor(next)
	if err != nil {
		return SyncResponse{}, err
	}
	if s.CursorStore != nil {
		if err := s.CursorStore.SaveSyncCursor(ctx, binding, next); err != nil {
			return SyncResponse{}, err
		}
	}
	response := SyncResponse{SchemaVersion: caps.ContractVersion, Status: SyncStatusDelta, SyncID: "sync_" + ScopeHash(binding.Scope)[:16], Events: events, Cursor: request.Cursor, NextCursor: nextCursor, SyncComplete: len(events) == 0}
	if response.SyncComplete {
		response.Status = SyncStatusComplete
	}
	return response, response.Validate(caps)
}

type MemorySyncSource struct {
	Snapshots       map[string]SyncSnapshot
	EventsByBinding map[string][]SyncEvent
	RetentionFloors map[string]int64
}

func (m *MemorySyncSource) RetentionFloor(_ context.Context, binding RuntimeBinding) (int64, error) {
	if m == nil {
		return 0, fmt.Errorf("synchronization source is not configured")
	}
	return m.RetentionFloors[binding.BindingID], nil
}

func (m *MemorySyncSource) Snapshot(_ context.Context, binding RuntimeBinding, maxBytes int) (SyncSnapshot, error) {
	return m.snapshotPage(binding, "", maxBytes)
}

func (m *MemorySyncSource) SnapshotContinuation(_ context.Context, binding RuntimeBinding, continuation string, maxBytes int) (SyncSnapshot, error) {
	return m.snapshotPage(binding, continuation, maxBytes)
}

func (m *MemorySyncSource) snapshotPage(binding RuntimeBinding, continuation string, maxBytes int) (SyncSnapshot, error) {
	if m == nil {
		return SyncSnapshot{}, fmt.Errorf("synchronization source is not configured")
	}
	snapshot, ok := m.Snapshots[binding.BindingID]
	if !ok {
		snapshot = SyncSnapshot{SnapshotID: "snapshot_" + binding.BindingID, Watermark: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	offset := 0
	if continuation != "" {
		var marker struct {
			Offset int `json:"offset"`
		}
		decoded, err := base64.RawURLEncoding.DecodeString(continuation)
		if err != nil || json.Unmarshal(decoded, &marker) != nil || marker.Offset < 0 || marker.Offset > len(snapshot.Items) {
			return SyncSnapshot{}, fmt.Errorf("resync required: snapshot continuation is invalid")
		}
		offset = marker.Offset
	}
	totalItems := len(snapshot.Items)
	items := make([]json.RawMessage, 0, totalItems-offset)
	used := 2
	for i := offset; i < len(snapshot.Items); i++ {
		itemSize := len(snapshot.Items[i])
		if len(items) > 0 && used+itemSize+1 > maxBytes {
			break
		}
		if len(items) == 0 && used+itemSize > maxBytes {
			return SyncSnapshot{}, fmt.Errorf("snapshot item exceeds configured limit")
		}
		items = append(items, snapshot.Items[i])
		used += itemSize + 1
	}
	nextOffset := offset + len(items)
	snapshot.Items = items
	snapshot.Continuation = ""
	if nextOffset < totalItems {
		marker, _ := json.Marshal(struct {
			Offset int `json:"offset"`
		}{Offset: nextOffset})
		snapshot.Continuation = base64.RawURLEncoding.EncodeToString(marker)
	}
	return snapshot, nil
}

func (m *MemorySyncSource) Events(_ context.Context, binding RuntimeBinding, after int64, max int) ([]SyncEvent, error) {
	if m == nil {
		return nil, fmt.Errorf("synchronization source is not configured")
	}
	all := append([]SyncEvent(nil), m.EventsByBinding[binding.BindingID]...)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Sequence == all[j].Sequence {
			return all[i].ReplayID < all[j].ReplayID
		}
		return all[i].Sequence < all[j].Sequence
	})
	out := make([]SyncEvent, 0, max)
	var previous int64
	for _, event := range all {
		if previous > 0 && event.Sequence <= previous {
			return nil, fmt.Errorf("synchronization event sequence is not monotonic")
		}
		previous = event.Sequence
		if event.Sequence > after {
			if err := event.Validate(); err != nil {
				return nil, err
			}
			out = append(out, event)
			if len(out) == max {
				break
			}
		}
	}
	return out, nil
}
