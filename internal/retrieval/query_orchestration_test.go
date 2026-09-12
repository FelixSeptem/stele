package retrieval

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

type orchestrationAnalyzer struct {
	result QueryAnalysisResult
	err    error
}

func (a orchestrationAnalyzer) Analyze(QueryAnalysisInput) (QueryAnalysisResult, error) {
	return a.result, a.err
}

type recordingLexical struct {
	inputs []SearchInput
	fail   map[string]error
	hits   map[string][]ScoredMemory
}

type recordingSemantic struct{ hits map[string][]ScoredMemory }

func (s recordingSemantic) SearchSemantic(_ context.Context, in SearchInput) ([]ScoredMemory, error) {
	return s.hits[in.Query], nil
}

func (s *recordingLexical) SearchLexical(_ context.Context, in SearchInput) ([]ScoredMemory, error) {
	s.inputs = append(s.inputs, in)
	if err := s.fail[in.Query]; err != nil {
		return nil, err
	}
	if hits, ok := s.hits[in.Query]; ok {
		return hits, nil
	}
	return []ScoredMemory{{Memory: memory.CanonicalMemory{ID: in.Query, Scope: in.Scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}}, nil
}

type blockingAnalyzer struct{ started chan struct{} }

func (a blockingAnalyzer) Analyze(QueryAnalysisInput) (QueryAnalysisResult, error) {
	close(a.started)
	select {}
}

type chunkRecorder struct {
	inputs     []ChunkSearchInput
	candidates []ChunkCandidate
}

func (c *chunkRecorder) SearchChunks(_ context.Context, in ChunkSearchInput) ([]ChunkCandidate, error) {
	c.inputs = append(c.inputs, in)
	return c.candidates, nil
}

type qaPolicyReader struct{ policy memory.RankingRolloutPolicy }

func (r qaPolicyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return memory.RankingRolloutPolicy{}, errors.New("unexpected active reader")
}

type effectiveOnlyReader struct{ policy memory.RankingRolloutPolicy }

func (r effectiveOnlyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
}
func (r effectiveOnlyReader) ReadEffectiveQueryAnalysisRolloutPolicy(context.Context, memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return r.policy, nil
}
func (r qaPolicyReader) ReadEffectiveQueryAnalysisRolloutPolicy(context.Context, memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return r.policy, nil
}

func activeQAPolicy(scope memory.Scope, limits QueryAnalysisLimits) memory.RankingRolloutPolicy {
	return memory.RankingRolloutPolicy{Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope, Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch}, QueryAnalysis: &memory.QueryAnalysisRolloutPolicy{SchemaVersion: memory.QueryAnalysisRolloutSchemaVersionV1, PolicyVersion: memory.QueryAnalysisPolicyVersionV1, LimitsVersion: memory.QueryAnalysisLimitsVersionV1, MaxQueryBytes: limits.MaxQueryBytes, MaxHints: limits.MaxHints, MaxSignals: limits.MaxSignals, MaxSubqueries: limits.MaxSubqueries, MaxTermBytes: limits.MaxTermBytes, MaxSubqueryBytes: limits.MaxSubqueryBytes, MaxAnalysisWork: limits.MaxAnalysisWork, MaxCandidatesPerSignal: limits.MaxCandidatesPerSignal, MaxAggregateCandidates: limits.MaxAggregateCandidates, MaxElapsed: limits.MaxElapsed, ExpiresAt: time.Now().Add(time.Hour)}}
}

func TestQueryRecallInputsRetainsOriginalAndConstraints(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	in := SearchInput{Scope: scope, Query: "Original", Classes: []memory.MemoryClass{memory.MemoryClassEpisodic}, TimeFrom: time.Unix(1, 0), TimeTo: time.Unix(2, 0), TopK: 10}
	ai := QueryAnalysisInput{AcceptedQuery: in.Query, PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits, Constraints: QueryAnalysisConstraints{Classes: in.Classes, TimeFrom: in.TimeFrom, TimeTo: in.TimeTo}}
	result, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(ServiceDependencies{QueryAnalyzer: orchestrationAnalyzer{result: result}, QueryAnalysisLimits: limits})
	got, _ := s.queryRecallInputs(context.Background(), in, ptrPolicy(activeQAPolicy(scope, limits)), memory.RankingRolloutSurfaceSearch)
	if len(got) != 2 || got[0].Query != "Original" || got[1].Query != "derived" {
		t.Fatalf("inputs=%+v", got)
	}
	if !reflect.DeepEqual(got[0].Scope, got[1].Scope) || !reflect.DeepEqual(got[0].Classes, got[1].Classes) || got[1].TimeFrom != in.TimeFrom || got[1].TimeTo != in.TimeTo {
		t.Fatal("derived constraints widened")
	}
}

func TestQueryRecallInputsFailureIsOriginalOnly(t *testing.T) {
	in := SearchInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Query: "Original"}
	s := NewService(ServiceDependencies{QueryAnalyzer: orchestrationAnalyzer{err: errors.New("boom")}})
	got, _ := s.queryRecallInputs(context.Background(), in, nil, memory.RankingRolloutSurfaceSearch)
	if len(got) != 1 || got[0].Query != in.Query {
		t.Fatalf("inputs=%+v", got)
	}
}

func TestSearchOptionalDerivedFailurePreservesOriginalResults(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	ai := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	lex := &recordingLexical{fail: map[string]error{"derived": errors.New("optional")}}
	s := NewService(ServiceDependencies{Lexical: lex, RankingRolloutPolicyReader: effectiveOnlyReader{policy: activeQAPolicy(scope, limits)}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "Original" {
		t.Fatalf("hits=%+v", result.Hits)
	}
	if got := []string{lex.inputs[0].Query, lex.inputs[1].Query}; !reflect.DeepEqual(got, []string{"Original", "derived"}) {
		t.Fatalf("order=%v", got)
	}
}

func TestSearchOriginalLexicalFailureKeepsPublicError(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	want := errors.New("original failed")
	lex := &recordingLexical{fail: map[string]error{"Original": want}}
	_, err := NewService(ServiceDependencies{Lexical: lex}).Search(context.Background(), SearchInput{Scope: scope, Query: "Original"})
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestSearchAnalyzerTimeoutPreservesOriginalRecall(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	started := make(chan struct{})
	lex := &recordingLexical{}
	s := NewService(ServiceDependencies{Lexical: lex, QueryAnalyzer: blockingAnalyzer{started: started}, QueryAnalysisLimits: func() QueryAnalysisLimits {
		l := DefaultQueryAnalysisLimits()
		l.MaxElapsed = time.Millisecond
		return l
	}()})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct{})
	var result SearchResult
	var err error
	go func() { result, err = s.Search(ctx, SearchInput{Scope: scope, Query: "Original"}); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("analyzer did not start")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("search did not return after analyzer timeout")
	}
	if err != nil || len(result.Hits) != 1 || result.Hits[0].Memory.ID != "Original" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSearchMalformedQueryAnalysisPolicyFallsBackWithoutLimits(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	limits.MaxCandidatesPerSignal = 1
	policy := activeQAPolicy(scope, limits)
	policy.QueryAnalysis.MaxCandidatesPerSignal = 0 // malformed limits
	lex := &recordingLexical{hits: map[string][]ScoredMemory{"Original": {
		{Memory: memory.CanonicalMemory{ID: "one", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}},
		{Memory: memory.CanonicalMemory{ID: "two", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}},
	}}}
	s := NewService(ServiceDependencies{Lexical: lex, QueryAnalyzer: orchestrationAnalyzer{result: QueryAnalysisResult{}}, RankingRolloutPolicyReader: effectiveOnlyReader{policy: policy}})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "Original"})
	if err != nil || len(result.Hits) != 2 {
		t.Fatalf("hits=%+v err=%v", result.Hits, err)
	}
}

func TestSearchChunkCandidatesRespectPerSignalAndAggregateCaps(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	limits.MaxCandidatesPerSignal = 2
	limits.MaxAggregateCandidates = 3
	chunks := &chunkRecorder{candidates: []ChunkCandidate{
		testChunk(scope, "m1", "chunk c1", memory.MemoryStateActive),
		testChunk(scope, "m2", "chunk c2", memory.MemoryStateActive),
		testChunk(scope, "m3", "chunk c3", memory.MemoryStateActive),
	}}
	s := NewService(ServiceDependencies{Chunks: chunks, ChunkRollout: memory.ChunkRolloutModeActive, QueryAnalyzer: orchestrationAnalyzer{result: QueryAnalysisResult{}}, QueryAnalysisLimits: limits})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", TopK: 10})
	if err != nil || len(result.Hits) != 2 {
		t.Fatalf("hits=%+v err=%v", result.Hits, err)
	}
}

func TestSearchOriginalSignalReservesCandidateBudgetBeforeDerived(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	limits.MaxCandidatesPerSignal = 1
	limits.MaxAggregateCandidates = 10
	ai := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	lex := &recordingLexical{hits: map[string][]ScoredMemory{
		"Original": {{Memory: memory.CanonicalMemory{ID: "original", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}, {Memory: memory.CanonicalMemory{ID: "extra", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
		"derived":  {{Memory: memory.CanonicalMemory{ID: "derived", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
	}}
	s := NewService(ServiceDependencies{Lexical: lex, RankingRolloutPolicyReader: effectiveOnlyReader{policy: activeQAPolicy(scope, limits)}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", TopK: 10})
	if err != nil || len(result.Hits) != 2 || result.Hits[0].Memory.ID != "original" {
		t.Fatalf("hits=%+v err=%v", result.Hits, err)
	}
}

func TestSearchOriginalSemanticIsProcessedBeforeDerivedLexical(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	limits.MaxCandidatesPerSignal = 2
	limits.MaxAggregateCandidates = 2
	ai := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	lex := &recordingLexical{hits: map[string][]ScoredMemory{
		"Original": {{Memory: memory.CanonicalMemory{ID: "original-lexical", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
		"derived":  {{Memory: memory.CanonicalMemory{ID: "derived-lexical", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
	}}
	semantic := recordingSemantic{hits: map[string][]ScoredMemory{
		"Original": {{Memory: memory.CanonicalMemory{ID: "original-semantic", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
	}}
	s := NewService(ServiceDependencies{Lexical: lex, Semantic: semantic, RankingRolloutPolicyReader: effectiveOnlyReader{policy: activeQAPolicy(scope, limits)}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, hit := range result.Hits {
		ids[hit.Memory.ID] = true
	}
	if !ids["original-lexical"] || !ids["original-semantic"] || ids["derived-lexical"] {
		t.Fatalf("hits=%+v", result.Hits)
	}
}

func ptrPolicy(p memory.RankingRolloutPolicy) *memory.RankingRolloutPolicy { return &p }
