package retrieval

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type qualityLexicalStub struct{}

func (qualityLexicalStub) SearchLexical(context.Context, SearchInput) ([]ScoredMemory, error) {
	return []ScoredMemory{
		{Memory: memory.CanonicalMemory{ID: "m1", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, State: memory.MemoryStateActive, Content: "one"}, LexicalScore: 1},
		{Memory: memory.CanonicalMemory{ID: "m2", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, State: memory.MemoryStateActive, Content: "two"}, LexicalScore: .9},
	}, nil
}

type qualityPolicyReader struct{ policy memory.RankingRolloutPolicy }

func (r qualityPolicyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return r.policy, nil
}

func TestSearchUsesRerankerOnlyWhenActiveModeIsConfigured(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	service := NewService(ServiceDependencies{Lexical: qualityLexicalStub{}, RankingRolloutPolicyReader: qualityPolicyReader{policy: memory.RankingRolloutPolicy{Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope, ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, LatestDryRunID: "dry-1", LatestDryRunStatus: memory.RankingRolloutThresholdStatusSatisfied, RerankerMode: string(RerankerModeActive), RerankerProvider: "static", RerankerVersion: "v1"}}, Reranker: StaticReranker{Scores: map[string]float64{"m2": 1, "m1": 0}}, RerankerMode: RerankerModeActive, RerankerProvider: "static", RerankerVersion: "v1", QualityBounds: QualityAdjustmentBounds{PerFeature: .01, Total: .02}})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "q", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 2 || result.Hits[0].Memory.ID != "m2" {
		t.Fatalf("hits=%+v, want reranked m2 first", result.Hits)
	}
}

func TestSearchFallsBackWhenRerankerIsShadowOnly(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	service := NewService(ServiceDependencies{Lexical: qualityLexicalStub{}, Reranker: StaticReranker{Scores: map[string]float64{"m2": 1, "m1": 0}}, RerankerMode: RerankerModeShadow})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "q", TopK: 2, IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hits[0].Memory.ID != "m1" {
		t.Fatalf("hits=%+v, shadow mode must retain baseline", result.Hits)
	}
}

func TestSearchDoesNotApplyActiveRerankerWithoutMatchingScopedPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	service := NewService(ServiceDependencies{Lexical: qualityLexicalStub{}, Reranker: StaticReranker{Scores: map[string]float64{"m2": 1, "m1": 0}}, RerankerMode: RerankerModeActive})
	baselineService := NewService(ServiceDependencies{Lexical: qualityLexicalStub{}})
	baseline, err := baselineService.Search(context.Background(), SearchInput{Scope: scope, Query: "q", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "q", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != len(baseline.Hits) || result.Hits[0].Memory.ID != baseline.Hits[0].Memory.ID {
		t.Fatalf("hits=%+v, active reranker without scoped policy must retain baseline", result.Hits)
	}
}
