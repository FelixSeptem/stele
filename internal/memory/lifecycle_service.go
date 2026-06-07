package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/policy"
)

type LifecycleActionProcessor interface {
	Apply(ctx context.Context, action LifecycleActionRecord) error
}

type LifecycleActionInput struct {
	Scope     Scope
	MemoryID  string
	Action    policy.ForgettingAction
	Reason    string
	Actor     string
	RequestID string
}

func (i LifecycleActionInput) Validate() error {
	switch {
	case strings.TrimSpace(i.MemoryID) == "":
		return fmt.Errorf("memory id is required")
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case i.Action.Validate() != nil:
		return i.Action.Validate()
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	default:
		return nil
	}
}

type LifecycleService struct {
	Processor LifecycleActionProcessor
	Now       func() time.Time
}

type LifecycleActionRecord struct {
	Scope     Scope
	MemoryID  string
	Action    policy.ForgettingAction
	Reason    string
	Actor     string
	RequestID string
	AppliedAt time.Time
}

func (s LifecycleService) Apply(ctx context.Context, input LifecycleActionInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if s.Processor == nil {
		return fmt.Errorf("lifecycle processor is not configured")
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}

	return s.Processor.Apply(ctx, LifecycleActionRecord{
		MemoryID:  input.MemoryID,
		Scope:     input.Scope,
		Action:    input.Action,
		Reason:    input.Reason,
		Actor:     input.Actor,
		RequestID: input.RequestID,
		AppliedAt: now().UTC(),
	})
}
