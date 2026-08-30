package retrieval

import "fmt"

type EvaluationMetricDelta struct {
	Metric    string  `json:"metric"`
	Baseline  float64 `json:"baseline"`
	Candidate float64 `json:"candidate"`
	Delta     float64 `json:"delta"`
}

type EvaluationProtectedRegression struct {
	Category  string  `json:"category"`
	Metric    string  `json:"metric"`
	Baseline  float64 `json:"baseline"`
	Candidate float64 `json:"candidate"`
	Delta     float64 `json:"delta"`
}

type EvaluationComparison struct {
	BaselineRankingVersion  string                          `json:"baseline_ranking_version"`
	CandidateRankingVersion string                          `json:"candidate_ranking_version"`
	MetricDeltas            []EvaluationMetricDelta         `json:"metric_deltas"`
	ProtectedRegressions    []EvaluationProtectedRegression `json:"protected_regressions,omitempty"`
	Advisories              []string                        `json:"advisories,omitempty"`
}

// CompareEvaluationReports compares reports only when their corpus and representation
// are compatible. Ranking versions may differ because that is the candidate change
// under review.
func CompareEvaluationReports(baseline, candidate EvaluationReport, protectedCategories []string) (EvaluationComparison, error) {
	if err := baseline.Metadata.Validate(); err != nil {
		return EvaluationComparison{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if err := candidate.Metadata.Validate(); err != nil {
		return EvaluationComparison{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if baseline.Metadata.FixtureVersion != candidate.Metadata.FixtureVersion {
		return EvaluationComparison{}, fmt.Errorf("incompatible fixture version")
	}
	if baseline.Metadata.RepresentationVersion != candidate.Metadata.RepresentationVersion {
		return EvaluationComparison{}, fmt.Errorf("incompatible representation version")
	}
	if baseline.Metadata.CompatibleEmbeddingRevision != candidate.Metadata.CompatibleEmbeddingRevision {
		return EvaluationComparison{}, fmt.Errorf("incompatible embedding revision")
	}

	comparison := EvaluationComparison{
		BaselineRankingVersion:  baseline.Metadata.RankingVersion,
		CandidateRankingVersion: candidate.Metadata.RankingVersion,
		MetricDeltas: []EvaluationMetricDelta{
			evaluationMetricDelta("recall_at_1", baseline.Metrics.RecallAt1, candidate.Metrics.RecallAt1),
			evaluationMetricDelta("recall_at_5", baseline.Metrics.RecallAt5, candidate.Metrics.RecallAt5),
			evaluationMetricDelta("recall_at_10", baseline.Metrics.RecallAt10, candidate.Metrics.RecallAt10),
			evaluationMetricDelta("mrr", baseline.Metrics.MRR, candidate.Metrics.MRR),
			evaluationMetricDelta("ndcg_at_10", baseline.Metrics.NDCGAt10, candidate.Metrics.NDCGAt10),
			evaluationMetricDelta("multi_hop_evidence_coverage", baseline.Metrics.MultiHopEvidenceCoverage, candidate.Metrics.MultiHopEvidenceCoverage),
			evaluationMetricDelta("duplicate_rate", baseline.Metrics.DuplicateRate, candidate.Metrics.DuplicateRate),
			evaluationMetricDelta("p95_latency_ms", baseline.Metrics.P95LatencyMS, candidate.Metrics.P95LatencyMS),
		},
	}
	baselineCategories := evaluationCategoryMetrics(baseline.Cases)
	candidateCategories := evaluationCategoryMetrics(candidate.Cases)
	for _, category := range protectedCategories {
		baselineMetric, baselineFound := baselineCategories[category]
		candidateMetric, candidateFound := candidateCategories[category]
		if !baselineFound || !candidateFound {
			comparison.Advisories = append(comparison.Advisories, "protected_category_missing")
			continue
		}
		comparison.ProtectedRegressions = appendProtectedRegression(comparison.ProtectedRegressions, category, "recall_at_1", baselineMetric.RecallAt1, candidateMetric.RecallAt1)
		comparison.ProtectedRegressions = appendProtectedRegression(comparison.ProtectedRegressions, category, "recall_at_5", baselineMetric.RecallAt5, candidateMetric.RecallAt5)
		comparison.ProtectedRegressions = appendProtectedRegression(comparison.ProtectedRegressions, category, "recall_at_10", baselineMetric.RecallAt10, candidateMetric.RecallAt10)
		comparison.ProtectedRegressions = appendProtectedRegression(comparison.ProtectedRegressions, category, "multi_hop_evidence_coverage", baselineMetric.MultiHopEvidenceCoverage, candidateMetric.MultiHopEvidenceCoverage)
	}
	return comparison, nil
}

func evaluationMetricDelta(metric string, baseline, candidate float64) EvaluationMetricDelta {
	return EvaluationMetricDelta{Metric: metric, Baseline: baseline, Candidate: candidate, Delta: candidate - baseline}
}

func appendProtectedRegression(regressions []EvaluationProtectedRegression, category, metric string, baseline, candidate float64) []EvaluationProtectedRegression {
	if candidate >= baseline {
		return regressions
	}
	return append(regressions, EvaluationProtectedRegression{Category: category, Metric: metric, Baseline: baseline, Candidate: candidate, Delta: candidate - baseline})
}

func evaluationCategoryMetrics(cases []EvaluationCaseReport) map[string]EvaluationMetricReport {
	metrics := make(map[string]EvaluationMetricReport)
	counts := make(map[string]int)
	for _, item := range cases {
		if item.Category == "" {
			continue
		}
		current := metrics[item.Category]
		current.RecallAt1 += item.Metrics.RecallAt1
		current.RecallAt5 += item.Metrics.RecallAt5
		current.RecallAt10 += item.Metrics.RecallAt10
		current.MultiHopEvidenceCoverage += item.Metrics.MultiHopEvidenceCoverage
		metrics[item.Category] = current
		counts[item.Category]++
	}
	for category, current := range metrics {
		count := float64(counts[category])
		current.RecallAt1 /= count
		current.RecallAt5 /= count
		current.RecallAt10 /= count
		current.MultiHopEvidenceCoverage /= count
		metrics[category] = current
	}
	return metrics
}
