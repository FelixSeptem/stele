package reasoning

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type insightProviderStub struct {
	candidate InsightCandidate
	err       error
}

func (p insightProviderStub) DeriveInsight(context.Context, InsightDerivationRequest) (InsightCandidate, error) {
	return p.candidate, p.err
}

func validInsightRequest(t *testing.T) (InsightDerivationRequest, memory.DerivedInsightEvidenceRef) {
	t.Helper()
	evidence := memory.DerivedInsightEvidenceRef{
		Kind: memory.DerivedInsightEvidenceKindCanonicalMemory,
		ID:   "memory-1", Relation: memory.DerivedInsightEvidenceRelationSupports,
		ObservedAt: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
	return InsightDerivationRequest{
		Scope:       memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		InsightType: memory.DerivedInsightTypeHypothesis, Mode: ModeOffline,
		Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "scope-proof-1",
		ProviderVersion: "provider-v1", SchemaVersion: SchemaVersionV1, PolicyVersion: "policy-v1", InputDigest: "input-1",
		LifecycleVisibility: "active_only", RedactionPolicy: "references_only",
		Limits: DefaultLimits(), Now: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}, evidence
}

func validInsightCandidate(t *testing.T, request InsightDerivationRequest) InsightCandidate {
	t.Helper()
	digest, err := EvidenceDigest(request.Evidence)
	if err != nil {
		t.Fatalf("evidence digest: %v", err)
	}
	replayID, err := InsightReplayID(request)
	if err != nil {
		t.Fatalf("replay id: %v", err)
	}
	return InsightCandidate{
		ID: "candidate-1", Scope: request.Scope, InsightType: request.InsightType,
		Title: "Bounded hypothesis", Summary: "Evidence-backed hypothesis", Evidence: request.Evidence,
		EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof,
		ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion,
		LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy,
		ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode,
		CreatedAt: request.Now,
	}
}

func TestInsightCandidateAllowsReservedTypesOnlyInOfflineOrShadowEnvelope(t *testing.T) {
	request, _ := validInsightRequest(t)
	candidate := validInsightCandidate(t, request)
	if err := ValidateInsightCandidate(request, candidate); err != nil {
		t.Fatalf("valid reasoning candidate rejected: %v", err)
	}
	bad := candidate
	bad.Mode = ModeLive
	if err := bad.Validate(request.Limits); err == nil {
		t.Fatal("live reasoning candidate accepted")
	}
	bad = candidate
	bad.DirectActivation = true
	if err := bad.Validate(request.Limits); err == nil {
		t.Fatal("direct activation candidate accepted")
	}
}

func TestInsightReplayIdentityIsOrderIndependentAndScopeBound(t *testing.T) {
	request, evidence := validInsightRequest(t)
	other := evidence
	other.ID = "memory-2"
	request.Evidence = append(request.Evidence, other)
	first, err := InsightReplayID(request)
	if err != nil {
		t.Fatalf("first replay id: %v", err)
	}
	request.Evidence[0], request.Evidence[1] = request.Evidence[1], request.Evidence[0]
	second, err := InsightReplayID(request)
	if err != nil {
		t.Fatalf("second replay id: %v", err)
	}
	if first != second {
		t.Fatalf("replay id changed after evidence reorder: %q != %q", first, second)
	}
	request.Scope.Namespace = "foreign"
	foreign, err := InsightReplayID(request)
	if err != nil {
		t.Fatalf("foreign replay id: %v", err)
	}
	if foreign == first {
		t.Fatal("scope change did not change replay identity")
	}
}

func TestEvaluateInsightCandidateIsShadowOnlyAndQuarantinesForeignEvidence(t *testing.T) {
	request, evidence := validInsightRequest(t)
	request.Mode = ModeShadow
	candidate := validInsightCandidate(t, request)
	result := EvaluateInsightCandidate(context.Background(), insightProviderStub{candidate: candidate}, request)
	if result.Disposition != InsightDispositionWouldActivate || result.Authoritative {
		t.Fatalf("shadow result = %+v", result)
	}
	bad := candidate
	bad.Evidence = append([]memory.DerivedInsightEvidenceRef{{Kind: evidence.Kind, ID: "foreign", Relation: evidence.Relation}}, bad.Evidence...)
	bad.EvidenceDigest, _ = EvidenceDigest(bad.Evidence)
	result = EvaluateInsightCandidate(context.Background(), insightProviderStub{candidate: bad}, request)
	if result.Disposition != InsightDispositionQuarantined {
		t.Fatalf("foreign evidence disposition = %s, want quarantined", result.Disposition)
	}
}
