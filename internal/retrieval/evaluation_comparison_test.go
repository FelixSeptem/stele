package retrieval

import (
	"strings"
	"testing"
)

func TestCompareEvaluationReportsRejectsIncompatibleStrategyAndPolicyMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*EvaluationReport)
		want   string
	}{
		{name: "representation", mutate: func(report *EvaluationReport) { report.Metadata.RepresentationVersion = "representation-v2" }, want: "incompatible representation version"},
		{name: "fusion", mutate: func(report *EvaluationReport) { report.Metadata.FusionStrategy = "normalized_weighted:v1" }, want: "incompatible fusion strategy"},
		{name: "ranking", mutate: func(report *EvaluationReport) { report.Metadata.RankingVersion = "ranking-v2" }, want: "incompatible ranking version"},
		{name: "analysis", mutate: func(report *EvaluationReport) { report.Metadata.AnalysisVersion = "analysis-v2" }, want: "incompatible analysis version"},
		{name: "release policy", mutate: func(report *EvaluationReport) { report.Metadata.PolicyVersion = "release-policy-v2" }, want: "incompatible release policy version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := compatibleAnalysisComparisonReport("original_only")
			candidate := compatibleAnalysisComparisonReport("active_for_scope")
			test.mutate(&candidate)

			_, err := CompareEvaluationReports(baseline, candidate, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("CompareEvaluationReports() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCompareEvaluationReportsReturnsStableCompatibilityCodes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*EvaluationReport)
		want   CompatibilityCode
	}{
		{"fixture", func(r *EvaluationReport) { r.Metadata.FixtureVersion = "fixture-v2" }, CompatibilityCodeFixtureVersion},
		{"representation", func(r *EvaluationReport) { r.Metadata.RepresentationVersion = "representation-v2" }, CompatibilityCodeRepresentationVersion},
		{"fusion", func(r *EvaluationReport) { r.Metadata.FusionStrategy = "rrf:v2" }, CompatibilityCodeFusionStrategy},
		{"ranking", func(r *EvaluationReport) { r.Metadata.RankingVersion = "ranking-v2" }, CompatibilityCodeRankingVersion},
		{"embedding", func(r *EvaluationReport) { r.Metadata.CompatibleEmbeddingRevision = "embedding-v2" }, CompatibilityCodeEmbeddingRevision},
		{"provider", func(r *EvaluationReport) { r.Metadata.EmbeddingProvider = "provider-v2" }, CompatibilityCodeEmbeddingProvider},
		{"analysis", func(r *EvaluationReport) { r.Metadata.AnalysisVersion = "analysis-v2" }, CompatibilityCodeAnalysisVersion},
		{"policy", func(r *EvaluationReport) { r.Metadata.PolicyVersion = "release-policy-v2" }, CompatibilityCodePolicyVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := compatibleAnalysisComparisonReport("original_only")
			candidate := compatibleAnalysisComparisonReport("active_for_scope")
			tt.mutate(&candidate)
			_, err := CompareEvaluationReports(baseline, candidate, nil)
			if err == nil {
				t.Fatal("CompareEvaluationReports() error = nil")
			}
			coded, ok := err.(interface{ CompatibilityCode() CompatibilityCode })
			if !ok || coded.CompatibilityCode() != tt.want {
				t.Fatalf("error=%T %v, code=%v want %v", err, err, coded, tt.want)
			}
		})
	}
}

func TestCompareEvaluationReportsRequiresImmutableOriginalQueryBaseline(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("shadow")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")

	_, err := CompareEvaluationReports(baseline, candidate, nil)
	if err == nil || !strings.Contains(err.Error(), "immutable original-query baseline") {
		t.Fatalf("CompareEvaluationReports() error = %v, want immutable original-query baseline rejection", err)
	}
}

func TestCompareEvaluationReportsSafetyFailuresOverrideAggregateGains(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	baseline.Metrics.RecallAt10 = 0.5
	candidate.Metrics.RecallAt10 = 1
	candidate.SafetyFailures = []EvaluationSafetyFailure{
		{Category: EvaluationSafetyFailureCrossScope, Count: 1},
		{Category: EvaluationSafetyFailureLifecycleVisibility, Count: 1},
	}

	comparison, err := CompareEvaluationReports(baseline, candidate, nil)
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.SafetyGatePassed {
		t.Fatalf("SafetyGatePassed = true, want false despite aggregate gains")
	}
	if len(comparison.SafetyFailures) != 2 {
		t.Fatalf("SafetyFailures = %+v, want bounded isolation and lifecycle failures", comparison.SafetyFailures)
	}
}

func TestGraphHardSafetyFailureOverridesQualityGain(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	baseline.Metrics.RecallAt10 = 0.5
	candidate.Metrics.RecallAt10 = 1
	candidate.SafetyFailures = []EvaluationSafetyFailure{{Category: EvaluationSafetyFailureGraphNondeterministic, Count: 1}}
	comparison, err := CompareEvaluationReports(baseline, candidate, nil)
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.SafetyGatePassed || len(comparison.SafetyFailures) != 1 || comparison.SafetyFailures[0].Category != EvaluationSafetyFailureGraphNondeterministic {
		t.Fatalf("graph safety comparison = %+v", comparison)
	}
}

func TestGraphTraversalEvidenceIsBoundedAndRedacted(t *testing.T) {
	report := compatibleAnalysisComparisonReport("active_for_scope")
	report.GraphTraversalEvidence = &EvaluationGraphTraversalEvidence{CandidateCountByHop: map[string]int{"one": 3, "two": 1}, TruncationRate: 0.25, FallbackRate: 0.1, CitationCoverage: 1, EligibleRelationRate: 0.8, LatencyBucket: "p95_250ms", QualityDelta: 0.2}
	encoded, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	for _, forbidden := range []string{"query_text", "tenant", "memory_id", "raw_score", "edge_id"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("graph evidence leaked %q: %s", forbidden, encoded)
		}
	}
	report.GraphTraversalEvidence.CandidateCountByHop["four"] = 1
	if _, err := MarshalEvaluationReport(report); err == nil {
		t.Fatal("expected invalid graph hop bucket to be rejected")
	}
}

func TestCompareEvaluationReportsValidatesPlannerIdentityCompatibility(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	candidate.Metadata.PlannerVersion = "unsupported-planner"
	candidate.Metadata.PlannerPolicyVersion = string(RetrievalPlanPolicyVersionV1)
	_, err := CompareEvaluationReports(baseline, candidate, nil)
	if err == nil {
		t.Fatal("unknown planner identity accepted")
	}
	coded, ok := err.(interface{ CompatibilityCode() CompatibilityCode })
	if !ok || coded.CompatibilityCode() != CompatibilityCodePlannerVersion {
		t.Fatalf("error=%T %v, code=%v", err, err, coded)
	}

	candidate.Metadata.PlannerVersion = string(RetrievalPlannerVersionV1)
	candidate.Metadata.PlannerPolicyVersion = string(RetrievalPlanPolicyVersionV1)
	if _, err := CompareEvaluationReports(baseline, candidate, nil); err != nil {
		t.Fatalf("supported planner identity rejected when baseline is legacy: %v", err)
	}

	baseline.Metadata.PlannerVersion = string(RetrievalPlannerVersionV1)
	baseline.Metadata.PlannerPolicyVersion = string(RetrievalPlanPolicyVersionV1)
	candidate.Metadata.PlannerPolicyVersion = "retrieval-plan-policy-v2"
	_, err = CompareEvaluationReports(baseline, candidate, nil)
	if err == nil {
		t.Fatal("mismatched baseline/candidate planner identities accepted")
	}
}

func TestCompareEvaluationReportsProtectsQueryFamilyRegression(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	baseline.Cases = []EvaluationCaseReport{{CaseID: "base", Category: "general", QueryFamily: RetrievalQueryFamilyExactLookup, Metrics: EvaluationMetricReport{RecallAt1: 1, RecallAt5: 1, RecallAt10: 1}}}
	candidate.Cases = []EvaluationCaseReport{{CaseID: "candidate", Category: "general", QueryFamily: RetrievalQueryFamilyExactLookup, Metrics: EvaluationMetricReport{RecallAt1: 0, RecallAt5: 0, RecallAt10: 0}}}
	policy := EvaluationReleasePolicy{Version: "release-policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, ProtectedFamilies: []RetrievalQueryFamily{RetrievalQueryFamilyExactLookup}}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	comparison, err := CompareEvaluationReports(baseline, candidate, policy.ProtectedCategories)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparison.ProtectedRegressions) != 0 {
		t.Fatalf("category selectors unexpectedly handled family regression: %+v", comparison.ProtectedRegressions)
	}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, "protected_family_regression") {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestProtectedQueryFamilyUsesConfiguredRegressionTolerance(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	baseline.Cases = []EvaluationCaseReport{{CaseID: "base", QueryFamily: RetrievalQueryFamilySemantic, Metrics: EvaluationMetricReport{RecallAt1: 1, RecallAt5: 1, RecallAt10: 1}}}
	candidate.Cases = []EvaluationCaseReport{{CaseID: "candidate", QueryFamily: RetrievalQueryFamilySemantic, Metrics: EvaluationMetricReport{RecallAt1: .95, RecallAt5: .95, RecallAt10: .95}}}
	policy := EvaluationReleasePolicy{Version: "release-policy-v1", ProtectedCutoffs: []int{1}, MaxRecallRegression: .1, MaxP95LatencyMS: 100, ProtectedFamilies: []RetrievalQueryFamily{RetrievalQueryFamilySemantic}}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Eligible {
		t.Fatalf("within-tolerance family regression rejected: %+v", decision)
	}
}

func TestFixtureProtectedPlannerFamilyAutomaticallyBlocksRegression(t *testing.T) {
	baseline := compatibleAnalysisComparisonReport("original_only")
	candidate := compatibleAnalysisComparisonReport("active_for_scope")
	baseline.Cases = []EvaluationCaseReport{{CaseID: "base", Category: "temporal", QueryFamily: RetrievalQueryFamilyTemporal, PlannerProtected: true, Metrics: EvaluationMetricReport{RecallAt1: 1, RecallAt5: 1, RecallAt10: 1}}}
	candidate.Cases = []EvaluationCaseReport{{CaseID: "candidate", Category: "temporal", QueryFamily: RetrievalQueryFamilyTemporal, PlannerProtected: true, Metrics: EvaluationMetricReport{RecallAt1: 0, RecallAt5: 0, RecallAt10: 0}}}
	policy := EvaluationReleasePolicy{Version: "release-policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, evaluationDecisionProtectedFamily) || !containsString(decision.HardFailures, evaluationDecisionProtectedCategory) {
		t.Fatalf("decision=%+v", decision)
	}
}

func compatibleAnalysisComparisonReport(disposition string) EvaluationReport {
	return EvaluationReport{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "fixture-v1",
			RepresentationVersion:       "representation-v1",
			RankingVersion:              "ranking-v1",
			FusionStrategy:              "rrf:v1",
			CompatibleEmbeddingRevision: "embedding-v1",
			PolicyVersion:               "release-policy-v1",
			AnalysisVersion:             "analysis-v1",
			AnalysisLimitsVersion:       "analysis-limits-v1",
			RolloutDisposition:          disposition,
		},
		Metrics: EvaluationMetricReport{RecallAt1: 1, RecallAt5: 1, RecallAt10: 1},
	}
}
