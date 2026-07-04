package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEmbeddingAdminQueryServiceAppliesCutoverActionWhenRuntimeSupportsTarget(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		cutoverPlan: EmbeddingCutoverPlan{
			ID:     "plan_123",
			Scope:  Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status: EmbeddingCutoverPlanStatusDraft,
			Target: EmbeddingCutoverTarget{
				Provider:   "openai",
				Model:      "text-embedding-3-small",
				Dimensions: 1536,
			},
			WaveSize:  25,
			CreatedBy: "operator-a",
			CreatedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"openai"},
	})

	plan, err := service.ApplyEmbeddingCutoverPlanAction(context.Background(), ApplyEmbeddingCutoverPlanActionInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		PlanID:    "plan_123",
		Action:    EmbeddingCutoverPlanActionActivate,
		Actor:     "operator-b",
		Reason:    "roll out now",
		AppliedAt: time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ApplyEmbeddingCutoverPlanAction() error = %v", err)
	}

	if store.gotReadCutover.PlanID != "plan_123" {
		t.Fatalf("read plan id = %q, want plan_123", store.gotReadCutover.PlanID)
	}
	if store.gotApplyCutover.Action != EmbeddingCutoverPlanActionActivate {
		t.Fatalf("apply action = %q, want activate", store.gotApplyCutover.Action)
	}
	if plan.ID != "plan_123" {
		t.Fatalf("plan.ID = %q, want plan_123", plan.ID)
	}
}

func TestEmbeddingAdminQueryServiceRejectsCutoverActivationWhenRuntimeProviderMissing(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		cutoverPlan: EmbeddingCutoverPlan{
			ID:     "plan_123",
			Scope:  Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status: EmbeddingCutoverPlanStatusDraft,
			Target: EmbeddingCutoverTarget{
				Provider:   "openai",
				Model:      "text-embedding-3-small",
				Dimensions: 1536,
			},
			WaveSize:  25,
			CreatedBy: "operator-a",
			CreatedAt: time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"voyage"},
	})

	_, err := service.ApplyEmbeddingCutoverPlanAction(context.Background(), ApplyEmbeddingCutoverPlanActionInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		PlanID:    "plan_123",
		Action:    EmbeddingCutoverPlanActionActivate,
		Actor:     "operator-b",
		Reason:    "roll out now",
		AppliedAt: time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrEmbeddingCutoverRejected) {
		t.Fatalf("error = %v, want ErrEmbeddingCutoverRejected", err)
	}
	if store.gotApplyCutover.PlanID != "" {
		t.Fatalf("apply input = %+v, want action not forwarded", store.gotApplyCutover)
	}
}

func TestEmbeddingAdminQueryServiceListsRecoveryHistory(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		recoveryHistory: []EmbeddingRecoveryRecord{
			{
				ID:            "erl_123",
				MemoryID:      "mem_123",
				Scope:         Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				CutoverPlanID: "plan_123",
				Action:        EmbeddingRecoveryActionRetry,
				Actor:         "operator-a",
				Reason:        "retry now",
				OccurredAt:    time.Date(2026, 6, 28, 11, 30, 0, 0, time.UTC),
			},
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{})

	history, err := service.ListEmbeddingRecoveryHistory(context.Background(), ListEmbeddingRecoveryHistoryInput{
		Scope:         Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:      "mem_123",
		Action:        EmbeddingRecoveryActionRetry,
		CutoverPlanID: "plan_123",
		Limit:         25,
	})
	if err != nil {
		t.Fatalf("ListEmbeddingRecoveryHistory() error = %v", err)
	}

	if store.gotRecoveryHistory.MemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", store.gotRecoveryHistory.MemoryID)
	}
	if len(history) != 1 || history[0].CutoverPlanID != "plan_123" {
		t.Fatalf("history = %+v, want one cutover-linked record", history)
	}
}
