package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

func TestRetentionProcessorAppliesExpiryWhenBoundaryReached(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	repo := &stubLifecycleRepository{}
	processor := RetentionProcessor{
		Policy: policy.DefaultRetentionPolicy(),
		Forgetting: ForgettingProcessor{
			Repository: repo,
			Now:        func() time.Time { return now },
		},
		Now: func() time.Time { return now },
	}

	err := processor.Evaluate(context.Background(), RetentionTarget{
		MemoryID:       "mem_123",
		Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RetentionClass: policy.RetentionClassEphemeral,
		UpdatedAt:      now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if len(repo.applied) != 1 {
		t.Fatalf("len(repo.applied) = %d, want %d", len(repo.applied), 1)
	}

	if repo.applied[0].Action != policy.ForgettingActionExpire {
		t.Fatalf("Action = %q, want %q", repo.applied[0].Action, policy.ForgettingActionExpire)
	}
}

func TestRetentionProcessorSkipsUnexpiredMemory(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	repo := &stubLifecycleRepository{}
	processor := RetentionProcessor{
		Policy: policy.DefaultRetentionPolicy(),
		Forgetting: ForgettingProcessor{
			Repository: repo,
			Now:        func() time.Time { return now },
		},
		Now: func() time.Time { return now },
	}

	err := processor.Evaluate(context.Background(), RetentionTarget{
		MemoryID:       "mem_123",
		Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RetentionClass: policy.RetentionClassDurable,
		UpdatedAt:      now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if len(repo.applied) != 0 {
		t.Fatalf("len(repo.applied) = %d, want %d", len(repo.applied), 0)
	}
}
