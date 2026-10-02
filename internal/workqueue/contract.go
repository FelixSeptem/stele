// Package workqueue defines the bounded identity and lifecycle shared by
// derived background work. It intentionally carries only references, never
// raw event or canonical-memory payloads.
package workqueue

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

var ErrLeaseOwnershipLost = errors.New("derived work lease ownership lost")

const (
	MaxWatermarkBytes   = 256
	MaxIdempotencyBytes = 256
	MaxReferenceBytes   = 256
)

type WorkKind string

const (
	WorkKindReflection         WorkKind = "reflection"
	WorkKindCompaction         WorkKind = "compaction"
	WorkKindProjectionRebuild  WorkKind = "projection_rebuild"
	WorkKindInsightMaintenance WorkKind = "insight_maintenance"
	WorkKindFreshnessRetention WorkKind = "freshness_retention"
)

func (k WorkKind) Valid() bool {
	switch k {
	case WorkKindReflection, WorkKindCompaction, WorkKindProjectionRebuild, WorkKindInsightMaintenance, WorkKindFreshnessRetention:
		return true
	default:
		return false
	}
}

type WorkState string

const (
	WorkStateQueued    WorkState = "queued"
	WorkStateClaimed   WorkState = "claimed"
	WorkStateRunning   WorkState = "running"
	WorkStateRetry     WorkState = "retry"
	WorkStateCompleted WorkState = "completed"
	WorkStateExhausted WorkState = "exhausted"
	WorkStateCancelled WorkState = "cancelled"
	WorkStateDropped   WorkState = "dropped"
)

func (s WorkState) Terminal() bool {
	switch s {
	case WorkStateCompleted, WorkStateExhausted, WorkStateCancelled, WorkStateDropped:
		return true
	default:
		return false
	}
}

func (s WorkState) Valid() bool {
	switch s {
	case WorkStateQueued, WorkStateClaimed, WorkStateRunning, WorkStateRetry, WorkStateCompleted, WorkStateExhausted, WorkStateCancelled, WorkStateDropped:
		return true
	default:
		return false
	}
}

type LossDisposition string

const (
	LossDispositionNone         LossDisposition = "none"
	LossDispositionOverflow     LossDisposition = "overflow"
	LossDispositionEvicted      LossDisposition = "evicted"
	LossDispositionFlushFailure LossDisposition = "flush_failure"
	LossDispositionProcessLoss  LossDisposition = "process_loss"
)

func (d LossDisposition) Valid() bool {
	switch d {
	case LossDispositionNone, LossDispositionOverflow, LossDispositionEvicted, LossDispositionFlushFailure, LossDispositionProcessLoss:
		return true
	default:
		return false
	}
}

type QueueMode string

const (
	QueueModePostgresDurable QueueMode = "postgres_durable"
	QueueModeMemoryBuffer    QueueMode = "memory_buffer"
)

func (m QueueMode) Valid() bool {
	return m == QueueModePostgresDurable || m == QueueModeMemoryBuffer
}

// QueueConfig uses bounded settings common to both adapters. Mode is selected
// at startup, so callers must construct a new adapter instead of mutating it.
type QueueConfig struct {
	Mode          QueueMode
	Capacity      int
	BatchSize     int
	FlushInterval time.Duration
	LeaseDuration time.Duration
	MaxAttempts   int
	Retention     time.Duration
}

func (c QueueConfig) Validate() error {
	switch {
	case !c.Mode.Valid():
		return fmt.Errorf("queue mode %q is invalid", c.Mode)
	case c.Capacity <= 0:
		return fmt.Errorf("queue capacity must be positive")
	case c.BatchSize <= 0 || c.BatchSize > c.Capacity:
		return fmt.Errorf("queue batch size must be positive and no greater than capacity")
	case c.FlushInterval <= 0:
		return fmt.Errorf("queue flush interval must be positive")
	case c.LeaseDuration <= 0:
		return fmt.Errorf("queue lease duration must be positive")
	case c.MaxAttempts <= 0:
		return fmt.Errorf("queue max attempts must be positive")
	case c.Retention <= 0:
		return fmt.Errorf("queue retention must be positive")
	default:
		return nil
	}
}

// DerivedWorkInput contains only immutable, bounded references to durable
// records. The work key is computed from all identity fields at submission.
type DerivedWorkInput struct {
	Scope       memory.Scope
	Kind        WorkKind
	Watermark   string
	Idempotency string
	Reference   string
}

func (i DerivedWorkInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case !i.Kind.Valid():
		return fmt.Errorf("derived work kind %q is invalid", i.Kind)
	case !boundedReference(i.Watermark, MaxWatermarkBytes):
		return fmt.Errorf("derived work watermark is required and must be at most %d bytes", MaxWatermarkBytes)
	case !boundedReference(i.Idempotency, MaxIdempotencyBytes):
		return fmt.Errorf("derived work idempotency is required and must be at most %d bytes", MaxIdempotencyBytes)
	case !boundedReference(i.Reference, MaxReferenceBytes):
		return fmt.Errorf("derived work reference is required and must be at most %d bytes", MaxReferenceBytes)
	}
	return nil
}

func boundedReference(value string, max int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(trimmed) <= max
}

type DerivedWorkIdentity struct {
	Scope   memory.Scope
	WorkKey string
}

type EnqueueInput struct {
	DerivedWorkInput
	MaxAttempts     int
	Now             time.Time
	DetailExpiresAt time.Time
}

func (i EnqueueInput) Validate() error {
	if err := i.DerivedWorkInput.Validate(); err != nil {
		return err
	}
	if i.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be positive")
	}
	if i.Now.IsZero() {
		return fmt.Errorf("enqueue time is required")
	}
	if i.DetailExpiresAt.IsZero() || !i.DetailExpiresAt.After(i.Now) {
		return fmt.Errorf("detail expiry must be after enqueue time")
	}
	return nil
}

func (i EnqueueInput) WorkKey() string {
	identity, err := NewDerivedWorkIdentity(i.DerivedWorkInput)
	if err != nil {
		return ""
	}
	return identity.WorkKey
}

type DerivedWorkItem struct {
	ID string
	DerivedWorkIdentity
	Kind            WorkKind
	Watermark       string
	Idempotency     string
	Reference       string
	State           WorkState
	AttemptCount    int
	MaxAttempts     int
	LeaseOwner      string
	LeaseUntil      time.Time
	NextAttemptAt   time.Time
	FailureCategory string
	LossDisposition LossDisposition
	CreatedAt       time.Time
	UpdatedAt       time.Time
	TerminalAt      time.Time
	DetailExpiresAt time.Time
}

type RenewLeaseInput struct {
	Scope      memory.Scope
	WorkID     string
	WorkerID   string
	RenewedAt  time.Time
	LeaseUntil time.Time
}

func (i RenewLeaseInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("work id and worker id are required")
	}
	if i.RenewedAt.IsZero() || i.LeaseUntil.IsZero() || !i.LeaseUntil.After(i.RenewedAt) {
		return fmt.Errorf("lease times are invalid")
	}
	return nil
}

type ClaimInput struct {
	Scope         memory.Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

func (i ClaimInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkerID) == "" || i.Now.IsZero() || i.LeaseDuration <= 0 || i.Limit <= 0 {
		return fmt.Errorf("claim input is invalid")
	}
	return nil
}

type CheckpointInput struct {
	Scope           memory.Scope
	WorkID          string
	WorkerID        string
	Sequence        int64
	ProcessedOffset int64
	SourceWatermark string
	CommittedAt     time.Time
}

func (i CheckpointInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || strings.TrimSpace(i.WorkerID) == "" || i.Sequence <= 0 || i.ProcessedOffset < 0 || !boundedReference(i.SourceWatermark, MaxWatermarkBytes) || i.CommittedAt.IsZero() {
		return fmt.Errorf("checkpoint input is invalid")
	}
	return nil
}

type TerminalInput struct {
	Scope              memory.Scope
	WorkID             string
	WorkerID           string
	CompletedAt        time.Time
	Disposition        string
	CheckpointSequence int64
	EvidenceReference  string
}

func (i TerminalInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || strings.TrimSpace(i.WorkerID) == "" || i.CompletedAt.IsZero() || strings.TrimSpace(i.Disposition) == "" || i.CheckpointSequence < 0 || (i.EvidenceReference != "" && !boundedReference(i.EvidenceReference, MaxReferenceBytes)) {
		return fmt.Errorf("terminal input is invalid")
	}
	return nil
}

type RetryInput struct {
	Scope           memory.Scope
	WorkID          string
	WorkerID        string
	FailureCategory string
	RetryAt         time.Time
	FailedAt        time.Time
}

func (i RetryInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || strings.TrimSpace(i.WorkerID) == "" || strings.TrimSpace(i.FailureCategory) == "" || i.RetryAt.IsZero() || i.FailedAt.IsZero() || i.RetryAt.Before(i.FailedAt) {
		return fmt.Errorf("retry input is invalid")
	}
	return nil
}

type CancelInput struct {
	Scope       memory.Scope
	WorkID      string
	CancelledAt time.Time
}

func (i CancelInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || i.CancelledAt.IsZero() {
		return fmt.Errorf("cancel input is invalid")
	}
	return nil
}

type RecoverInput struct {
	Scope       memory.Scope
	WorkID      string
	RecoveredAt time.Time
}

func (i RecoverInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkID) == "" || i.RecoveredAt.IsZero() {
		return fmt.Errorf("recovery input is invalid")
	}
	return nil
}

type ListInput struct {
	Scope  memory.Scope
	Limit  int
	Cursor string
}

func (i ListInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Limit <= 0 || i.Limit > 200 {
		return fmt.Errorf("list limit must be between 1 and 200")
	}
	return nil
}

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func (c Cursor) Encode() (string, error) {
	if c.CreatedAt.IsZero() || strings.TrimSpace(c.ID) == "" {
		return "", fmt.Errorf("cursor is incomplete")
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeCursor(value string) (Cursor, error) {
	if strings.TrimSpace(value) == "" {
		return Cursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}
	var cursor Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.CreatedAt.IsZero() || strings.TrimSpace(cursor.ID) == "" {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}
	return cursor, nil
}

type WorkPage struct {
	Items      []DerivedWorkItem
	NextCursor string
}

type Status struct {
	Mode            QueueMode `json:"mode"`
	Depth           int64     `json:"depth"`
	Queued          int64     `json:"queued"`
	Running         int64     `json:"running"`
	Retry           int64     `json:"retry"`
	Completed       int64     `json:"completed"`
	Exhausted       int64     `json:"exhausted"`
	Cancelled       int64     `json:"cancelled"`
	Dropped         int64     `json:"dropped"`
	OldestPendingAt time.Time `json:"oldest_pending_at,omitempty"`
	ObservedAt      time.Time `json:"observed_at"`
}

type RetentionInput struct {
	Scope     memory.Scope
	Before    time.Time
	BatchSize int
}

func (i RetentionInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Before.IsZero() || i.BatchSize <= 0 || i.BatchSize > 1000 {
		return fmt.Errorf("retention input is invalid")
	}
	return nil
}

func NewDerivedWorkIdentity(input DerivedWorkInput) (DerivedWorkIdentity, error) {
	if err := input.Validate(); err != nil {
		return DerivedWorkIdentity{}, err
	}
	scope := input.Scope.Normalized()
	identity := strings.Join([]string{
		string(input.Kind),
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		strings.TrimSpace(input.Watermark),
		strings.TrimSpace(input.Idempotency),
		strings.TrimSpace(input.Reference),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return DerivedWorkIdentity{Scope: scope, WorkKey: hex.EncodeToString(digest[:])}, nil
}
