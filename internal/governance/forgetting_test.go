package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
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
	processor := ForgettingProcessor{
		Repository: repo,
		Now:        func() time.Time { return now },
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
}
