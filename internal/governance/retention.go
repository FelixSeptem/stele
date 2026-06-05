package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type RetentionTarget struct {
	MemoryID       string
	Scope          memory.Scope
	RetentionClass policy.RetentionClass
	UpdatedAt      time.Time
}

func (t RetentionTarget) Validate() error {
	switch {
	case t.MemoryID == "":
		return fmt.Errorf("memory id is required")
	case t.Scope.Validate() != nil:
		return t.Scope.Validate()
	case t.RetentionClass.Validate() != nil:
		return t.RetentionClass.Validate()
	case t.UpdatedAt.IsZero():
		return fmt.Errorf("updated at is required")
	default:
		return nil
	}
}

type RetentionProcessor struct {
	Policy     policy.RetentionPolicy
	Forgetting ForgettingProcessor
	Now        func() time.Time
}

func (p RetentionProcessor) Evaluate(ctx context.Context, target RetentionTarget) error {
	if err := target.Validate(); err != nil {
		return err
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	current := now().UTC()

	expired, err := p.Policy.Expired(target.RetentionClass, target.UpdatedAt, current)
	if err != nil {
		return err
	}
	if !expired {
		return nil
	}

	return p.Forgetting.Apply(ctx, LifecycleAction{
		MemoryID:  target.MemoryID,
		Scope:     target.Scope,
		Action:    policy.ForgettingActionExpire,
		AppliedAt: current,
	})
}
