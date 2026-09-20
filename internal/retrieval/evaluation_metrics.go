package retrieval

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

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
		Metadata:                   replay.Metadata,
		Cases:                      make([]EvaluationCaseReport, 0, len(replay.Cases)),
		DispositionAggregates:      make(map[string]int),
		AnalysisFallbackAggregates: make(map[string]int),
		AnalysisCategoryAggregates: make(map[string]int),
	}
	latencies := make([]float64, 0, len(replay.Cases))
	safetyCounts := make(map[EvaluationSafetyFailureCategory]int)
	temporalCoverage := EvaluationTemporalCoverage{
		PolicyVersion:     replay.Metadata.TemporalPolicyVersion,
		CoverageVersion:   replay.Metadata.TemporalCoverageVersion,
		KindCounts:        make(map[string]int),
		ModeCounts:        make(map[string]int),
		FallbackCounts:    make(map[string]int),
		DispositionCounts: make(map[string]int),
	}
	protectedCases := 0
	temporalCases := 0
	multiHopCases := 0
	analysisCases := 0
	plannerCases := 0
	plannerFallbacks := 0
	plannerRerankerUses := 0
	plannerRerankerSafe := true
	plannerCompatible := true
	plannerRollbackTested := true
	for _, item := range replay.Cases {
		if err := validateEvaluationReplayAnalysis(item.AnalysisDiagnostics, replay.Metadata); err != nil {
			return EvaluationReport{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
		}
		safetyFailures := evaluationReplaySafetyFailures(item)
		for _, failure := range safetyFailures {
			safetyCounts[failure.Category] += failure.Count
		}
		metrics := calculateCaseEvaluationMetrics(item)
		for _, diagnostic := range item.Diagnostics {
			if diagnostic.Disposition == "" {
				continue
			}
			report.DispositionAggregates[string(diagnostic.Disposition)]++
		}
		chunkDerivedCount := 0
		for _, candidate := range item.Candidates {
			if candidate.ChunkDerived {
				chunkDerivedCount++
			}
		}
		caseReport := EvaluationCaseReport{
			CaseID:            item.CaseID,
			Category:          item.Category,
			Metrics:           metrics,
			SafetyFailures:    safetyFailures,
			CandidatePoolSize: item.CandidatePoolSize,
			LatencyMS:         float64(item.Latency) / float64(1_000_000),
			ChunkDerivedCount: chunkDerivedCount,
		}
		if item.PlannerFamily != "" || len(item.Passes) > 0 {
			plannerCases++
			caseReport.QueryFamily = item.PlannerFamily
			caseReport.PlannerIdentity = item.PlannerIdentity
			caseReport.PlannerVersion = item.PlannerVersion
			caseReport.PlannerPolicyVersion = item.PlannerPolicyVersion
			caseReport.PassCount = len(item.Passes)
			caseReport.PlannerFallbackCategory = item.FallbackCategory
			caseReport.PlannerRerankerUsed = item.RerankerUsed
			caseReport.PlannerProtected = item.PlannerProtected
			if !item.RerankerSafe {
				plannerRerankerSafe = false
			}
			if !item.RollbackVerified {
				plannerRollbackTested = false
			}
			if item.PlannerFamily == "" || !item.PlannerFamily.valid() || strings.TrimSpace(item.PlannerIdentity) == "" || len(item.Passes) == 0 || len(item.Passes) > 2 {
				plannerCompatible = false
			}
			for index, pass := range item.Passes {
				if pass.Pass != index+1 || pass.CandidateCount < 0 || pass.CandidateCount > 5000 || pass.VisibleCount < 0 || pass.VisibleCount > 5000 || !boundedRate(pass.EvidenceCoverage) || pass.Latency < 0 || pass.Evidence.Disposition == "" {
					plannerCompatible = false
					continue
				}
				caseReport.MaxPlannerCandidates += pass.CandidateCount
				coverage := pass.EvidenceCoverage
				if index == 0 {
					caseReport.FirstPassEvidenceCoverage = coverage
					caseReport.FirstPassCandidates = pass.CandidateCount
					caseReport.FirstPassLatencyMS = float64(pass.Latency) / float64(time.Millisecond)
				} else {
					caseReport.SecondPassEvidenceCoverage = coverage
					caseReport.SecondPassCandidates = pass.CandidateCount
					caseReport.SecondPassLatencyMS = float64(pass.Latency) / float64(time.Millisecond)
				}
			}
			caseReport.SecondPassEvidenceGain = math.Max(caseReport.SecondPassEvidenceCoverage-caseReport.FirstPassEvidenceCoverage, 0)
			if caseReport.PassCount > report.Metrics.MaxPassesObserved {
				report.Metrics.MaxPassesObserved = caseReport.PassCount
			}
			if caseReport.MaxPlannerCandidates > report.Metrics.MaxPlannerCandidates {
				report.Metrics.MaxPlannerCandidates = caseReport.MaxPlannerCandidates
			}
			report.Metrics.FirstPassEvidenceCoverage += caseReport.FirstPassEvidenceCoverage
			if caseReport.PassCount == 2 {
				report.Metrics.SecondPassCount++
				report.Metrics.SecondPassEvidenceCoverage += caseReport.SecondPassEvidenceCoverage
				report.Metrics.SecondPassEvidenceGain += caseReport.SecondPassEvidenceGain
			}
			if item.FallbackCategory != "" && item.FallbackCategory != "none" {
				plannerFallbacks++
			}
			if item.RerankerUsed {
				plannerRerankerUses++
			}
		}
		if item.TemporalKind != "" {
			temporalCases++
			caseReport.TemporalKind = item.TemporalKind
			caseReport.TemporalMode = item.TemporalMode
			caseReport.TemporalSelectorKind = item.TemporalSelectorKind
			caseReport.TemporalSelectedVersions = item.TemporalSelectedVersions
			caseReport.TemporalHiddenAliasCount = item.TemporalHiddenAliasCount
			caseReport.TemporalStaleVersions = item.TemporalStaleVersions
			caseReport.TemporalAmbiguousVersions = item.TemporalAmbiguousVersions
			caseReport.TemporalConflictDisposition = item.TemporalConflictDisposition
			caseReport.TemporalProvenanceMismatch = item.TemporalProvenanceMismatch
			caseReport.TemporalHiddenVersionLeak = item.TemporalHiddenVersionLeak
			caseReport.TemporalFallbackCategory = item.TemporalFallbackCategory
			temporalCoverage.Cases++
			temporalCoverage.KindCounts[string(item.TemporalKind)]++
			if item.TemporalMode != "" {
				temporalCoverage.ModeCounts[string(item.TemporalMode)]++
			}
			temporalCoverage.SelectedVersions += item.TemporalSelectedVersions
			temporalCoverage.StaleVersions += item.TemporalStaleVersions
			temporalCoverage.AmbiguousVersions += item.TemporalAmbiguousVersions
			temporalCoverage.ProvenanceMismatch += item.TemporalProvenanceMismatch
			temporalCoverage.HiddenVersionLeaks += item.TemporalHiddenVersionLeak
			if item.TemporalFallbackCategory != "" {
				temporalCoverage.FallbackCounts[item.TemporalFallbackCategory]++
			}
			if item.TemporalConflictDisposition != "" {
				temporalCoverage.DispositionCounts[item.TemporalConflictDisposition]++
			}
			if item.TemporalSelectorKind != "" {
				temporalCoverage.EvaluationInstants++
			}
		}
		if diagnostic := item.AnalysisDiagnostics; diagnostic != nil {
			analysisCases++
			caseReport.AnalysisSignalCount = diagnostic.SignalCount
			caseReport.AnalysisSubqueryCount = diagnostic.SubqueryCount
			caseReport.AnalysisCandidateCount = diagnostic.CandidateCount
			caseReport.AnalysisOriginalRetained = diagnostic.OriginalRetained
			caseReport.AnalysisDisposition = diagnostic.Disposition
			caseReport.AnalysisFallback = diagnostic.Fallback
			caseReport.AnalysisCategories = append([]QueryAnalysisDiagnosticCount(nil), diagnostic.Categories...)
			sort.Slice(caseReport.AnalysisCategories, func(i, j int) bool {
				return caseReport.AnalysisCategories[i].Category < caseReport.AnalysisCategories[j].Category
			})
			caseReport.AnalysisElapsedMS = float64(diagnostic.Elapsed) / float64(1_000_000)
			report.Metrics.AnalysisSignalCount += diagnostic.SignalCount
			report.Metrics.AnalysisSubqueryCount += diagnostic.SubqueryCount
			report.Metrics.AnalysisCandidateCount += diagnostic.CandidateCount
			report.AnalysisFallbackAggregates[string(diagnostic.Fallback)] = boundedEvaluationCount(report.AnalysisFallbackAggregates[string(diagnostic.Fallback)], 1)
			for _, count := range diagnostic.Categories {
				key := string(count.Category)
				report.AnalysisCategoryAggregates[key] = boundedEvaluationCount(report.AnalysisCategoryAggregates[key], count.Count)
			}
		}
		report.Cases = append(report.Cases, caseReport)
		latencies = append(latencies, float64(item.Latency)/float64(1_000_000))
		report.Metrics.RecallAt1 += metrics.RecallAt1
		report.Metrics.RecallAt5 += metrics.RecallAt5
		report.Metrics.RecallAt10 += metrics.RecallAt10
		report.Metrics.MRR += metrics.MRR
		report.Metrics.NDCGAt1 += metrics.NDCGAt1
		report.Metrics.NDCGAt5 += metrics.NDCGAt5
		report.Metrics.NDCGAt10 += metrics.NDCGAt10
		report.Metrics.EvidenceCoverage += metrics.EvidenceCoverage
		switch {
		case item.Category == "single-fact":
			protectedCases++
			report.Metrics.ProtectedRecall += metrics.RecallAt10
		case item.Category == "temporal":
			temporalCases++
			report.Metrics.TemporalEvidenceCoverage += metrics.EvidenceCoverage
		case strings.HasPrefix(item.Category, "multi-hop"):
			multiHopCases++
			report.Metrics.MultiHopEvidenceCoverage += metrics.MultiHopEvidenceCoverage
		}
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
	if len(report.RerankFallbackCounts) == 0 {
		report.RerankFallbackCounts = nil
	}

	caseCount := float64(len(replay.Cases))
	report.Metrics.RecallAt1 /= caseCount
	report.Metrics.RecallAt5 /= caseCount
	report.Metrics.RecallAt10 /= caseCount
	report.Metrics.MRR /= caseCount
	report.Metrics.NDCGAt1 /= caseCount
	report.Metrics.NDCGAt5 /= caseCount
	report.Metrics.NDCGAt10 /= caseCount
	report.Metrics.EvidenceCoverage /= caseCount
	if protectedCases > 0 {
		report.Metrics.ProtectedRecall /= float64(protectedCases)
	} else {
		report.Metrics.ProtectedRecall = report.Metrics.RecallAt10
	}
	if temporalCases > 0 {
		report.Metrics.TemporalEvidenceCoverage /= float64(temporalCases)
	}
	// Coverage is attached only when a case actually declared a temporal kind.
	// Category-level "temporal" fixtures predate fact-valid selection and must
	// not publish an empty coverage block claiming a policy was measured.
	if temporalCoverage.Cases > 0 {
		report.TemporalCoverage = &temporalCoverage
	}
	if multiHopCases > 0 {
		report.Metrics.MultiHopEvidenceCoverage /= float64(multiHopCases)
	} else {
		report.Metrics.MultiHopEvidenceCoverage = report.Metrics.EvidenceCoverage
	}
	if analysisCases > 0 {
		report.Metrics.AnalysisSignalCount = int(math.Round(float64(report.Metrics.AnalysisSignalCount) / float64(analysisCases)))
		report.Metrics.AnalysisSubqueryCount = int(math.Round(float64(report.Metrics.AnalysisSubqueryCount) / float64(analysisCases)))
		report.Metrics.AnalysisCandidateCount = int(math.Round(float64(report.Metrics.AnalysisCandidateCount) / float64(analysisCases)))
	}
	if plannerCases > 0 {
		report.Metrics.FirstPassEvidenceCoverage /= float64(plannerCases)
		if report.Metrics.SecondPassCount > 0 {
			report.Metrics.SecondPassEvidenceCoverage /= float64(report.Metrics.SecondPassCount)
			report.Metrics.SecondPassEvidenceGain /= float64(report.Metrics.SecondPassCount)
		}
		report.Metrics.PlannerFallbackRate = float64(plannerFallbacks) / float64(plannerCases)
		report.Metrics.PlannerRerankerUseRate = float64(plannerRerankerUses) / float64(plannerCases)
		report.PlannerEvidence = EvaluationPlannerReleaseEvidence{Compatible: plannerCompatible, SafetyFailures: evaluationSafetyFailureCount(report.SafetyFailures), MaxPasses: report.Metrics.MaxPassesObserved, MaxCandidateCount: report.Metrics.MaxPlannerCandidates, FallbackRate: report.Metrics.PlannerFallbackRate, RerankerSafe: plannerRerankerSafe, RollbackTested: plannerRollbackTested}
	}
	report.Metrics.DuplicateRate /= caseCount
	report.Metrics.CandidatePoolSize = int(math.Round(float64(report.Metrics.CandidatePoolSize) / caseCount))
	report.Metrics.P50LatencyMS = evaluationPercentile(latencies, 0.50)
	report.Metrics.P95LatencyMS = evaluationPercentile(latencies, 0.95)
	if total := dispositionTotal(report.DispositionAggregates); total > 0 {
		report.Metrics.BudgetOmissionRate = float64(report.DispositionAggregates["omitted_by_budget"]) / float64(total)
	}
	return report, nil
}

func validateEvaluationReplayAnalysis(diagnostic *QueryAnalysisDiagnostics, metadata EvaluationRankingMetadata) error {
	if diagnostic == nil {
		if metadata.AnalysisVersion != "" && metadata.RolloutDisposition != "original_only" {
			return fmt.Errorf("query-analysis diagnostics are missing")
		}
		return nil
	}
	if err := diagnostic.Validate(evaluationHardQueryAnalysisLimits()); err != nil {
		return err
	}
	if string(diagnostic.PolicyVersion) != metadata.AnalysisVersion || string(diagnostic.LimitsVersion) != metadata.AnalysisLimitsVersion || !evaluationRolloutDiagnosticsMatch(metadata.RolloutDisposition, diagnostic.RolloutStage) {
		return fmt.Errorf("query-analysis diagnostic identity mismatch")
	}
	return nil
}

func boundedEvaluationCount(current, increment int) int {
	if increment <= 0 {
		return current
	}
	if current >= QueryAnalysisHardMaxDiagnosticCount-increment {
		return QueryAnalysisHardMaxDiagnosticCount
	}
	return current + increment
}

func dispositionTotal(dispositions map[string]int) int {
	total := 0
	for _, count := range dispositions {
		total += count
	}
	return total
}

func evaluationReplaySafetyFailures(item EvaluationReplayCase) []EvaluationSafetyFailure {
	counts := make(map[EvaluationSafetyFailureCategory]int)
	if err := item.Scope.Validate(); err != nil {
		counts[EvaluationSafetyFailureInvalidFixtureScope]++
	}
	if item.CandidatePoolSize > QueryAnalysisHardMaxAggregateCandidates {
		counts[EvaluationSafetyFailureResourceOverflow]++
	}
	if item.TemporalStaleVersions > 0 {
		counts[EvaluationSafetyFailureStaleFactWin] += item.TemporalStaleVersions
	}
	if item.TemporalAmbiguousVersions > 0 {
		counts[EvaluationSafetyFailureValidityAmbiguity] += item.TemporalAmbiguousVersions
	}
	if item.TemporalProvenanceMismatch > 0 {
		counts[EvaluationSafetyFailureProvenanceMismatch] += item.TemporalProvenanceMismatch
	}
	if item.TemporalHiddenVersionLeak > 0 {
		counts[EvaluationSafetyFailureHiddenVersionLeakage] += item.TemporalHiddenVersionLeak
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
	metrics.ProtectedRecall = metrics.RecallAt10
	metrics.EvidenceCoverage = metrics.MultiHopEvidenceCoverage
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
