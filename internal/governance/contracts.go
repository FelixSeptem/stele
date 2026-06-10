package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

var ErrClaimOwnershipLost = errors.New("claimed raw event ownership lost")

type ClaimPendingRawEventsInput struct {
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	Now           time.Time
}

func (i ClaimPendingRawEventsInput) Validate() error {
	switch {
	case strings.TrimSpace(i.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case i.BatchSize <= 0:
		return fmt.Errorf("batch size must be greater than zero")
	case i.LeaseDuration <= 0:
		return fmt.Errorf("lease duration must be greater than zero")
	case i.Now.IsZero():
		return fmt.Errorf("claim time is required")
	default:
		return nil
	}
}

type ClaimedRawEvent struct {
	Event      memory.RawEvent
	WorkerID   string
	ClaimedAt  time.Time
	LeaseUntil time.Time
	Attempt    int
}

func (c ClaimedRawEvent) Validate() error {
	switch {
	case strings.TrimSpace(c.Event.ID) == "":
		return fmt.Errorf("raw event id is required")
	case c.Event.Scope.Validate() != nil:
		return c.Event.Scope.Validate()
	case strings.TrimSpace(c.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case c.ClaimedAt.IsZero():
		return fmt.Errorf("claimed at is required")
	case !c.LeaseUntil.After(c.ClaimedAt):
		return fmt.Errorf("lease until must be after claimed at")
	case c.Attempt <= 0:
		return fmt.Errorf("attempt must be greater than zero")
	default:
		return nil
	}
}

type RawEventClaimer interface {
	ClaimPendingRawEvents(ctx context.Context, input ClaimPendingRawEventsInput) ([]ClaimedRawEvent, error)
}

type RecordClaimedRawEventFailureInput struct {
	RawEventID    string
	WorkerID      string
	FailedAt      time.Time
	ErrorMessage  string
	NextAttemptAt time.Time
	ExhaustedAt   time.Time
}

func (i RecordClaimedRawEventFailureInput) Validate() error {
	switch {
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	case strings.TrimSpace(i.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case i.FailedAt.IsZero():
		return fmt.Errorf("failed at is required")
	case strings.TrimSpace(i.ErrorMessage) == "":
		return fmt.Errorf("error message is required")
	case i.NextAttemptAt.IsZero() && i.ExhaustedAt.IsZero():
		return fmt.Errorf("next attempt at or exhausted at is required")
	case !i.NextAttemptAt.IsZero() && !i.ExhaustedAt.IsZero():
		return fmt.Errorf("next attempt at and exhausted at are mutually exclusive")
	case !i.NextAttemptAt.IsZero() && !i.NextAttemptAt.After(i.FailedAt):
		return fmt.Errorf("next attempt at must be after failed at")
	case !i.ExhaustedAt.IsZero() && i.ExhaustedAt.Before(i.FailedAt):
		return fmt.Errorf("exhausted at must not be before failed at")
	default:
		return nil
	}
}

type RawEventFailureRecorder interface {
	RecordClaimedRawEventFailure(ctx context.Context, input RecordClaimedRawEventFailureInput) error
}

type RenewClaimedRawEventLeaseInput struct {
	RawEventID string
	WorkerID   string
	RenewedAt  time.Time
	LeaseUntil time.Time
}

func (i RenewClaimedRawEventLeaseInput) Validate() error {
	switch {
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	case strings.TrimSpace(i.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case i.RenewedAt.IsZero():
		return fmt.Errorf("renewed at is required")
	case !i.LeaseUntil.After(i.RenewedAt):
		return fmt.Errorf("lease until must be after renewed at")
	default:
		return nil
	}
}

type RawEventLeaseRenewer interface {
	RenewClaimedRawEventLease(ctx context.Context, input RenewClaimedRawEventLeaseInput) error
}

type CompleteClaimedRawEventInput struct {
	RawEventID  string
	WorkerID    string
	ProcessedAt time.Time
}

func (i CompleteClaimedRawEventInput) Validate() error {
	switch {
	case strings.TrimSpace(i.RawEventID) == "":
		return fmt.Errorf("raw event id is required")
	case strings.TrimSpace(i.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case i.ProcessedAt.IsZero():
		return fmt.Errorf("processed at is required")
	default:
		return nil
	}
}

type RawEventCompletionRecorder interface {
	MarkRawEventProcessed(ctx context.Context, input CompleteClaimedRawEventInput) error
}

type RawEventProcessor interface {
	ProcessClaimedRawEvent(ctx context.Context, claim ClaimedRawEvent) error
}
