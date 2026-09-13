package retrieval

import "fmt"

type CompatibilityCode string

const (
	CompatibilityCodeFixtureVersion        CompatibilityCode = "incompatible_fixture_version"
	CompatibilityCodeRepresentationVersion CompatibilityCode = "incompatible_representation_version"
	CompatibilityCodeFusionStrategy        CompatibilityCode = "incompatible_fusion_strategy"
	CompatibilityCodeRankingVersion        CompatibilityCode = "incompatible_ranking_version"
	CompatibilityCodeEmbeddingRevision     CompatibilityCode = "incompatible_embedding_revision"
	CompatibilityCodeEmbeddingProvider     CompatibilityCode = "incompatible_embedding_provider"
	CompatibilityCodeAnalysisVersion       CompatibilityCode = "incompatible_analysis_version"
	CompatibilityCodePolicyVersion         CompatibilityCode = "incompatible_release_policy_version"
)

type evaluationCompatibilityError struct {
	code    CompatibilityCode
	message string
}

func (e evaluationCompatibilityError) Error() string                        { return e.message }
func (e evaluationCompatibilityError) CompatibilityCode() CompatibilityCode { return e.code }

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
	BaselineFusionStrategy  string                          `json:"baseline_fusion_strategy,omitempty"`
	CandidateFusionStrategy string                          `json:"candidate_fusion_strategy,omitempty"`
	BaselinePolicyVersion   string                          `json:"baseline_policy_version"`
	CandidatePolicyVersion  string                          `json:"candidate_policy_version"`
	MetricDeltas            []EvaluationMetricDelta         `json:"metric_deltas"`
	ProtectedRegressions    []EvaluationProtectedRegression `json:"protected_regressions,omitempty"`
	SafetyGatePassed        bool                            `json:"safety_gate_passed"`
	SafetyFailures          []EvaluationSafetyFailure       `json:"safety_failures,omitempty"`
	Advisories              []string                        `json:"advisories,omitempty"`
}

// CompareEvaluationReports compares a candidate only with an immutable baseline
// produced by the same retrieval stack. The query-analysis policy is the candidate
// change under review; fixture, representation, fusion, ranking, and release-policy
// identities must remain fixed.
func CompareEvaluationReports(baseline, candidate EvaluationReport, protectedCategories []string) (EvaluationComparison, error) {
	if err := baseline.Metadata.Validate(); err != nil {
		return EvaluationComparison{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if err := candidate.Metadata.Validate(); err != nil {
		return EvaluationComparison{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if baseline.Metadata.FixtureVersion != candidate.Metadata.FixtureVersion {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeFixtureVersion, "incompatible fixture version"}
	}
	if baseline.Metadata.RepresentationVersion != candidate.Metadata.RepresentationVersion {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeRepresentationVersion, "incompatible representation version"}
	}
	if baseline.Metadata.CompatibleEmbeddingRevision != candidate.Metadata.CompatibleEmbeddingRevision {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeEmbeddingRevision, "incompatible embedding revision"}
	}
	if baseline.Metadata.EmbeddingProvider != candidate.Metadata.EmbeddingProvider {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeEmbeddingProvider, "incompatible embedding provider"}
	}
	if baseline.Metadata.FusionStrategy != candidate.Metadata.FusionStrategy {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeFusionStrategy, "incompatible fusion strategy"}
	}
	if evaluationIsAnalysisComparison(baseline, candidate) && baseline.Metadata.RankingVersion != candidate.Metadata.RankingVersion {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeRankingVersion, "incompatible ranking version"}
	}
	if baseline.Metadata.PolicyVersion != candidate.Metadata.PolicyVersion {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodePolicyVersion, "incompatible release policy version"}
	}
	if baseline.Metadata.AnalysisVersion != candidate.Metadata.AnalysisVersion || baseline.Metadata.AnalysisLimitsVersion != candidate.Metadata.AnalysisLimitsVersion {
		return EvaluationComparison{}, evaluationCompatibilityError{CompatibilityCodeAnalysisVersion, "incompatible analysis version"}
	}
	if evaluationIsAnalysisComparison(baseline, candidate) && baseline.Metadata.RolloutDisposition != "original_only" {
		return EvaluationComparison{}, fmt.Errorf("immutable original-query baseline must use original_only disposition")
	}
	if len(baseline.SafetyFailures) > 0 {
		return EvaluationComparison{}, fmt.Errorf("immutable original-query baseline contains safety failures")
	}

	comparison := EvaluationComparison{
		BaselineRankingVersion:  baseline.Metadata.RankingVersion,
		CandidateRankingVersion: candidate.Metadata.RankingVersion,
		BaselineFusionStrategy:  baseline.Metadata.FusionStrategy,
		CandidateFusionStrategy: candidate.Metadata.FusionStrategy,
		BaselinePolicyVersion:   baseline.Metadata.PolicyVersion,
		CandidatePolicyVersion:  candidate.Metadata.PolicyVersion,
		SafetyGatePassed:        len(candidate.SafetyFailures) == 0,
		SafetyFailures:          append([]EvaluationSafetyFailure(nil), candidate.SafetyFailures...),
		MetricDeltas: []EvaluationMetricDelta{
			evaluationMetricDelta("recall_at_1", baseline.Metrics.RecallAt1, candidate.Metrics.RecallAt1),
			evaluationMetricDelta("recall_at_5", baseline.Metrics.RecallAt5, candidate.Metrics.RecallAt5),
			evaluationMetricDelta("recall_at_10", baseline.Metrics.RecallAt10, candidate.Metrics.RecallAt10),
			evaluationMetricDelta("mrr", baseline.Metrics.MRR, candidate.Metrics.MRR),
			evaluationMetricDelta("ndcg_at_10", baseline.Metrics.NDCGAt10, candidate.Metrics.NDCGAt10),
			evaluationMetricDelta("multi_hop_evidence_coverage", baseline.Metrics.MultiHopEvidenceCoverage, candidate.Metrics.MultiHopEvidenceCoverage),
			evaluationMetricDelta("duplicate_rate", baseline.Metrics.DuplicateRate, candidate.Metrics.DuplicateRate),
			evaluationMetricDelta("protected_recall", baseline.Metrics.ProtectedRecall, candidate.Metrics.ProtectedRecall),
			evaluationMetricDelta("evidence_coverage", baseline.Metrics.EvidenceCoverage, candidate.Metrics.EvidenceCoverage),
			evaluationMetricDelta("candidate_pool_size", float64(baseline.Metrics.CandidatePoolSize), float64(candidate.Metrics.CandidatePoolSize)),
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

func evaluationIsAnalysisComparison(baseline, candidate EvaluationReport) bool {
	return baseline.Metadata.AnalysisVersion != "" ||
		baseline.Metadata.AnalysisLimitsVersion != "" ||
		candidate.Metadata.AnalysisVersion != "" ||
		candidate.Metadata.AnalysisLimitsVersion != ""
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
