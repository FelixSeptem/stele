package retrieval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type stubLexicalSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubLexicalSource) SearchLexical(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubSemanticSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubSemanticSource) SearchSemantic(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubRelationSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubRelationSource) SearchRelations(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubCitationSource struct {
	gotScope     memory.Scope
	gotMemoryIDs []string
	citations    map[string][]Citation
	err          error
}

func (s *stubCitationSource) ListCitations(ctx context.Context, scope memory.Scope, memoryIDs []string) (map[string][]Citation, error) {
	s.gotScope = scope
	s.gotMemoryIDs = append([]string(nil), memoryIDs...)
	return s.citations, s.err
}

type stubDerivedInsightSource struct {
	gotInputs []memory.ListDerivedInsightsInput
	items     map[memory.DerivedInsightType][]memory.DerivedInsight
	err       error
}

func (s *stubDerivedInsightSource) ListDerivedInsights(ctx context.Context, input memory.ListDerivedInsightsInput) ([]memory.DerivedInsight, error) {
	s.gotInputs = append(s.gotInputs, input)
	if s.err != nil {
		return nil, s.err
	}
	items := s.items[input.Type]
	filtered := make([]memory.DerivedInsight, 0, len(items))
	for _, item := range items {
		if input.State != "" && item.State != input.State {
			continue
		}
		if !input.IncludeHidden && isHiddenDerivedInsightState(item.State) {
			continue
		}
		filtered = append(filtered, item)
	}
	if input.Limit > 0 && len(filtered) > input.Limit {
		filtered = filtered[:input.Limit]
	}
	return filtered, nil
}

type stubRetrievalUsefulnessSummarizer struct {
	summaries map[string]memory.UsefulnessFeedbackSummary
}

func (s stubRetrievalUsefulnessSummarizer) SummarizeUsefulnessFeedback(ctx context.Context, input memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error) {
	if summary, ok := s.summaries[input.Subject.ID]; ok {
		return summary, nil
	}
	return memory.UsefulnessFeedbackSummary{Subject: input.Subject, EffectiveQuality: memory.UsefulnessQualityUnknown}, nil
}

type stubRetrievalTaskSummarizer struct {
	summaries map[string]memory.TaskEvaluationSummary
}

func (s stubRetrievalTaskSummarizer) SummarizeTaskEvaluations(ctx context.Context, input memory.SummarizeTaskEvaluationsInput) (memory.TaskEvaluationSummary, error) {
	if summary, ok := s.summaries[input.EvidenceTargetID]; ok {
		return summary, nil
	}
	return memory.TaskEvaluationSummary{
		Scope:              input.Scope.Normalized(),
		EvidenceTargetKind: input.EvidenceTargetKind,
		EvidenceTargetID:   input.EvidenceTargetID,
		VerdictCounts:      map[memory.TaskEvaluationVerdict]int{},
		ContributionCounts: map[memory.TaskContributionCategory]int{},
	}, nil
}

type stubRankingRolloutPolicyReader struct {
	policy memory.RankingRolloutPolicy
	err    error
	inputs []memory.ReadActiveRankingRolloutPolicyInput
}

type stubContextProjectionReader struct {
	projections map[memory.ContextProjectionKind]memory.ContextProjection
}

func (s stubContextProjectionReader) ReadLatestContextProjection(ctx context.Context, scope memory.Scope, kind memory.ContextProjectionKind) (memory.ContextProjection, error) {
	projection, ok := s.projections[kind]
	if !ok {
		return memory.ContextProjection{}, fmt.Errorf("projection not found")
	}
	return projection, nil
}

func (s *stubRankingRolloutPolicyReader) ReadActiveRankingRolloutPolicy(ctx context.Context, input memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return memory.RankingRolloutPolicy{}, s.err
	}
	return s.policy, nil
}

type stubRetrievalObserver struct {
	operations      []telemetry.OperationEvent
	rankingRollouts []telemetry.RankingRolloutEvent
}

func (s *stubRetrievalObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubRetrievalObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {}

func (s *stubRetrievalObserver) RecordRankingRollout(ctx context.Context, event telemetry.RankingRolloutEvent) {
	s.rankingRollouts = append(s.rankingRollouts, event)
}

func TestServiceSearchMergesRankedHitsAcrossSources(t *testing.T) {
	now := time.Date(2026, 6, 6, 11, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	lexical := &stubLexicalSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-2 * time.Hour),
					ModifiedAt: now.Add(-30 * time.Minute),
				},
				LexicalScore: 0.9,
			},
		},
	}
	semantic := &stubSemanticSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-2 * time.Hour),
					ModifiedAt: now.Add(-30 * time.Minute),
				},
				SemanticScore: 0.5,
			},
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_summary",
					Scope:      scope,
					Class:      memory.MemoryClassSummary,
					State:      memory.MemoryStateActive,
					Content:    "Summary: the user asked about travel planning.",
					CreatedAt:  now.Add(-90 * time.Minute),
					ModifiedAt: now.Add(-10 * time.Minute),
				},
				SemanticScore: 0.7,
			},
		},
	}
	relations := &stubRelationSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_relation",
					Scope:      scope,
					Class:      memory.MemoryClassRelation,
					State:      memory.MemoryStateActive,
					Content:    "entity:user relation:interested_in target:travel",
					CreatedAt:  now.Add(-70 * time.Minute),
					ModifiedAt: now.Add(-5 * time.Minute),
				},
				RelationScore: 0.6,
			},
		},
	}
	citations := &stubCitationSource{
		citations: map[string][]Citation{
			"mem_profile":  {{MemoryID: "mem_profile", RawEventID: "evt_profile", Operation: "promote_candidate"}},
			"mem_summary":  {{MemoryID: "mem_summary", RawEventID: "evt_summary", Operation: "create_summary_memory"}},
			"mem_relation": {{MemoryID: "mem_relation", RawEventID: "evt_relation", Operation: "promote_candidate"}},
		},
	}

	service := NewService(ServiceDependencies{
		Lexical:   lexical,
		Semantic:  semantic,
		Relations: relations,
		Citations: citations,
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "travel preferences",
		TopK:             5,
		IncludeSummaries: true,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 3 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 3)
	}

	if result.Hits[0].Memory.ID != "mem_profile" {
		t.Fatalf("result.Hits[0].Memory.ID = %q, want %q", result.Hits[0].Memory.ID, "mem_profile")
	}

	if result.Hits[0].Score.Overall <= result.Hits[1].Score.Overall {
		t.Fatalf("overall score ordering = %v then %v, want descending", result.Hits[0].Score.Overall, result.Hits[1].Score.Overall)
	}

	if len(result.Hits[0].Citations) != 1 || result.Hits[0].Citations[0].RawEventID != "evt_profile" {
		t.Fatalf("profile citations = %+v, want evt_profile", result.Hits[0].Citations)
	}

	if len(citations.gotMemoryIDs) != 3 {
		t.Fatalf("citation memory ids = %v, want three memory ids", citations.gotMemoryIDs)
	}
}

func TestServiceSearchAddsFeedbackDiagnosticsWithoutChangingDefaultRanking(t *testing.T) {
	now := time.Date(2026, 7, 11, 22, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	lexical := &stubLexicalSource{hits: []ScoredMemory{
		{
			Memory:       memory.CanonicalMemory{ID: "mem_noisy", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
			LexicalScore: 0.9,
		},
		{
			Memory:       memory.CanonicalMemory{ID: "mem_useful", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Minute)},
			LexicalScore: 0.8,
		},
	}}
	service := NewService(ServiceDependencies{
		Lexical: lexical,
		UsefulnessSummarizer: stubRetrievalUsefulnessSummarizer{summaries: map[string]memory.UsefulnessFeedbackSummary{
			"mem_noisy": {
				Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_noisy"},
				TotalActive:      2,
				NegativeCount:    2,
				EffectiveQuality: memory.UsefulnessQualityNegative,
				Counts:           map[memory.UsefulnessFeedbackType]int{memory.UsefulnessFeedbackTypeNoisy: 2},
			},
			"mem_useful": {
				Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_useful"},
				TotalActive:      1,
				PositiveCount:    1,
				EffectiveQuality: memory.UsefulnessQualityPositive,
				Counts:           map[memory.UsefulnessFeedbackType]int{memory.UsefulnessFeedbackTypeUseful: 1},
			},
		}},
	})

	defaultResult, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "preference", TopK: 2})
	if err != nil {
		t.Fatalf("Search() default error = %v", err)
	}
	if defaultResult.Hits[0].Memory.ID != "mem_noisy" {
		t.Fatalf("default hits = %+v, want score ranking unchanged", defaultResult.Hits)
	}
	if len(defaultResult.Diagnostics) != 0 {
		t.Fatalf("default diagnostics = %+v, want no feedback diagnostics unless requested", defaultResult.Diagnostics)
	}

	diagnosticResult, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "preference", TopK: 2, IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatalf("Search() diagnostics error = %v", err)
	}
	if diagnosticResult.Hits[0].Memory.ID != "mem_noisy" {
		t.Fatalf("diagnostic hits = %+v, want diagnostics without ranking change", diagnosticResult.Hits)
	}
	if !hasContextDiagnostic(diagnosticResult.Diagnostics, "search_feedback", "negative_signal") {
		t.Fatalf("diagnostics = %+v, want negative feedback signal", diagnosticResult.Diagnostics)
	}

	reranked, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "preference", TopK: 2, IncludeFeedbackDiagnostics: true, FeedbackAwareRanking: true})
	if err != nil {
		t.Fatalf("Search() feedback ranking error = %v", err)
	}
	if reranked.Hits[0].Memory.ID != "mem_useful" {
		t.Fatalf("reranked hits = %+v, want explicit feedback-aware ranking hint", reranked.Hits)
	}
	if !hasContextDiagnostic(reranked.Diagnostics, "search_feedback", "ranking_hint_applied") {
		t.Fatalf("diagnostics = %+v, want ranking hint diagnostic", reranked.Diagnostics)
	}
}

func TestServiceSearchReadsActiveRankingPolicyWhenConfigured(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policyReader := &stubRankingRolloutPolicyReader{}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)}, LexicalScore: 1},
			},
		},
		RankingRolloutPolicyReader: policyReader,
	})

	_, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile", TopK: 1})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(policyReader.inputs) != 1 {
		t.Fatalf("policy reader calls = %d, want 1", len(policyReader.inputs))
	}
	if policyReader.inputs[0].Surface != memory.RankingRolloutSurfaceSearch {
		t.Fatalf("policy surface = %q, want search", policyReader.inputs[0].Surface)
	}
}

func TestServiceSearchAppliesActiveTaskEvaluationRankingPolicyWithEvidenceThreshold(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	observer := &stubRetrievalObserver{}
	policy := memory.RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           scope,
		Status:          memory.RankingRolloutPolicyStatusActiveForScope,
		Mode:            memory.RankingRolloutModeActiveForScope,
		Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch},
		SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
		EvidenceMinimum: 2,
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_single_failure", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 0.95},
			{Memory: memory.CanonicalMemory{ID: "mem_success", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Minute)}, LexicalScore: 0.90},
		}},
		TaskEvaluationSummarizer: stubRetrievalTaskSummarizer{summaries: map[string]memory.TaskEvaluationSummary{
			"mem_single_failure": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_single_failure",
				ActiveEvaluations:  1,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictFailed: 1},
				ContributionCounts: map[memory.TaskContributionCategory]int{},
			},
			"mem_success": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_success",
				ActiveEvaluations:  2,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictSucceeded: 2},
				ContributionCounts: map[memory.TaskContributionCategory]int{},
			},
		}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: policy},
	}, observer)

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile", TopK: 2})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Hits[0].Memory.ID != "mem_success" {
		t.Fatalf("hits = %+v, want sufficient task-success evidence to rerank mem_success first while single failure is below threshold", result.Hits)
	}
	if !hasContextDiagnostic(result.Diagnostics, "search_feedback", "ranking_policy_applied") {
		t.Fatalf("diagnostics = %+v, want ranking policy applied", result.Diagnostics)
	}
	if len(observer.rankingRollouts) != 1 {
		t.Fatalf("ranking rollout metrics = %+v, want one policy evaluation event", observer.rankingRollouts)
	}
	got := observer.rankingRollouts[0]
	if got.Operation != "policy_evaluation" || got.Result != "applied" || got.Surface != "search" || got.SignalSource != "task_evaluations" || got.ThresholdStatus != "satisfied" || got.PolicyStatus != "active_for_scope" || got.ReasonCode != "active_policy" {
		t.Fatalf("ranking rollout metric = %+v, want bounded policy evaluation labels", got)
	}
}

func TestServiceAssembleContextUsesFeedbackDiagnosticsOnlyWhenRequested(t *testing.T) {
	now := time.Date(2026, 7, 11, 22, 30, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	lexical := &stubLexicalSource{hits: []ScoredMemory{
		{
			Memory:       memory.CanonicalMemory{ID: "mem_noisy", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now},
			LexicalScore: 0.9,
		},
		{
			Memory:       memory.CanonicalMemory{ID: "mem_useful", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Minute)},
			LexicalScore: 0.8,
		},
	}}
	service := NewService(ServiceDependencies{
		Lexical: lexical,
		UsefulnessSummarizer: stubRetrievalUsefulnessSummarizer{summaries: map[string]memory.UsefulnessFeedbackSummary{
			"mem_noisy": {
				Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_noisy"},
				TotalActive:      2,
				NegativeCount:    2,
				EffectiveQuality: memory.UsefulnessQualityNegative,
				Counts:           map[memory.UsefulnessFeedbackType]int{memory.UsefulnessFeedbackTypeStale: 2},
			},
			"mem_useful": {
				Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_useful"},
				TotalActive:      1,
				PositiveCount:    1,
				EffectiveQuality: memory.UsefulnessQualityPositive,
				Counts:           map[memory.UsefulnessFeedbackType]int{memory.UsefulnessFeedbackTypeUseful: 1},
			},
		}},
	})

	defaultContext, err := service.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "preference", Budget: 1, IncludeDiagnostics: true})
	if err != nil {
		t.Fatalf("AssembleContext() default error = %v", err)
	}
	if len(defaultContext.Profile) != 1 || defaultContext.Profile[0].Memory.ID != "mem_noisy" {
		t.Fatalf("default profile = %+v, want score ranking unchanged", defaultContext.Profile)
	}
	if hasContextDiagnostic(defaultContext.Diagnostics, "search_feedback", "negative_signal") {
		t.Fatalf("default diagnostics = %+v, want no feedback diagnostics by default", defaultContext.Diagnostics)
	}

	feedbackContext, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                      scope,
		Query:                      "preference",
		Budget:                     1,
		IncludeDiagnostics:         true,
		IncludeFeedbackDiagnostics: true,
		FeedbackAwareRanking:       true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() feedback error = %v", err)
	}
	if len(feedbackContext.Profile) != 1 || feedbackContext.Profile[0].Memory.ID != "mem_useful" {
		t.Fatalf("feedback profile = %+v, want explicit feedback-aware ranking hint", feedbackContext.Profile)
	}
	if !hasContextDiagnostic(feedbackContext.Diagnostics, "search_feedback", "negative_signal") || !hasContextDiagnostic(feedbackContext.Diagnostics, "search_feedback", "ranking_hint_applied") {
		t.Fatalf("feedback diagnostics = %+v, want feedback diagnostics and ranking hint", feedbackContext.Diagnostics)
	}
}

func TestServiceAssembleContextAppliesActiveTaskEvaluationRankingPolicyWithinBudget(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policyReader := &stubRankingRolloutPolicyReader{policy: memory.RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           scope,
		Status:          memory.RankingRolloutPolicyStatusActiveForScope,
		Mode:            memory.RankingRolloutModeActiveForScope,
		Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext},
		SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
		EvidenceMinimum: 2,
	}}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_failed", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 0.95},
			{Memory: memory.CanonicalMemory{ID: "mem_succeeded", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Minute)}, LexicalScore: 0.90},
		}},
		TaskEvaluationSummarizer: stubRetrievalTaskSummarizer{summaries: map[string]memory.TaskEvaluationSummary{
			"mem_failed": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_failed",
				ActiveEvaluations:  2,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictFailed: 2},
				ContributionCounts: map[memory.TaskContributionCategory]int{memory.TaskContributionCategoryMemoryIrrelevant: 2},
			},
			"mem_succeeded": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_succeeded",
				ActiveEvaluations:  2,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictSucceeded: 2},
				ContributionCounts: map[memory.TaskContributionCategory]int{},
			},
		}},
		RankingRolloutPolicyReader: policyReader,
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:              scope,
		Query:              "profile",
		Budget:             1,
		IncludeDiagnostics: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(policyReader.inputs) != 1 || policyReader.inputs[0].Surface != memory.RankingRolloutSurfaceContext {
		t.Fatalf("policy reads = %+v, want context surface lookup", policyReader.inputs)
	}
	if len(contextResult.Profile) != 1 || contextResult.Profile[0].Memory.ID != "mem_succeeded" {
		t.Fatalf("profile = %+v, want active context policy to fit succeeded memory within budget", contextResult.Profile)
	}
	if !hasContextDiagnostic(contextResult.Diagnostics, "search_feedback", "ranking_policy_applied") {
		t.Fatalf("diagnostics = %+v, want active policy diagnostic", contextResult.Diagnostics)
	}
}

func TestServiceAssembleContextPreservesBaselineAfterRankingPolicyRollback(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_failed", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now}, LexicalScore: 0.95},
			{Memory: memory.CanonicalMemory{ID: "mem_succeeded", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, ModifiedAt: now.Add(-time.Minute)}, LexicalScore: 0.90},
		}},
		TaskEvaluationSummarizer: stubRetrievalTaskSummarizer{summaries: map[string]memory.TaskEvaluationSummary{
			"mem_failed": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_failed",
				ActiveEvaluations:  2,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictFailed: 2},
				ContributionCounts: map[memory.TaskContributionCategory]int{memory.TaskContributionCategoryMemoryIrrelevant: 2},
			},
			"mem_succeeded": {
				Scope:              scope,
				EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
				EvidenceTargetID:   "mem_succeeded",
				ActiveEvaluations:  2,
				VerdictCounts:      map[memory.TaskEvaluationVerdict]int{memory.TaskEvaluationVerdictSucceeded: 2},
				ContributionCounts: map[memory.TaskContributionCategory]int{},
			},
		}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: memory.RankingRolloutPolicy{
			ID:              "policy_1",
			Scope:           scope,
			Status:          memory.RankingRolloutPolicyStatusRolledBack,
			Mode:            memory.RankingRolloutModeDiagnosticsOnly,
			Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext},
			SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
			ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
			EvidenceMinimum: 2,
		}},
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:              scope,
		Query:              "profile",
		Budget:             1,
		IncludeDiagnostics: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(contextResult.Profile) != 1 || contextResult.Profile[0].Memory.ID != "mem_failed" {
		t.Fatalf("profile = %+v, want rolled-back policy to preserve baseline ranking", contextResult.Profile)
	}
	if !hasContextDiagnostic(contextResult.Diagnostics, "search_feedback", "ranking_policy_skipped") {
		t.Fatalf("diagnostics = %+v, want skipped policy diagnostic", contextResult.Diagnostics)
	}
}

func TestServiceAssembleContextReturnsStructuredSections(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	lexical := &stubLexicalSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-3 * time.Hour),
					ModifiedAt: now.Add(-40 * time.Minute),
				},
				LexicalScore: 0.8,
			},
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_episode",
					Scope:      scope,
					Class:      memory.MemoryClassEpisodic,
					State:      memory.MemoryStateActive,
					Content:    "The user asked for a weekend travel plan.",
					CreatedAt:  now.Add(-90 * time.Minute),
					ModifiedAt: now.Add(-20 * time.Minute),
				},
				LexicalScore: 0.7,
			},
		},
	}
	semantic := &stubSemanticSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_summary",
					Scope:      scope,
					Class:      memory.MemoryClassSummary,
					State:      memory.MemoryStateActive,
					Content:    "Summary: the user is planning weekend travel.",
					CreatedAt:  now.Add(-80 * time.Minute),
					ModifiedAt: now.Add(-10 * time.Minute),
				},
				SemanticScore: 0.9,
			},
		},
	}
	relations := &stubRelationSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_relation",
					Scope:      scope,
					Class:      memory.MemoryClassRelation,
					State:      memory.MemoryStateActive,
					Content:    "entity:user relation:interested_in target:travel",
					CreatedAt:  now.Add(-70 * time.Minute),
					ModifiedAt: now.Add(-5 * time.Minute),
				},
				RelationScore: 0.5,
			},
		},
	}
	citations := &stubCitationSource{
		citations: map[string][]Citation{
			"mem_profile":  {{MemoryID: "mem_profile", RawEventID: "evt_profile", Operation: "promote_candidate"}},
			"mem_episode":  {{MemoryID: "mem_episode", RawEventID: "evt_episode", Operation: "promote_candidate"}},
			"mem_summary":  {{MemoryID: "mem_summary", RawEventID: "evt_summary", Operation: "create_summary_memory"}},
			"mem_relation": {{MemoryID: "mem_relation", RawEventID: "evt_relation", Operation: "promote_candidate"}},
		},
	}

	service := NewService(ServiceDependencies{
		Lexical:   lexical,
		Semantic:  semantic,
		Relations: relations,
		Citations: citations,
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:            scope,
		Query:            "weekend travel",
		Budget:           4,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.Profile) != 1 {
		t.Fatalf("len(Profile) = %d, want %d", len(contextResult.Profile), 1)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	if len(contextResult.RelatedEntities) != 1 {
		t.Fatalf("len(RelatedEntities) = %d, want %d", len(contextResult.RelatedEntities), 1)
	}

	if len(contextResult.Citations) < 3 {
		t.Fatalf("len(Citations) = %d, want at least %d", len(contextResult.Citations), 3)
	}
}

func TestServiceAssembleContextConsumesExactScopeProjectionWhenOptedIn(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	projection := memory.ContextProjection{ID: "projection", Scope: scope, Kind: memory.ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "schema-v1", PolicyVersion: "policy-v1", RendererVersion: "renderer-v1", Status: memory.ContextProjectionStatusActive, Items: []memory.ContextProjectionItem{{ID: "item", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "mem-profile", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "projected preference", SortKey: "01", Citation: memory.ProjectionCitation{MemoryID: "mem-profile", Operation: "context_projection"}}}}
	service := NewService(ServiceDependencies{Lexical: &stubLexicalSource{}, Semantic: &stubSemanticSource{}, Relations: &stubRelationSource{}, Projections: stubContextProjectionReader{projections: map[memory.ContextProjectionKind]memory.ContextProjection{memory.ContextProjectionKindAlwaysVisible: projection}}})
	result, err := service.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "preference", Budget: 1, IncludeDiagnostics: true, UseProjections: true})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(result.Profile) != 1 || result.Profile[0].Memory.Content != "projected preference" {
		t.Fatalf("profile = %+v, want projection-backed profile", result.Profile)
	}
	if len(result.Citations) != 1 || result.Citations[0].MemoryID != "mem-profile" {
		t.Fatalf("citations = %+v, want redacted projection citation", result.Citations)
	}
}

func TestServiceAssembleContextExcludesExperienceInsightsByDefault(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:  scope,
		Query:  "provider failure",
		Budget: 3,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(insights.gotInputs) != 0 {
		t.Fatalf("insight list calls = %d, want 0 by default", len(insights.gotInputs))
	}
	if len(result.KnownFailures) != 0 || len(result.ExperienceLessons) != 0 {
		t.Fatalf("insight sections = %+v/%+v, want empty by default", result.KnownFailures, result.ExperienceLessons)
	}
}

func TestServiceAssembleContextIncludesRequestedExperienceInsightsWithCitations(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{
		items: map[memory.DerivedInsightType][]memory.DerivedInsight{
			memory.DerivedInsightTypeFailurePattern: {
				{
					ID:      "insight_failure",
					Scope:   scope,
					Type:    memory.DerivedInsightTypeFailurePattern,
					State:   memory.DerivedInsightStateActive,
					Title:   "Repeated provider failure",
					Summary: "Provider unavailable failed twice.",
					Confidence: memory.DerivedInsightConfidence{
						Score: 0.75,
					},
					Derivation: memory.DerivedInsightDerivation{
						Source:      "failure_pattern_evaluator",
						Fingerprint: "failure_pattern:fingerprint",
						DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
					},
					Evidence: []memory.DerivedInsightEvidenceRef{
						{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
					},
				},
			},
			memory.DerivedInsightTypeLesson: {
				{
					ID:      "insight_lesson",
					Scope:   scope,
					Type:    memory.DerivedInsightTypeLesson,
					State:   memory.DerivedInsightStateActive,
					Title:   "Avoid retry storm",
					Summary: "Wait for provider recovery.",
					Confidence: memory.DerivedInsightConfidence{
						Score: 0.75,
					},
					Lesson: &memory.DerivedInsightLesson{
						SourceFailurePatternID: "insight_failure",
						Guidance:               "Wait for provider recovery.",
					},
					Derivation: memory.DerivedInsightDerivation{
						Source:      "lesson_projection",
						Fingerprint: "lesson:fingerprint",
						DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
					},
					Evidence: []memory.DerivedInsightEvidenceRef{
						{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
					},
				},
			},
		},
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                     scope,
		Query:                     "provider failure",
		Budget:                    3,
		IncludeExperienceInsights: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(insights.gotInputs) != 2 {
		t.Fatalf("insight list calls = %d, want failure patterns and lessons", len(insights.gotInputs))
	}
	for _, input := range insights.gotInputs {
		if input.Scope != scope || input.State != memory.DerivedInsightStateActive || input.IncludeHidden {
			t.Fatalf("insight input = %+v, want active scoped visible input", input)
		}
	}
	if len(result.KnownFailures) != 1 || len(result.ExperienceLessons) != 1 {
		t.Fatalf("insight sections = %+v/%+v, want one failure and one lesson", result.KnownFailures, result.ExperienceLessons)
	}
	if len(result.KnownFailures[0].Citations) != 1 || result.KnownFailures[0].Citations[0].EvidenceID != "job_1" {
		t.Fatalf("failure citations = %+v, want evidence job_1", result.KnownFailures[0].Citations)
	}
}

func TestServiceAssembleContextDiagnosticsExplainReplayedInsightVisibility(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{
		items: map[memory.DerivedInsightType][]memory.DerivedInsight{
			memory.DerivedInsightTypeFailurePattern: {
				{
					ID:       "insight_useful",
					Scope:    scope,
					Type:     memory.DerivedInsightTypeFailurePattern,
					State:    memory.DerivedInsightStateActive,
					Evidence: []memory.DerivedInsightEvidenceRef{{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports}},
					FeedbackSummary: memory.DerivedInsightFeedbackSummary{
						Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeUseful: 1},
						PositiveCount: 1,
						TotalActive:   1,
					},
				},
				{
					ID:       "insight_noisy",
					Scope:    scope,
					Type:     memory.DerivedInsightTypeFailurePattern,
					State:    memory.DerivedInsightStateActive,
					Evidence: []memory.DerivedInsightEvidenceRef{{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", Relation: memory.DerivedInsightEvidenceRelationSupports}},
					FeedbackSummary: memory.DerivedInsightFeedbackSummary{
						Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNoisy: 1},
						NegativeCount: 1,
						TotalActive:   1,
					},
				},
				{
					ID:       "insight_hidden",
					Scope:    scope,
					Type:     memory.DerivedInsightTypeFailurePattern,
					State:    memory.DerivedInsightStateSuppressed,
					Evidence: []memory.DerivedInsightEvidenceRef{{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_3", Relation: memory.DerivedInsightEvidenceRelationSupports}},
				},
			},
		},
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                     scope,
		Query:                     "provider failure",
		Budget:                    2,
		IncludeExperienceInsights: true,
		IncludeDiagnostics:        true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(result.KnownFailures) != 1 || result.KnownFailures[0].Insight.ID != "insight_useful" {
		t.Fatalf("known failures = %+v, want useful insight included", result.KnownFailures)
	}
	if !hasContextDiagnostic(result.Diagnostics, "known_failures", "included") {
		t.Fatalf("diagnostics = %+v, want included known_failures diagnostic", result.Diagnostics)
	}
	if !hasContextDiagnostic(result.Diagnostics, "known_failures", "omitted_by_quality") {
		t.Fatalf("diagnostics = %+v, want omitted_by_quality known_failures diagnostic", result.Diagnostics)
	}
	if !hasContextDiagnostic(result.Diagnostics, "known_failures", "hidden_by_lifecycle_or_scope") {
		t.Fatalf("diagnostics = %+v, want hidden lifecycle diagnostic", result.Diagnostics)
	}
}

func TestServiceAssembleContextTrimsExperienceInsightsUnderBudgetPressure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{
		items: map[memory.DerivedInsightType][]memory.DerivedInsight{
			memory.DerivedInsightTypeFailurePattern: {
				{ID: "insight_failure", Scope: scope, Type: memory.DerivedInsightTypeFailurePattern, State: memory.DerivedInsightStateActive},
			},
		},
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                     scope,
		Query:                     "provider failure",
		Budget:                    1,
		IncludeExperienceInsights: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(insights.gotInputs) != 0 {
		t.Fatalf("insight calls = %d, want no insight fetch when budget is exhausted by higher priority context", len(insights.gotInputs))
	}
	if len(result.KnownFailures) != 0 {
		t.Fatalf("known failures = %+v, want empty under budget pressure", result.KnownFailures)
	}
}

func hasContextDiagnostic(items []ContextDiagnostic, section, status string) bool {
	for _, item := range items {
		if item.Section == section && item.Status == status {
			return true
		}
	}
	return false
}

func TestServiceAssembleContextPrioritizesUsefulExperienceInsightUnderBudget(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{
		items: map[memory.DerivedInsightType][]memory.DerivedInsight{
			memory.DerivedInsightTypeFailurePattern: {
				{
					ID:      "insight_noisy",
					Scope:   scope,
					Type:    memory.DerivedInsightTypeFailurePattern,
					State:   memory.DerivedInsightStateActive,
					Title:   "Noisy pattern",
					Summary: "Too broad.",
					FeedbackSummary: memory.DerivedInsightFeedbackSummary{
						Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNoisy: 1},
						NegativeCount: 1,
						TotalActive:   1,
					},
				},
				{
					ID:      "insight_useful",
					Scope:   scope,
					Type:    memory.DerivedInsightTypeFailurePattern,
					State:   memory.DerivedInsightStateActive,
					Title:   "Useful pattern",
					Summary: "Actionable.",
					FeedbackSummary: memory.DerivedInsightFeedbackSummary{
						Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeUseful: 1},
						PositiveCount: 1,
						TotalActive:   1,
					},
				},
			},
		},
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                     scope,
		Query:                     "provider failure",
		Budget:                    2,
		IncludeExperienceInsights: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(result.KnownFailures) != 1 || result.KnownFailures[0].Insight.ID != "insight_useful" {
		t.Fatalf("known failures = %+v, want useful insight first under budget", result.KnownFailures)
	}
}

func TestServiceAssembleContextOmitsNeedsReviewInsightWhenAlternativesExist(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	insights := &stubDerivedInsightSource{
		items: map[memory.DerivedInsightType][]memory.DerivedInsight{
			memory.DerivedInsightTypeFailurePattern: {
				{
					ID:    "insight_review",
					Scope: scope,
					Type:  memory.DerivedInsightTypeFailurePattern,
					State: memory.DerivedInsightStateActive,
					FeedbackSummary: memory.DerivedInsightFeedbackSummary{
						Counts:      map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNeedsReview: 1},
						NeedsReview: true,
						TotalActive: 1,
					},
				},
				{
					ID:    "insight_plain",
					Scope: scope,
					Type:  memory.DerivedInsightTypeFailurePattern,
					State: memory.DerivedInsightStateActive,
				},
			},
		},
	}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
		Insights: insights,
	})

	result, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:                     scope,
		Query:                     "provider failure",
		Budget:                    2,
		IncludeExperienceInsights: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(result.KnownFailures) != 1 || result.KnownFailures[0].Insight.ID != "insight_plain" {
		t.Fatalf("known failures = %+v, want non-review insight selected", result.KnownFailures)
	}
}

func TestServiceSearchAppliesSummaryRelationAndClassFilters(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.9},
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, LexicalScore: 0.8},
				{Memory: memory.CanonicalMemory{ID: "mem_relation", Scope: scope, Class: memory.MemoryClassRelation, State: memory.MemoryStateActive, Content: "relation"}, LexicalScore: 0.7},
			},
		},
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "profile",
		Classes:          []memory.MemoryClass{memory.MemoryClassProfile},
		IncludeSummaries: false,
		IncludeRelations: false,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 1 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 1)
	}

	if result.Hits[0].Memory.Class != memory.MemoryClassProfile {
		t.Fatalf("result.Hits[0].Memory.Class = %q, want %q", result.Hits[0].Memory.Class, memory.MemoryClassProfile)
	}
}

func TestServiceSearchKeepsLexicalAndRelationHitsWhenSemanticHasNoMatches(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{
					Memory:       memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "User prefers concise answers."},
					LexicalScore: 0.9,
				},
			},
		},
		Semantic: &stubSemanticSource{},
		Relations: &stubRelationSource{
			hits: []ScoredMemory{
				{
					Memory:        memory.CanonicalMemory{ID: "mem_relation", Scope: scope, Class: memory.MemoryClassRelation, State: memory.MemoryStateActive, Content: "entity:user relation:interested_in target:concise_answers"},
					RelationScore: 0.4,
				},
			},
		},
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "concise answers",
		QueryEmbedding:   []float32{0.1, 0.2, 0.3},
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 2 {
		t.Fatalf("len(result.Hits) = %d, want 2", len(result.Hits))
	}

	if result.Hits[0].Memory.ID != "mem_profile" {
		t.Fatalf("result.Hits[0].Memory.ID = %q, want mem_profile", result.Hits[0].Memory.ID)
	}

	if result.Hits[1].Memory.ID != "mem_relation" {
		t.Fatalf("result.Hits[1].Memory.ID = %q, want mem_relation", result.Hits[1].Memory.ID)
	}

	if result.Hits[0].Score.Semantic != 0 || result.Hits[1].Score.Semantic != 0 {
		t.Fatalf("semantic scores = (%v, %v), want zero semantic contribution", result.Hits[0].Score.Semantic, result.Hits[1].Score.Semantic)
	}
}

func TestServiceAssembleContextHonorsBudgetAndKeepsSummary(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.9},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_1", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 1"}, LexicalScore: 0.8},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_2", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 2"}, LexicalScore: 0.7},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, SemanticScore: 1.0},
			},
		},
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:  scope,
		Query:  "travel",
		Budget: 2,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	total := len(contextResult.Profile) + len(contextResult.RecentSession) + len(contextResult.RecentEpisodes) + len(contextResult.RelevantSummaries) + len(contextResult.RelatedEntities)
	if total > 2 {
		t.Fatalf("total assembled hits = %d, want <= %d", total, 2)
	}
}

func TestServiceSearchTopKUsesSortedHitsForCitationLookup(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_low", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "low"}, LexicalScore: 0.2},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_high", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "high"}, SemanticScore: 0.9},
			},
		},
		Citations: &stubCitationSource{
			citations: map[string][]Citation{
				"mem_high": {{MemoryID: "mem_high", RawEventID: "evt_high", Operation: "create_summary_memory"}},
			},
		},
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "high",
		TopK:             1,
		IncludeSummaries: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 1 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 1)
	}

	if result.Hits[0].Memory.ID != "mem_high" {
		t.Fatalf("result.Hits[0].Memory.ID = %q, want %q", result.Hits[0].Memory.ID, "mem_high")
	}

	if len(result.Hits[0].Citations) != 1 || result.Hits[0].Citations[0].RawEventID != "evt_high" {
		t.Fatalf("result.Hits[0].Citations = %+v, want evt_high citation", result.Hits[0].Citations)
	}
}

func TestSearchInputValidateRejectsInvalidTimeWindow(t *testing.T) {
	err := (SearchInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Query:    "travel",
		TimeFrom: time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC),
		TimeTo:   time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC),
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid time window")
	}
}

func TestServiceAssembleContextPrefersSummaryOverExtraEpisodes(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.95},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_1", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 1"}, LexicalScore: 0.90},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_2", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 2"}, LexicalScore: 0.89},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, SemanticScore: 0.40},
			},
		},
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:  scope,
		Query:  "travel",
		Budget: 2,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.Profile) != 1 {
		t.Fatalf("len(Profile) = %d, want %d", len(contextResult.Profile), 1)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	if len(contextResult.RecentSession)+len(contextResult.RecentEpisodes) != 0 {
		t.Fatalf("episodic sections = %d, want 0 after summary-preferred packing", len(contextResult.RecentSession)+len(contextResult.RecentEpisodes))
	}
}

func TestServiceSearchEmitsTelemetryOperation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := &stubRetrievalObserver{}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 1},
			},
		},
	}, observer)

	_, err := service.Search(context.Background(), SearchInput{
		Scope: scope,
		Query: "profile",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Operation != "search" || observer.operations[0].Status != "ok" {
		t.Fatalf("operation event = %+v, want search ok", observer.operations[0])
	}
}
