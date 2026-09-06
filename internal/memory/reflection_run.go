package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ReflectionTrigger string

const (
	ReflectionTriggerSessionCompletion ReflectionTrigger = "session_completion"
	ReflectionTriggerEventThreshold    ReflectionTrigger = "event_threshold"
	ReflectionTriggerCompaction        ReflectionTrigger = "compaction_pressure"
	ReflectionTriggerSchedule          ReflectionTrigger = "schedule"
	ReflectionTriggerOperator          ReflectionTrigger = "operator"
)

func (t ReflectionTrigger) Valid() bool {
	switch t {
	case ReflectionTriggerSessionCompletion, ReflectionTriggerEventThreshold, ReflectionTriggerCompaction, ReflectionTriggerSchedule, ReflectionTriggerOperator:
		return true
	}
	return false
}

type ReflectionRunStatus string

const (
	ReflectionRunPending   ReflectionRunStatus = "pending"
	ReflectionRunRunning   ReflectionRunStatus = "running"
	ReflectionRunCompleted ReflectionRunStatus = "completed"
	ReflectionRunFailed    ReflectionRunStatus = "failed"
	ReflectionRunExhausted ReflectionRunStatus = "exhausted"
)

func (s ReflectionRunStatus) Valid() bool {
	switch s {
	case ReflectionRunPending, ReflectionRunRunning, ReflectionRunCompleted, ReflectionRunFailed, ReflectionRunExhausted:
		return true
	}
	return false
}

type ReflectionFailureCategory string

const (
	ReflectionFailureTransient    ReflectionFailureCategory = "transient"
	ReflectionFailureInvalidInput ReflectionFailureCategory = "invalid_input"
	ReflectionFailureScope        ReflectionFailureCategory = "scope_violation"
	ReflectionFailurePolicy       ReflectionFailureCategory = "policy"
	ReflectionFailureUnknown      ReflectionFailureCategory = "unknown"
)

func (c ReflectionFailureCategory) Valid() bool {
	switch c {
	case ReflectionFailureTransient, ReflectionFailureInvalidInput, ReflectionFailureScope, ReflectionFailurePolicy, ReflectionFailureUnknown:
		return true
	}
	return false
}

type CreateReflectionRunInput struct {
	Scope                   Scope
	Trigger                 ReflectionTrigger
	TriggerRef              string
	InputWatermark          string
	TranscriptSchemaVersion string
	IdempotencyKey          string
	ReplayOf                string
	MaxAttempts             int
	CreatedAt               time.Time
}

func (i CreateReflectionRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case !i.Trigger.Valid():
		return fmt.Errorf("reflection trigger %q is invalid", i.Trigger)
	case strings.TrimSpace(i.InputWatermark) == "":
		return fmt.Errorf("input watermark is required")
	case strings.TrimSpace(i.TranscriptSchemaVersion) == "":
		return fmt.Errorf("transcript schema version is required")
	case strings.TrimSpace(i.IdempotencyKey) == "":
		return fmt.Errorf("idempotency key is required")
	}
	if i.MaxAttempts < 0 {
		return fmt.Errorf("max attempts cannot be negative")
	}
	return nil
}

type ReflectionRun struct {
	ID                      string
	Scope                   Scope
	Trigger                 ReflectionTrigger
	TriggerRef              string
	InputWatermark          string
	TranscriptSchemaVersion string
	ProcessedOffset         int64
	LeaseOwner              string
	LeaseUntil              time.Time
	Attempt                 int
	MaxAttempts             int
	Status                  ReflectionRunStatus
	FailureCategory         ReflectionFailureCategory
	FailureMessage          string
	NextAttemptAt           time.Time
	ReplayOf                string
	IdempotencyKey          string
	OutputCandidateIDs      []string
	EvidenceReferences      []string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type ClaimReflectionRunsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

func (i ClaimReflectionRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if i.Now.IsZero() {
		return fmt.Errorf("now is required")
	}
	if i.LeaseDuration <= 0 {
		return fmt.Errorf("lease duration must be positive")
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	return nil
}

type CheckpointReflectionRunInput struct {
	Scope                                  Scope
	RunID, WorkerID                        string
	ProcessedOffset                        int64
	OutputCandidateIDs, EvidenceReferences []string
	UpdatedAt                              time.Time
}
type CompleteReflectionRunInput struct {
	Scope                                  Scope
	RunID, WorkerID                        string
	ProcessedOffset                        int64
	OutputCandidateIDs, EvidenceReferences []string
	CompletedAt                            time.Time
}
type FailReflectionRunInput struct {
	Scope           Scope
	RunID, WorkerID string
	Category        ReflectionFailureCategory
	Message         string
	RetryAt         time.Time
	FailedAt        time.Time
}
type ReplayReflectionRunInput struct {
	Scope                                                                               Scope
	OriginalRunID, InputWatermark, TranscriptSchemaVersion, IdempotencyKey, RequestedBy string
	RequestedAt                                                                         time.Time
}

type ReflectionRunStore interface {
	CreateReflectionRun(context.Context, CreateReflectionRunInput) (ReflectionRun, error)
	ClaimReflectionRuns(context.Context, ClaimReflectionRunsInput) ([]ReflectionRun, error)
	CheckpointReflectionRun(context.Context, CheckpointReflectionRunInput) error
	CompleteReflectionRun(context.Context, CompleteReflectionRunInput) error
	FailReflectionRun(context.Context, FailReflectionRunInput) error
	ReplayReflectionRun(context.Context, ReplayReflectionRunInput) (ReflectionRun, error)
}

type ReflectionTriggerService struct {
	Store       ReflectionRunStore
	Now         func() time.Time
	NewID       func() string
	MaxAttempts int
}

func (s ReflectionTriggerService) Create(ctx context.Context, input CreateReflectionRunInput) (ReflectionRun, error) {
	if err := input.Validate(); err != nil {
		return ReflectionRun{}, err
	}
	if s.Store == nil {
		return ReflectionRun{}, fmt.Errorf("reflection run store is required")
	}
	if input.MaxAttempts == 0 {
		input.MaxAttempts = s.MaxAttempts
	}
	if input.MaxAttempts <= 0 {
		input.MaxAttempts = 3
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
		if s.Now != nil {
			input.CreatedAt = s.Now().UTC()
		}
	}
	return s.Store.CreateReflectionRun(ctx, input)
}
func (s ReflectionTriggerService) SessionCompletion(ctx context.Context, scope Scope, watermark, schema, key string) (ReflectionRun, error) {
	return s.Create(ctx, CreateReflectionRunInput{Scope: scope, Trigger: ReflectionTriggerSessionCompletion, InputWatermark: watermark, TranscriptSchemaVersion: schema, IdempotencyKey: key})
}
func (s ReflectionTriggerService) EventThreshold(ctx context.Context, scope Scope, watermark, schema, key string) (ReflectionRun, error) {
	return s.Create(ctx, CreateReflectionRunInput{Scope: scope, Trigger: ReflectionTriggerEventThreshold, InputWatermark: watermark, TranscriptSchemaVersion: schema, IdempotencyKey: key})
}
func (s ReflectionTriggerService) CompactionPressure(ctx context.Context, scope Scope, watermark, schema, key string) (ReflectionRun, error) {
	return s.Create(ctx, CreateReflectionRunInput{Scope: scope, Trigger: ReflectionTriggerCompaction, InputWatermark: watermark, TranscriptSchemaVersion: schema, IdempotencyKey: key})
}
func (s ReflectionTriggerService) Schedule(ctx context.Context, scope Scope, watermark, schema, key string) (ReflectionRun, error) {
	return s.Create(ctx, CreateReflectionRunInput{Scope: scope, Trigger: ReflectionTriggerSchedule, InputWatermark: watermark, TranscriptSchemaVersion: schema, IdempotencyKey: key})
}
func (s ReflectionTriggerService) Operator(ctx context.Context, scope Scope, watermark, schema, key string) (ReflectionRun, error) {
	return s.Create(ctx, CreateReflectionRunInput{Scope: scope, Trigger: ReflectionTriggerOperator, InputWatermark: watermark, TranscriptSchemaVersion: schema, IdempotencyKey: key})
}
