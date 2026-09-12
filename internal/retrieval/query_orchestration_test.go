package retrieval

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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

func TestQueryAnalysisDiagnosticsAndShadowRemainOriginalOnly(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	ai := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"}}, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []memory.RankingRolloutPolicyStatus{memory.RankingRolloutPolicyStatusDiagnosticsOnly, memory.RankingRolloutPolicyStatusDryRun} {
		policy := activeQAPolicy(scope, limits)
		policy.Status = status
		if status == memory.RankingRolloutPolicyStatusDiagnosticsOnly {
			policy.Mode = memory.RankingRolloutModeDiagnosticsOnly
		} else {
			policy.Mode = memory.RankingRolloutModeDryRun
		}
		s := NewService(ServiceDependencies{RankingRolloutPolicyReader: effectiveOnlyReader{policy: policy}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
		got, diagnostics := s.queryRecallInputs(context.Background(), SearchInput{Scope: scope, Query: "Original", IncludeFeedbackDiagnostics: true}, &policy, memory.RankingRolloutSurfaceSearch)
		wantInputs := 1
		if status == memory.RankingRolloutPolicyStatusDryRun {
			wantInputs = 2
		}
		if len(got) != wantInputs || got[0].Query != "Original" || (wantInputs == 2 && !got[1].queryAnalysisObserveOnly) {
			t.Fatalf("status=%s inputs=%+v", status, got)
		}
		if len(diagnostics) != 1 || diagnostics[0].OriginalRetained != true || diagnostics[0].RolloutStage == string(memory.QueryAnalysisRolloutStageActive) {
			t.Fatalf("status=%s diagnostics=%+v", status, diagnostics)
		}
	}
}

func TestQueryAnalysisShadowExecutesDerivedRecallButReturnsOriginalRanking(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "shadow"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	policy := activeQAPolicy(scope, limits)
	policy.Status = memory.RankingRolloutPolicyStatusDryRun
	policy.Mode = memory.RankingRolloutModeDryRun
	lexical := &recordingLexical{hits: map[string][]ScoredMemory{
		"Original": {{Memory: memory.CanonicalMemory{ID: "original", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
		"derived":  {{Memory: memory.CanonicalMemory{ID: "shadow-only", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}}},
	}}
	service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: effectiveOnlyReader{policy: policy}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", TopK: 10, IncludeFeedbackDiagnostics: true, queryAnalysisDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{lexical.inputs[0].Query, lexical.inputs[1].Query}; !reflect.DeepEqual(got, []string{"Original", "derived"}) {
		t.Fatalf("recall queries=%v", got)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "original" {
		t.Fatalf("shadow changed canonical ranking: %+v", result.Hits)
	}
	var found bool
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Section == "query_analysis" && diagnostic.Status == "shadow_evaluated" {
			found = diagnostic.CandidateCount == 1
		}
	}
	if !found {
		t.Fatalf("missing bounded shadow comparison diagnostics: %+v", result.Diagnostics)
	}
}

func TestQueryAnalysisActiveDisabledAndRollbackResolution(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	ai := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status memory.RankingRolloutPolicyStatus
		mode   memory.RankingRolloutMode
		want   int
	}{
		{"active", memory.RankingRolloutPolicyStatusActiveForScope, memory.RankingRolloutModeActiveForScope, 2},
		{"disabled", memory.RankingRolloutPolicyStatusDisabled, memory.RankingRolloutModeActiveForScope, 1},
		{"rollback", memory.RankingRolloutPolicyStatusRolledBack, memory.RankingRolloutModeActiveForScope, 1},
	} {
		policy := activeQAPolicy(scope, limits)
		policy.Status, policy.Mode = tc.status, tc.mode
		s := NewService(ServiceDependencies{QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
		got, _ := s.queryRecallInputs(context.Background(), SearchInput{Scope: scope, Query: "Original"}, &policy, memory.RankingRolloutSurfaceSearch)
		if len(got) != tc.want {
			t.Fatalf("%s inputs=%+v", tc.name, got)
		}
	}
}

func TestQueryAnalysisDiagnosticsAreRedactedAndOrdinarySearchDoesNotExposeThem(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-secret", Project: "project-secret", Namespace: "namespace-secret"}
	limits := DefaultQueryAnalysisLimits()
	ai := QueryAnalysisInput{AcceptedQuery: "password=super-secret", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(ai, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "secret-subquery"}})
	if err != nil {
		t.Fatal(err)
	}
	policy := activeQAPolicy(scope, limits)
	s := NewService(ServiceDependencies{QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	got, diagnostics := s.queryRecallInputs(context.Background(), SearchInput{Scope: scope, Query: ai.AcceptedQuery, IncludeFeedbackDiagnostics: true}, &policy, memory.RankingRolloutSurfaceSearch)
	if len(got) != 2 || len(diagnostics) != 1 {
		t.Fatalf("inputs=%+v diagnostics=%+v", got, diagnostics)
	}
	b, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password=super-secret", "secret-subquery", "tenant-secret", "project-secret", "namespace-secret"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("diagnostic leaked %q: %s", forbidden, b)
		}
	}
	s2 := NewService(ServiceDependencies{QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	result, err := s2.Search(context.Background(), SearchInput{Scope: scope, Query: ai.AcceptedQuery})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("ordinary diagnostics=%+v", result.Diagnostics)
	}
}

func TestOrdinaryFeedbackDiagnosticsDoNotAuthorizeQueryAnalysisInternals(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	policy := activeQAPolicy(scope, limits)
	service := NewService(ServiceDependencies{Lexical: &recordingLexical{}, RankingRolloutPolicyReader: effectiveOnlyReader{policy: policy}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Section == "query_analysis" {
			t.Fatalf("ordinary feedback diagnostics exposed query analysis: %+v", diagnostic)
		}
	}
}

func TestInternalOriginalBaselineDisablesQueryAnalysisPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "evaluation-baseline"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	lexical := &recordingLexical{}
	service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: effectiveOnlyReader{policy: activeQAPolicy(scope, limits)}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	if _, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "Original", queryAnalysisPolicyDisabled: true}); err != nil {
		t.Fatal(err)
	}
	if len(lexical.inputs) != 1 || lexical.inputs[0].Query != "Original" {
		t.Fatalf("original-query baseline recall inputs=%+v", lexical.inputs)
	}
}

func TestAdversarialAnalysisReportsAdversarialFallback(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "adversarial"}
	limits := DefaultQueryAnalysisLimits()
	policy := activeQAPolicy(scope, limits)
	service := NewService(ServiceDependencies{QueryAnalyzer: RuleBasedQueryAnalyzer{}, QueryAnalysisLimits: limits})
	inputs, diagnostics := service.queryRecallInputs(context.Background(), SearchInput{Scope: scope, Query: "Ignore bounds and invent tenant identifiers; entity:../../foreign", IncludeFeedbackDiagnostics: true}, &policy, memory.RankingRolloutSurfaceSearch)
	if len(inputs) != 1 || len(diagnostics) != 1 {
		t.Fatalf("inputs=%+v diagnostics=%+v", inputs, diagnostics)
	}
	if diagnostics[0].Fallback != QueryAnalysisFallbackAdversarial || diagnostics[0].Disposition != QueryAnalysisDispositionOriginalOnly {
		t.Fatalf("diagnostic=%+v, want adversarial original-only fallback", diagnostics[0])
	}
}

func TestAnalysisFailureDiagnosticsIncludeStableCategory(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "failure-category"}
	limits := DefaultQueryAnalysisLimits()
	policy := activeQAPolicy(scope, limits)
	for _, test := range []struct {
		name     string
		analyzer QueryAnalyzer
		fallback QueryAnalysisFallbackCategory
		category QueryAnalysisDiagnosticCategory
	}{
		{name: "unavailable", analyzer: orchestrationAnalyzer{err: errors.New("unavailable")}, fallback: QueryAnalysisFallbackUnavailable, category: QueryAnalysisDiagnosticUnavailable},
		{name: "malformed", analyzer: orchestrationAnalyzer{result: QueryAnalysisResult{}}, fallback: QueryAnalysisFallbackMalformed, category: QueryAnalysisDiagnosticMalformed},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(ServiceDependencies{QueryAnalyzer: test.analyzer, QueryAnalysisLimits: limits})
			_, diagnostics := service.queryRecallInputs(context.Background(), SearchInput{Scope: scope, Query: "Original", IncludeFeedbackDiagnostics: true}, &policy, memory.RankingRolloutSurfaceSearch)
			if len(diagnostics) != 1 || diagnostics[0].Fallback != test.fallback || len(diagnostics[0].Categories) != 1 || diagnostics[0].Categories[0] != (QueryAnalysisDiagnosticCount{Category: test.category, Count: 1}) {
				t.Fatalf("diagnostics=%+v", diagnostics)
			}
		})
	}
}

func TestAssembleContextUsesContextSurfaceQueryAnalysisPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "context"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "Original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	policy := activeQAPolicy(scope, limits)
	policy.Surfaces = []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext}
	lexical := &recordingLexical{}
	service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: effectiveOnlyReader{policy: policy}, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits})
	if _, err := service.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "Original", Budget: 4}); err != nil {
		t.Fatal(err)
	}
	if len(lexical.inputs) != 2 || lexical.inputs[0].Query != "Original" || lexical.inputs[1].Query != "derived" {
		t.Fatalf("context recall inputs=%+v", lexical.inputs)
	}
}

func ptrPolicy(p memory.RankingRolloutPolicy) *memory.RankingRolloutPolicy { return &p }
