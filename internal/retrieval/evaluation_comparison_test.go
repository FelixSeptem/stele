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
