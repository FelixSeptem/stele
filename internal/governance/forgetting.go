package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type LifecycleAction struct {
	MemoryID  string
	Scope     memory.Scope
	Action    policy.ForgettingAction
	Content   string
	Reason    string
	Actor     string
	RequestID string
	AppliedAt time.Time
}

func (a LifecycleAction) Validate() error {
	switch {
	case strings.TrimSpace(a.MemoryID) == "":
		return fmt.Errorf("memory id is required")
	case a.Scope.Validate() != nil:
		return a.Scope.Validate()
	case a.Action.Validate() != nil:
		return a.Action.Validate()
	case a.AppliedAt.IsZero():
		return fmt.Errorf("applied at is required")
	default:
		return nil
	}
}

func (a LifecycleAction) TargetState() memory.MemoryState {
	switch a.Action {
	case policy.ForgettingActionSuppress:
		return memory.MemoryStateSuppressed
	case policy.ForgettingActionExpire:
		return memory.MemoryStateForgotten
	case policy.ForgettingActionDelete:
		return memory.MemoryStateDeleted
	default:
		return memory.MemoryStateActive
	}
}

type LifecycleRepository interface {
	ApplyLifecycleAction(ctx context.Context, action LifecycleAction) (memory.CanonicalMemory, error)
}

type ForgettingProcessor struct {
	Repository LifecycleRepository
	Now        func() time.Time
	Observer   telemetry.Observer
}

func (p ForgettingProcessor) Apply(ctx context.Context, action LifecycleAction) (err error) {
	started := time.Now()
	defer func() {
		if p.Observer == nil {
			return
		}

		now := time.Now
		if p.Now != nil {
			now = p.Now
		}
		observedAt := action.AppliedAt
		if observedAt.IsZero() {
			observedAt = now().UTC()
		}

		status := "ok"
		count := 1
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		p.Observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "scheduler",
			Component:  "forgetting_processor",
			Operation:  "forget",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: observedAt,
		})
	}()

	if err := action.Validate(); err != nil {
		return err
	}

	if p.Repository == nil {
		return fmt.Errorf("lifecycle repository is required")
	}

	if action.AppliedAt.IsZero() {
		now := time.Now
		if p.Now != nil {
			now = p.Now
		}
		action.AppliedAt = now().UTC()
	}

	_, err = p.Repository.ApplyLifecycleAction(ctx, action)
	return err
}
