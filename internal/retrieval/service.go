package retrieval

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
)

type SearchInput struct {
	Scope                      memory.Scope
	Query                      string
	QueryEmbedding             []float32
	LexicalMatchMode           LexicalMatchMode
	Classes                    []memory.MemoryClass
	rankingSurface             memory.RankingRolloutSurface
	rankingPolicyDisabled      bool
	TimeFrom                   time.Time
	TimeTo                     time.Time
	TopK                       int
	IncludeSummaries           bool
	IncludeRelations           bool
	IncludeFeedbackDiagnostics bool
	FeedbackAwareRanking       bool
}

// LexicalMatchMode selects the full-text query composition used by an internal
// retrieval caller. Ordinary search keeps the all-terms default.
type LexicalMatchMode string

const (
	LexicalMatchAllTerms LexicalMatchMode = "all_terms"
	LexicalMatchAnyTerms LexicalMatchMode = "any_terms"
)

func (i SearchInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if i.LexicalMatchMode != "" && i.LexicalMatchMode != LexicalMatchAllTerms && i.LexicalMatchMode != LexicalMatchAnyTerms {
		return fmt.Errorf("unsupported lexical match mode %q", i.LexicalMatchMode)
	}
	if !i.TimeFrom.IsZero() && !i.TimeTo.IsZero() && i.TimeFrom.After(i.TimeTo) {
		return fmt.Errorf("time_from must be before or equal to time_to")
	}
	if i.TopK < 0 {
		return fmt.Errorf("top_k must be greater than or equal to zero")
	}
	return nil
}

type AssembleContextInput struct {
	Scope                      memory.Scope
	Query                      string
	Budget                     int
	IncludeRelations           bool
	IncludeExperienceInsights  bool
	IncludeDiagnostics         bool
	IncludeFeedbackDiagnostics bool
	FeedbackAwareRanking       bool
}

func (i AssembleContextInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if i.Budget <= 0 {
		return fmt.Errorf("budget must be greater than zero")
	}
	return nil
}

type ScoreBreakdown struct {
	Overall  float64 `json:"overall"`
	Lexical  float64 `json:"lexical"`
	Semantic float64 `json:"semantic"`
	Relation float64 `json:"relation"`
}

type Citation struct {
	MemoryID   string `json:"memory_id"`
	RawEventID string `json:"raw_event_id"`
	Operation  string `json:"operation"`
}

type SearchHit struct {
	Memory    memory.CanonicalMemory `json:"memory"`
	Score     ScoreBreakdown         `json:"score"`
	Citations []Citation             `json:"citations"`
}

type SearchResult struct {
	Hits        []SearchHit         `json:"hits"`
	Diagnostics []ContextDiagnostic `json:"diagnostics,omitempty"`
}

type InsightCitation struct {
	InsightID    string `json:"insight_id"`
	EvidenceKind string `json:"evidence_kind"`
	EvidenceID   string `json:"evidence_id"`
	Relation     string `json:"relation"`
}

type ExperienceInsightContext struct {
	Insight   memory.DerivedInsight `json:"insight"`
	Citations []InsightCitation     `json:"citations"`
}

type ContextDiagnostic struct {
	Section     string                    `json:"section"`
	InsightType memory.DerivedInsightType `json:"insight_type,omitempty"`
	Status      string                    `json:"status"`
	Reason      string                    `json:"reason,omitempty"`
	Available   int                       `json:"available,omitempty"`
	Included    int                       `json:"included,omitempty"`
	Omitted     int                       `json:"omitted,omitempty"`
	Hidden      int                       `json:"hidden,omitempty"`
}

type AssembledContext struct {
	Profile           []SearchHit                `json:"profile"`
	RecentSession     []SearchHit                `json:"recent_session"`
	RecentEpisodes    []SearchHit                `json:"recent_episodes"`
	RelevantSummaries []SearchHit                `json:"relevant_summaries"`
	RelatedEntities   []SearchHit                `json:"related_entities"`
	Citations         []Citation                 `json:"citations"`
	KnownFailures     []ExperienceInsightContext `json:"known_failures,omitempty"`
	ExperienceLessons []ExperienceInsightContext `json:"experience_lessons,omitempty"`
	Diagnostics       []ContextDiagnostic        `json:"diagnostics,omitempty"`
}

type ScoredMemory struct {
	Memory        memory.CanonicalMemory
	LexicalScore  float64
	SemanticScore float64
	RelationScore float64
}

type LexicalSearcher interface {
	SearchLexical(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type SemanticSearcher interface {
	SearchSemantic(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type RelationSearcher interface {
	SearchRelations(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type CitationLister interface {
	ListCitations(ctx context.Context, scope memory.Scope, memoryIDs []string) (map[string][]Citation, error)
}

type DerivedInsightLister interface {
	ListDerivedInsights(ctx context.Context, input memory.ListDerivedInsightsInput) ([]memory.DerivedInsight, error)
}

type UsefulnessSummarizer interface {
	SummarizeUsefulnessFeedback(ctx context.Context, input memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error)
}

type TaskEvaluationSummarizer interface {
	SummarizeTaskEvaluations(ctx context.Context, input memory.SummarizeTaskEvaluationsInput) (memory.TaskEvaluationSummary, error)
}

type RankingRolloutPolicyReader interface {
	ReadActiveRankingRolloutPolicy(ctx context.Context, input memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
}

type rankingRolloutMetricObserver interface {
	RecordRankingRollout(ctx context.Context, event telemetry.RankingRolloutEvent)
}

type MemorySearcher interface {
	Search(ctx context.Context, input SearchInput) (SearchResult, error)
}

type ContextAssembler interface {
	AssembleContext(ctx context.Context, input AssembleContextInput) (AssembledContext, error)
}

type ServiceDependencies struct {
	Lexical                    LexicalSearcher
	Semantic                   SemanticSearcher
	Relations                  RelationSearcher
	Citations                  CitationLister
	Insights                   DerivedInsightLister
	UsefulnessSummarizer       UsefulnessSummarizer
	TaskEvaluationSummarizer   TaskEvaluationSummarizer
	RankingRolloutPolicyReader RankingRolloutPolicyReader
}

type Service struct {
	lexical                    LexicalSearcher
	semantic                   SemanticSearcher
	relations                  RelationSearcher
	citations                  CitationLister
	insights                   DerivedInsightLister
	usefulnessSummarizer       UsefulnessSummarizer
	taskEvaluationSummarizer   TaskEvaluationSummarizer
	rankingRolloutPolicyReader RankingRolloutPolicyReader
	observer                   telemetry.Observer
}

func NewService(deps ServiceDependencies, observers ...telemetry.Observer) *Service {
	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}

	return &Service{
		lexical:                    deps.Lexical,
		semantic:                   deps.Semantic,
		relations:                  deps.Relations,
		citations:                  deps.Citations,
		insights:                   deps.Insights,
		usefulnessSummarizer:       deps.UsefulnessSummarizer,
		taskEvaluationSummarizer:   deps.TaskEvaluationSummarizer,
		rankingRolloutPolicyReader: deps.RankingRolloutPolicyReader,
		observer:                   observer,
	}
}

func (s *Service) Search(ctx context.Context, input SearchInput) (result SearchResult, err error) {
	started := time.Now()
	defer func() {
		if s.observer == nil {
			return
		}

		status := "ok"
		count := len(result.Hits)
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		s.observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "api",
			Component:  "retrieval_service",
			Operation:  "search",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: time.Now().UTC(),
		})
	}()

	if err := input.Validate(); err != nil {
		return SearchResult{}, err
	}
	surface := input.rankingSurface
	if surface == "" {
		surface = memory.RankingRolloutSurfaceSearch
	}

	merged := map[string]ScoredMemory{}
	orderedIDs := make([]string, 0)

	appendHits := func(hits []ScoredMemory) {
		for _, hit := range hits {
			if !matchClassFilter(hit.Memory.Class, input.Classes) {
				continue
			}
			if !input.IncludeSummaries && hit.Memory.Class == memory.MemoryClassSummary {
				continue
			}
			if !input.IncludeRelations && hit.Memory.Class == memory.MemoryClassRelation {
				continue
			}

			current, ok := merged[hit.Memory.ID]
			if !ok {
				merged[hit.Memory.ID] = hit
				orderedIDs = append(orderedIDs, hit.Memory.ID)
				continue
			}

			if hit.Memory.ModifiedAt.After(current.Memory.ModifiedAt) || current.Memory.ModifiedAt.IsZero() {
				current.Memory = hit.Memory
			}
			current.LexicalScore += hit.LexicalScore
			current.SemanticScore += hit.SemanticScore
			current.RelationScore += hit.RelationScore
			merged[hit.Memory.ID] = current
		}
	}

	if s.lexical != nil {
		hits, err := s.lexical.SearchLexical(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	if s.semantic != nil {
		hits, err := s.semantic.SearchSemantic(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	if input.IncludeRelations && s.relations != nil {
		hits, err := s.relations.SearchRelations(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	scored := make([]SearchHit, 0, len(orderedIDs))
	memoryIDs := make([]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		hit := merged[id]
		memoryIDs = append(memoryIDs, hit.Memory.ID)
		scored = append(scored, SearchHit{
			Memory: hit.Memory,
			Score: ScoreBreakdown{
				Overall:  hit.LexicalScore + hit.SemanticScore + hit.RelationScore,
				Lexical:  hit.LexicalScore,
				Semantic: hit.SemanticScore,
				Relation: hit.RelationScore,
			},
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score.Overall == scored[j].Score.Overall {
			return scored[i].Memory.ModifiedAt.After(scored[j].Memory.ModifiedAt)
		}
		return scored[i].Score.Overall > scored[j].Score.Overall
	})

	diagnostics, err := s.applyUsefulnessFeedbackSignals(ctx, input, scored)
	if err != nil {
		return SearchResult{}, err
	}

	if !input.rankingPolicyDisabled {
		adjusted, policyDiagnostics, err := s.applyRankingRolloutPolicy(ctx, input.Scope, surface, input.FeedbackAwareRanking, scored)
		if err != nil {
			return SearchResult{}, err
		}
		if len(policyDiagnostics) > 0 {
			diagnostics = append(diagnostics, policyDiagnostics...)
		}
		if adjusted {
			sort.Slice(scored, func(i, j int) bool {
				if scored[i].Score.Overall == scored[j].Score.Overall {
					return scored[i].Memory.ModifiedAt.After(scored[j].Memory.ModifiedAt)
				}
				return scored[i].Score.Overall > scored[j].Score.Overall
			})
		}
	}

	if input.TopK > 0 && len(scored) > input.TopK {
		scored = scored[:input.TopK]
	}

	if s.citations != nil && len(scored) > 0 {
		memoryIDs = make([]string, 0, len(scored))
		for _, hit := range scored {
			memoryIDs = append(memoryIDs, hit.Memory.ID)
		}
		citationMap, err := s.citations.ListCitations(ctx, input.Scope, memoryIDs)
		if err != nil {
			return SearchResult{}, err
		}
		for i := range scored {
			scored[i].Citations = citationMap[scored[i].Memory.ID]
		}
	}

	result = SearchResult{Hits: scored, Diagnostics: diagnostics}
	return result, nil
}

func (s *Service) applyRankingRolloutPolicy(ctx context.Context, scope memory.Scope, surface memory.RankingRolloutSurface, feedbackAwareRanking bool, scored []SearchHit) (bool, []ContextDiagnostic, error) {
	if s.rankingRolloutPolicyReader == nil {
		if feedbackAwareRanking {
			applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
			return true, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
		}
		return false, nil, nil
	}

	policy, err := s.rankingRolloutPolicyReader.ReadActiveRankingRolloutPolicy(ctx, memory.ReadActiveRankingRolloutPolicyInput{
		Scope:   scope,
		Surface: surface,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if feedbackAwareRanking {
				applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
				s.recordRankingRolloutPolicyEvaluation(ctx, "request_hint", surface, memory.RankingRolloutPolicy{}, "no_active_policy")
				return true, []ContextDiagnostic{{Section: "search_feedback", Status: "request_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
			}
			s.recordRankingRolloutPolicyEvaluation(ctx, "not_applied", surface, memory.RankingRolloutPolicy{}, "no_active_policy")
			return false, nil, nil
		}
		return false, nil, err
	}

	applied := policy.Status == memory.RankingRolloutPolicyStatusActiveForScope && policy.Mode == memory.RankingRolloutModeActiveForScope
	if applied {
		for i := range scored {
			adjustment, err := s.rankingRolloutAdjustmentForHit(ctx, scope, policy, scored[i].Memory.ID)
			if err != nil {
				return false, nil, err
			}
			scored[i].Score.Overall += adjustment
		}
		s.recordRankingRolloutPolicyEvaluation(ctx, "applied", surface, policy, "active_policy")
		return true, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_policy_applied", Reason: "active scoped ranking rollout policy applied"}}, nil
	}

	if feedbackAwareRanking {
		applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
		s.recordRankingRolloutPolicyEvaluation(ctx, "request_hint", surface, policy, "ineligible_policy")
		return true, []ContextDiagnostic{{Section: "search_feedback", Status: "request_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
	}

	s.recordRankingRolloutPolicyEvaluation(ctx, "not_applied", surface, policy, "ineligible_policy")
	return false, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_policy_skipped", Reason: "active scoped ranking rollout policy is not eligible for default ranking"}}, nil
}

func (s *Service) recordRankingRolloutPolicyEvaluation(ctx context.Context, result string, surface memory.RankingRolloutSurface, policy memory.RankingRolloutPolicy, reasonCode string) {
	observer, ok := s.observer.(rankingRolloutMetricObserver)
	if !ok || observer == nil {
		return
	}
	observer.RecordRankingRollout(ctx, telemetry.RankingRolloutEvent{
		Operation:       "policy_evaluation",
		Result:          result,
		Surface:         string(surface),
		SignalSource:    string(firstRankingRolloutSignalSource(policy.SignalSources)),
		ThresholdStatus: string(policy.ThresholdStatus),
		PolicyStatus:    string(policy.Status),
		ReasonCode:      reasonCode,
	})
}

func (s *Service) rankingRolloutAdjustmentForHit(ctx context.Context, scope memory.Scope, policy memory.RankingRolloutPolicy, memoryID string) (float64, error) {
	if policy.ThresholdStatus == memory.RankingRolloutThresholdStatusBlocked {
		return 0, nil
	}
	adjustment := 0.0
	if rankingRolloutPolicyUsesSignal(policy, memory.RankingRolloutSignalSourceUsefulnessFeedback) {
		summary, err := s.usefulnessSummaryForHit(ctx, scope, memoryID)
		if err != nil {
			return 0, err
		}
		if rankingRolloutEvidenceSatisfied(policy, summary.TotalActive) {
			adjustment += feedbackRankingAdjustment(summary)
		}
	}
	if rankingRolloutPolicyUsesSignal(policy, memory.RankingRolloutSignalSourceTaskEvaluations) && s.taskEvaluationSummarizer != nil {
		summary, err := s.taskEvaluationSummarizer.SummarizeTaskEvaluations(ctx, memory.SummarizeTaskEvaluationsInput{
			Scope:              scope,
			EvidenceTargetKind: memory.TaskEvidenceTargetMemory,
			EvidenceTargetID:   memoryID,
		})
		if err != nil {
			return 0, err
		}
		if rankingRolloutEvidenceSatisfied(policy, summary.ActiveEvaluations) {
			adjustment += taskEvaluationRankingAdjustment(summary)
		}
	}
	return adjustment, nil
}

func (s *Service) usefulnessSummaryForHit(ctx context.Context, scope memory.Scope, memoryID string) (memory.UsefulnessFeedbackSummary, error) {
	if s.usefulnessSummarizer == nil {
		return memory.UsefulnessFeedbackSummary{}, nil
	}
	return s.usefulnessSummarizer.SummarizeUsefulnessFeedback(ctx, memory.SummarizeUsefulnessFeedbackInput{
		Scope: scope,
		Subject: memory.UsefulnessFeedbackSubject{
			Kind: memory.UsefulnessFeedbackSubjectMemory,
			ID:   memoryID,
		},
	})
}

func applyFeedbackAwareRankingHint(scored []SearchHit, summarizer UsefulnessSummarizer, ctx context.Context, scope memory.Scope) {
	if summarizer == nil {
		return
	}
	for i := range scored {
		summary, err := summarizer.SummarizeUsefulnessFeedback(ctx, memory.SummarizeUsefulnessFeedbackInput{
			Scope: scope,
			Subject: memory.UsefulnessFeedbackSubject{
				Kind: memory.UsefulnessFeedbackSubjectMemory,
				ID:   scored[i].Memory.ID,
			},
		})
		if err != nil {
			continue
		}
		scored[i].Score.Overall += feedbackRankingAdjustment(summary)
	}
}

func rankingRolloutAdjustment(policy memory.RankingRolloutPolicy, summary memory.UsefulnessFeedbackSummary) float64 {
	adjustment := feedbackRankingAdjustment(summary)
	if policy.Mode == memory.RankingRolloutModeDiagnosticsOnly || policy.Mode == memory.RankingRolloutModeDryRun {
		return adjustment
	}
	switch policy.ThresholdStatus {
	case memory.RankingRolloutThresholdStatusSatisfied:
		return adjustment
	case memory.RankingRolloutThresholdStatusBlocked:
		return 0
	case memory.RankingRolloutThresholdStatusInsufficient:
		if summary.TotalActive >= policy.EvidenceMinimum && policy.EvidenceMinimum > 0 {
			return adjustment
		}
		return 0
	default:
		return 0
	}
}

func taskEvaluationRankingAdjustment(summary memory.TaskEvaluationSummary) float64 {
	adjustment := 0.0
	adjustment += float64(summary.VerdictCounts[memory.TaskEvaluationVerdictSucceeded]) * 0.05
	adjustment -= float64(summary.VerdictCounts[memory.TaskEvaluationVerdictFailed]) * 0.25
	adjustment -= float64(summary.VerdictCounts[memory.TaskEvaluationVerdictPartial]) * 0.15
	adjustment -= float64(summary.VerdictCounts[memory.TaskEvaluationVerdictInconclusive]) * 0.20
	adjustment -= float64(summary.ContributionCounts[memory.TaskContributionCategoryMemoryNoisy]) * 0.10
	adjustment -= float64(summary.ContributionCounts[memory.TaskContributionCategoryMemoryStale]) * 0.10
	adjustment -= float64(summary.ContributionCounts[memory.TaskContributionCategoryMemoryIrrelevant]) * 0.10
	adjustment -= float64(summary.ContributionCounts[memory.TaskContributionCategoryMemoryMissing]) * 0.10
	adjustment -= float64(summary.ContributionCounts[memory.TaskContributionCategoryHiddenMemory]) * 0.35
	return adjustment
}

func rankingRolloutPolicyUsesSignal(policy memory.RankingRolloutPolicy, source memory.RankingRolloutSignalSource) bool {
	for _, value := range policy.SignalSources {
		if value == source {
			return true
		}
	}
	return false
}

func firstRankingRolloutSignalSource(sources []memory.RankingRolloutSignalSource) memory.RankingRolloutSignalSource {
	if len(sources) == 0 {
		return ""
	}
	return sources[0]
}

func rankingRolloutEvidenceSatisfied(policy memory.RankingRolloutPolicy, count int) bool {
	if policy.EvidenceMinimum <= 0 {
		return count > 0
	}
	return count >= policy.EvidenceMinimum
}

func (s *Service) applyUsefulnessFeedbackSignals(ctx context.Context, input SearchInput, scored []SearchHit) ([]ContextDiagnostic, error) {
	if s.usefulnessSummarizer == nil || (!input.IncludeFeedbackDiagnostics && !input.FeedbackAwareRanking) {
		return nil, nil
	}
	var positive, negative, needsReview, unsafe int
	for i := range scored {
		summary, err := s.usefulnessSummarizer.SummarizeUsefulnessFeedback(ctx, memory.SummarizeUsefulnessFeedbackInput{
			Scope: input.Scope,
			Subject: memory.UsefulnessFeedbackSubject{
				Kind: memory.UsefulnessFeedbackSubjectMemory,
				ID:   scored[i].Memory.ID,
			},
		})
		if err != nil {
			return nil, err
		}
		if summary.TotalActive == 0 {
			continue
		}
		switch summary.EffectiveQuality {
		case memory.UsefulnessQualityPositive:
			positive++
		case memory.UsefulnessQualityNegative, memory.UsefulnessQualityMixed:
			negative++
		case memory.UsefulnessQualityNeedsReview:
			needsReview++
		}
		if summary.Counts[memory.UsefulnessFeedbackTypeUnsafeOrHidden] > 0 {
			unsafe++
		}
		if input.FeedbackAwareRanking {
			scored[i].Score.Overall += feedbackRankingAdjustment(summary)
		}
	}
	diagnostics := make([]ContextDiagnostic, 0, 4)
	if positive > 0 {
		diagnostics = append(diagnostics, ContextDiagnostic{Section: "search_feedback", Status: "positive_signal", Reason: "active usefulness feedback indicates useful returned memory", Included: positive})
	}
	if negative > 0 {
		diagnostics = append(diagnostics, ContextDiagnostic{Section: "search_feedback", Status: "negative_signal", Reason: "active usefulness feedback indicates noisy stale irrelevant or missing expected memory", Omitted: negative})
	}
	if needsReview > 0 {
		diagnostics = append(diagnostics, ContextDiagnostic{Section: "search_feedback", Status: "needs_review_signal", Reason: "active usefulness feedback requires review", Omitted: needsReview})
	}
	if unsafe > 0 {
		diagnostics = append(diagnostics, ContextDiagnostic{Section: "search_feedback", Status: "safety_signal", Reason: "unsafe or hidden feedback exists without exposing hidden content", Hidden: unsafe})
	}
	if input.FeedbackAwareRanking && len(diagnostics) > 0 {
		diagnostics = append(diagnostics, ContextDiagnostic{Section: "search_feedback", Status: "ranking_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"})
	}
	return diagnostics, nil
}

func feedbackRankingAdjustment(summary memory.UsefulnessFeedbackSummary) float64 {
	switch summary.EffectiveQuality {
	case memory.UsefulnessQualityPositive:
		return 0.05
	case memory.UsefulnessQualityNegative, memory.UsefulnessQualityMixed:
		return -0.25
	case memory.UsefulnessQualityNeedsReview:
		return -0.35
	default:
		return 0
	}
}

func (s *Service) AssembleContext(ctx context.Context, input AssembleContextInput) (output AssembledContext, err error) {
	started := time.Now()
	defer func() {
		if s.observer == nil {
			return
		}

		status := "ok"
		count := len(output.Profile) + len(output.RecentSession) + len(output.RecentEpisodes) + len(output.RelevantSummaries) + len(output.RelatedEntities) + len(output.KnownFailures) + len(output.ExperienceLessons)
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		s.observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "api",
			Component:  "retrieval_service",
			Operation:  "assemble_context",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: time.Now().UTC(),
		})
	}()

	if err := input.Validate(); err != nil {
		return AssembledContext{}, err
	}

	result, err := s.Search(ctx, SearchInput{
		Scope:                      input.Scope,
		Query:                      input.Query,
		TopK:                       maxInt(input.Budget*3, input.Budget),
		IncludeSummaries:           true,
		IncludeRelations:           input.IncludeRelations,
		IncludeFeedbackDiagnostics: input.IncludeDiagnostics && input.IncludeFeedbackDiagnostics,
		FeedbackAwareRanking:       false,
		rankingPolicyDisabled:      true,
	})
	if err != nil {
		return AssembledContext{}, err
	}
	adjusted, policyDiagnostics, err := s.applyRankingRolloutPolicy(ctx, input.Scope, memory.RankingRolloutSurfaceContext, input.FeedbackAwareRanking, result.Hits)
	if err != nil {
		return AssembledContext{}, err
	}
	if adjusted {
		sort.Slice(result.Hits, func(i, j int) bool {
			if result.Hits[i].Score.Overall == result.Hits[j].Score.Overall {
				return result.Hits[i].Memory.ModifiedAt.After(result.Hits[j].Memory.ModifiedAt)
			}
			return result.Hits[i].Score.Overall > result.Hits[j].Score.Overall
		})
	}

	output = AssembledContext{
		Profile:           make([]SearchHit, 0),
		RecentSession:     make([]SearchHit, 0),
		RecentEpisodes:    make([]SearchHit, 0),
		RelevantSummaries: make([]SearchHit, 0),
		RelatedEntities:   make([]SearchHit, 0),
		Citations:         make([]Citation, 0),
		KnownFailures:     make([]ExperienceInsightContext, 0),
		ExperienceLessons: make([]ExperienceInsightContext, 0),
	}
	if input.IncludeDiagnostics {
		output.Diagnostics = append(output.Diagnostics, result.Diagnostics...)
		output.Diagnostics = append(output.Diagnostics, policyDiagnostics...)
	}

	profiles := make([]SearchHit, 0)
	summaries := make([]SearchHit, 0)
	relations := make([]SearchHit, 0)
	episodes := make([]SearchHit, 0)
	others := make([]SearchHit, 0)
	citationSeen := map[string]struct{}{}
	for _, hit := range result.Hits {
		switch hit.Memory.Class {
		case memory.MemoryClassProfile:
			profiles = append(profiles, hit)
		case memory.MemoryClassSummary:
			summaries = append(summaries, hit)
		case memory.MemoryClassRelation:
			relations = append(relations, hit)
		case memory.MemoryClassEpisodic:
			episodes = append(episodes, hit)
		default:
			others = append(others, hit)
		}

		for _, citation := range hit.Citations {
			key := citation.MemoryID + ":" + citation.RawEventID + ":" + citation.Operation
			if _, ok := citationSeen[key]; ok {
				continue
			}
			citationSeen[key] = struct{}{}
			output.Citations = append(output.Citations, citation)
		}
	}

	remaining := input.Budget
	if remaining > 0 && len(profiles) > 0 {
		output.Profile = append(output.Profile, profiles[0])
		remaining--
	}

	if remaining > 0 && len(summaries) > 0 {
		take := minInt(len(summaries), remaining)
		output.RelevantSummaries = append(output.RelevantSummaries, summaries[:take]...)
		remaining -= take
	}

	if remaining > 0 && input.IncludeRelations && len(relations) > 0 {
		take := minInt(len(relations), remaining)
		output.RelatedEntities = append(output.RelatedEntities, relations[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(summaries) == 0 && len(episodes) > 0 {
		take := minInt(len(episodes), remaining)
		output.RecentSession = append(output.RecentSession, episodes[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(others) > 0 {
		take := minInt(len(others), remaining)
		output.RecentEpisodes = append(output.RecentEpisodes, others[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(summaries) > 0 && len(episodes) > 0 {
		take := minInt(len(episodes), remaining)
		output.RecentSession = append(output.RecentSession, episodes[:take]...)
		remaining -= take
	}

	if input.IncludeExperienceInsights && remaining > 0 && s.insights != nil {
		if err := s.appendExperienceInsights(ctx, input.Scope, input.IncludeDiagnostics, &output, &remaining); err != nil {
			return AssembledContext{}, err
		}
	} else if input.IncludeDiagnostics {
		output.Diagnostics = append(output.Diagnostics, skippedExperienceDiagnostics(input.IncludeExperienceInsights, s.insights != nil, remaining)...)
	}

	return output, nil
}

func (s *Service) appendExperienceInsights(ctx context.Context, scope memory.Scope, includeDiagnostics bool, output *AssembledContext, remaining *int) error {
	if remaining == nil || *remaining <= 0 {
		return nil
	}

	if err := s.appendExperienceInsightSection(ctx, scope, experienceInsightSection{
		Name:        "known_failures",
		InsightType: memory.DerivedInsightTypeFailurePattern,
	}, includeDiagnostics, output, remaining); err != nil {
		return err
	}
	if remaining == nil || *remaining <= 0 {
		if includeDiagnostics {
			output.Diagnostics = append(output.Diagnostics, ContextDiagnostic{
				Section:     "experience_lessons",
				InsightType: memory.DerivedInsightTypeLesson,
				Status:      "omitted_by_budget",
				Reason:      "context budget exhausted before section evaluation",
			})
		}
		return nil
	}

	return s.appendExperienceInsightSection(ctx, scope, experienceInsightSection{
		Name:        "experience_lessons",
		InsightType: memory.DerivedInsightTypeLesson,
	}, includeDiagnostics, output, remaining)
}

type experienceInsightSection struct {
	Name        string
	InsightType memory.DerivedInsightType
}

func (s *Service) appendExperienceInsightSection(ctx context.Context, scope memory.Scope, section experienceInsightSection, includeDiagnostics bool, output *AssembledContext, remaining *int) error {
	limit := maxInt(*remaining+10, 10)
	visible, err := s.insights.ListDerivedInsights(ctx, memory.ListDerivedInsightsInput{
		Scope:            scope,
		Type:             section.InsightType,
		State:            memory.DerivedInsightStateActive,
		MinEvidenceCount: 1,
		IncludeHidden:    false,
		Limit:            limit,
	})
	if err != nil {
		return err
	}
	sortExperienceInsights(visible)

	take := minInt(len(visible), *remaining)
	for _, insight := range visible[:take] {
		context := experienceInsightContext(insight)
		switch section.Name {
		case "known_failures":
			output.KnownFailures = append(output.KnownFailures, context)
		case "experience_lessons":
			output.ExperienceLessons = append(output.ExperienceLessons, context)
		}
	}
	*remaining -= take

	if includeDiagnostics {
		output.Diagnostics = append(output.Diagnostics, experienceVisibilityDiagnostics(ctx, s.insights, scope, section, visible, take, limit)...)
	}

	return nil
}

func experienceVisibilityDiagnostics(ctx context.Context, insights DerivedInsightLister, scope memory.Scope, section experienceInsightSection, visible []memory.DerivedInsight, included int, limit int) []ContextDiagnostic {
	items := make([]ContextDiagnostic, 0)
	status := "no_visible_insights"
	if included > 0 {
		status = "included"
	}
	items = append(items, ContextDiagnostic{
		Section:     section.Name,
		InsightType: section.InsightType,
		Status:      status,
		Available:   len(visible),
		Included:    included,
		Omitted:     maxInt(len(visible)-included, 0),
	})

	omittedQuality := 0
	omittedBudget := 0
	for _, insight := range visible[included:] {
		if experienceInsightQualityScore(insight) < 0 {
			omittedQuality++
			continue
		}
		omittedBudget++
	}
	if omittedQuality > 0 {
		items = append(items, ContextDiagnostic{
			Section:     section.Name,
			InsightType: section.InsightType,
			Status:      "omitted_by_quality",
			Reason:      "lower ranked by feedback quality policy within available context budget",
			Omitted:     omittedQuality,
		})
	}
	if omittedBudget > 0 {
		items = append(items, ContextDiagnostic{
			Section:     section.Name,
			InsightType: section.InsightType,
			Status:      "omitted_by_budget",
			Reason:      "context budget exhausted before all visible insights were included",
			Omitted:     omittedBudget,
		})
	}

	hidden, err := insights.ListDerivedInsights(ctx, memory.ListDerivedInsightsInput{
		Scope:            scope,
		Type:             section.InsightType,
		MinEvidenceCount: 1,
		IncludeHidden:    true,
		Limit:            limit,
	})
	if err == nil {
		hiddenCount := 0
		for _, insight := range hidden {
			if insight.State != memory.DerivedInsightStateActive || isHiddenDerivedInsightState(insight.State) {
				hiddenCount++
			}
		}
		if hiddenCount > 0 {
			items = append(items, ContextDiagnostic{
				Section:     section.Name,
				InsightType: section.InsightType,
				Status:      "hidden_by_lifecycle_or_scope",
				Reason:      "non-active or hidden lifecycle state is excluded from ordinary context assembly",
				Hidden:      hiddenCount,
			})
		}
	}

	return items
}

func skippedExperienceDiagnostics(includeExperienceInsights bool, insightsConfigured bool, remaining int) []ContextDiagnostic {
	status := "not_requested"
	reason := "experience insight sections were not requested"
	if includeExperienceInsights && !insightsConfigured {
		status = "unavailable"
		reason = "derived insight source is not configured"
	} else if includeExperienceInsights && remaining <= 0 {
		status = "omitted_by_budget"
		reason = "context budget exhausted before experience insight sections"
	}
	return []ContextDiagnostic{
		{Section: "known_failures", InsightType: memory.DerivedInsightTypeFailurePattern, Status: status, Reason: reason},
		{Section: "experience_lessons", InsightType: memory.DerivedInsightTypeLesson, Status: status, Reason: reason},
	}
}

func isHiddenDerivedInsightState(state memory.DerivedInsightState) bool {
	switch state {
	case memory.DerivedInsightStateSuppressed, memory.DerivedInsightStateForgotten, memory.DerivedInsightStateDeleted:
		return true
	default:
		return false
	}
}

func sortExperienceInsights(items []memory.DerivedInsight) {
	sort.SliceStable(items, func(i, j int) bool {
		left := experienceInsightQualityScore(items[i])
		right := experienceInsightQualityScore(items[j])
		if left == right {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return left > right
	})
}

func experienceInsightQualityScore(insight memory.DerivedInsight) int {
	summary := insight.FeedbackSummary
	score := summary.PositiveCount * 10
	score -= summary.NegativeCount * 10
	if summary.NeedsReview {
		score -= 20
	}
	score -= summary.Counts[memory.InsightFeedbackTypeStale] * 5
	score -= summary.Counts[memory.InsightFeedbackTypeRedundant] * 5
	return score
}

func experienceInsightContext(insight memory.DerivedInsight) ExperienceInsightContext {
	citations := make([]InsightCitation, 0, len(insight.Evidence))
	for _, evidence := range insight.Evidence {
		citations = append(citations, InsightCitation{
			InsightID:    insight.ID,
			EvidenceKind: string(evidence.Kind),
			EvidenceID:   evidence.ID,
			Relation:     string(evidence.Relation),
		})
	}

	return ExperienceInsightContext{
		Insight:   insight,
		Citations: citations,
	}
}

func matchClassFilter(class memory.MemoryClass, filters []memory.MemoryClass) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if filter == class {
			return true
		}
	}

	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
