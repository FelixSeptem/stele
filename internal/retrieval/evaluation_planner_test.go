package retrieval

import (
	"encoding/json"
	"github.com/FelixSeptem/stele/internal/memory"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlannerFixtureCarriesVersionedExpectations(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "retrieval-planner-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture EvaluationFixture
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Validate(); err != nil {
		t.Fatal(err)
	}
	families := make(map[RetrievalQueryFamily]bool)
	aliases := make(map[string]bool)
	for _, item := range fixture.Cases {
		if item.Planner == nil {
			t.Fatalf("fixture case %q has no planner expectation", item.ID)
		}
		families[item.Planner.Family] = true
		for _, source := range item.Sources {
			if aliases[source.Alias] {
				t.Fatalf("planner fixture alias %q is not globally unique", source.Alias)
			}
			aliases[source.Alias] = true
		}
		if item.Planner.PlanIdentity == "" || len(item.Planner.Channels) == 0 {
			t.Fatalf("planner=%+v", item.Planner)
		}
	}
	for _, family := range retrievalQueryFamilies {
		if !families[family] {
			t.Fatalf("fixture missing family %q", family)
		}
	}
}

func TestPlannerFixtureRejectsMissingOrMismatchedIdentity(t *testing.T) {
	base := EvaluationPlannerExpectation{
		PlannerVersion:          string(RetrievalPlannerVersionV1),
		PolicyVersion:           string(RetrievalPlanPolicyVersionV1),
		Family:                  RetrievalQueryFamilyExactLookup,
		PlanIdentity:            "retrieval-planner-v1:retrieval-plan-policy-v1:exact_lookup",
		Channels:                []FusionChannel{FusionChannelLexical},
		MaxCandidates:           10,
		MaxCandidatesPerChannel: 10,
		ExpectedPasses:          1,
	}
	tests := []struct {
		name   string
		mutate func(*EvaluationPlannerExpectation)
	}{
		{"missing planner version", func(e *EvaluationPlannerExpectation) { e.PlannerVersion = "" }},
		{"missing policy version", func(e *EvaluationPlannerExpectation) { e.PolicyVersion = "" }},
		{"mismatched identity", func(e *EvaluationPlannerExpectation) { e.PlanIdentity = "planner:policy:general" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectation := base
			tt.mutate(&expectation)
			if err := expectation.validate(); err == nil {
				t.Fatalf("expectation accepted: %+v", expectation)
			}
		})
	}
}

func TestPlannerFixtureIdentityIncludesFallbackChannelAllocations(t *testing.T) {
	fallback := map[FusionChannel]int{FusionChannelSemantic: 1, FusionChannelRelation: 2}
	expectation := EvaluationPlannerExpectation{
		PlannerVersion:            string(RetrievalPlannerVersionV1),
		PolicyVersion:             string(RetrievalPlanPolicyVersionV1),
		Family:                    RetrievalQueryFamilyExactLookup,
		PlanIdentity:              RetrievalPlanIdentity{PlannerVersion: RetrievalPlannerVersionV1, PolicyVersion: RetrievalPlanPolicyVersionV1, Family: RetrievalQueryFamilyExactLookup, FallbackChannelCandidates: fallback}.String(),
		Channels:                  []FusionChannel{FusionChannelLexical},
		FallbackChannelCandidates: fallback,
		MaxCandidates:             10,
		MaxCandidatesPerChannel:   10,
		ExpectedPasses:            1,
	}
	if err := expectation.validate(); err != nil {
		t.Fatalf("fallback-aware expectation rejected: %v", err)
	}
	overlap := expectation
	overlap.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1}
	overlap.PlanIdentity = RetrievalPlanIdentity{PlannerVersion: RetrievalPlannerVersionV1, PolicyVersion: RetrievalPlanPolicyVersionV1, Family: RetrievalQueryFamilyExactLookup, FallbackChannelCandidates: overlap.FallbackChannelCandidates}.String()
	if err := overlap.validate(); err == nil {
		t.Fatal("planner fixture accepted fallback allocation overlapping a planned channel")
	}
}

func TestEvaluationMetadataRejectsPartialPlannerIdentity(t *testing.T) {
	metadata := EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1", PlannerVersion: string(RetrievalPlannerVersionV1)}
	if err := metadata.Validate(); err == nil {
		t.Fatal("partial planner metadata accepted")
	}
}

func TestPlannerEvaluationDeclaresSemanticAvailabilityOnlyForSemanticFamily(t *testing.T) {
	if embedding := evaluationPlannerQueryEmbedding(&EvaluationPlannerExpectation{Family: RetrievalQueryFamilySemantic}); len(embedding) == 0 {
		t.Fatal("semantic planner evaluation did not declare embedding availability")
	}
	if embedding := evaluationPlannerQueryEmbedding(&EvaluationPlannerExpectation{Family: RetrievalQueryFamilyGeneral}); len(embedding) != 0 {
		t.Fatalf("general planner evaluation received embedding=%v", embedding)
	}
}

func TestEvaluationReplayObservesActualPlannerExecution(t *testing.T) {
	input := plannerInputForTest(nil, nil, false)
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := RetrievalPlannerDiagnosticsFromExecution(RetrievalPlannerDiagnosticsInput{
		Plan: plan, RolloutStage: memory.RetrievalPlannerRolloutStageActive,
		Evidence:  EvidenceAssessment{Disposition: EvidenceDispositionSufficient, VisibleBucket: "one_to_five", AttritionBucket: "none"},
		PassCount: 1, CandidateCount: 1, Elapsed: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectation := EvaluationPlannerExpectation{PlannerVersion: string(plan.Identity.PlannerVersion), PolicyVersion: string(plan.Identity.PolicyVersion), Family: plan.Family, PlanIdentity: plan.Identity.String(), Channels: append([]FusionChannel(nil), plan.Channels...), MaxCandidates: plan.TotalCandidates, ExpectedPasses: 1, RerankerEligible: plan.RerankerEligible}
	caseRun := EvaluationReplayCase{ExpectedEvidenceGroups: [][]string{{"fact"}}, CandidatePoolSize: 1, Candidates: []EvaluationReplayCandidate{{Alias: "fact", MemoryID: "memory-fact", FinalRank: 1}}}
	result := SearchResult{retrievalPlan: &plan, plannerDiagnostics: []RetrievalPlannerDiagnostics{diagnostic}, retrievalPassObservations: []RetrievalPassObservation{{Pass: 1, CandidateCount: 1, VisibleMemoryIDs: []string{"memory-fact"}, Evidence: diagnosticEvidence(diagnostic), Latency: time.Millisecond}}, rerankerObservation: RetrievalRerankerObservation{Safe: true}}
	if err := evaluationReplayPlannerObservation(&caseRun, result, &expectation); err != nil {
		t.Fatal(err)
	}
	if caseRun.PlannerFamily != plan.Family || caseRun.PlannerIdentity != plan.Identity.String() || len(caseRun.Passes) != 1 || caseRun.Passes[0].EvidenceCoverage != 1 {
		t.Fatalf("caseRun=%+v", caseRun)
	}
	expectation.ExpectedPasses = 2
	if err := evaluationReplayPlannerObservation(&caseRun, result, &expectation); err == nil {
		t.Fatal("planner replay accepted fewer passes than the fixture requires")
	}
	diagnostic.PassCount = 2
	result.plannerDiagnostics = []RetrievalPlannerDiagnostics{diagnostic}
	result.retrievalPassObservations = append(result.retrievalPassObservations, RetrievalPassObservation{Pass: 2, CandidateCount: 0, VisibleMemoryIDs: []string{"memory-fact"}, Evidence: EvidenceAssessment{Disposition: EvidenceDispositionSufficient, VisibleBucket: "one_to_five", AttritionBucket: "none"}, Latency: 2 * time.Millisecond})
	if err := evaluationReplayPlannerObservation(&caseRun, result, &expectation); err != nil {
		t.Fatalf("actual two-pass observations rejected: %v", err)
	}
	if len(caseRun.Passes) != 2 || caseRun.Passes[0].Latency != time.Millisecond || caseRun.Passes[1].Latency != 2*time.Millisecond || caseRun.Passes[1].CandidateCount != 0 {
		t.Fatalf("caseRun passes=%+v", caseRun.Passes)
	}

	expectation.Family = RetrievalQueryFamilyExactLookup
	if err := evaluationReplayPlannerObservation(&caseRun, result, &expectation); err == nil {
		t.Fatal("mismatched planner execution accepted")
	}
}

func diagnosticEvidence(diagnostic RetrievalPlannerDiagnostics) EvidenceAssessment {
	return EvidenceAssessment{Disposition: diagnostic.EvidenceDisposition, VisibleBucket: diagnostic.VisibleBucket, AttritionBucket: diagnostic.AttritionBucket}
}

func TestEvaluationMetricsIncludeFamilyPassEvidenceAndResources(t *testing.T) {
	fixture := EvaluationFixture{Version: "retrieval-fixture-v1", Cases: []EvaluationCase{{ID: "case", Category: "single-fact", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Query: "q", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "fact"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}, Planner: &EvaluationPlannerExpectation{PlannerVersion: string(RetrievalPlannerVersionV1), PolicyVersion: string(RetrievalPlanPolicyVersionV1), Family: RetrievalQueryFamilyExactLookup, PlanIdentity: "retrieval-planner-v1:retrieval-plan-policy-v1:exact_lookup", Channels: []FusionChannel{FusionChannelLexical}, MaxCandidates: 10, Protected: true, ExpectedPasses: 2, FollowUpEligible: true}}}}
	if err := fixture.Validate(); err != nil {
		t.Fatal(err)
	}
	replay := EvaluationReplay{Metadata: EvaluationRankingMetadata{FixtureVersion: fixture.Version, RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, Cases: []EvaluationReplayCase{{CaseID: "case", Category: "single-fact", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, PlannerFamily: RetrievalQueryFamilyExactLookup, PlannerIdentity: "planner:policy:exact_lookup", Passes: []EvaluationReplayPass{{Pass: 1, CandidateCount: 8, VisibleCount: 1, EvidenceCoverage: .5, Latency: 2 * time.Millisecond, Evidence: EvidenceAssessment{Disposition: EvidenceDispositionBelowMinimum}}, {Pass: 2, CandidateCount: 2, VisibleCount: 2, EvidenceCoverage: 1, Latency: 1 * time.Millisecond, Evidence: EvidenceAssessment{Disposition: EvidenceDispositionSufficient}}}, RerankerUsed: false, RerankerSafe: true, FallbackCategory: "none", RollbackVerified: true}}}
	report, err := CalculateEvaluationMetrics(replay)
	if err != nil {
		t.Fatal(err)
	}
	caseReport := report.Cases[0]
	if caseReport.QueryFamily != RetrievalQueryFamilyExactLookup || caseReport.PlannerIdentity == "" || caseReport.PassCount != 2 || caseReport.SecondPassEvidenceGain <= 0 {
		t.Fatalf("case report=%+v", caseReport)
	}
	if report.Metrics.SecondPassCount != 1 || report.Metrics.FirstPassEvidenceCoverage <= 0 || report.Metrics.SecondPassEvidenceCoverage <= report.Metrics.FirstPassEvidenceCoverage {
		t.Fatalf("metrics=%+v", report.Metrics)
	}
	if report.Metrics.FirstPassEvidenceCoverage != .5 || report.Metrics.SecondPassEvidenceCoverage != 1 {
		t.Fatalf("explicit evidence coverage was replaced by candidate visibility: %+v", report.Metrics)
	}
}

func TestEvaluationMetricsDeriveRerankerAndRollbackSafetyFromObservations(t *testing.T) {
	replay := EvaluationReplay{
		Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"},
		Cases: []EvaluationReplayCase{{
			CaseID: "case", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"},
			PlannerFamily: RetrievalQueryFamilyGeneral, PlannerIdentity: "retrieval-planner-v1:retrieval-plan-policy-v1:general",
			Passes:       []EvaluationReplayPass{{Pass: 1, CandidateCount: 1, VisibleCount: 1, EvidenceCoverage: 1, Evidence: EvidenceAssessment{Disposition: EvidenceDispositionSufficient}}},
			RerankerUsed: true, RerankerSafe: false, RollbackVerified: false,
		}},
	}
	report, err := CalculateEvaluationMetrics(replay)
	if err != nil {
		t.Fatal(err)
	}
	if report.PlannerEvidence.RerankerSafe || report.PlannerEvidence.RollbackTested || report.Metrics.PlannerRerankerUseRate != 1 {
		t.Fatalf("planner evidence=%+v metrics=%+v", report.PlannerEvidence, report.Metrics)
	}
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, Planner: EvaluationPlannerReleasePolicy{RequireCompatible: true, MaxPasses: 2, MaxCandidateCount: 10, RequireRollbackTested: true}}
	baseline := EvaluationReport{Metadata: replay.Metadata}
	decision, err := EvaluateReleasePolicy(policy, baseline, report)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, evaluationDecisionPlannerReranker) || !containsString(decision.HardFailures, evaluationDecisionPlannerRollback) {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestEvaluateReleasePolicyRejectsEveryPlannerHardGate(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, Planner: EvaluationPlannerReleasePolicy{RequireCompatible: true, MaxPasses: 2, MaxCandidateCount: 20, MaxFallbackRate: 0, RequireRollbackTested: true}}
	baseline := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, Metrics: EvaluationMetricReport{RecallAt1: 1, P95LatencyMS: 10}}
	valid := EvaluationPlannerReleaseEvidence{Compatible: true, MaxPasses: 2, MaxCandidateCount: 20, RerankerSafe: true, RollbackTested: true}
	tests := []struct {
		name   string
		want   string
		mutate func(*EvaluationPlannerReleaseEvidence)
	}{
		{"compatibility", "planner_compatibility_failure", func(e *EvaluationPlannerReleaseEvidence) { e.Compatible = false }},
		{"safety", "planner_safety_failure", func(e *EvaluationPlannerReleaseEvidence) { e.SafetyFailures = 1 }},
		{"resource flag", "planner_resource_failure", func(e *EvaluationPlannerReleaseEvidence) { e.ResourceFailure = true }},
		{"pass limit", "planner_resource_failure", func(e *EvaluationPlannerReleaseEvidence) { e.MaxPasses = 3 }},
		{"candidate limit", "planner_resource_failure", func(e *EvaluationPlannerReleaseEvidence) { e.MaxCandidateCount = 21 }},
		{"fallback", "planner_fallback_failure", func(e *EvaluationPlannerReleaseEvidence) { e.FallbackRate = .1 }},
		{"reranker", "planner_reranker_failure", func(e *EvaluationPlannerReleaseEvidence) { e.RerankerSafe = false }},
		{"rollback", "planner_rollback_failure", func(e *EvaluationPlannerReleaseEvidence) { e.RollbackTested = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := baseline
			candidate.PlannerEvidence = valid
			tt.mutate(&candidate.PlannerEvidence)
			decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Eligible || !containsString(decision.HardFailures, tt.want) {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestEvaluateReleasePolicyRequiresCompatibilityAndRerankerSafetyWheneverPlannerGatesAreEnabled(t *testing.T) {
	policy := EvaluationReleasePolicy{
		Version:          "policy-v1",
		ProtectedCutoffs: []int{1},
		MaxP95LatencyMS:  100,
		Planner: EvaluationPlannerReleasePolicy{
			MaxPasses:         2,
			MaxCandidateCount: 20,
			MaxFallbackRate:   0,
		},
	}
	baseline := EvaluationReport{
		Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"},
		Metrics:  EvaluationMetricReport{RecallAt1: 1, P95LatencyMS: 10},
	}
	candidate := baseline
	candidate.PlannerEvidence = EvaluationPlannerReleaseEvidence{
		Compatible:        false,
		MaxPasses:         2,
		MaxCandidateCount: 20,
		RerankerSafe:      false,
		RollbackTested:    true,
	}

	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, evaluationDecisionPlannerCompatibility) || !containsString(decision.HardFailures, evaluationDecisionPlannerReranker) {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestLegacyReleasePolicyDoesNotImplicitlyEnablePlannerGates(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100}
	baseline := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, Metrics: EvaluationMetricReport{RecallAt1: 1, P95LatencyMS: 10}}
	candidate := baseline
	candidate.PlannerEvidence = EvaluationPlannerReleaseEvidence{FallbackRate: 1}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Eligible {
		t.Fatalf("legacy policy unexpectedly enabled planner gates: %+v", decision)
	}
}

func TestRenderEvaluationReportIncludesPlannerAggregatesOnly(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1", PlannerVersion: string(RetrievalPlannerVersionV1), PlannerPolicyVersion: string(RetrievalPlanPolicyVersionV1)}, Metrics: EvaluationMetricReport{FirstPassEvidenceCoverage: .5, SecondPassEvidenceCoverage: 1, SecondPassEvidenceGain: .5, SecondPassCount: 1, MaxPassesObserved: 2, MaxPlannerCandidates: 10, PlannerFallbackRate: .1}}
	text, err := RenderEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"planner_version=retrieval-planner-v1", "planner_policy_version=retrieval-plan-policy-v1", "first_pass_evidence_coverage=0.5000", "max_planner_candidates=10"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q: %s", want, text)
		}
	}
}

func TestMarshalEvaluationReportRejectsInvalidPlannerEvidence(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, PlannerEvidence: EvaluationPlannerReleaseEvidence{Compatible: true, MaxPasses: 3, FallbackRate: 2}}
	if _, err := MarshalEvaluationReport(report); err == nil {
		t.Fatal("invalid planner release evidence was serialized")
	}
}

func TestPlannerRealStackEvidenceRecordsStableNonPass(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("testdata", "retrieval-planner-real-stack-evidence-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Version         string `json:"version"`
		Status          string `json:"status"`
		Reason          string `json:"reason"`
		RealStack       bool   `json:"real_stack"`
		MaxRolloutStage string `json:"max_rollout_stage"`
	}
	if err := json.Unmarshal(payload, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Version != "retrieval-planner-real-stack-evidence-v1" || evidence.Status != "non_pass" || evidence.Reason != RetrievalEvaluationDSNSkip || evidence.RealStack || evidence.MaxRolloutStage != "shadow" {
		t.Fatalf("evidence=%+v", evidence)
	}
}

func TestEvaluateReleasePolicyRejectsPlannerHardFailures(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, Planner: EvaluationPlannerReleasePolicy{RequireCompatible: true, MaxPasses: 2, MaxCandidateCount: 20, MaxFallbackRate: 0, RequireRollbackTested: true}}
	baseline := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, Metrics: EvaluationMetricReport{RecallAt1: 1, P95LatencyMS: 10}}
	candidate := baseline
	candidate.PlannerEvidence = EvaluationPlannerReleaseEvidence{Compatible: true, SafetyFailures: 0, MaxPasses: 2, MaxCandidateCount: 20, FallbackRate: 0, RerankerSafe: true, RollbackTested: true}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Eligible {
		t.Fatalf("valid planner evidence rejected: %+v", decision)
	}
	candidate.PlannerEvidence.SafetyFailures = 1
	decision, err = EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, "planner_safety_failure") {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestEvaluationPlannerReleaseEvidenceRejectsFallbackAndRollback(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, Planner: EvaluationPlannerReleasePolicy{RequireCompatible: true, MaxPasses: 2, MaxCandidateCount: 20, MaxFallbackRate: 0, RequireRollbackTested: true}}
	baseline := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"}, Metrics: EvaluationMetricReport{RecallAt1: 1, P95LatencyMS: 10}}
	candidate := baseline
	candidate.PlannerEvidence = EvaluationPlannerReleaseEvidence{Compatible: true, MaxPasses: 2, MaxCandidateCount: 20, FallbackRate: .1, RerankerSafe: true, RollbackTested: false}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, "planner_fallback_failure") || !containsString(decision.HardFailures, "planner_rollback_failure") {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestPlannerRollbackEquivalentRequiresBaselinePublicResultsCitationsSafetyResourcesAndNoPlannerEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	baseline := EvaluationReplay{
		Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"},
		Cases: []EvaluationReplayCase{{
			CaseID: "case", Category: "protected", Scope: scope, CandidatePoolSize: 1,
			Candidates:          []EvaluationReplayCandidate{{Alias: "fact", MemoryID: "memory", Scope: scope, State: memory.MemoryStateActive, Lexical: true, FinalRank: 1, Citations: []Citation{{MemoryID: "memory", RawEventID: "event", Operation: "promote"}}}},
			ChannelAvailability: map[string]EvaluationChannelStatus{"lexical": EvaluationChannelStatusAvailable},
		}},
	}
	rollback := baseline
	rollback.Cases = append([]EvaluationReplayCase(nil), baseline.Cases...)
	rollback.Cases[0].Candidates = append([]EvaluationReplayCandidate(nil), baseline.Cases[0].Candidates...)

	if !PlannerRollbackEquivalent(baseline, rollback) {
		t.Fatal("equivalent rollback was rejected")
	}
	tests := []struct {
		name   string
		mutate func(*EvaluationReplay)
	}{
		{name: "planner identity", mutate: func(run *EvaluationReplay) { run.Cases[0].PlannerIdentity = "planner" }},
		{name: "planner passes", mutate: func(run *EvaluationReplay) {
			run.Cases[0].Passes = []EvaluationReplayPass{{Pass: 1, CandidateCount: 1}}
		}},
		{name: "public order", mutate: func(run *EvaluationReplay) { run.Cases[0].Candidates[0].FinalRank = 2 }},
		{name: "citations", mutate: func(run *EvaluationReplay) { run.Cases[0].Candidates[0].Citations[0].Operation = "different" }},
		{name: "safety", mutate: func(run *EvaluationReplay) { run.Cases[0].Candidates[0].Scope.Namespace = "foreign" }},
		{name: "resource", mutate: func(run *EvaluationReplay) { run.Cases[0].CandidatePoolSize = 2 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := rollback
			changed.Cases = append([]EvaluationReplayCase(nil), rollback.Cases...)
			changed.Cases[0].Candidates = append([]EvaluationReplayCandidate(nil), rollback.Cases[0].Candidates...)
			changed.Cases[0].Candidates[0].Citations = append([]Citation(nil), rollback.Cases[0].Candidates[0].Citations...)
			test.mutate(&changed)
			if PlannerRollbackEquivalent(baseline, changed) {
				t.Fatal("unsafe rollback proof was accepted")
			}
		})
	}
}

func TestAggregateEvaluationReplaySamplesUsesDeterministicMedianLatency(t *testing.T) {
	base := EvaluationReplay{
		Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v1", CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1"},
		Cases:    []EvaluationReplayCase{{CaseID: "case", CandidatePoolSize: 1, Latency: 30 * time.Millisecond, Passes: []EvaluationReplayPass{{Pass: 1, CandidateCount: 1, Latency: 9 * time.Millisecond}}}},
	}
	second, third := base, base
	second.Cases = append([]EvaluationReplayCase(nil), base.Cases...)
	second.Cases[0].Passes = append([]EvaluationReplayPass(nil), base.Cases[0].Passes...)
	second.Cases[0].Latency, second.Cases[0].Passes[0].Latency = 10*time.Millisecond, 3*time.Millisecond
	third.Cases = append([]EvaluationReplayCase(nil), base.Cases...)
	third.Cases[0].Passes = append([]EvaluationReplayPass(nil), base.Cases[0].Passes...)
	third.Cases[0].Latency, third.Cases[0].Passes[0].Latency = 20*time.Millisecond, 6*time.Millisecond

	aggregated, err := AggregateEvaluationReplaySamples([]EvaluationReplay{base, second, third})
	if err != nil {
		t.Fatal(err)
	}
	if aggregated.Cases[0].Latency != 20*time.Millisecond || aggregated.Cases[0].Passes[0].Latency != 6*time.Millisecond {
		t.Fatalf("case latency=%s pass latency=%s", aggregated.Cases[0].Latency, aggregated.Cases[0].Passes[0].Latency)
	}
	divergent := third
	divergent.Cases = append([]EvaluationReplayCase(nil), third.Cases...)
	divergent.Cases[0].CandidatePoolSize = 2
	if _, err := AggregateEvaluationReplaySamples([]EvaluationReplay{base, divergent}); err == nil {
		t.Fatal("structurally divergent latency samples were accepted")
	}
}

func TestOwnedRunnerRequiresExplicitOwnedPostgresDSN(t *testing.T) {
	if _, err := RunOwnedPlannerEvaluation("", "postgres://runtime", true); err == nil || !strings.Contains(err.Error(), RetrievalEvaluationDSNSkip) {
		t.Fatalf("err=%v", err)
	}
	if _, err := RunOwnedPlannerEvaluation("postgres://owned/eval", "postgres://owned/eval", true); err == nil {
		t.Fatal("runtime DSN reuse accepted")
	}
}
