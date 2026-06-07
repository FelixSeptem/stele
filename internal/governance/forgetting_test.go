package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type stubLifecycleRepository struct {
	applied []LifecycleAction
	err     error
}

func (s *stubLifecycleRepository) ApplyLifecycleAction(ctx context.Context, action LifecycleAction) (memory.CanonicalMemory, error) {
	if s.err != nil {
		return memory.CanonicalMemory{}, s.err
	}

	s.applied = append(s.applied, action)
	return memory.CanonicalMemory{
		ID:         action.MemoryID,
		Scope:      action.Scope,
		Class:      memory.MemoryClassProfile,
		State:      action.TargetState(),
		Content:    action.Content,
		CreatedAt:  action.AppliedAt.Add(-time.Hour),
		ModifiedAt: action.AppliedAt,
	}, nil
}

type stubGovernanceObserver struct {
	operations []telemetry.OperationEvent
}

func (s *stubGovernanceObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubGovernanceObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {}

func TestLifecycleActionTargetState(t *testing.T) {
	now := time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}

	cases := []struct {
		action policy.ForgettingAction
		want   memory.MemoryState
	}{
		{action: policy.ForgettingActionSuppress, want: memory.MemoryStateSuppressed},
		{action: policy.ForgettingActionExpire, want: memory.MemoryStateForgotten},
		{action: policy.ForgettingActionDelete, want: memory.MemoryStateDeleted},
	}

	for _, tc := range cases {
		input := LifecycleAction{
			MemoryID:  "mem_123",
			Scope:     scope,
			Action:    tc.action,
			Content:   "payload",
			AppliedAt: now,
		}
		if got := input.TargetState(); got != tc.want {
			t.Fatalf("TargetState(%q) = %q, want %q", tc.action, got, tc.want)
		}
	}
}

func TestForgettingProcessorApplyRetentionAction(t *testing.T) {
	now := time.Date(2026, 6, 1, 17, 5, 0, 0, time.UTC)
	repo := &stubLifecycleRepository{}
	observer := &stubGovernanceObserver{}
	processor := ForgettingProcessor{
		Repository: repo,
		Now:        func() time.Time { return now },
		Observer:   observer,
	}

	err := processor.Apply(context.Background(), LifecycleAction{
		MemoryID:  "mem_123",
		Scope:     memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Action:    policy.ForgettingActionSuppress,
		Content:   "User prefers concise answers.",
		AppliedAt: now,
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(repo.applied) != 1 {
		t.Fatalf("len(repo.applied) = %d, want %d", len(repo.applied), 1)
	}

	if repo.applied[0].Action != policy.ForgettingActionSuppress {
		t.Fatalf("Action = %q, want %q", repo.applied[0].Action, policy.ForgettingActionSuppress)
	}

	if len(observer.operations) != 1 || observer.operations[0].Operation != "forget" {
		t.Fatalf("observer operations = %+v, want one forget operation", observer.operations)
	}
}
