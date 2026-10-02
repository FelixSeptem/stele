package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

func TestPlanReasoningCandidateReplayReportsContradictionDisposition(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	candidate := reasoning.InsightCandidate{ID: "contradiction-1", Scope: scope, InsightType: memory.DerivedInsightTypeContradiction, Title: "Conflict", Summary: "Conflict", Evidence: []memory.DerivedInsightEvidenceRef{{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "a", Relation: memory.DerivedInsightEvidenceRelationSupports}, {Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "b", Relation: memory.DerivedInsightEvidenceRelationSupports}}, LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-v1", SchemaVersion: "schema-v1", PolicyVersion: "policy-v1", SourceWatermark: "w1", ScopeProof: "proof", Mode: reasoning.ModeShadow, Uncertainty: 0.2, CreatedAt: now}
	digest, err := reasoning.EvidenceDigest(candidate.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	candidate.EvidenceDigest = digest
	candidate.Mode = reasoning.ModeOffline
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: candidate.InsightType, Mode: reasoning.ModeOffline, Evidence: candidate.Evidence, SourceWatermark: "w1", ScopeProof: "proof", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-v1", SchemaVersion: "schema-v1", PolicyVersion: "policy-v1", InputDigest: "input", Limits: reasoning.DefaultLimits(), Now: now}
	candidate.ReplayID, err = reasoning.InsightReplayID(request)
	if err != nil {
		t.Fatal(err)
	}
	envelope := reasoning.InsightReplayEnvelope{Request: request, Candidate: candidate, ReplayID: candidate.ReplayID}
	policy := DefaultReservedInsightActivationPolicy(scope)
	report, err := PlanReasoningCandidateReplay(envelope, policy, candidate.Evidence, now)
	if err != nil {
		t.Fatalf("PlanReasoningCandidateReplay() error = %v", err)
	}
	if len(report.Decisions) != 1 || report.Decisions[0].Decision != memory.DerivedInsightReplayDecisionReject {
		t.Fatalf("report decisions = %+v", report.Decisions)
	}
}
