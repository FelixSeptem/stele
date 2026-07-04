package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubEmbeddingAdminStore struct {
	gotListInput       ListEmbeddingRebuildsInput
	gotReadScope       Scope
	gotReadMemoryID    string
	gotApplyInput      ApplyEmbeddingRecoveryInput
	gotCreateCutover   CreateEmbeddingCutoverPlanInput
	gotListCutovers    ListEmbeddingCutoverPlansInput
	gotReadCutover     ReadEmbeddingCutoverPlanInput
	gotPreflight       EmbeddingCutoverPreflightInput
	gotApplyCutover    ApplyEmbeddingCutoverPlanActionInput
	gotRecoveryHistory ListEmbeddingRecoveryHistoryInput
	items              []EmbeddingRebuildView
	inspection         EmbeddingMemoryInspection
	outcome            EmbeddingRecoveryOutcome
	cutoverPlan        EmbeddingCutoverPlan
	admissionSnapshot  EmbeddingCutoverAdmissionSnapshot
	cutoverPlans       []EmbeddingCutoverPlan
	recoveryHistory    []EmbeddingRecoveryRecord
	err                error
	applyErr           error
	cutoverErr         error
}

func (s *stubEmbeddingAdminStore) ListEmbeddingRebuilds(ctx context.Context, input ListEmbeddingRebuildsInput) ([]EmbeddingRebuildView, error) {
	s.gotListInput = input
	return s.items, s.err
}

func (s *stubEmbeddingAdminStore) ReadMemoryEmbedding(ctx context.Context, scope Scope, memoryID string) (EmbeddingMemoryInspection, error) {
	s.gotReadScope = scope
	s.gotReadMemoryID = memoryID
	return s.inspection, s.err
}

func (s *stubEmbeddingAdminStore) ApplyEmbeddingRecovery(ctx context.Context, input ApplyEmbeddingRecoveryInput) (EmbeddingRecoveryOutcome, error) {
	s.gotApplyInput = input
	return s.outcome, s.applyErr
}

func (s *stubEmbeddingAdminStore) CreateEmbeddingCutoverPlan(ctx context.Context, input CreateEmbeddingCutoverPlanInput) (EmbeddingCutoverPlan, error) {
	s.gotCreateCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminStore) ListEmbeddingCutoverPlans(ctx context.Context, input ListEmbeddingCutoverPlansInput) ([]EmbeddingCutoverPlan, error) {
	s.gotListCutovers = input
	return s.cutoverPlans, s.cutoverErr
}

func (s *stubEmbeddingAdminStore) ReadEmbeddingCutoverPlan(ctx context.Context, input ReadEmbeddingCutoverPlanInput) (EmbeddingCutoverPlan, error) {
	s.gotReadCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminStore) ReadEmbeddingCutoverAdmission(ctx context.Context, input EmbeddingCutoverPreflightInput) (EmbeddingCutoverAdmissionSnapshot, error) {
	s.gotPreflight = input
	if s.admissionSnapshot.Plan.ID == "" {
		s.admissionSnapshot.Plan = s.cutoverPlan
		s.admissionSnapshot.EligibleTotal = 1
	}
	return s.admissionSnapshot, s.cutoverErr
}

func (s *stubEmbeddingAdminStore) ApplyEmbeddingCutoverPlanAction(ctx context.Context, input ApplyEmbeddingCutoverPlanActionInput) (EmbeddingCutoverPlan, error) {
	s.gotApplyCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminStore) ListEmbeddingRecoveryHistory(ctx context.Context, input ListEmbeddingRecoveryHistoryInput) ([]EmbeddingRecoveryRecord, error) {
	s.gotRecoveryHistory = input
	return s.recoveryHistory, s.cutoverErr
}

func TestEmbeddingAdminQueryServiceListRebuildsIncludesRuntimeStatus(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		items: []EmbeddingRebuildView{
			{
				MemoryID:            "mem_123",
				Scope:               Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:               MemoryClassProfile,
				State:               MemoryStateActive,
				Status:              EmbeddingRebuildStatusFailed,
				RequestedProvider:   "openai",
				RequestedModel:      "text-embedding-3-small",
				RequestedDimensions: 1536,
				FailureReason:       "provider unavailable",
				Drifted:             true,
				RequestedAt:         time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             false,
		SemanticRebuildEnabled: false,
		Reason:                 "semantic rebuild execution is inactive because no embedding routes are configured",
	})

	page, err := service.ListEmbeddingRebuilds(context.Background(), ListEmbeddingRebuildsInput{
		Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListEmbeddingRebuilds() error = %v", err)
	}

	if store.gotListInput.Scope.Namespace != "namespace-a" {
		t.Fatalf("scope = %+v, want request scope", store.gotListInput.Scope)
	}
	if !page.Items[0].Drifted {
		t.Fatalf("page.Items[0].Drifted = false, want true")
	}
	if page.Runtime.SemanticRebuildEnabled {
		t.Fatal("page.Runtime.SemanticRebuildEnabled = true, want false")
	}
	if page.Runtime.Reason == "" {
		t.Fatal("page.Runtime.Reason = empty, want degraded runtime reason")
	}
}

func TestEmbeddingAdminQueryServiceGetMemoryEmbeddingIncludesRuntimeStatus(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		inspection: EmbeddingMemoryInspection{
			Memory: EmbeddingMemorySummary{
				ID:                   "mem_123",
				Scope:                Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:                MemoryClassProfile,
				State:                MemoryStateActive,
				CurrentSourceVersion: 3,
				CurrentContentHash:   "hash_123",
			},
			Rebuild: EmbeddingRebuildView{
				MemoryID:             "mem_123",
				Scope:                Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status:               EmbeddingRebuildStatusCurrent,
				RequestedProvider:    "openai",
				RequestedModel:       "text-embedding-3-small",
				RequestedDimensions:  1536,
				ActiveVectorRevision: "vec_active",
			},
			Revisions: []EmbeddingVectorRevisionView{
				{
					ID:            "vec_active",
					Provider:      "openai",
					Model:         "text-embedding-3-small",
					Dimensions:    1536,
					Status:        VectorRevisionStatusActive,
					SourceVersion: 3,
					GeneratedAt:   time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"openai"},
	})

	inspection, err := service.GetMemoryEmbedding(context.Background(), Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, "mem_123")
	if err != nil {
		t.Fatalf("GetMemoryEmbedding() error = %v", err)
	}

	if store.gotReadMemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", store.gotReadMemoryID)
	}
	if !inspection.Runtime.SemanticRebuildEnabled {
		t.Fatal("inspection.Runtime.SemanticRebuildEnabled = false, want true")
	}
	if len(inspection.Revisions) != 1 || inspection.Revisions[0].ID != "vec_active" {
		t.Fatalf("revisions = %+v, want active revision detail", inspection.Revisions)
	}
}

func TestApplyEmbeddingRecoveryRejectsRebuildingStatus(t *testing.T) {
	_, err := ApplyEmbeddingRecovery(EmbeddingRebuildView{
		MemoryID: "mem_123",
		Status:   EmbeddingRebuildStatusRebuilding,
	}, ApplyEmbeddingRecoveryInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:  "mem_123",
		Action:    EmbeddingRecoveryActionRetry,
		Actor:     "operator-a",
		Reason:    "retry now",
		AppliedAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrEmbeddingRecoveryConflict) {
		t.Fatalf("error = %v, want ErrEmbeddingRecoveryConflict", err)
	}
}

func TestApplyEmbeddingRecoveryTransitionsFailedRetryToPending(t *testing.T) {
	appliedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	next, err := ApplyEmbeddingRecovery(EmbeddingRebuildView{
		MemoryID:      "mem_123",
		Status:        EmbeddingRebuildStatusFailed,
		FailureReason: "provider unavailable",
		RequestedAt:   appliedAt.Add(-time.Hour),
	}, ApplyEmbeddingRecoveryInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:  "mem_123",
		Action:    EmbeddingRecoveryActionRetry,
		Actor:     "operator-a",
		Reason:    "retry now",
		AppliedAt: appliedAt,
	})
	if err != nil {
		t.Fatalf("ApplyEmbeddingRecovery() error = %v", err)
	}

	if next.Status != EmbeddingRebuildStatusPending {
		t.Fatalf("Status = %q, want pending", next.Status)
	}
	if next.FailureReason != "" {
		t.Fatalf("FailureReason = %q, want empty", next.FailureReason)
	}
	if !next.RequestedAt.Equal(appliedAt) {
		t.Fatalf("RequestedAt = %v, want %v", next.RequestedAt, appliedAt)
	}
}

func TestApplyEmbeddingRecoveryTransitionsCurrentRequeueToPending(t *testing.T) {
	appliedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	next, err := ApplyEmbeddingRecovery(EmbeddingRebuildView{
		MemoryID:             "mem_123",
		Status:               EmbeddingRebuildStatusCurrent,
		RequestedProvider:    "openai",
		RequestedModel:       "text-embedding-3-small",
		RequestedDimensions:  1536,
		ActiveVectorRevision: "vec_active",
		RequestedAt:          appliedAt.Add(-time.Hour),
	}, ApplyEmbeddingRecoveryInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:  "mem_123",
		Action:    EmbeddingRecoveryActionRequeue,
		Actor:     "operator-a",
		Reason:    "refresh with current routing",
		AppliedAt: appliedAt,
	})
	if err != nil {
		t.Fatalf("ApplyEmbeddingRecovery() error = %v", err)
	}

	if next.Status != EmbeddingRebuildStatusPending {
		t.Fatalf("Status = %q, want pending", next.Status)
	}
	if !next.RequestedAt.Equal(appliedAt) {
		t.Fatalf("RequestedAt = %v, want %v", next.RequestedAt, appliedAt)
	}
	if next.ActiveVectorRevision != "vec_active" {
		t.Fatalf("ActiveVectorRevision = %q, want vec_active", next.ActiveVectorRevision)
	}
}

func TestEmbeddingAdminQueryServiceAppliesRecoveryAction(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		outcome: EmbeddingRecoveryOutcome{
			Rebuild: EmbeddingRebuildView{
				MemoryID: "mem_123",
				Status:   EmbeddingRebuildStatusPending,
			},
			Recovery: EmbeddingRecoveryRecord{
				ID:         "erl_1",
				MemoryID:   "mem_123",
				Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Action:     EmbeddingRecoveryActionRetry,
				Actor:      "operator-a",
				Reason:     "retry now",
				OccurredAt: time.Date(2026, 6, 28, 12, 5, 0, 0, time.UTC),
			},
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{})

	outcome, err := service.ApplyEmbeddingRecovery(context.Background(), ApplyEmbeddingRecoveryInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:  "mem_123",
		Action:    EmbeddingRecoveryActionRetry,
		Actor:     "operator-a",
		Reason:    "retry now",
		AppliedAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ApplyEmbeddingRecovery() error = %v", err)
	}

	if store.gotApplyInput.Action != EmbeddingRecoveryActionRetry {
		t.Fatalf("Action = %q, want retry", store.gotApplyInput.Action)
	}
	if outcome.Recovery.ID != "erl_1" {
		t.Fatalf("Recovery.ID = %q, want erl_1", outcome.Recovery.ID)
	}
}
