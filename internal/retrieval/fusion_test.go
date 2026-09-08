package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestFusionStrategyValidateRejectsUnsafeParameters(t *testing.T) {
	tests := []struct {
		name     string
		strategy FusionStrategy
	}{
		{
			name: "missing version",
			strategy: FusionStrategy{
				Name:                FusionStrategyRRF,
				RankConstant:        60,
				PerChannelCandidate: 10,
				TotalCandidates:     20,
			},
		},
		{
			name: "nonpositive rank constant",
			strategy: FusionStrategy{
				Name:                FusionStrategyRRF,
				Version:             "rrf-v1",
				RankConstant:        0,
				PerChannelCandidate: 10,
				TotalCandidates:     20,
			},
		},
		{
			name: "negative channel weight",
			strategy: FusionStrategy{
				Name:                FusionStrategyRRF,
				Version:             "rrf-v1",
				RankConstant:        60,
				PerChannelCandidate: 10,
				TotalCandidates:     20,
				ChannelWeights: map[FusionChannel]float64{
					FusionChannelLexical: -1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.strategy.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want unsafe strategy rejected")
			}
		})
	}
}

func TestFuseRRFCombinesRanksAndPreservesCanonicalIdentity(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	strategy := DefaultRRFStrategy()
	strategy.PerChannelCandidate = 5
	strategy.TotalCandidates = 5

	fused, err := FuseCandidates(strategy, []FusionChannelCandidates{
		{
			Channel: FusionChannelLexical,
			Candidates: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_shared", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)}, LexicalScore: 0.2},
				{Memory: memory.CanonicalMemory{ID: "mem_lexical", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, LexicalScore: 0.9},
			},
		},
		{
			Channel: FusionChannelSemantic,
			Candidates: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_semantic", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, SemanticScore: 0.8},
				{Memory: memory.CanonicalMemory{ID: "mem_shared", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)}, SemanticScore: 0.1},
			},
		},
	})
	if err != nil {
		t.Fatalf("FuseCandidates() error = %v", err)
	}
	if len(fused) != 3 {
		t.Fatalf("len(FuseCandidates()) = %d, want 3", len(fused))
	}
	if fused[0].Memory.ID != "mem_shared" {
		t.Fatalf("first fused memory = %q, want multi-channel mem_shared", fused[0].Memory.ID)
	}
	if fused[0].ChannelRanks[FusionChannelLexical] != 1 || fused[0].ChannelRanks[FusionChannelSemantic] != 2 {
		t.Fatalf("channel ranks = %+v, want lexical=1 semantic=2", fused[0].ChannelRanks)
	}
	if fused[0].Score <= fused[1].Score {
		t.Fatalf("shared RRF score = %v, next score = %v, want multi-channel score first", fused[0].Score, fused[1].Score)
	}
}

func TestFuseCandidatesUsesStableTieBreaks(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	strategy := DefaultRRFStrategy()
	strategy.PerChannelCandidate = 5
	strategy.TotalCandidates = 5
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

	fused, err := FuseCandidates(strategy, []FusionChannelCandidates{{
		Channel: FusionChannelLexical,
		Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_later", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 1},
			{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Hour)}, LexicalScore: 0.9},
		},
	}})
	if err != nil {
		t.Fatalf("FuseCandidates() error = %v", err)
	}
	if fused[0].Memory.ID != "mem_later" {
		t.Fatalf("first fused memory = %q, want higher rank to win before tie-break", fused[0].Memory.ID)
	}

	strategy.ChannelWeights = map[FusionChannel]float64{FusionChannelLexical: 0}
	if _, err := FuseCandidates(strategy, []FusionChannelCandidates{{
		Channel: FusionChannelLexical,
		Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_b", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 1},
			{Memory: memory.CanonicalMemory{ID: "mem_a", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 1},
		},
	}}); err == nil {
		t.Fatal("FuseCandidates() error = nil, want zero effective channel weight rejected")
	}
}

func TestFuseCandidatesAppliesDeterministicBoundsAndTieOrdering(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	strategy := DefaultRRFStrategy()
	strategy.PerChannelCandidate = 2
	strategy.TotalCandidates = 4

	fused, err := FuseCandidates(strategy, []FusionChannelCandidates{
		{Channel: FusionChannelLexical, Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "shared", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1},
			{Memory: memory.CanonicalMemory{ID: "lexical", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, LexicalScore: 0.9},
			{Memory: memory.CanonicalMemory{ID: "lexical-over-limit", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, LexicalScore: 0.8},
		}},
		{Channel: FusionChannelSemantic, Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "semantic", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, SemanticScore: 1},
			{Memory: memory.CanonicalMemory{ID: "shared", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, SemanticScore: 0.9},
		}},
		{Channel: FusionChannelRelation, Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "relation", Scope: scope, Class: memory.MemoryClassRelation, State: memory.MemoryStateActive}, RelationScore: 1},
		}},
		{Channel: FusionChannelChunk, Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "chunk-parent", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive}, LexicalScore: 1},
		}},
	})
	if err != nil {
		t.Fatalf("FuseCandidates() error = %v", err)
	}
	if len(fused) != strategy.TotalCandidates {
		t.Fatalf("len(FuseCandidates()) = %d, want total bound %d", len(fused), strategy.TotalCandidates)
	}
	for _, candidate := range fused {
		if candidate.Memory.ID == "lexical-over-limit" {
			t.Fatalf("over-limit candidate contributed to fusion: %+v", fused)
		}
	}

	tied := func(left, right memory.CanonicalMemory) []FusedCandidate {
		t.Helper()
		strategy := DefaultRRFStrategy()
		strategy.PerChannelCandidate = 2
		strategy.TotalCandidates = 2
		result, err := FuseCandidates(strategy, []FusionChannelCandidates{
			{Channel: FusionChannelLexical, Candidates: []ScoredMemory{{Memory: left, LexicalScore: 1}, {Memory: right, LexicalScore: 0.9}}},
			{Channel: FusionChannelSemantic, Candidates: []ScoredMemory{{Memory: right, SemanticScore: 1}, {Memory: left, SemanticScore: 0.9}}},
		})
		if err != nil {
			t.Fatalf("FuseCandidates() error = %v", err)
		}
		return result
	}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	if result := tied(
		memory.CanonicalMemory{ID: "profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
		memory.CanonicalMemory{ID: "summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, ModifiedAt: now},
	); result[0].Memory.ID != "summary" {
		t.Fatalf("class tie ordering = %+v, want summary first", result)
	}
	if result := tied(
		memory.CanonicalMemory{ID: "earlier", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Hour)},
		memory.CanonicalMemory{ID: "later", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
	); result[0].Memory.ID != "later" {
		t.Fatalf("timestamp tie ordering = %+v, want later first", result)
	}
	if result := tied(
		memory.CanonicalMemory{ID: "b", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
		memory.CanonicalMemory{ID: "a", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
	); result[0].Memory.ID != "a" {
		t.Fatalf("canonical ID tie ordering = %+v, want a first", result)
	}
}

func TestFuseNormalizedWeightedIsExplicit(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	strategy := DefaultNormalizedWeightedStrategy()
	strategy.PerChannelCandidate = 5
	strategy.TotalCandidates = 5

	fused, err := FuseCandidates(strategy, []FusionChannelCandidates{{
		Channel: FusionChannelLexical,
		Candidates: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_high", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 10},
			{Memory: memory.CanonicalMemory{ID: "mem_low", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1},
		},
	}})
	if err != nil {
		t.Fatalf("FuseCandidates() error = %v", err)
	}
	if len(fused) != 2 || fused[0].Memory.ID != "mem_high" {
		t.Fatalf("normalized fusion = %+v, want explicit score-aware mem_high first", fused)
	}
}
