package retrieval

import (
	"fmt"
	"sort"
	"strings"
)

// RenderEvaluationReport produces a concise local/CI summary from the same redacted
// report model used by MarshalEvaluationReport. It deliberately does not list queries,
// source payloads, database identifiers, scopes, or raw error causes.
func RenderEvaluationReport(report EvaluationReport) (string, error) {
	if err := report.Metadata.Validate(); err != nil {
		return "", NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if err := report.validateSafeOutput(); err != nil {
		return "", NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	metrics := report.Metrics
	var builder strings.Builder
	fmt.Fprintf(&builder, "fixture_version=%s\n", report.Metadata.FixtureVersion)
	fmt.Fprintf(&builder, "representation_version=%s\n", report.Metadata.RepresentationVersion)
	fmt.Fprintf(&builder, "ranking_version=%s\n", report.Metadata.RankingVersion)
	if strings.TrimSpace(report.Metadata.FusionStrategy) != "" {
		fmt.Fprintf(&builder, "fusion_strategy=%s\n", report.Metadata.FusionStrategy)
	}
	fmt.Fprintf(&builder, "compatible_embedding_revision=%s\n", report.Metadata.CompatibleEmbeddingRevision)
	fmt.Fprintf(&builder, "policy_version=%s\n", report.Metadata.PolicyVersion)
	if report.Metadata.AnalysisVersion != "" {
		fmt.Fprintf(&builder, "analysis_version=%s\n", report.Metadata.AnalysisVersion)
		fmt.Fprintf(&builder, "analysis_limits_version=%s\n", report.Metadata.AnalysisLimitsVersion)
		fmt.Fprintf(&builder, "rollout_disposition=%s\n", report.Metadata.RolloutDisposition)
	}
	fmt.Fprintf(&builder, "case_count=%d\n", len(report.Cases))
	fmt.Fprintf(&builder, "safety_failure_count=%d\n", evaluationSafetyFailureCount(report.SafetyFailures))
	fmt.Fprintf(&builder, "recall@1=%.4f\n", metrics.RecallAt1)
	fmt.Fprintf(&builder, "recall@5=%.4f\n", metrics.RecallAt5)
	fmt.Fprintf(&builder, "recall@10=%.4f\n", metrics.RecallAt10)
	fmt.Fprintf(&builder, "mrr=%.4f\n", metrics.MRR)
	fmt.Fprintf(&builder, "ndcg@1=%.4f\n", metrics.NDCGAt1)
	fmt.Fprintf(&builder, "ndcg@5=%.4f\n", metrics.NDCGAt5)
	fmt.Fprintf(&builder, "ndcg@10=%.4f\n", metrics.NDCGAt10)
	fmt.Fprintf(&builder, "multi_hop_evidence_coverage=%.4f\n", metrics.MultiHopEvidenceCoverage)
	fmt.Fprintf(&builder, "temporal_evidence_coverage=%.4f\n", metrics.TemporalEvidenceCoverage)
	fmt.Fprintf(&builder, "protected_recall=%.4f\n", metrics.ProtectedRecall)
	fmt.Fprintf(&builder, "duplicate_rate=%.4f\n", metrics.DuplicateRate)
	fmt.Fprintf(&builder, "candidate_pool_size=%d\n", metrics.CandidatePoolSize)
	fmt.Fprintf(&builder, "analysis_signal_count=%d\n", metrics.AnalysisSignalCount)
	fmt.Fprintf(&builder, "analysis_subquery_count=%d\n", metrics.AnalysisSubqueryCount)
	fmt.Fprintf(&builder, "analysis_candidate_count=%d\n", metrics.AnalysisCandidateCount)
	for _, fallback := range sortedEvaluationAggregateKeys(report.AnalysisFallbackAggregates) {
		fmt.Fprintf(&builder, "analysis_fallback.%s=%d\n", fallback, report.AnalysisFallbackAggregates[fallback])
	}
	for _, category := range sortedEvaluationAggregateKeys(report.AnalysisCategoryAggregates) {
		fmt.Fprintf(&builder, "analysis_category.%s=%d\n", category, report.AnalysisCategoryAggregates[category])
	}
	fmt.Fprintf(&builder, "p50_latency_ms=%.3f\n", metrics.P50LatencyMS)
	fmt.Fprintf(&builder, "p95_latency_ms=%.3f", metrics.P95LatencyMS)
	return builder.String(), nil
}

func sortedEvaluationAggregateKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func evaluationSafetyFailureCount(failures []EvaluationSafetyFailure) int {
	count := 0
	for _, failure := range failures {
		count += failure.Count
	}
	return count
}
