package app

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestRetrievalQualityProbeFindsLifecycleAndExpectedRecallIssues(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observedAt := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)
	probe := retrievalQualityProbe{
		Searcher: stubQualityProbeSearcher{result: retrieval.SearchResult{Hits: []retrieval.SearchHit{
			{Memory: memory.CanonicalMemory{ID: "mem_hidden", Scope: scope, State: memory.MemoryStateSuppressed}},
		}}},
		Assembler: stubQualityProbeAssembler{context: retrieval.AssembledContext{
			Profile: []retrieval.SearchHit{
				{Memory: memory.CanonicalMemory{ID: "mem_expired", Scope: scope, State: memory.MemoryState("expired")}},
			},
		}},
	}

	findings, err := probe.RunQualityProbe(context.Background(), memory.QualityProbeInput{
		Scope:             scope,
		EvaluationRunID:   "eval_1",
		Checks:            []memory.QualityEvaluationCheck{memory.QualityEvaluationCheckRetrieval, memory.QualityEvaluationCheckContext},
		Query:             "projection quality",
		ExpectedMemoryIDs: []string{"mem_expected"},
		ContextBudget:     1,
		ObservedAt:        observedAt,
	})
	if err != nil {
		t.Fatalf("RunQualityProbe() error = %v", err)
	}
	if !hasProbeFinding(findings, memory.QualityFindingLifecycleHiddenReturned, "mem_hidden") {
		t.Fatalf("findings = %+v, want lifecycle finding for suppressed memory", findings)
	}
	if !hasProbeFinding(findings, memory.QualityFindingLifecycleHiddenReturned, "mem_expired") {
		t.Fatalf("findings = %+v, want lifecycle finding for expired memory", findings)
	}
	if !hasProbeFinding(findings, memory.QualityFindingExpectedRecallMissing, "mem_expected") {
		t.Fatalf("findings = %+v, want expected recall missing finding", findings)
	}
	for _, finding := range findings {
		if finding.Code == memory.QualityFindingExpectedRecallMissing && finding.SuggestedActionCategory != memory.RepairActionCategoryEmbeddingRetry {
			t.Fatalf("expected recall finding action = %q, want embedding_retry", finding.SuggestedActionCategory)
		}
	}
}

func hasProbeFinding(findings []memory.QualityEvaluationFinding, code memory.QualityFindingCode, memoryID string) bool {
	for _, finding := range findings {
		if finding.Code != code {
			continue
		}
		if got, _ := finding.Evidence["memory_id"].(string); got == memoryID {
			return true
		}
	}
	return false
}

type stubQualityProbeSearcher struct {
	result retrieval.SearchResult
}

func (s stubQualityProbeSearcher) Search(ctx context.Context, input retrieval.SearchInput) (retrieval.SearchResult, error) {
	return s.result, nil
}

type stubQualityProbeAssembler struct {
	context retrieval.AssembledContext
}

func (s stubQualityProbeAssembler) AssembleContext(ctx context.Context, input retrieval.AssembleContextInput) (retrieval.AssembledContext, error) {
	return s.context, nil
}
