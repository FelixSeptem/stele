package app

import (
	"context"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type retrievalQualityProbe struct {
	Searcher  retrieval.MemorySearcher
	Assembler retrieval.ContextAssembler
}

func (p retrievalQualityProbe) RunQualityProbe(ctx context.Context, input memory.QualityProbeInput) ([]memory.QualityEvaluationFinding, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return nil, nil
	}

	findings := make([]memory.QualityEvaluationFinding, 0)
	expected := expectedMemorySet(input.ExpectedMemoryIDs)
	if hasProbeCheck(input.Checks, memory.QualityEvaluationCheckRetrieval) && p.Searcher != nil {
		result, err := p.Searcher.Search(ctx, retrieval.SearchInput{
			Scope:            input.Scope,
			Query:            query,
			TopK:             maxInt(len(expected)+10, 10),
			IncludeSummaries: true,
			IncludeRelations: true,
		})
		if err != nil {
			return nil, err
		}
		seen := make(map[string]struct{})
		for _, hit := range result.Hits {
			seen[hit.Memory.ID] = struct{}{}
			if hiddenMemoryState(hit.Memory.State) {
				findings = append(findings, qualityProbeFinding(input, memory.QualityFindingLifecycleHiddenReturned, memory.QualityFindingSeverityBlocker, memory.QualityFindingComponentRetrieval, memory.QualityFindingCategoryLifecycle, "retrieval returned lifecycle-hidden memory", hit.Memory.ID))
			}
		}
		for id := range expected {
			if _, ok := seen[id]; !ok {
				findings = append(findings, qualityProbeFinding(input, memory.QualityFindingExpectedRecallMissing, memory.QualityFindingSeverityWarning, memory.QualityFindingComponentRetrieval, memory.QualityFindingCategorySemanticProjection, "expected memory was not returned by retrieval", id))
			}
		}
	}

	if hasProbeCheck(input.Checks, memory.QualityEvaluationCheckContext) && p.Assembler != nil {
		budget := input.ContextBudget
		if budget <= 0 {
			budget = maxInt(len(expected), 1)
		}
		assembled, err := p.Assembler.AssembleContext(ctx, retrieval.AssembleContextInput{
			Scope:              input.Scope,
			Query:              query,
			Budget:             budget,
			IncludeRelations:   true,
			IncludeDiagnostics: true,
		})
		if err != nil {
			return nil, err
		}
		seen := make(map[string]struct{})
		for _, hit := range flattenContextHits(assembled) {
			seen[hit.Memory.ID] = struct{}{}
			if hiddenMemoryState(hit.Memory.State) {
				findings = append(findings, qualityProbeFinding(input, memory.QualityFindingLifecycleHiddenReturned, memory.QualityFindingSeverityBlocker, memory.QualityFindingComponentRetrieval, memory.QualityFindingCategoryLifecycle, "context assembly returned lifecycle-hidden memory", hit.Memory.ID))
			}
		}
		for id := range expected {
			if _, ok := seen[id]; !ok {
				findings = append(findings, qualityProbeFinding(input, memory.QualityFindingExpectedRecallMissing, memory.QualityFindingSeverityWarning, memory.QualityFindingComponentRetrieval, memory.QualityFindingCategorySemanticProjection, "expected memory was not included by context assembly", id))
			}
		}
	}

	return findings, nil
}

func qualityProbeFinding(input memory.QualityProbeInput, code memory.QualityFindingCode, severity memory.QualityFindingSeverity, component memory.QualityFindingComponent, category memory.QualityFindingCategory, message string, memoryID string) memory.QualityEvaluationFinding {
	return memory.QualityEvaluationFinding{
		EvaluationRunID:         input.EvaluationRunID,
		Scope:                   input.Scope,
		Code:                    code,
		Severity:                severity,
		Component:               component,
		Category:                category,
		Message:                 message,
		SuggestedActionCategory: qualityProbeSuggestedAction(code),
		Metadata: map[string]string{
			"probe": "retrieval_context",
		},
		Evidence: map[string]any{
			"query":     input.Query,
			"memory_id": memoryID,
		},
		CreatedAt: input.ObservedAt,
	}
}

func expectedMemorySet(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func hasProbeCheck(checks []memory.QualityEvaluationCheck, target memory.QualityEvaluationCheck) bool {
	for _, check := range checks {
		if check == target {
			return true
		}
	}
	return false
}

func flattenContextHits(context retrieval.AssembledContext) []retrieval.SearchHit {
	total := len(context.Profile) + len(context.RecentSession) + len(context.RecentEpisodes) + len(context.RelevantSummaries) + len(context.RelatedEntities)
	hits := make([]retrieval.SearchHit, 0, total)
	hits = append(hits, context.Profile...)
	hits = append(hits, context.RecentSession...)
	hits = append(hits, context.RecentEpisodes...)
	hits = append(hits, context.RelevantSummaries...)
	hits = append(hits, context.RelatedEntities...)
	return hits
}

func hiddenMemoryState(state memory.MemoryState) bool {
	return state != memory.MemoryStateActive
}

func qualityProbeSuggestedAction(code memory.QualityFindingCode) memory.RepairActionCategory {
	switch code {
	case memory.QualityFindingExpectedRecallMissing:
		return memory.RepairActionCategoryEmbeddingRetry
	default:
		return memory.RepairActionCategoryManualReview
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ memory.QualityProbe = retrievalQualityProbe{}
