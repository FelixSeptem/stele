package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEmbeddingRecoveryConflict = errors.New("embedding recovery conflict")

func (s EmbeddingRebuildStatus) Valid() bool {
	switch s {
	case EmbeddingRebuildStatusPending,
		EmbeddingRebuildStatusRebuilding,
		EmbeddingRebuildStatusFailed,
		EmbeddingRebuildStatusCurrent:
		return true
	default:
		return false
	}
}

type EmbeddingRuntimeStatus struct {
	Configured             bool     `json:"configured"`
	SemanticRebuildEnabled bool     `json:"semantic_rebuild_enabled"`
	RegisteredProviders    []string `json:"registered_providers,omitempty"`
	Reason                 string   `json:"reason,omitempty"`
}

type ListEmbeddingRebuildsInput struct {
	Scope             Scope
	Status            EmbeddingRebuildStatus
	RequestedProvider string
	RequestedModel    string
	Drifted           *bool
	Limit             int
}

func (i ListEmbeddingRebuildsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("embedding rebuild status %q is invalid", i.Status)
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}

	return nil
}

type EmbeddingRebuildView struct {
	MemoryID             string                 `json:"memory_id"`
	Scope                Scope                  `json:"scope"`
	Class                MemoryClass            `json:"class"`
	State                MemoryState            `json:"state"`
	Status               EmbeddingRebuildStatus `json:"status"`
	RequestedProvider    string                 `json:"requested_provider,omitempty"`
	RequestedModel       string                 `json:"requested_model,omitempty"`
	RequestedDimensions  int                    `json:"requested_dimensions,omitempty"`
	ActiveVectorRevision string                 `json:"active_vector_revision_id,omitempty"`
	ActiveProvider       string                 `json:"active_provider,omitempty"`
	ActiveModel          string                 `json:"active_model,omitempty"`
	ActiveDimensions     int                    `json:"active_dimensions,omitempty"`
	FailureReason        string                 `json:"failure_reason,omitempty"`
	Drifted              bool                   `json:"drifted"`
	RequestedAt          time.Time              `json:"requested_at,omitempty"`
	LastAttemptedAt      time.Time              `json:"last_attempted_at,omitempty"`
}

type EmbeddingRebuildPage struct {
	Runtime EmbeddingRuntimeStatus `json:"runtime"`
	Items   []EmbeddingRebuildView `json:"items"`
}

type EmbeddingMemorySummary struct {
	ID                   string      `json:"id"`
	Scope                Scope       `json:"scope"`
	Class                MemoryClass `json:"class"`
	State                MemoryState `json:"state"`
	CurrentSourceVersion int64       `json:"current_source_version"`
	CurrentContentHash   string      `json:"current_content_hash"`
}

type EmbeddingVectorRevisionView struct {
	ID                 string               `json:"id"`
	Provider           string               `json:"provider"`
	Model              string               `json:"model"`
	Dimensions         int                  `json:"dimensions"`
	Status             VectorRevisionStatus `json:"status"`
	FailureReason      string               `json:"failure_reason,omitempty"`
	SupersededBy       string               `json:"superseded_by,omitempty"`
	SourceVersion      int64                `json:"source_version"`
	ContentHash        string               `json:"content_hash"`
	GeneratedAt        time.Time            `json:"generated_at"`
	ActivatedAt        time.Time            `json:"activated_at,omitempty"`
	LastRebuildRequest time.Time            `json:"last_rebuild_request_at,omitempty"`
}

type EmbeddingMemoryInspection struct {
	Runtime   EmbeddingRuntimeStatus        `json:"runtime"`
	Memory    EmbeddingMemorySummary        `json:"memory"`
	Rebuild   EmbeddingRebuildView          `json:"rebuild"`
	Revisions []EmbeddingVectorRevisionView `json:"revisions"`
}

type EmbeddingRecoveryAction string

const (
	EmbeddingRecoveryActionRetry   EmbeddingRecoveryAction = "retry"
	EmbeddingRecoveryActionRequeue EmbeddingRecoveryAction = "requeue"
)

func (a EmbeddingRecoveryAction) Valid() bool {
	switch a {
	case EmbeddingRecoveryActionRetry, EmbeddingRecoveryActionRequeue:
		return true
	default:
		return false
	}
}

type ApplyEmbeddingRecoveryInput struct {
	Scope     Scope                   `json:"scope"`
	MemoryID  string                  `json:"memory_id"`
	Action    EmbeddingRecoveryAction `json:"action"`
	Actor     string                  `json:"actor"`
	Reason    string                  `json:"reason"`
	AppliedAt time.Time               `json:"applied_at"`
}

func (i ApplyEmbeddingRecoveryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.MemoryID) == "":
		return fmt.Errorf("memory id is required")
	case !i.Action.Valid():
		return fmt.Errorf("embedding recovery action %q is invalid", i.Action)
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case i.AppliedAt.IsZero():
		return fmt.Errorf("applied at is required")
	default:
		return nil
	}
}

type EmbeddingRecoverySnapshot struct {
	Status               EmbeddingRebuildStatus `json:"status"`
	RequestedProvider    string                 `json:"requested_provider,omitempty"`
	RequestedModel       string                 `json:"requested_model,omitempty"`
	RequestedDimensions  int                    `json:"requested_dimensions,omitempty"`
	FailureReason        string                 `json:"failure_reason,omitempty"`
	RequestedAt          time.Time              `json:"requested_at,omitempty"`
	LastAttemptedAt      time.Time              `json:"last_attempted_at,omitempty"`
	ActiveVectorRevision string                 `json:"active_vector_revision_id,omitempty"`
	ActiveProvider       string                 `json:"active_provider,omitempty"`
	ActiveModel          string                 `json:"active_model,omitempty"`
	ActiveDimensions     int                    `json:"active_dimensions,omitempty"`
}

func NewEmbeddingRecoverySnapshot(rebuild EmbeddingRebuildView) EmbeddingRecoverySnapshot {
	return EmbeddingRecoverySnapshot{
		Status:               rebuild.Status,
		RequestedProvider:    rebuild.RequestedProvider,
		RequestedModel:       rebuild.RequestedModel,
		RequestedDimensions:  rebuild.RequestedDimensions,
		FailureReason:        rebuild.FailureReason,
		RequestedAt:          rebuild.RequestedAt,
		LastAttemptedAt:      rebuild.LastAttemptedAt,
		ActiveVectorRevision: rebuild.ActiveVectorRevision,
		ActiveProvider:       rebuild.ActiveProvider,
		ActiveModel:          rebuild.ActiveModel,
		ActiveDimensions:     rebuild.ActiveDimensions,
	}
}

type EmbeddingRecoveryRecord struct {
	ID         string                    `json:"id"`
	MemoryID   string                    `json:"memory_id"`
	Scope      Scope                     `json:"scope"`
	Action     EmbeddingRecoveryAction   `json:"action"`
	Actor      string                    `json:"actor"`
	Reason     string                    `json:"reason"`
	Before     EmbeddingRecoverySnapshot `json:"before"`
	After      EmbeddingRecoverySnapshot `json:"after"`
	OccurredAt time.Time                 `json:"occurred_at"`
}

type EmbeddingRecoveryOutcome struct {
	Rebuild  EmbeddingRebuildView    `json:"rebuild"`
	Recovery EmbeddingRecoveryRecord `json:"recovery"`
}

type EmbeddingAdminStore interface {
	ListEmbeddingRebuilds(ctx context.Context, input ListEmbeddingRebuildsInput) ([]EmbeddingRebuildView, error)
	ReadMemoryEmbedding(ctx context.Context, scope Scope, memoryID string) (EmbeddingMemoryInspection, error)
	ApplyEmbeddingRecovery(ctx context.Context, input ApplyEmbeddingRecoveryInput) (EmbeddingRecoveryOutcome, error)
}

type EmbeddingAdminQueryService struct {
	store   EmbeddingAdminStore
	runtime EmbeddingRuntimeStatus
}

func NewEmbeddingAdminQueryService(store EmbeddingAdminStore, runtime EmbeddingRuntimeStatus) *EmbeddingAdminQueryService {
	return &EmbeddingAdminQueryService{
		store:   store,
		runtime: runtime,
	}
}

func (s *EmbeddingAdminQueryService) ListEmbeddingRebuilds(ctx context.Context, input ListEmbeddingRebuildsInput) (EmbeddingRebuildPage, error) {
	if err := input.Validate(); err != nil {
		return EmbeddingRebuildPage{}, err
	}
	if s.store == nil {
		return EmbeddingRebuildPage{}, fmt.Errorf("embedding admin store is not configured")
	}

	items, err := s.store.ListEmbeddingRebuilds(ctx, input)
	if err != nil {
		return EmbeddingRebuildPage{}, err
	}

	return EmbeddingRebuildPage{
		Runtime: cloneEmbeddingRuntimeStatus(s.runtime),
		Items:   items,
	}, nil
}

func (s *EmbeddingAdminQueryService) GetMemoryEmbedding(ctx context.Context, scope Scope, memoryID string) (EmbeddingMemoryInspection, error) {
	if err := scope.Validate(); err != nil {
		return EmbeddingMemoryInspection{}, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return EmbeddingMemoryInspection{}, fmt.Errorf("memory id is required")
	}
	if s.store == nil {
		return EmbeddingMemoryInspection{}, fmt.Errorf("embedding admin store is not configured")
	}

	inspection, err := s.store.ReadMemoryEmbedding(ctx, scope, memoryID)
	if err != nil {
		return EmbeddingMemoryInspection{}, err
	}
	inspection.Runtime = cloneEmbeddingRuntimeStatus(s.runtime)
	return inspection, nil
}

func (s *EmbeddingAdminQueryService) ApplyEmbeddingRecovery(ctx context.Context, input ApplyEmbeddingRecoveryInput) (EmbeddingRecoveryOutcome, error) {
	if err := input.Validate(); err != nil {
		return EmbeddingRecoveryOutcome{}, err
	}
	if s.store == nil {
		return EmbeddingRecoveryOutcome{}, fmt.Errorf("embedding admin store is not configured")
	}

	return s.store.ApplyEmbeddingRecovery(ctx, input)
}

func ApplyEmbeddingRecovery(current EmbeddingRebuildView, input ApplyEmbeddingRecoveryInput) (EmbeddingRebuildView, error) {
	if err := input.Validate(); err != nil {
		return EmbeddingRebuildView{}, err
	}

	state := current.Status
	switch input.Action {
	case EmbeddingRecoveryActionRetry:
		if state != EmbeddingRebuildStatusFailed {
			return EmbeddingRebuildView{}, fmt.Errorf("%w: action %q is not allowed for status %q", ErrEmbeddingRecoveryConflict, input.Action, state)
		}
	case EmbeddingRecoveryActionRequeue:
		if state != EmbeddingRebuildStatusCurrent && state != EmbeddingRebuildStatusFailed {
			return EmbeddingRebuildView{}, fmt.Errorf("%w: action %q is not allowed for status %q", ErrEmbeddingRecoveryConflict, input.Action, state)
		}
	default:
		return EmbeddingRebuildView{}, fmt.Errorf("embedding recovery action %q is invalid", input.Action)
	}

	next := current
	next.Status = EmbeddingRebuildStatusPending
	next.FailureReason = ""
	next.RequestedAt = input.AppliedAt.UTC()
	return next, nil
}

func cloneEmbeddingRuntimeStatus(status EmbeddingRuntimeStatus) EmbeddingRuntimeStatus {
	cloned := status
	if len(status.RegisteredProviders) == 0 {
		return cloned
	}

	cloned.RegisteredProviders = append([]string(nil), status.RegisteredProviders...)
	return cloned
}
