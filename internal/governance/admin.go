package governance

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

var ErrGovernanceRecoveryConflict = errors.New("governance recovery conflict")
var ErrGovernanceRecoveryRejected = errors.New("governance recovery rejected")

type GovernanceRawEventState string

const (
	GovernanceRawEventStatePending   GovernanceRawEventState = "pending"
	GovernanceRawEventStateRetryWait GovernanceRawEventState = "retry_wait"
	GovernanceRawEventStateLeased    GovernanceRawEventState = "leased"
	GovernanceRawEventStateExhausted GovernanceRawEventState = "exhausted"
	GovernanceRawEventStateProcessed GovernanceRawEventState = "processed"
)

func (s GovernanceRawEventState) Valid() bool {
	switch s {
	case GovernanceRawEventStatePending,
		GovernanceRawEventStateRetryWait,
		GovernanceRawEventStateLeased,
		GovernanceRawEventStateExhausted,
		GovernanceRawEventStateProcessed:
		return true
	default:
		return false
	}
}

type GovernanceRecoveryAction string

const (
	GovernanceRecoveryActionRetry      GovernanceRecoveryAction = "retry"
	GovernanceRecoveryActionReschedule GovernanceRecoveryAction = "reschedule"
	GovernanceRecoveryActionRequeue    GovernanceRecoveryAction = "requeue"
)

func (a GovernanceRecoveryAction) Valid() bool {
	switch a {
	case GovernanceRecoveryActionRetry,
		GovernanceRecoveryActionReschedule,
		GovernanceRecoveryActionRequeue:
		return true
	default:
		return false
	}
}

type RawEventGovernanceSnapshot struct {
	Attempt       int
	WorkerID      string
	ClaimedAt     time.Time
	LeaseUntil    time.Time
	LastFailedAt  time.Time
	LastError     string
	NextAttemptAt time.Time
	ExhaustedAt   time.Time
	ProcessedAt   time.Time
}

func (s RawEventGovernanceSnapshot) DerivedState(now time.Time) GovernanceRawEventState {
	if !s.ProcessedAt.IsZero() {
		return GovernanceRawEventStateProcessed
	}
	if !s.ExhaustedAt.IsZero() {
		return GovernanceRawEventStateExhausted
	}
	if !s.LeaseUntil.IsZero() && s.LeaseUntil.After(now) {
		return GovernanceRawEventStateLeased
	}
	if !s.NextAttemptAt.IsZero() && s.NextAttemptAt.After(now) {
		return GovernanceRawEventStateRetryWait
	}

	return GovernanceRawEventStatePending
}

type GovernanceRawEventCursor struct {
	CreatedAt  time.Time
	RawEventID string
}

func (c GovernanceRawEventCursor) Validate() error {
	switch {
	case c.CreatedAt.IsZero():
		return fmt.Errorf("cursor created at is required")
	case strings.TrimSpace(c.RawEventID) == "":
		return fmt.Errorf("cursor raw event id is required")
	default:
		return nil
	}
}

func (c GovernanceRawEventCursor) Encode() string {
	if err := c.Validate(); err != nil {
		return ""
	}

	payload := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + strings.TrimSpace(c.RawEventID)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func DecodeGovernanceRawEventCursor(value string) (GovernanceRawEventCursor, error) {
	if strings.TrimSpace(value) == "" {
		return GovernanceRawEventCursor{}, fmt.Errorf("cursor is required")
	}

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return GovernanceRawEventCursor{}, fmt.Errorf("decode cursor: %w", err)
	}

	createdAtRaw, rawEventID, ok := strings.Cut(string(raw), "|")
	if !ok {
		return GovernanceRawEventCursor{}, fmt.Errorf("decode cursor: invalid cursor payload")
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return GovernanceRawEventCursor{}, fmt.Errorf("decode cursor created at: %w", err)
	}

	cursor := GovernanceRawEventCursor{
		CreatedAt:  createdAt.UTC(),
		RawEventID: strings.TrimSpace(rawEventID),
	}
	if err := cursor.Validate(); err != nil {
		return GovernanceRawEventCursor{}, err
	}

	return cursor, nil
}

type ListGovernanceRawEventsInput struct {
	Scope           memory.Scope
	State           GovernanceRawEventState
	EventType       string
	AttemptGTE      *int
	AttemptLTE      *int
	FailedFrom      time.Time
	FailedTo        time.Time
	NextAttemptFrom time.Time
	NextAttemptTo   time.Time
	Limit           int
	Cursor          string
	Now             time.Time
}

func (i ListGovernanceRawEventsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.State != "" && !i.State.Valid() {
		return fmt.Errorf("state %q is invalid", i.State)
	}
	if i.AttemptGTE != nil && *i.AttemptGTE < 0 {
		return fmt.Errorf("attempt_gte must be greater than or equal to zero")
	}
	if i.AttemptLTE != nil && *i.AttemptLTE < 0 {
		return fmt.Errorf("attempt_lte must be greater than or equal to zero")
	}
	if i.AttemptGTE != nil && i.AttemptLTE != nil && *i.AttemptGTE > *i.AttemptLTE {
		return fmt.Errorf("attempt_gte must be less than or equal to attempt_lte")
	}
	if !i.FailedFrom.IsZero() && !i.FailedTo.IsZero() && i.FailedFrom.After(i.FailedTo) {
		return fmt.Errorf("failed_from must be before or equal to failed_to")
	}
	if !i.NextAttemptFrom.IsZero() && !i.NextAttemptTo.IsZero() && i.NextAttemptFrom.After(i.NextAttemptTo) {
		return fmt.Errorf("next_attempt_from must be before or equal to next_attempt_to")
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	if !i.Now.IsZero() {
		i.Now = i.Now.UTC()
	} else {
		return fmt.Errorf("current time is required")
	}
	if strings.TrimSpace(i.Cursor) != "" {
		if _, err := DecodeGovernanceRawEventCursor(i.Cursor); err != nil {
			return err
		}
	}

	return nil
}

type ReadGovernanceRawEventInput struct {
	Scope      memory.Scope
	RawEventID string
	Now        time.Time
}

func (i ReadGovernanceRawEventInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	case i.Now.IsZero():
		return fmt.Errorf("current time is required")
	default:
		return nil
	}
}

type ListGovernanceRecoveryHistoryInput struct {
	Scope      memory.Scope
	RawEventID string
}

func (i ListGovernanceRecoveryHistoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	default:
		return nil
	}
}

type GovernanceRawEvent struct {
	ID              string                  `json:"id"`
	Scope           memory.Scope            `json:"scope"`
	EventType       string                  `json:"event_type"`
	Content         string                  `json:"content"`
	SourceTimestamp time.Time               `json:"source_timestamp,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	State           GovernanceRawEventState `json:"state"`
	Attempt         int                     `json:"attempt"`
	WorkerID        string                  `json:"worker_id,omitempty"`
	ClaimedAt       time.Time               `json:"claimed_at,omitempty"`
	LeaseUntil      time.Time               `json:"lease_until,omitempty"`
	LastFailedAt    time.Time               `json:"last_failed_at,omitempty"`
	LastError       string                  `json:"last_error,omitempty"`
	NextAttemptAt   time.Time               `json:"next_attempt_at,omitempty"`
	ExhaustedAt     time.Time               `json:"exhausted_at,omitempty"`
	ProcessedAt     time.Time               `json:"processed_at,omitempty"`
}

func NewGovernanceRawEvent(event memory.RawEvent, snapshot RawEventGovernanceSnapshot, now time.Time) GovernanceRawEvent {
	return GovernanceRawEvent{
		ID:              event.ID,
		Scope:           event.Scope,
		EventType:       event.EventType,
		Content:         event.Content,
		SourceTimestamp: event.SourceTimestamp,
		CreatedAt:       event.CreatedAt,
		State:           snapshot.DerivedState(now.UTC()),
		Attempt:         snapshot.Attempt,
		WorkerID:        strings.TrimSpace(snapshot.WorkerID),
		ClaimedAt:       snapshot.ClaimedAt,
		LeaseUntil:      snapshot.LeaseUntil,
		LastFailedAt:    snapshot.LastFailedAt,
		LastError:       strings.TrimSpace(snapshot.LastError),
		NextAttemptAt:   snapshot.NextAttemptAt,
		ExhaustedAt:     snapshot.ExhaustedAt,
		ProcessedAt:     snapshot.ProcessedAt,
	}
}

type GovernanceRawEventPage struct {
	Items      []GovernanceRawEvent `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type GovernanceRecoverySnapshot struct {
	State         GovernanceRawEventState `json:"state"`
	Attempt       int                     `json:"attempt"`
	WorkerID      string                  `json:"worker_id,omitempty"`
	ClaimedAt     time.Time               `json:"claimed_at,omitempty"`
	LeaseUntil    time.Time               `json:"lease_until,omitempty"`
	LastFailedAt  time.Time               `json:"last_failed_at,omitempty"`
	LastError     string                  `json:"last_error,omitempty"`
	NextAttemptAt time.Time               `json:"next_attempt_at,omitempty"`
	ExhaustedAt   time.Time               `json:"exhausted_at,omitempty"`
	ProcessedAt   time.Time               `json:"processed_at,omitempty"`
}

func NewGovernanceRecoverySnapshot(snapshot RawEventGovernanceSnapshot, now time.Time) GovernanceRecoverySnapshot {
	return GovernanceRecoverySnapshot{
		State:         snapshot.DerivedState(now.UTC()),
		Attempt:       snapshot.Attempt,
		WorkerID:      strings.TrimSpace(snapshot.WorkerID),
		ClaimedAt:     snapshot.ClaimedAt,
		LeaseUntil:    snapshot.LeaseUntil,
		LastFailedAt:  snapshot.LastFailedAt,
		LastError:     strings.TrimSpace(snapshot.LastError),
		NextAttemptAt: snapshot.NextAttemptAt,
		ExhaustedAt:   snapshot.ExhaustedAt,
		ProcessedAt:   snapshot.ProcessedAt,
	}
}

type GovernanceRecoveryRecord struct {
	ID         string                     `json:"id"`
	RawEventID string                     `json:"raw_event_id"`
	Scope      memory.Scope               `json:"scope"`
	Action     GovernanceRecoveryAction   `json:"action"`
	Actor      string                     `json:"actor"`
	Reason     string                     `json:"reason"`
	Before     GovernanceRecoverySnapshot `json:"before"`
	After      GovernanceRecoverySnapshot `json:"after"`
	OccurredAt time.Time                  `json:"occurred_at"`
}

type GovernanceRecoveryOutcome struct {
	RawEvent GovernanceRawEvent     `json:"raw_event"`
	Recovery GovernanceRecoveryRecord `json:"recovery"`
}

type GovernanceRawEventReader interface {
	ListGovernanceRawEvents(ctx context.Context, input ListGovernanceRawEventsInput) (GovernanceRawEventPage, error)
	ReadGovernanceRawEvent(ctx context.Context, input ReadGovernanceRawEventInput) (GovernanceRawEvent, error)
	ListGovernanceRecoveryHistory(ctx context.Context, input ListGovernanceRecoveryHistoryInput) ([]GovernanceRecoveryRecord, error)
}

type GovernanceRecoveryApplier interface {
	ApplyGovernanceRecovery(ctx context.Context, input ApplyGovernanceRecoveryInput) (GovernanceRecoveryOutcome, error)
}

type ApplyGovernanceRecoveryInput struct {
	Scope        memory.Scope
	RawEventID   string
	Action       GovernanceRecoveryAction
	Actor        string
	Reason       string
	AppliedAt    time.Time
	ScheduledFor time.Time
}

func (i ApplyGovernanceRecoveryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	case !i.Action.Valid():
		return fmt.Errorf("recovery action %q is invalid", i.Action)
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case i.AppliedAt.IsZero():
		return fmt.Errorf("applied at is required")
	}

	switch i.Action {
	case GovernanceRecoveryActionReschedule:
		switch {
		case i.ScheduledFor.IsZero():
			return fmt.Errorf("scheduled_for is required")
		case !i.ScheduledFor.After(i.AppliedAt):
			return fmt.Errorf("%w: scheduled_for must be after applied at", ErrGovernanceRecoveryRejected)
		}
	default:
		if !i.ScheduledFor.IsZero() {
			return fmt.Errorf("%w: scheduled_for is only allowed for reschedule", ErrGovernanceRecoveryRejected)
		}
	}

	return nil
}

func ApplyGovernanceRecovery(current RawEventGovernanceSnapshot, input ApplyGovernanceRecoveryInput) (RawEventGovernanceSnapshot, error) {
	if err := input.Validate(); err != nil {
		return RawEventGovernanceSnapshot{}, err
	}

	state := current.DerivedState(input.AppliedAt.UTC())
	switch state {
	case GovernanceRawEventStateProcessed, GovernanceRawEventStateLeased:
		return RawEventGovernanceSnapshot{}, fmt.Errorf("%w: action %q is not allowed for state %q", ErrGovernanceRecoveryConflict, input.Action, state)
	}

	next := current
	switch input.Action {
	case GovernanceRecoveryActionRetry:
		if state != GovernanceRawEventStateRetryWait {
			return RawEventGovernanceSnapshot{}, fmt.Errorf("%w: action %q is not allowed for state %q", ErrGovernanceRecoveryConflict, input.Action, state)
		}
		next = clearClaimOwnership(next)
		next.NextAttemptAt = input.AppliedAt.UTC()
	case GovernanceRecoveryActionReschedule:
		if state != GovernanceRawEventStatePending && state != GovernanceRawEventStateRetryWait {
			return RawEventGovernanceSnapshot{}, fmt.Errorf("%w: action %q is not allowed for state %q", ErrGovernanceRecoveryConflict, input.Action, state)
		}
		next = clearClaimOwnership(next)
		next.NextAttemptAt = input.ScheduledFor.UTC()
	case GovernanceRecoveryActionRequeue:
		if state != GovernanceRawEventStateExhausted {
			return RawEventGovernanceSnapshot{}, fmt.Errorf("%w: action %q is not allowed for state %q", ErrGovernanceRecoveryConflict, input.Action, state)
		}
		next = clearClaimOwnership(next)
		next.Attempt = 0
		next.ExhaustedAt = time.Time{}
		next.NextAttemptAt = input.AppliedAt.UTC()
	default:
		return RawEventGovernanceSnapshot{}, fmt.Errorf("recovery action %q is invalid", input.Action)
	}

	return next, nil
}

func clearClaimOwnership(snapshot RawEventGovernanceSnapshot) RawEventGovernanceSnapshot {
	snapshot.WorkerID = ""
	snapshot.ClaimedAt = time.Time{}
	snapshot.LeaseUntil = time.Time{}
	return snapshot
}
