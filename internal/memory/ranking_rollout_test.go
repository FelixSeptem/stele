package memory

import (
	"testing"
	"time"
)

func TestRankingRolloutActivationGate(t *testing.T) {
	gate := RankingRolloutActivationGate{
		DryRunSucceeded:        true,
		EvidenceThresholdStatus: RankingRolloutThresholdStatusSatisfied,
		BlockersPresent:        false,
		AttributionRecorded:    true,
	}

	if !gate.CanActivate() {
		t.Fatal("CanActivate() = false, want true")
	}

	gate.BlockersPresent = true
	if gate.CanActivate() {
		t.Fatal("CanActivate() = true, want false when blockers are present")
	}
}

func TestRankingRolloutPolicyValidate(t *testing.T) {
	policy := RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status:          RankingRolloutPolicyStatusDraft,
		Mode:            RankingRolloutModeDryRun,
		Surfaces:        []RankingRolloutSurface{RankingRolloutSurfaceSearch},
		SignalSources:   []RankingRolloutSignalSource{RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: RankingRolloutThresholdStatusSatisfied,
		EvidenceMinimum: 2,
		Actor:           "operator-a",
		Reason:          "enable dry-run",
		CreatedAt:       time.Date(2026, 7, 12, 8, 20, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 7, 12, 8, 20, 0, 0, time.UTC),
	}

	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	policy.Mode = RankingRolloutMode("free_form")
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid rollout mode")
	}
}

func TestRecordRankingRolloutDryRunInputValidateAcceptsComparisonFields(t *testing.T) {
	input := RecordRankingRolloutDryRunInput{
		PolicyID:            "policy_1",
		Scope:               Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Surface:             RankingRolloutSurfaceContext,
		SignalSource:        RankingRolloutSignalSourceTaskEvaluations,
		ThresholdStatus:     RankingRolloutThresholdStatusSatisfied,
		BaselineRank:        1,
		AdjustedRank:        2,
		ChangedSubjectIDs:   []string{"mem_1"},
		ReasonCodes:         []RankingRolloutImpactReasonCode{RankingRolloutImpactReasonCodeSubjectBoosted},
		SignalCategories:    []string{"task_evaluations"},
		EvidenceCount:       3,
		HiddenEvidenceCount: 1,
		CreatedAt:           time.Date(2026, 7, 12, 8, 30, 0, 0, time.UTC),
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
