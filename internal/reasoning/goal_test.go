package reasoning

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func validGoalRequest(t *testing.T) (InsightDerivationRequest, InsightCandidate) {
	t.Helper()
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "memory-goal-1", Relation: memory.DerivedInsightEvidenceRelationSupports, ObservedAt: now}
	validTo := now.Add(24 * time.Hour)
	goal := &GoalMetadata{State: GoalStateProposed, ReviewState: GoalReviewRequired, ValidFrom: &now, ValidTo: &validTo}
	request := InsightDerivationRequest{
		Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, InsightType: memory.DerivedInsightTypeGoal, Mode: ModeOffline,
		Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-goal-1", ScopeProof: "scope-proof-goal-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only",
		ProviderVersion: "provider-goal-v1", SchemaVersion: SchemaVersionV1, PolicyVersion: "goal-policy-v1", InputDigest: "goal-input-1", Limits: DefaultLimits(), Now: now, Goal: goal,
	}
	digest, err := EvidenceDigest(request.Evidence)
	if err != nil { t.Fatal(err) }
	replay, err := InsightReplayID(request)
	if err != nil { t.Fatal(err) }
	candidate := InsightCandidate{ID: "goal-candidate-1", Scope: request.Scope, InsightType: request.InsightType, Title: "Ship the governed goal path", Summary: "A bounded reviewable goal", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replay, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now, Goal: goal}
	return request, candidate
}

func TestGoalCandidateRequiresBoundedMetadataAndEvidence(t *testing.T) {
	request, candidate := validGoalRequest(t)
	if err := ValidateInsightCandidate(request, candidate); err != nil { t.Fatalf("valid goal rejected: %v", err) }

	bad := candidate
	bad.Goal = nil
	if err := bad.Validate(request.Limits); err == nil { t.Fatal("goal candidate without metadata accepted") }

	bad = candidate
	bad.Goal = &GoalMetadata{State: GoalState("running"), ReviewState: GoalReviewRequired}
	if err := bad.Validate(request.Limits); err == nil { t.Fatal("goal candidate with unsupported state accepted") }
}

func TestGoalReplayIdentityIncludesGoalStateAndValidity(t *testing.T) {
	request, _ := validGoalRequest(t)
	first, err := InsightReplayID(request)
	if err != nil { t.Fatal(err) }
	request.Goal.State = GoalStateCompleted
	second, err := InsightReplayID(request)
	if err != nil { t.Fatal(err) }
	if first == second { t.Fatal("goal state change did not change replay identity") }
}

func TestGoalCandidateRejectsExecutionRequest(t *testing.T) {
	request, candidate := validGoalRequest(t)
	candidate.Goal.ExecuteRequested = true
	if err := candidate.Validate(request.Limits); err == nil { t.Fatal("goal execution request accepted") }
}

func TestGoalShadowDerivationPreservesMetadataAndRejectsForeignProjectEvidence(t *testing.T) {
	request, candidate := validGoalRequest(t)
	request.Mode = ModeShadow
	candidate.Mode = ModeShadow
	candidate.ReplayID, _ = InsightReplayID(request)
	result := EvaluateInsightCandidate(context.Background(), insightProviderStub{candidate: candidate}, request)
	if result.Disposition != InsightDispositionWouldActivate || result.Authoritative || result.Candidate.Goal == nil {
		t.Fatalf("goal shadow result = %+v", result)
	}

	foreign := candidate
	foreign.Evidence = append([]memory.DerivedInsightEvidenceRef(nil), candidate.Evidence...)
	foreign.Evidence[0].Metadata = map[string]any{"tenant": request.Scope.Tenant, "project": "foreign-project", "namespace": request.Scope.Namespace}
	foreign.EvidenceDigest, _ = EvidenceDigest(foreign.Evidence)
	result = EvaluateInsightCandidate(context.Background(), insightProviderStub{candidate: foreign}, request)
	if result.Disposition != InsightDispositionQuarantined {
		t.Fatalf("foreign project disposition = %s, want quarantined", result.Disposition)
	}
}
