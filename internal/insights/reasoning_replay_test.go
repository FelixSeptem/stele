package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

func TestPlanReasoningCandidateReplayProducesWouldActivateWithoutStoreMutation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeOffline, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: ReservedInsightActivationPolicyVersion, InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: now}
	digest, _ := reasoning.EvidenceDigest(request.Evidence)
	replayID, _ := reasoning.InsightReplayID(request)
	candidate := reasoning.InsightCandidate{ID: "candidate-1", Scope: scope, InsightType: request.InsightType, Title: "Hypothesis", Summary: "Evidence-backed", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	report, err := PlanReasoningCandidateReplay(reasoning.InsightReplayEnvelope{Request: request, Candidate: candidate, ReplayID: replayID}, policy, request.Evidence, now)
	if err != nil {
		t.Fatalf("plan reasoning replay: %v", err)
	}
	if report.Counters.WouldActivate != 1 || report.Decisions[0].Decision != memory.DerivedInsightReplayDecisionWouldActivate {
		t.Fatalf("report = %+v, want one would_activate decision", report)
	}
}
