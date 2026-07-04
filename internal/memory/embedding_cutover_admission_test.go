package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/diagnostics"
)

func TestEmbeddingCutoverPreflightInputValidate(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	if err := (EmbeddingCutoverPreflightInput{
		Scope:      scope,
		PlanID:     "plan_123",
		ObservedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	}).Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if err := (EmbeddingCutoverPreflightInput{Scope: scope}).Validate(); err == nil || err.Error() != "cutover plan id is required" {
		t.Fatalf("Validate() error = %v, want plan id validation", err)
	}
}

func TestEvaluateEmbeddingCutoverAdmissionAllowsWarningsOnly(t *testing.T) {
	report := EvaluateEmbeddingCutoverAdmission(EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"openai"},
	}, EmbeddingCutoverAdmissionSnapshot{
		Plan: EmbeddingCutoverPlan{
			ID:       "plan_123",
			Scope:    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status:   EmbeddingCutoverPlanStatusDraft,
			Target:   EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
			WaveSize: 2,
		},
		EligibleTotal: 5,
		ClassBreakdown: []EmbeddingCutoverClassBreakdown{
			{Class: MemoryClassProfile, Eligible: 3, Drifted: 2, MissingActiveVector: 1},
		},
	}, time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC))

	if report.Decision != diagnostics.AdmissionDecisionAllow {
		t.Fatalf("Decision = %q, want allow", report.Decision)
	}
	if report.EligibleTotal != 5 {
		t.Fatalf("EligibleTotal = %d, want 5", report.EligibleTotal)
	}
	for _, blocker := range report.Blockers {
		t.Fatalf("unexpected blocker: %+v", blocker)
	}
	if !hasFindingCode(report.Warnings, "semantic_drift") || !hasFindingCode(report.Warnings, "missing_active_vector") || !hasFindingCode(report.Warnings, "many_waves") {
		t.Fatalf("Warnings = %+v, want semantic_drift, missing_active_vector, many_waves", report.Warnings)
	}
}

func TestEvaluateEmbeddingCutoverAdmissionDeniesHardBlockers(t *testing.T) {
	conflict := EmbeddingCutoverPlanSummary{ID: "plan_active", Status: EmbeddingCutoverPlanStatusActive}
	report := EvaluateEmbeddingCutoverAdmission(EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"voyage"},
	}, EmbeddingCutoverAdmissionSnapshot{
		Plan: EmbeddingCutoverPlan{
			ID:       "plan_123",
			Scope:    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status:   EmbeddingCutoverPlanStatusDraft,
			Target:   EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
			WaveSize: 25,
		},
		EligibleTotal: 0,
		ClassBreakdown: []EmbeddingCutoverClassBreakdown{
			{Class: MemoryClassProfile, MissingRoute: 2},
		},
		ConflictingPlan: &conflict,
	}, time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC))

	if report.Decision != diagnostics.AdmissionDecisionDeny {
		t.Fatalf("Decision = %q, want deny", report.Decision)
	}
	for _, code := range []string{"target_unresolved", "unsupported_class_route", "scoped_plan_conflict", "zero_eligible_memory"} {
		if !hasFindingCode(report.Blockers, code) {
			t.Fatalf("Blockers = %+v, want code %q", report.Blockers, code)
		}
	}
	if report.ConflictingPlan == nil || report.ConflictingPlan.ID != "plan_active" {
		t.Fatalf("ConflictingPlan = %+v, want plan_active", report.ConflictingPlan)
	}
}

func TestEmbeddingAdminQueryServicePreflightsCutoverPlan(t *testing.T) {
	observedAt := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	store := &stubEmbeddingAdminStore{
		admissionSnapshot: EmbeddingCutoverAdmissionSnapshot{
			Plan: EmbeddingCutoverPlan{
				ID:       "plan_123",
				Scope:    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status:   EmbeddingCutoverPlanStatusDraft,
				Target:   EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize: 25,
			},
			EligibleTotal: 3,
			ClassBreakdown: []EmbeddingCutoverClassBreakdown{
				{Class: MemoryClassProfile, Eligible: 3},
			},
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"openai"},
	})

	report, err := service.PreflightEmbeddingCutoverPlan(context.Background(), EmbeddingCutoverPreflightInput{
		Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		PlanID:     "plan_123",
		ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatalf("PreflightEmbeddingCutoverPlan() error = %v", err)
	}
	if store.gotPreflight.PlanID != "plan_123" {
		t.Fatalf("got preflight plan id = %q, want plan_123", store.gotPreflight.PlanID)
	}
	if report.Decision != diagnostics.AdmissionDecisionAllow {
		t.Fatalf("Decision = %q, want allow", report.Decision)
	}
}

func TestEmbeddingAdminQueryServiceRejectsActivationWhenAdmissionDenied(t *testing.T) {
	store := &stubEmbeddingAdminStore{
		admissionSnapshot: EmbeddingCutoverAdmissionSnapshot{
			Plan: EmbeddingCutoverPlan{
				ID:       "plan_123",
				Scope:    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status:   EmbeddingCutoverPlanStatusDraft,
				Target:   EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize: 25,
			},
			EligibleTotal: 0,
		},
	}
	service := NewEmbeddingAdminQueryService(store, EmbeddingRuntimeStatus{
		Configured:             true,
		SemanticRebuildEnabled: true,
		RegisteredProviders:    []string{"openai"},
	})

	_, err := service.ApplyEmbeddingCutoverPlanAction(context.Background(), ApplyEmbeddingCutoverPlanActionInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		PlanID:    "plan_123",
		Action:    EmbeddingCutoverPlanActionActivate,
		Actor:     "operator-a",
		Reason:    "roll out",
		AppliedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrEmbeddingCutoverRejected) {
		t.Fatalf("error = %v, want ErrEmbeddingCutoverRejected", err)
	}
	if store.gotApplyCutover.PlanID != "" {
		t.Fatalf("got apply = %+v, want activation not forwarded", store.gotApplyCutover)
	}
}

func hasFindingCode(findings []diagnostics.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
