package retrieval

import (
	"fmt"
	"math"
	"sort"

	"github.com/FelixSeptem/stele/internal/memory"
)

// CalculateEvaluationMetrics converts an internal replay into a bounded report. It
// evaluates required evidence groups independently, allowing multi-hop fixtures to
// require several supporting facts without storing a generated answer record.
func CalculateEvaluationMetrics(replay EvaluationReplay) (EvaluationReport, error) {
	if err := replay.Metadata.Validate(); err != nil {
		return EvaluationReport{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if len(replay.Cases) == 0 {
		return EvaluationReport{}, fmt.Errorf("evaluation replay must include at least one case")
	}

	report := EvaluationReport{
		Metadata: replay.Metadata,
		Cases:    make([]EvaluationCaseReport, 0, len(replay.Cases)),
	}
	latencies := make([]float64, 0, len(replay.Cases))
	safetyCounts := make(map[EvaluationSafetyFailureCategory]int)
	for _, item := range replay.Cases {
		safetyFailures := evaluationReplaySafetyFailures(item)
		for _, failure := range safetyFailures {
			safetyCounts[failure.Category] += failure.Count
		}
		metrics := calculateCaseEvaluationMetrics(item)
		report.Cases = append(report.Cases, EvaluationCaseReport{
			CaseID:            item.CaseID,
			Category:          item.Category,
			Metrics:           metrics,
			SafetyFailures:    safetyFailures,
			CandidatePoolSize: item.CandidatePoolSize,
			LatencyMS:         float64(item.Latency) / float64(1_000_000),
		})
		latencies = append(latencies, float64(item.Latency)/float64(1_000_000))
		report.Metrics.RecallAt1 += metrics.RecallAt1
		report.Metrics.RecallAt5 += metrics.RecallAt5
		report.Metrics.RecallAt10 += metrics.RecallAt10
		report.Metrics.MRR += metrics.MRR
		report.Metrics.NDCGAt1 += metrics.NDCGAt1
		report.Metrics.NDCGAt5 += metrics.NDCGAt5
		report.Metrics.NDCGAt10 += metrics.NDCGAt10
		report.Metrics.MultiHopEvidenceCoverage += metrics.MultiHopEvidenceCoverage
		report.Metrics.DuplicateRate += metrics.DuplicateRate
		report.Metrics.CandidatePoolSize += metrics.CandidatePoolSize
	}
	if len(safetyCounts) > 0 {
		report.SafetyFailures = evaluationSafetyFailureSlice(safetyCounts)
		for index := range report.Cases {
			if len(report.Cases[index].SafetyFailures) == 0 {
				continue
			}
			report.Cases[index].Metrics = EvaluationMetricReport{}
		}
		// Quality metrics are intentionally suppressed whenever an isolation,
		// lifecycle, fixture, or diagnostics safety assertion has failed.
		report.Metrics = EvaluationMetricReport{}
		return report, nil
	}

	caseCount := float64(len(replay.Cases))
	report.Metrics.RecallAt1 /= caseCount
	report.Metrics.RecallAt5 /= caseCount
	report.Metrics.RecallAt10 /= caseCount
	report.Metrics.MRR /= caseCount
	report.Metrics.NDCGAt1 /= caseCount
	report.Metrics.NDCGAt5 /= caseCount
	report.Metrics.NDCGAt10 /= caseCount
	report.Metrics.MultiHopEvidenceCoverage /= caseCount
	report.Metrics.DuplicateRate /= caseCount
	report.Metrics.CandidatePoolSize = int(math.Round(float64(report.Metrics.CandidatePoolSize) / caseCount))
	report.Metrics.P50LatencyMS = evaluationPercentile(latencies, 0.50)
	report.Metrics.P95LatencyMS = evaluationPercentile(latencies, 0.95)
	return report, nil
}

func evaluationReplaySafetyFailures(item EvaluationReplayCase) []EvaluationSafetyFailure {
	counts := make(map[EvaluationSafetyFailureCategory]int)
	if err := item.Scope.Validate(); err != nil {
		counts[EvaluationSafetyFailureInvalidFixtureScope]++
	}
	excluded := make(map[string]struct{}, len(item.ExcludedAliases))
	for _, alias := range item.ExcludedAliases {
		excluded[alias] = struct{}{}
	}
	for _, candidate := range item.Candidates {
		if candidate.Scope.Normalized() != item.Scope.Normalized() {
			counts[EvaluationSafetyFailureCrossScope]++
			continue
		}
		if candidate.State != memory.MemoryStateActive {
			counts[EvaluationSafetyFailureLifecycleVisibility]++
			continue
		}
		if _, isExcluded := excluded[candidate.Alias]; isExcluded {
			counts[EvaluationSafetyFailureLifecycleVisibility]++
		}
	}
	return evaluationSafetyFailureSlice(counts)
}

func evaluationSafetyFailureSlice(counts map[EvaluationSafetyFailureCategory]int) []EvaluationSafetyFailure {
	if len(counts) == 0 {
		return nil
	}
	categories := make([]string, 0, len(counts))
	for category := range counts {
		categories = append(categories, string(category))
	}
	sort.Strings(categories)
	result := make([]EvaluationSafetyFailure, 0, len(categories))
	for _, category := range categories {
		result = append(result, EvaluationSafetyFailure{Category: EvaluationSafetyFailureCategory(category), Count: counts[EvaluationSafetyFailureCategory(category)]})
	}
	return result
}

func calculateCaseEvaluationMetrics(item EvaluationReplayCase) EvaluationMetricReport {
	candidates := append([]EvaluationReplayCandidate(nil), item.Candidates...)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].FinalRank < candidates[j].FinalRank })
	groupByAlias := make(map[string][]int)
	for groupIndex, group := range item.ExpectedEvidenceGroups {
		for _, alias := range group {
			groupByAlias[alias] = append(groupByAlias[alias], groupIndex)
		}
	}
	groupRanks := make([]int, len(item.ExpectedEvidenceGroups))
	relevantAtRank := make(map[int]bool)
	firstRelevantRank := 0
	clusters := make(map[string]struct{})
	duplicates := 0
	for _, candidate := range candidates {
		if candidate.FactCluster != "" {
			if _, exists := clusters[candidate.FactCluster]; exists {
				duplicates++
			} else {
				clusters[candidate.FactCluster] = struct{}{}
			}
		}
		for _, groupIndex := range groupByAlias[candidate.Alias] {
			if groupRanks[groupIndex] != 0 {
				continue
			}
			groupRanks[groupIndex] = candidate.FinalRank
			relevantAtRank[candidate.FinalRank] = true
			if firstRelevantRank == 0 || candidate.FinalRank < firstRelevantRank {
				firstRelevantRank = candidate.FinalRank
			}
		}
	}

	groupCount := len(groupRanks)
	metrics := EvaluationMetricReport{CandidatePoolSize: item.CandidatePoolSize}
	if metrics.CandidatePoolSize == 0 {
		metrics.CandidatePoolSize = len(candidates)
	}
	if groupCount == 0 {
		return metrics
	}
	metrics.RecallAt1 = evaluationGroupRecall(groupRanks, 1)
	metrics.RecallAt5 = evaluationGroupRecall(groupRanks, 5)
	metrics.RecallAt10 = evaluationGroupRecall(groupRanks, 10)
	metrics.MultiHopEvidenceCoverage = metrics.RecallAt10
	if firstRelevantRank > 0 {
		metrics.MRR = 1 / float64(firstRelevantRank)
	}
	metrics.NDCGAt1 = evaluationNDCG(relevantAtRank, groupCount, 1)
	metrics.NDCGAt5 = evaluationNDCG(relevantAtRank, groupCount, 5)
	metrics.NDCGAt10 = evaluationNDCG(relevantAtRank, groupCount, 10)
	if len(candidates) > 0 {
		metrics.DuplicateRate = float64(duplicates) / float64(len(candidates))
	}
	metrics.P50LatencyMS = float64(item.Latency) / float64(1_000_000)
	metrics.P95LatencyMS = metrics.P50LatencyMS
	return metrics
}

func evaluationGroupRecall(groupRanks []int, cutoff int) float64 {
	if len(groupRanks) == 0 {
		return 0
	}
	coveredAtCutoff := 0
	for _, rank := range groupRanks {
		if rank > 0 && rank <= cutoff {
			coveredAtCutoff++
		}
	}
	return float64(coveredAtCutoff) / float64(len(groupRanks))
}

func evaluationNDCG(relevantAtRank map[int]bool, relevantCount, cutoff int) float64 {
	if relevantCount == 0 || cutoff <= 0 {
		return 0
	}
	dcg := 0.0
	for rank := 1; rank <= cutoff; rank++ {
		if relevantAtRank[rank] {
			dcg += 1 / math.Log2(float64(rank)+1)
		}
	}
	ideal := 0.0
	for rank := 1; rank <= cutoff && rank <= relevantCount; rank++ {
		ideal += 1 / math.Log2(float64(rank)+1)
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func evaluationPercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
