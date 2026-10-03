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
	Scope      Scope
	MemoryID   string
	MemoryPath string
	Action     policy.ForgettingAction
	Reason     string
	Actor      string
	RequestID  string
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
	Scope      Scope
	MemoryID   string
	MemoryPath string
	Action     policy.ForgettingAction
	Reason     string
	Actor      string
	RequestID  string
	AppliedAt  time.Time
}

func (s LifecycleService) Apply(ctx context.Context, input LifecycleActionInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.MemoryPath) != "" {
		path, err := NormalizeMemoryPath(input.MemoryPath)
		if err != nil {
			return err
		}
		input.MemoryPath = path
	}
	if s.Processor == nil {
		return fmt.Errorf("lifecycle processor is not configured")
	}
	precedence, err := EvaluateOperationPrecedence(OperationPrecedenceInput{
		Operation: "manual.lifecycle", Scope: input.Scope, GrantedScope: input.Scope,
		PrincipalID: strings.TrimSpace(input.Actor), GrantID: strings.TrimSpace(input.Actor),
		LifecycleChecked: true, LifecycleVisible: true, PrincipalGranted: strings.TrimSpace(input.Actor) != "",
		HandoffAllowed: true, MutationAllowed: true,
	})
	if err != nil {
		return fmt.Errorf("evaluate lifecycle precedence: %w", err)
	}
	if precedence.Outcome != OperationOutcomeAccepted {
		return fmt.Errorf("lifecycle precedence %s at %s", precedence.Outcome, precedence.Stage)
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}

	return s.Processor.Apply(ctx, LifecycleActionRecord{
		MemoryID:   input.MemoryID,
		MemoryPath: input.MemoryPath,
		Scope:      input.Scope,
		Action:     input.Action,
		Reason:     input.Reason,
		Actor:      input.Actor,
		RequestID:  input.RequestID,
		AppliedAt:  now().UTC(),
	})
}
