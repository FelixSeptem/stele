package retrieval

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

type plannerPolicyReader struct {
	policy   memory.RankingRolloutPolicy
	qaPolicy *memory.RankingRolloutPolicy
	err      error
}

type plannerSelectorRecorder struct {
	policy memory.RankingRolloutPolicy
	inputs []memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput
}

func (reader *plannerSelectorRecorder) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
}

func (reader *plannerSelectorRecorder) ReadEffectiveRetrievalPlannerRolloutPolicy(_ context.Context, input memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	reader.inputs = append(reader.inputs, input)
	return reader.policy, nil
}

func (reader plannerPolicyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if reader.policy.Status == memory.RankingRolloutPolicyStatusActiveForScope {
		return reader.policy, nil
	}
	return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
}

func (reader plannerPolicyReader) ReadEffectiveRetrievalPlannerRolloutPolicy(context.Context, memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return reader.policy, reader.err
}

func (reader plannerPolicyReader) ReadEffectiveQueryAnalysisRolloutPolicy(context.Context, memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if reader.qaPolicy == nil {
		return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
	}
	return *reader.qaPolicy, nil
}

type plannerChannelRecorder struct {
	inputs []SearchInput
	hits   [][]ScoredMemory
	errAt  map[int]error
}

func (recorder *plannerChannelRecorder) next(input SearchInput) ([]ScoredMemory, error) {
	recorder.inputs = append(recorder.inputs, input)
	call := len(recorder.inputs)
	if err := recorder.errAt[call]; err != nil {
		return nil, err
	}
	if call <= len(recorder.hits) {
		return recorder.hits[call-1], nil
	}
	return nil, nil
}

func (recorder *plannerChannelRecorder) SearchLexical(_ context.Context, input SearchInput) ([]ScoredMemory, error) {
	return recorder.next(input)
}

func (recorder *plannerChannelRecorder) SearchSemantic(_ context.Context, input SearchInput) ([]ScoredMemory, error) {
	return recorder.next(input)
}

func (recorder *plannerChannelRecorder) SearchRelations(_ context.Context, input SearchInput) ([]ScoredMemory, error) {
	return recorder.next(input)
}

type plannerReranker struct{ calls int }

func (reranker *plannerReranker) Rerank(_ context.Context, request RerankRequest) ([]RerankScore, error) {
	reranker.calls++
	scores := make([]RerankScore, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		scores = append(scores, RerankScore{ID: candidate.ID, Score: 0})
	}
	return scores, nil
}

type plannerQueryRecorder struct {
	scope   memory.Scope
	queries []string
	topKs   []int
	counts  map[string]int
}

func (recorder *plannerQueryRecorder) SearchLexical(_ context.Context, input SearchInput) ([]ScoredMemory, error) {
	recorder.queries = append(recorder.queries, input.Query)
	recorder.topKs = append(recorder.topKs, input.TopK)
	if recorder.counts == nil {
		recorder.counts = make(map[string]int)
	}
	recorder.counts[input.Query]++
	switch input.Query {
	case "original":
		if recorder.counts[input.Query] == 1 {
			return []ScoredMemory{plannerTestHit(recorder.scope, "base", memory.MemoryClassEpisodic)}, nil
		}
		return []ScoredMemory{plannerTestHit(recorder.scope, "follow-up", memory.MemoryClassEpisodic)}, nil
	case "derived-one", "derived-two":
		return []ScoredMemory{plannerTestHit(recorder.scope, "shadow-"+input.Query, memory.MemoryClassEpisodic)}, nil
	default:
		return nil, nil
	}
}

type plannerSlowUsefulness struct{ delay time.Duration }

func (summarizer plannerSlowUsefulness) SummarizeUsefulnessFeedback(context.Context, memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error) {
	if summarizer.delay > 0 {
		time.Sleep(summarizer.delay)
	}
	return memory.UsefulnessFeedbackSummary{}, nil
}

type plannerDeadlineSearcher struct {
	calls        int
	deadlines    []bool
	fallbackHits []ScoredMemory
}

func (searcher *plannerDeadlineSearcher) SearchLexical(ctx context.Context, _ SearchInput) ([]ScoredMemory, error) {
	searcher.calls++
	_, hasDeadline := ctx.Deadline()
	searcher.deadlines = append(searcher.deadlines, hasDeadline)
	if searcher.calls == 1 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return searcher.fallbackHits, nil
}

type plannerExpiryLexical struct {
	calls       int
	scope       memory.Scope
	fallbackErr error
}

func (searcher *plannerExpiryLexical) SearchLexical(ctx context.Context, _ SearchInput) ([]ScoredMemory, error) {
	searcher.calls++
	if searcher.calls == 1 {
		<-ctx.Done()
		return []ScoredMemory{plannerTestHit(searcher.scope, "expired-planned", memory.MemoryClassEpisodic)}, nil
	}
	if searcher.fallbackErr != nil {
		return nil, searcher.fallbackErr
	}
	return []ScoredMemory{plannerTestHit(searcher.scope, "baseline", memory.MemoryClassEpisodic)}, nil
}

type plannerDeadlineSemantic struct{ deadlines []bool }

func (searcher *plannerDeadlineSemantic) SearchSemantic(ctx context.Context, _ SearchInput) ([]ScoredMemory, error) {
	_, hasDeadline := ctx.Deadline()
	searcher.deadlines = append(searcher.deadlines, hasDeadline)
	return nil, nil
}

type plannerFollowUpExpiryLexical struct {
	calls int
	scope memory.Scope
}

func (searcher *plannerFollowUpExpiryLexical) SearchLexical(ctx context.Context, _ SearchInput) ([]ScoredMemory, error) {
	searcher.calls++
	switch searcher.calls {
	case 1:
		return []ScoredMemory{plannerTestHit(searcher.scope, "first-pass", memory.MemoryClassEpisodic)}, nil
	case 2:
		<-ctx.Done()
		return []ScoredMemory{plannerTestHit(searcher.scope, "expired-follow-up", memory.MemoryClassEpisodic)}, nil
	default:
		return []ScoredMemory{plannerTestHit(searcher.scope, "baseline", memory.MemoryClassEpisodic)}, nil
	}
}

type plannerChunkRecorder struct {
	calls      int
	candidates []ChunkCandidate
	err        error
}

func (recorder *plannerChunkRecorder) SearchChunks(context.Context, ChunkSearchInput) ([]ChunkCandidate, error) {
	recorder.calls++
	return recorder.candidates, recorder.err
}

func plannerTestHit(scope memory.Scope, id string, class memory.MemoryClass) ScoredMemory {
	return ScoredMemory{Memory: memory.CanonicalMemory{ID: id, Scope: scope, Class: class, State: memory.MemoryStateActive}}
}

func plannerTestRollout(scope memory.Scope, surface memory.RankingRolloutSurface, status memory.RankingRolloutPolicyStatus) memory.RankingRolloutPolicy {
	var mode memory.RankingRolloutMode
	switch status {
	case memory.RankingRolloutPolicyStatusDiagnosticsOnly:
		mode = memory.RankingRolloutModeDiagnosticsOnly
	case memory.RankingRolloutPolicyStatusDryRun:
		mode = memory.RankingRolloutModeDryRun
	case memory.RankingRolloutPolicyStatusActiveForScope:
		mode = memory.RankingRolloutModeActiveForScope
	}
	return memory.RankingRolloutPolicy{
		Scope: scope, Status: status, Mode: mode, Surfaces: []memory.RankingRolloutSurface{surface},
		RetrievalPlanner: &memory.RetrievalPlannerRolloutPolicy{
			SchemaVersion:  memory.RetrievalPlannerRolloutSchemaVersionV1,
			PlannerVersion: string(RetrievalPlannerVersionV1), PolicyVersion: string(RetrievalPlanPolicyVersionV1),
			AnalysisPolicyVersion: string(QueryAnalysisPolicyVersionV1), FusionVersion: DefaultRRFStrategy().Version,
			RankingVersion: retrievalPlannerRankingVersion, RendererVersion: retrievalPlannerRendererVersion,
			MaxCandidates: 200, MaxCandidatesPerChannel: 100, MaxPasses: 2, MaxLatency: 5 * time.Second,
			MaxContextItems: 100, MaxRerankerHeadroom: 100, ExpiresAt: time.Now().Add(time.Hour),
		},
	}
}

func plannerTestPolicy(configure func(*RetrievalPlanTemplate)) RetrievalPlanPolicy {
	policy := DefaultRetrievalPlanPolicy()
	for family, template := range policy.Templates {
		configure(&template)
		template.Fusion.TotalCandidates = template.TotalCandidates
		minimumAllocation := template.TotalCandidates
		for _, channel := range template.Channels {
			if allocation := template.ChannelCandidates[channel]; allocation < minimumAllocation {
				minimumAllocation = allocation
			}
		}
		template.Fusion.PerChannelCandidate = minimumAllocation
		policy.Templates[family] = template
	}
	return policy
}

func lexicalOnlyPlannerPolicy(total int) RetrievalPlanPolicy {
	return plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: total}
		template.TotalCandidates = total
		template.Fusion = DefaultRRFStrategy()
		template.RerankerEligible = false
		template.RerankerHeadroom = 0
		template.MaxPasses = 1
		template.FollowUp = RetrievalPlanFollowUpRule{}
	})
}

func TestSearchRetrievalPlannerRolloutModesPreserveBaselineUnlessActive(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-modes"}
	statuses := []struct {
		name   string
		status memory.RankingRolloutPolicyStatus
		active bool
	}{
		{name: "diagnostics", status: memory.RankingRolloutPolicyStatusDiagnosticsOnly},
		{name: "shadow", status: memory.RankingRolloutPolicyStatusDryRun},
		{name: "active", status: memory.RankingRolloutPolicyStatusActiveForScope, active: true},
		{name: "disabled", status: memory.RankingRolloutPolicyStatusDisabled},
		{name: "rollback", status: memory.RankingRolloutPolicyStatusRolledBack},
	}
	for _, test := range statuses {
		t.Run(test.name, func(t *testing.T) {
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
			semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
			policy := lexicalOnlyPlannerPolicy(4)
			for family, template := range policy.Templates {
				template.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 1}
				policy.Templates[family] = template
			}
			service := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, test.status)}, RetrievalPlanPolicy: policy})
			result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
			if err != nil {
				t.Fatal(err)
			}
			wantHits, wantSemanticCalls := 2, 1
			if test.active {
				wantHits = 1
			}
			if len(result.Hits) != wantHits || len(semantic.inputs) != wantSemanticCalls {
				t.Fatalf("hits=%v semantic calls=%d", hitIDs(result.Hits), len(semantic.inputs))
			}
		})
	}

	t.Run("incompatible", func(t *testing.T) {
		rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
		rollout.RetrievalPlanner.FusionVersion = "foreign"
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
		semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(4)}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || len(result.Hits) != 2 || len(semantic.inputs) != 1 {
			t.Fatalf("hits=%v semantic calls=%d err=%v", hitIDs(result.Hits), len(semantic.inputs), err)
		}
	})
}

func TestSearchActivePlanUsesChannelBudgetFusionAndOriginalQuery(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-budget"}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{
		plannerTestHit(scope, "one", memory.MemoryClassEpisodic), plannerTestHit(scope, "two", memory.MemoryClassEpisodic), plannerTestHit(scope, "three", memory.MemoryClassEpisodic),
	}}}
	policy := lexicalOnlyPlannerPolicy(2)
	template := policy.Templates[RetrievalQueryFamilyGeneral]
	template.Fusion.RankConstant = 7
	policy.Templates[RetrievalQueryFamilyGeneral] = template
	service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "mandatory original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 2 || len(lexical.inputs) != 1 || lexical.inputs[0].Query != "mandatory original" || lexical.inputs[0].TopK != 2 {
		t.Fatalf("hits=%v inputs=%+v", hitIDs(result.Hits), lexical.inputs)
	}
	if result.fusionStrategy.RankConstant != 7 {
		t.Fatalf("fusion=%+v", result.fusionStrategy)
	}
}

func TestSearchPlannerRerankerRequiresEligibilityAndLedgerHeadroom(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-rerank"}
	for _, test := range []struct {
		name          string
		eligible      bool
		headroom      int
		wantCalls     int
		wantAttempted bool
		wantUsed      bool
	}{
		{name: "not eligible", eligible: false},
		{name: "no headroom", eligible: true},
		{name: "eligible with headroom", eligible: true, headroom: 1, wantCalls: 1, wantAttempted: true, wantUsed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy := lexicalOnlyPlannerPolicy(2)
			for family, template := range policy.Templates {
				template.RerankerEligible = test.eligible
				template.RerankerHeadroom = test.headroom
				policy.Templates[family] = template
			}
			rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
			rollout.ThresholdStatus = memory.RankingRolloutThresholdStatusSatisfied
			rollout.LatestDryRunStatus = memory.RankingRolloutThresholdStatusSatisfied
			rollout.LatestDryRunID = "dry-run"
			rollout.RerankerMode = string(RerankerModeActive)
			reranker := &plannerReranker{}
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "one", memory.MemoryClassEpisodic)}}}
			service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy, Reranker: reranker, RerankerMode: RerankerModeActive})
			result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
			if err != nil {
				t.Fatal(err)
			}
			if reranker.calls != test.wantCalls {
				t.Fatalf("reranker calls=%d want=%d", reranker.calls, test.wantCalls)
			}
			if result.rerankerObservation.Attempted != test.wantAttempted || result.rerankerObservation.Used != test.wantUsed || !result.rerankerObservation.Safe {
				t.Fatalf("reranker observation=%+v", result.rerankerObservation)
			}
		})
	}
}

func TestSearchPlannerShadowRerankerRequiresMatchingRolloutBeforeProviderCall(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-shadow-rerank-policy"}
	policy := lexicalOnlyPlannerPolicy(4)
	for family, template := range policy.Templates {
		template.RerankerEligible = true
		template.RerankerHeadroom = 2
		policy.Templates[family] = template
	}
	for _, test := range []struct {
		name      string
		configure func(*memory.RankingRolloutPolicy)
		wantCalls int
	}{
		{name: "no reranker rollout"},
		{name: "foreign provider", configure: func(rollout *memory.RankingRolloutPolicy) {
			rollout.RerankerMode = string(RerankerModeShadow)
			rollout.RerankerProvider = "foreign"
			rollout.RerankerVersion = "v1"
		}},
		{name: "matching shadow rollout", configure: func(rollout *memory.RankingRolloutPolicy) {
			rollout.RerankerMode = string(RerankerModeShadow)
			rollout.RerankerProvider = "static"
			rollout.RerankerVersion = "v1"
		}, wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusDryRun)
			if test.configure != nil {
				test.configure(&rollout)
			}
			reranker := &plannerReranker{}
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{
				{plannerTestHit(scope, "baseline-one", memory.MemoryClassEpisodic), plannerTestHit(scope, "baseline-two", memory.MemoryClassEpisodic)},
				{plannerTestHit(scope, "planned-one", memory.MemoryClassEpisodic), plannerTestHit(scope, "planned-two", memory.MemoryClassEpisodic)},
			}}
			result, err := NewService(ServiceDependencies{
				Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy,
				Reranker: reranker, RerankerMode: RerankerModeShadow, RerankerProvider: "static", RerankerVersion: "v1",
			}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
			if err != nil {
				t.Fatal(err)
			}
			if reranker.calls != test.wantCalls {
				t.Fatalf("reranker calls=%d want=%d", reranker.calls, test.wantCalls)
			}
			if !reflect.DeepEqual(hitIDs(result.Hits), []string{"baseline-one", "baseline-two"}) {
				t.Fatalf("shadow reranker changed ordinary order: %v", hitIDs(result.Hits))
			}
		})
	}
}

func TestSearchActivePlannerKeepsQueryAnalysisShadowOutsidePlannerBudget(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-qa-shadow-budget"}
	limits := DefaultQueryAnalysisLimits()
	analysis, err := NewQueryAnalysisResult(
		QueryAnalysisInput{AcceptedQuery: "original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits},
		QueryAnalysisDispositionComplete,
		nil,
		[]QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived-one"}, {Kind: QueryAnalysisSignalTerm, Text: "derived-two"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	qaRollout := activeQAPolicy(scope, limits)
	qaRollout.Status = memory.RankingRolloutPolicyStatusDryRun
	qaRollout.Mode = memory.RankingRolloutModeDryRun
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 4}
		template.TotalCandidates = 4
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelLexical}}
		template.RerankerEligible = true
		template.RerankerHeadroom = 2
	})
	plannerRollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	plannerRollout.ThresholdStatus = memory.RankingRolloutThresholdStatusSatisfied
	plannerRollout.LatestDryRunStatus = memory.RankingRolloutThresholdStatusSatisfied
	plannerRollout.LatestDryRunID = "dry-run"
	plannerRollout.RerankerMode = string(RerankerModeActive)
	plannerRollout.RerankerProvider = "static"
	plannerRollout.RerankerVersion = "v1"
	lexical := &plannerQueryRecorder{scope: scope}
	reranker := &plannerReranker{}
	result, err := NewService(ServiceDependencies{
		Lexical: lexical, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerRollout, qaPolicy: &qaRollout}, RetrievalPlanPolicy: policy,
		Reranker: reranker, RerankerMode: RerankerModeActive, RerankerProvider: "static", RerankerVersion: "v1",
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hitIDs(result.Hits), []string{"base"}) {
		t.Fatalf("ordinary hits=%v, want only active planner results", hitIDs(result.Hits))
	}
	if lexical.counts["original"] != 2 || lexical.counts["derived-one"] != 1 || lexical.counts["derived-two"] != 1 {
		t.Fatalf("query calls=%v", lexical.queries)
	}
	if reranker.calls != 1 || !result.rerankerObservation.Used {
		t.Fatalf("reranker calls=%d observation=%+v", reranker.calls, result.rerankerObservation)
	}
}

func TestSearchPlannerLateFallbackRetainsCandidatesWithoutReissuingProviderWork(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-bounded-late-fallback"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.LatencyBudget = 2 * time.Millisecond
		policy.Templates[family] = template
	}
	lexical := &plannerExpiryLexical{scope: scope}
	result, err := NewService(ServiceDependencies{
		Lexical:                    lexical,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if lexical.calls != 1 || result.retrievalPlan != nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"expired-planned"}) {
		t.Fatalf("calls=%d plan=%v hits=%v", lexical.calls, result.retrievalPlan, hitIDs(result.Hits))
	}
}

func TestSearchPlannerFailureDoesNotRestartProvidersBeyondOneCandidateEnvelope(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-bounded-provider-fallback"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2, FusionChannelSemantic: 2}
		template.TotalCandidates = 4
	})
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "planned", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "unmetered-baseline", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{errAt: map[int]error{1: errors.New("semantic failed")}}
	want := errors.New("semantic failed")
	semantic.errAt[1] = want
	result, err := NewService(ServiceDependencies{
		Lexical: lexical, Semantic: semantic,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	requested := 0
	for _, input := range append(append([]SearchInput(nil), lexical.inputs...), semantic.inputs...) {
		requested += input.TopK
	}
	if len(lexical.inputs) != 1 || len(semantic.inputs) != 1 || requested > 4 {
		t.Fatalf("lexical=%d semantic=%d requested=%d", len(lexical.inputs), len(semantic.inputs), requested)
	}
	if result.retrievalPlan != nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"planned"}) {
		t.Fatalf("plan=%v hits=%v", result.retrievalPlan, hitIDs(result.Hits))
	}
}

func TestSearchPlannerFallbackRetainsOmittedCanonicalChannelInsidePlanEnvelope(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-omitted-channel-fallback"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelSemantic}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 2}
		template.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1}
		template.TotalCandidates = 2
	})
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "canonical-lexical", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{errAt: map[int]error{1: errors.New("semantic failed")}}
	result, err := NewService(ServiceDependencies{
		Lexical: lexical, Semantic: semantic,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	requested := 0
	for _, input := range append(append([]SearchInput(nil), lexical.inputs...), semantic.inputs...) {
		requested += input.TopK
	}
	if len(lexical.inputs) != 1 || len(semantic.inputs) != 1 || requested > 2 {
		t.Fatalf("lexical=%d semantic=%d requested=%d", len(lexical.inputs), len(semantic.inputs), requested)
	}
	if result.retrievalPlan != nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"canonical-lexical"}) {
		t.Fatalf("plan=%v hits=%v", result.retrievalPlan, hitIDs(result.Hits))
	}
}

func TestSearchPlannerRerankerLatencyCauseStaysPrivateAndAccurate(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-rerank-latency"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.RerankerEligible, template.RerankerHeadroom, template.LatencyBudget = true, 1, 5*time.Millisecond
		policy.Templates[family] = template
	}
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	rollout.ThresholdStatus, rollout.LatestDryRunStatus, rollout.LatestDryRunID = memory.RankingRolloutThresholdStatusSatisfied, memory.RankingRolloutThresholdStatusSatisfied, "dry-run"
	rollout.RerankerMode = string(RerankerModeActive)
	reranker := &plannerReranker{}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "one", memory.MemoryClassEpisodic)}}}
	result, err := NewService(ServiceDependencies{Lexical: lexical, UsefulnessSummarizer: plannerSlowUsefulness{delay: 10 * time.Millisecond}, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy, Reranker: reranker, RerankerMode: RerankerModeActive}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10, IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.retrievalPlan == nil || result.retrievalPlan.LatencyBudget != 5*time.Millisecond {
		t.Fatalf("retrieval plan=%+v", result.retrievalPlan)
	}
	if reranker.calls != 0 || result.rerankerObservation.FallbackCategory != "planner_latency_exhausted" {
		t.Fatalf("calls=%d observation=%+v", reranker.calls, result.rerankerObservation)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Section == "rerank" && (diagnostic.Reason != "optional reranker not applied" || strings.Contains(strings.ToLower(diagnostic.Reason), "planner") || strings.Contains(strings.ToLower(diagnostic.Reason), "latency")) {
			t.Fatalf("public reranker diagnostic=%+v", diagnostic)
		}
	}
}

func TestSearchPlannerRunsAtMostOneEvidenceFollowUpWithinBudget(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-follow-up"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 4}
		template.TotalCandidates = 4
		template.Fusion = DefaultRRFStrategy()
		template.RerankerEligible = false
		template.RerankerHeadroom = 0
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 2, Channels: []FusionChannel{FusionChannelLexical}}
	})
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)

	t.Run("sufficient first pass does not follow up", func(t *testing.T) {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "one", memory.MemoryClassEpisodic), plannerTestHit(scope, "two", memory.MemoryClassEpisodic)}}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || len(lexical.inputs) != 1 || len(result.Hits) != 2 {
			t.Fatalf("calls=%d hits=%v err=%v", len(lexical.inputs), hitIDs(result.Hits), err)
		}
	})

	t.Run("no remaining headroom does not follow up", func(t *testing.T) {
		noHeadroom := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
			template.Channels = []FusionChannel{FusionChannelLexical}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2}
			template.TotalCandidates = 2
			template.RerankerEligible = true
			template.RerankerHeadroom = 1
			template.MaxPasses = 2
			template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelLexical}}
		})
		duplicate := plannerTestHit(scope, "duplicate", memory.MemoryClassEpisodic)
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{duplicate, duplicate}}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: noHeadroom}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || len(lexical.inputs) != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"duplicate"}) {
			t.Fatalf("calls=%d hits=%v err=%v", len(lexical.inputs), hitIDs(result.Hits), err)
		}
	})

	t.Run("insufficient first pass adds one terminal pass and deduplicates", func(t *testing.T) {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{
			{plannerTestHit(scope, "duplicate", memory.MemoryClassEpisodic)},
			{plannerTestHit(scope, "duplicate", memory.MemoryClassEpisodic), plannerTestHit(scope, "new", memory.MemoryClassEpisodic)},
			{plannerTestHit(scope, "forbidden-third", memory.MemoryClassEpisodic)},
		}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(lexical.inputs) != 2 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"duplicate", "new"}) {
			t.Fatalf("calls=%d hits=%v", len(lexical.inputs), hitIDs(result.Hits))
		}
	})

	t.Run("follow-up failure preserves first pass", func(t *testing.T) {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "first", memory.MemoryClassEpisodic)}}, errAt: map[int]error{2: errors.New("optional follow-up failed")}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || len(result.Hits) != 1 || result.Hits[0].Memory.ID != "first" || len(lexical.inputs) != 2 {
			t.Fatalf("calls=%d hits=%v err=%v", len(lexical.inputs), hitIDs(result.Hits), err)
		}
	})
}

func TestSearchPlannerBoundsRequestedCandidatesAcrossSignals(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-request-signals"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "derived"}})
	if err != nil {
		t.Fatal(err)
	}
	qaRollout := activeQAPolicy(scope, limits)
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{nil, nil}}
	result, err := NewService(ServiceDependencies{
		Lexical: lexical, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope), qaPolicy: &qaRollout},
		RetrievalPlanPolicy:        lexicalOnlyPlannerPolicy(4),
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 3})
	if err != nil {
		t.Fatal(err)
	}
	requested := 0
	for _, call := range lexical.inputs {
		requested += call.TopK
	}
	if requested != 4 || len(lexical.inputs) != 2 || lexical.inputs[0].TopK != 3 || lexical.inputs[1].TopK != 1 {
		t.Fatalf("requested=%d inputs=%+v", requested, lexical.inputs)
	}
	if result.retrievalPlan == nil || len(result.Hits) != 0 {
		t.Fatalf("plan=%v hits=%v", result.retrievalPlan, hitIDs(result.Hits))
	}
}

func TestSearchPlannerBoundsRequestedCandidatesAcrossFollowUpPass(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-request-followup"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 4}
		template.TotalCandidates = 4
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 2, Channels: []FusionChannel{FusionChannelLexical}}
	})
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "first", memory.MemoryClassEpisodic)}, nil}}
	result, err := NewService(ServiceDependencies{
		Lexical:                    lexical,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 3})
	if err != nil {
		t.Fatal(err)
	}
	requested := 0
	for _, call := range lexical.inputs {
		requested += call.TopK
	}
	if len(lexical.inputs) != 2 || requested != 4 || lexical.inputs[1].TopK > 2 {
		t.Fatalf("requested=%d inputs=%+v", requested, lexical.inputs)
	}
	if !reflect.DeepEqual(hitIDs(result.Hits), []string{"first"}) {
		t.Fatalf("hits=%v", hitIDs(result.Hits))
	}
}

func TestSearchPlannerFollowUpFailureRecordsTerminalEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-followup-terminal-failure"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 4}
		template.TotalCandidates = 4
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 2, Channels: []FusionChannel{FusionChannelLexical}}
	})
	foreign := plannerTestHit(memory.Scope{Tenant: "other", Project: "p", Namespace: "planner-followup-terminal-failure"}, "foreign", memory.MemoryClassEpisodic)
	lexical := &plannerChannelRecorder{
		hits:  [][]ScoredMemory{{plannerTestHit(scope, "visible", memory.MemoryClassEpisodic), foreign}},
		errAt: map[int]error{2: errors.New("follow-up unavailable")},
	}
	result, err := NewService(ServiceDependencies{
		Lexical:                    lexical,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.retrievalPassObservations) != 2 {
		t.Fatalf("pass observations=%+v", result.retrievalPassObservations)
	}
	terminal := result.retrievalPassObservations[1].Evidence
	if terminal.Disposition != EvidenceDispositionTerminalIncomplete || terminal.FollowUpEligible {
		t.Fatalf("terminal evidence=%+v", terminal)
	}
}

func TestAssembleContextActivePlannerPreservesSummaryPreference(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-context-summary-preference"}
	policy := lexicalOnlyPlannerPolicy(4)
	for family, template := range policy.Templates {
		template.ContextPriorities = []memory.MemoryClass{memory.MemoryClassEpisodic, memory.MemoryClassSummary, memory.MemoryClassProfile, memory.MemoryClassProcedural, memory.MemoryClassRelation}
		policy.Templates[family] = template
	}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{
		{Memory: plannerTestHit(scope, "episode", memory.MemoryClassEpisodic).Memory, Citations: []Citation{{MemoryID: "episode", Operation: "source"}}},
		{Memory: plannerTestHit(scope, "summary", memory.MemoryClassSummary).Memory, Citations: []Citation{{MemoryID: "summary", Operation: "source"}}},
	}}}
	contextResult, err := NewService(ServiceDependencies{
		Lexical:                    lexical,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceContext, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "original", Budget: 1, IncludeDiagnostics: true, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hitIDs(contextResult.RelevantSummaries), []string{"summary"}) || len(contextResult.RecentSession) != 0 {
		t.Fatalf("context=%+v", contextResult)
	}
	foundSummaryCitation, foundOmission := false, false
	for _, citation := range contextResult.Citations {
		if citation.MemoryID == "summary" {
			foundSummaryCitation = true
		}
	}
	for _, diagnostic := range contextResult.plannerDiagnostics {
		if diagnostic.Section == "context_planner" && diagnostic.Status == "omitted_by_quota_or_budget" && diagnostic.Omitted > 0 {
			foundOmission = true
		}
	}
	if !foundSummaryCitation || !foundOmission {
		t.Fatalf("citations=%+v planner diagnostics=%+v", contextResult.Citations, contextResult.plannerDiagnostics)
	}
	for _, diagnostic := range contextResult.Diagnostics {
		if diagnostic.Section == "context_planner" {
			t.Fatalf("public diagnostics leaked planner omission: %+v", contextResult.Diagnostics)
		}
	}
}

func TestAssembleContextActivePlannerPriorityAndQuotaKeepExistingSections(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-context"}
	policy := lexicalOnlyPlannerPolicy(10)
	for family, template := range policy.Templates {
		template.ContextPriorities = []memory.MemoryClass{memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassRelation}
		template.MemoryClassQuotas = map[memory.MemoryClass]int{memory.MemoryClassProcedural: 1, memory.MemoryClassSummary: 1, memory.MemoryClassProfile: 1}
		policy.Templates[family] = template
	}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{
		plannerTestHit(scope, "profile", memory.MemoryClassProfile),
		plannerTestHit(scope, "summary", memory.MemoryClassSummary),
		plannerTestHit(scope, "procedure", memory.MemoryClassProcedural),
	}}}
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceContext, memory.RankingRolloutPolicyStatusActiveForScope)
	contextResult, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "original", Budget: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(contextResult.RecentEpisodes) != 1 || contextResult.RecentEpisodes[0].Memory.ID != "procedure" || len(contextResult.RelevantSummaries) != 1 || contextResult.RelevantSummaries[0].Memory.ID != "summary" || len(contextResult.Profile) != 0 {
		t.Fatalf("context=%+v", contextResult)
	}
}

func TestSearchPlannerFaultsFallBackButBaselineErrorsRemainErrors(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-fault"}
	invalid := lexicalOnlyPlannerPolicy(2)
	invalid.Version = "unknown"
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: invalid}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if err != nil || len(result.Hits) != 2 {
		t.Fatalf("planner fault did not fall back: hits=%v err=%v", hitIDs(result.Hits), err)
	}

	want := errors.New("baseline failed")
	lexical = &plannerChannelRecorder{errAt: map[int]error{1: want}}
	_, err = NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{err: errors.New("planner read failed")}}).Search(context.Background(), SearchInput{Scope: scope, Query: "original"})
	if !errors.Is(err, want) {
		t.Fatalf("error=%v want=%v", err, want)
	}
}

func TestSearchPlannerEnforcesLatencyDeadlineAndFallsBack(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-deadline"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.LatencyBudget = 2 * time.Millisecond
		policy.Templates[family] = template
	}
	lexical := &plannerDeadlineSearcher{fallbackHits: []ScoredMemory{plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}}
	_, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if !errors.Is(err, context.DeadlineExceeded) || lexical.calls != 1 || !reflect.DeepEqual(lexical.deadlines, []bool{true}) {
		t.Fatalf("calls=%d deadlines=%v error=%v", lexical.calls, lexical.deadlines, err)
	}
}

func TestSearchPlannerDoesNotStartAnotherChannelAfterDeadline(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-deadline-next-channel"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1, FusionChannelSemantic: 1}
		template.TotalCandidates = 2
		template.LatencyBudget = 2 * time.Millisecond
	})
	lexical := &plannerExpiryLexical{scope: scope}
	semantic := &plannerDeadlineSemantic{}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if lexical.calls != 1 || len(semantic.deadlines) != 0 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"expired-planned"}) || result.retrievalPlan != nil {
		t.Fatalf("lexical calls=%d semantic deadlines=%v hits=%v", lexical.calls, semantic.deadlines, hitIDs(result.Hits))
	}
}

func TestSearchPlannerAcceptedAccountingFailureUsesBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-accepted-accounting-fallback"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.LatencyBudget = 50 * time.Millisecond
		policy.Templates[family] = template
	}
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)

	t.Run("baseline result", func(t *testing.T) {
		lexical := &plannerExpiryLexical{scope: scope}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
		if err != nil {
			t.Fatal(err)
		}
		if lexical.calls != 1 || result.retrievalPlan != nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"expired-planned"}) {
			t.Fatalf("calls=%d plan=%v hits=%v", lexical.calls, result.retrievalPlan, hitIDs(result.Hits))
		}
	})

	t.Run("baseline error", func(t *testing.T) {
		lexical := &plannerExpiryLexical{scope: scope, fallbackErr: errors.New("unmetered rerun must not execute")}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
		if err != nil || lexical.calls != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"expired-planned"}) {
			t.Fatalf("calls=%d error=%v hits=%v", lexical.calls, err, hitIDs(result.Hits))
		}
	})
}

func TestSearchPlannerFollowUpAcceptedAccountingFailureUsesBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-followup-accounting-fallback"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 4}
		template.TotalCandidates = 4
		template.MaxPasses = 2
		template.LatencyBudget = 50 * time.Millisecond
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 2, Channels: []FusionChannel{FusionChannelLexical}}
	})
	lexical := &plannerFollowUpExpiryLexical{scope: scope}
	result, err := NewService(ServiceDependencies{
		Lexical:                    lexical,
		RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)},
		RetrievalPlanPolicy:        policy,
	}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if lexical.calls != 2 || result.retrievalPlan == nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"first-pass"}) {
		t.Fatalf("calls=%d plan=%v hits=%v", lexical.calls, result.retrievalPlan, hitIDs(result.Hits))
	}
}

func TestSearchPlannerAssessesCanonicalEvidenceAndRecordsTerminalPass(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-evidence"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 3, FusionChannelSemantic: 2}
		template.TotalCandidates = 5
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelLexical}}
	})
	duplicate := plannerTestHit(scope, "duplicate", memory.MemoryClassEpisodic)
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{duplicate}, {duplicate}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{duplicate}}}
	observer := &plannerTelemetryObserver{}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}, observer).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(lexical.inputs) != 2 || len(result.plannerDiagnostics) != 1 || result.plannerDiagnostics[0].PassCount != 2 || result.plannerDiagnostics[0].EvidenceDisposition != EvidenceDispositionTerminalIncomplete {
		t.Fatalf("lexical calls=%d diagnostics=%+v", len(lexical.inputs), result.plannerDiagnostics)
	}
	if len(observer.events) != 1 {
		t.Fatalf("planner events=%+v", observer.events)
	}
	if event := observer.events[0]; event.Pass != 2 || event.Evidence != string(EvidenceDispositionTerminalIncomplete) {
		t.Fatalf("planner event=%+v", event)
	}
	if len(observer.channels) != 2 {
		t.Fatalf("planner channel events=%+v", observer.channels)
	}
	for _, event := range observer.channels {
		if event.Availability != "available" {
			t.Fatalf("planner channel event=%+v", event)
		}
	}
	if len(result.retrievalPassObservations) != 2 || result.retrievalPassObservations[0].Pass != 1 || result.retrievalPassObservations[0].CandidateCount != 2 || result.retrievalPassObservations[0].Evidence.Disposition != EvidenceDispositionBelowMinimum || result.retrievalPassObservations[1].Pass != 2 || result.retrievalPassObservations[1].CandidateCount != 1 || result.retrievalPassObservations[1].Evidence.Disposition != EvidenceDispositionTerminalIncomplete {
		t.Fatalf("pass observations=%+v", result.retrievalPassObservations)
	}
}

func TestSearchPlannerUsesSpecializedAnalysisOnlyWhenActive(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-analysis"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "original", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 1}
		policy.Templates[family] = template
	}
	procedural := policy.Templates[RetrievalQueryFamilyProcedural]
	procedural.Channels = []FusionChannel{FusionChannelSemantic}
	procedural.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 2}
	procedural.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1}
	policy.Templates[RetrievalQueryFamilyProcedural] = procedural
	plannerRollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	qaRollout := activeQAPolicy(scope, limits)
	for _, test := range []struct {
		name         string
		qa           *memory.RankingRolloutPolicy
		wantLexical  int
		wantSemantic int
	}{
		{name: "unavailable", wantLexical: 1},
		{name: "active", qa: &qaRollout, wantSemantic: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
			semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassProcedural)}}}
			result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerRollout, qaPolicy: test.qa}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
			if err != nil {
				t.Fatal(err)
			}
			wantHit := "lexical"
			if test.qa != nil {
				wantHit = "semantic"
			}
			if len(lexical.inputs) != 1 || len(semantic.inputs) != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{wantHit}) {
				t.Fatalf("lexical=%d semantic=%d hits=%v", len(lexical.inputs), len(semantic.inputs), hitIDs(result.Hits))
			}
		})
	}
}

func TestSearchPlannerRejectsPlanFusionDependencyMismatch(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-fusion-dependency"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.Fusion.Version = "foreign-fusion-v1"
		policy.Templates[family] = template
	}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.retrievalPlan != nil || result.fusionStrategy.Version != DefaultRRFStrategy().Version || len(result.Hits) != 2 {
		t.Fatalf("plan=%v fusion=%+v hits=%v", result.retrievalPlan, result.fusionStrategy, hitIDs(result.Hits))
	}
}

func TestSearchPlannerFallsBackWhenFusionNameMismatchesApprovedVersion(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-fusion-name"}
	policy := lexicalOnlyPlannerPolicy(2)
	for family, template := range policy.Templates {
		template.Fusion.Name = FusionStrategyNormalizedWeighted
		template.Fusion.Version = DefaultRRFStrategy().Version
		policy.Templates[family] = template
	}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.retrievalPlan != nil || result.fusionStrategy.Name != FusionStrategyRRF || !reflect.DeepEqual(hitIDs(result.Hits), []string{"lexical", "semantic"}) {
		t.Fatalf("plan=%v fusion=%+v hits=%v", result.retrievalPlan, result.fusionStrategy, hitIDs(result.Hits))
	}
}

func TestSearchPlannerMissingOrFailedChannelsUseBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-channel-fallback"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2, FusionChannelSemantic: 2}
		template.TotalCandidates = 4
	})
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)

	t.Run("missing", func(t *testing.T) {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || result.retrievalPlan != nil || !reflect.DeepEqual(hitIDs(result.Hits), []string{"baseline"}) {
			t.Fatalf("hits=%v plan=%v err=%v", hitIDs(result.Hits), result.retrievalPlan, err)
		}
	})

	t.Run("semantic failure", func(t *testing.T) {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "planned", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}}}
		semantic := &plannerChannelRecorder{errAt: map[int]error{1: errors.New("planned semantic failed")}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || result.retrievalPlan != nil || len(lexical.inputs) != 1 || len(semantic.inputs) != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"planned"}) {
			t.Fatalf("hits=%v plan=%v err=%v", hitIDs(result.Hits), result.retrievalPlan, err)
		}
	})

	t.Run("relation failure", func(t *testing.T) {
		relationPolicy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
			template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelRelation}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2, FusionChannelRelation: 2}
			template.TotalCandidates = 4
		})
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "planned", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}}}
		relations := &plannerChannelRecorder{errAt: map[int]error{1: errors.New("planned relation failed")}}
		result, err := NewService(ServiceDependencies{Lexical: lexical, Relations: relations, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: relationPolicy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10, IncludeRelations: true})
		if err != nil || result.retrievalPlan != nil || len(lexical.inputs) != 1 || len(relations.inputs) != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"planned"}) {
			t.Fatalf("hits=%v plan=%v err=%v", hitIDs(result.Hits), result.retrievalPlan, err)
		}
	})

	t.Run("chunk failure", func(t *testing.T) {
		chunkPolicy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
			template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelChunk}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2, FusionChannelChunk: 2}
			template.TotalCandidates = 4
		})
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "planned", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}}}
		chunks := &plannerChunkRecorder{err: errors.New("planned chunk failed")}
		result, err := NewService(ServiceDependencies{Lexical: lexical, Chunks: chunks, ChunkRollout: memory.ChunkRolloutModeActive, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: chunkPolicy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if err != nil || result.retrievalPlan != nil || len(lexical.inputs) != 1 || chunks.calls != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"planned"}) {
			t.Fatalf("hits=%v plan=%v err=%v", hitIDs(result.Hits), result.retrievalPlan, err)
		}
	})

	t.Run("fallback keeps baseline error", func(t *testing.T) {
		want := errors.New("baseline lexical failed")
		lexical := &plannerChannelRecorder{errAt: map[int]error{1: want}}
		semantic := &plannerChannelRecorder{}
		_, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
		if !errors.Is(err, want) || len(lexical.inputs) != 1 || len(semantic.inputs) != 0 {
			t.Fatalf("lexical=%d semantic=%d error=%v want=%v", len(lexical.inputs), len(semantic.inputs), err, want)
		}
	})
}

func TestSearchPlannerShadowExecutesBoundedComparisonWithoutChangingResults(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-shadow-comparison"}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "baseline-lexical", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "shadow-only", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "baseline-semantic", memory.MemoryClassEpisodic)}}}
	observer := &plannerTelemetryObserver{}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusDryRun)}, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(2)}, observer).Search(context.Background(), SearchInput{Scope: scope, Query: "private query tenant-secret", TopK: 10, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(lexical.inputs) != 2 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"baseline-lexical", "baseline-semantic"}) {
		t.Fatalf("lexical calls=%d hits=%v", len(lexical.inputs), hitIDs(result.Hits))
	}
	if len(result.plannerDiagnostics) != 1 || result.plannerDiagnostics[0].ChangedRankCount != 2 || result.plannerDiagnostics[0].ChangedRankBucket != "1_5" || !reflect.DeepEqual(result.plannerDiagnostics[0].ChannelAvailability, []RetrievalPlannerChannelAvailability{{Channel: FusionChannelLexical, Availability: "available"}}) {
		t.Fatalf("shadow comparison diagnostics=%+v", result.plannerDiagnostics)
	}
	if len(observer.events) != 1 || observer.events[0].Stage != string(memory.RetrievalPlannerRolloutStageShadow) {
		t.Fatalf("shadow execution telemetry=%+v", observer.events)
	}
	if len(observer.channels) != 1 || observer.channels[0].Channel != string(FusionChannelLexical) || observer.channels[0].Availability != "available" {
		t.Fatalf("shadow channel telemetry=%+v", observer.channels)
	}
	if len(observer.changedRanks) != 1 || observer.changedRanks[0].Bucket != "1_5" || observer.changedRanks[0].Count != 2 {
		t.Fatalf("shadow changed-rank telemetry=%+v", observer.changedRanks)
	}
	payload, marshalErr := MarshalRetrievalPlannerDiagnostics(result.plannerDiagnostics[0])
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, forbidden := range []string{"private query", "tenant-secret", "baseline-lexical", "shadow-only", "score", "provider_payload"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("shadow diagnostics leaked %q: %s", forbidden, payload)
		}
	}
}

func TestSearchPlannerDiagnosticsAndTelemetryCoverNonActiveStages(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-non-active-diagnostics"}
	for _, test := range []struct {
		name   string
		status memory.RankingRolloutPolicyStatus
		stage  memory.RetrievalPlannerRolloutStage
	}{
		{name: "diagnostics only", status: memory.RankingRolloutPolicyStatusDiagnosticsOnly, stage: memory.RetrievalPlannerRolloutStageDiagnosticsOnly},
		{name: "shadow", status: memory.RankingRolloutPolicyStatusDryRun, stage: memory.RetrievalPlannerRolloutStageShadow},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &plannerTelemetryObserver{}
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "baseline", memory.MemoryClassEpisodic)}, {plannerTestHit(scope, "shadow", memory.MemoryClassEpisodic)}}}
			result, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, test.status)}, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(2)}, observer).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10, retrievalPlannerDiagnosticsAuthorized: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.plannerDiagnostics) != 1 || result.plannerDiagnostics[0].RolloutStage != test.stage || result.plannerDiagnostics[0].PassCount != 1 {
				t.Fatalf("diagnostics=%+v", result.plannerDiagnostics)
			}
			if test.stage == memory.RetrievalPlannerRolloutStageDiagnosticsOnly && (result.plannerDiagnostics[0].ChangedRankBucket != "not_evaluated" || result.plannerDiagnostics[0].ChannelAvailability[0].Availability != "not_evaluated") {
				t.Fatalf("diagnostics-only comparison evidence=%+v", result.plannerDiagnostics[0])
			}
			if len(observer.events) != 1 || observer.events[0].Stage != string(test.stage) || observer.events[0].Pass != 1 {
				t.Fatalf("events=%+v", observer.events)
			}
			if len(observer.channels) != 1 {
				t.Fatalf("channel events=%+v", observer.channels)
			}
			if test.stage == memory.RetrievalPlannerRolloutStageDiagnosticsOnly && (observer.channels[0].Availability != "not_evaluated" || len(observer.changedRanks) != 0) {
				t.Fatalf("diagnostics-only telemetry channels=%+v changed_ranks=%+v", observer.channels, observer.changedRanks)
			}
		})
	}
}

func TestSearchPlannerRejectsChunkFollowUpBeforeExecution(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-chunk-followup"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelChunk}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelChunk: 2}
		template.TotalCandidates = 2
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 2, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelChunk}}
	})
	chunks := &plannerChunkRecorder{candidates: []ChunkCandidate{testChunk(scope, "parent", "evidence", memory.MemoryStateActive)}}
	result, err := NewService(ServiceDependencies{Chunks: chunks, ChunkRollout: memory.ChunkRolloutModeActive, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10, IncludeSummaries: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.retrievalPlan != nil || chunks.calls != 1 {
		t.Fatalf("plan=%v chunk calls=%d", result.retrievalPlan, chunks.calls)
	}
}

func TestSearchPlannerInvalidFollowUpFallsBackBeforeExecution(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-invalid-followup"}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2}
		template.TotalCandidates = 2
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelSemantic}}
	})
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "lexical", memory.MemoryClassEpisodic)}}}
	semantic := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "semantic", memory.MemoryClassEpisodic)}}}
	result, err := NewService(ServiceDependencies{Lexical: lexical, Semantic: semantic, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy}).Search(context.Background(), SearchInput{Scope: scope, Query: "original", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.retrievalPlan != nil || len(lexical.inputs) != 1 || len(semantic.inputs) != 1 || !reflect.DeepEqual(hitIDs(result.Hits), []string{"lexical", "semantic"}) {
		t.Fatalf("plan=%v lexical=%d semantic=%d hits=%v", result.retrievalPlan, len(lexical.inputs), len(semantic.inputs), hitIDs(result.Hits))
	}
}

func TestAssembleContextPlannerReportsBoundedOmissionsAndPreservesCitations(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-context-diagnostics"}
	policy := lexicalOnlyPlannerPolicy(10)
	for family, template := range policy.Templates {
		template.ContextPriorities = []memory.MemoryClass{memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassRelation}
		template.MemoryClassQuotas = map[memory.MemoryClass]int{memory.MemoryClassProcedural: 1, memory.MemoryClassSummary: 1, memory.MemoryClassProfile: 1}
		policy.Templates[family] = template
	}
	newService := func(status memory.RankingRolloutPolicyStatus) *Service {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{
			{Memory: plannerTestHit(scope, "profile", memory.MemoryClassProfile).Memory, Citations: []Citation{{MemoryID: "profile", Operation: "source"}}},
			plannerTestHit(scope, "procedure-one", memory.MemoryClassProcedural), plannerTestHit(scope, "procedure-two", memory.MemoryClassProcedural),
			plannerTestHit(scope, "summary", memory.MemoryClassSummary),
		}}}
		return NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceContext, status)}, RetrievalPlanPolicy: policy})
	}
	active, err := newService(memory.RankingRolloutPolicyStatusActiveForScope).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "original", Budget: 2, IncludeDiagnostics: true, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(active.RecentEpisodes) != 1 || len(active.RelevantSummaries) != 1 || len(active.Citations) != 1 {
		t.Fatalf("active context=%+v", active)
	}
	foundOmission := false
	for _, diagnostic := range active.plannerDiagnostics {
		if diagnostic.Section == "context_planner" && diagnostic.Status == "omitted_by_quota_or_budget" && diagnostic.Omitted > 0 {
			foundOmission = true
		}
	}
	if !foundOmission {
		t.Fatalf("planner diagnostics=%+v", active.plannerDiagnostics)
	}
	for _, diagnostic := range active.Diagnostics {
		if diagnostic.Section == "context_planner" {
			t.Fatalf("public diagnostics leaked planner omission: %+v", active.Diagnostics)
		}
	}
	for _, status := range []memory.RankingRolloutPolicyStatus{memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutPolicyStatusRolledBack} {
		baseline, err := newService(status).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "original", Budget: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(baseline.Profile) != 1 || len(baseline.RelevantSummaries) != 1 || len(baseline.RecentEpisodes) != 0 {
			t.Fatalf("status=%s context=%+v", status, baseline)
		}
	}
}

func TestSearchPlannerAppliesOnlyMatchingSessionAndUserSelectors(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-search-selector"}
	policy := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	policy.RetrievalPlannerSelector = memory.RetrievalPlannerRolloutSelector{SessionID: "session-match", UserID: "user-match"}
	for _, test := range []struct {
		name      string
		sessionID string
		userID    string
		wantTopK  int
	}{
		{name: "matching", sessionID: "session-match", userID: "user-match", wantTopK: 2},
		{name: "foreign session", sessionID: "session-foreign", userID: "user-match", wantTopK: 10},
		{name: "foreign user", sessionID: "session-match", userID: "user-foreign", wantTopK: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &plannerSelectorRecorder{policy: policy}
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "visible", memory.MemoryClassEpisodic)}}}
			_, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: reader, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(2)}).Search(context.Background(), SearchInput{
				Scope: scope, Query: "original", TopK: 10, SessionID: test.sessionID, UserID: test.userID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.inputs) != 1 || reader.inputs[0].SessionID != test.sessionID || reader.inputs[0].UserID != test.userID || reader.inputs[0].Surface != memory.RankingRolloutSurfaceSearch {
				t.Fatalf("planner selector inputs=%+v", reader.inputs)
			}
			if len(lexical.inputs) != 1 || lexical.inputs[0].TopK != test.wantTopK {
				t.Fatalf("lexical inputs=%+v want top_k=%d", lexical.inputs, test.wantTopK)
			}
		})
	}
}

func TestAssembleContextAppliesOnlyMatchingSessionAndUserSelectors(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-context-selector"}
	policy := plannerTestRollout(scope, memory.RankingRolloutSurfaceContext, memory.RankingRolloutPolicyStatusActiveForScope)
	policy.RetrievalPlannerSelector = memory.RetrievalPlannerRolloutSelector{SessionID: "session-match", UserID: "user-match"}
	for _, test := range []struct {
		name      string
		sessionID string
		userID    string
		wantTopK  int
	}{
		{name: "matching", sessionID: "session-match", userID: "user-match", wantTopK: 2},
		{name: "foreign session", sessionID: "session-foreign", userID: "user-match", wantTopK: 3},
		{name: "foreign user", sessionID: "session-match", userID: "user-foreign", wantTopK: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &plannerSelectorRecorder{policy: policy}
			lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "visible", memory.MemoryClassEpisodic)}}}
			_, err := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: reader, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(2)}).AssembleContext(context.Background(), AssembleContextInput{
				Scope: scope, Query: "original", Budget: 1, SessionID: test.sessionID, UserID: test.userID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.inputs) != 1 || reader.inputs[0].SessionID != test.sessionID || reader.inputs[0].UserID != test.userID || reader.inputs[0].Surface != memory.RankingRolloutSurfaceContext {
				t.Fatalf("planner selector inputs=%+v", reader.inputs)
			}
			if len(lexical.inputs) != 1 || lexical.inputs[0].TopK != test.wantTopK {
				t.Fatalf("lexical inputs=%+v want top_k=%d", lexical.inputs, test.wantTopK)
			}
		})
	}
}

func hitIDs(hits []SearchHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.Memory.ID)
	}
	return ids
}
