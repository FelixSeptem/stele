package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

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
