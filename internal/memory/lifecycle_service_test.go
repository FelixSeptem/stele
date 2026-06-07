package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type stubLifecycleProcessor struct {
	gotAction memory.LifecycleActionRecord
	err       error
}

func (s *stubLifecycleProcessor) Apply(ctx context.Context, action memory.LifecycleActionRecord) error {
	s.gotAction = action
	return s.err
}

func TestLifecycleServiceApplySuppressRequiresActorAndReason(t *testing.T) {
	service := memory.LifecycleService{
		Now: func() time.Time { return time.Date(2026, 6, 7, 17, 0, 0, 0, time.UTC) },
	}

	err := service.Apply(context.Background(), memory.LifecycleActionInput{
		Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID: "mem_123",
		Action:   policy.ForgettingActionSuppress,
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want missing actor or reason error")
	}
}

func TestLifecycleServiceApplyNormalizesGovernanceAction(t *testing.T) {
	processor := &stubLifecycleProcessor{}
	service := memory.LifecycleService{
		Processor: processor,
		Now:       func() time.Time { return time.Date(2026, 6, 7, 17, 5, 0, 0, time.UTC) },
	}

	err := service.Apply(context.Background(), memory.LifecycleActionInput{
		Scope:     memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:  "mem_123",
		Action:    policy.ForgettingActionSuppress,
		Reason:    "manual override",
		Actor:     "operator-a",
		RequestID: "req_123",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if processor.gotAction.Actor != "operator-a" {
		t.Fatalf("Actor = %q, want operator-a", processor.gotAction.Actor)
	}

	if processor.gotAction.Reason != "manual override" {
		t.Fatalf("Reason = %q, want manual override", processor.gotAction.Reason)
	}

	if processor.gotAction.AppliedAt.IsZero() {
		t.Fatal("AppliedAt = zero, want normalized timestamp")
	}
}
