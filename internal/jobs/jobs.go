package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
)

type NoopWorker struct{}

func (NoopWorker) Start() error {
	return nil
}

type GovernanceWorker struct {
	Claimer       governance.RawEventClaimer
	Processor     governance.RawEventProcessor
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	Now           func() time.Time
}

func (w GovernanceWorker) RunOnce(ctx context.Context) (int, error) {
	if w.Claimer == nil {
		return 0, fmt.Errorf("governance raw event claimer is required")
	}

	if w.Processor == nil {
		return 0, fmt.Errorf("governance raw event processor is required")
	}

	now := time.Now
	if w.Now != nil {
		now = w.Now
	}

	input := governance.ClaimPendingRawEventsInput{
		WorkerID:      w.WorkerID,
		BatchSize:     w.BatchSize,
		LeaseDuration: w.LeaseDuration,
		Now:           now().UTC(),
	}
	if err := input.Validate(); err != nil {
		return 0, err
	}

	claims, err := w.Claimer.ClaimPendingRawEvents(ctx, input)
	if err != nil {
		return 0, err
	}

	for _, claim := range claims {
		if err := claim.Validate(); err != nil {
			return 0, err
		}

		if err := w.Processor.ProcessClaimedRawEvent(ctx, claim); err != nil {
			return 0, err
		}
	}

	return len(claims), nil
}

type NoopScheduler struct{}

func (NoopScheduler) Start() error {
	return nil
}
