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
	Scope                                 memory.Scope
	Query                                 string
	QueryEmbedding                        []float32
	LexicalMatchMode                      LexicalMatchMode
	Classes                               []memory.MemoryClass
	rankingSurface                        memory.RankingRolloutSurface
	rankingPolicyDisabled                 bool
	queryAnalysisPolicyDisabled           bool
	queryAnalysisObserveOnly              bool
	queryAnalysisDiagnosticsAuthorized    bool
	retrievalPlannerDiagnosticsAuthorized bool
	retrievalPlannerDisabled              bool
	fusionStrategyOverride                *FusionStrategy
	TimeFrom                              time.Time
	TimeTo                                time.Time
	// TemporalConstraint selects fact-valid time explicitly. The zero value
	// resolves to current-valid selection; recorded-time filters keep their
	// existing meaning and are never reinterpreted as valid time.
	TemporalConstraint         memory.TemporalConstraint
	TopK                       int
	IncludeSummaries           bool
	IncludeRelations           bool
	IncludeFeedbackDiagnostics bool
	FeedbackAwareRanking       bool
	SessionID                  string
	UserID                     string
	queryAnalysis              *QueryAnalysisResult
	// temporalPolicyDisabled disables temporal-aware policy resolution and
	// returns the approved current baseline. It is the operational rollback
	// switch: setting it must never delete temporal history or rewrite canonical
	// data, it only stops consulting valid-time selectors during retrieval.
	temporalPolicyDisabled bool
}

// WithTemporalPolicyDisabled returns a copy of the input with temporal-aware
// policy resolution disabled, which is the supported operational rollback path.
//
// Rollback is data-preserving by construction: the validity columns and the
// correction ledger keep their contents and remain readable through the
// privileged history surface; only ordinary retrieval stops filtering on them.
func (input SearchInput) WithTemporalPolicyDisabled() SearchInput {
	input.temporalPolicyDisabled = true
	input.TemporalConstraint = memory.TemporalConstraint{}
	return input
}

// TemporalPolicyDisabled reports whether temporal-aware policy resolution has
// been disabled for this input.
func (input SearchInput) TemporalPolicyDisabled() bool {
	return input.temporalPolicyDisabled
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
	if i.TemporalConstraint.Mode != "" {
		if err := i.TemporalConstraint.Validate(); err != nil {
			return fmt.Errorf("invalid temporal constraint: %w", err)
		}
	}
	if i.TopK < 0 {
		return fmt.Errorf("top_k must be greater than or equal to zero")
	}
	return nil
}

type AssembleContextInput struct {
	Scope                                 memory.Scope
	Query                                 string
	SessionID                             string
	UserID                                string
	Budget                                int
	CharacterBudget                       int
	IncludeRelations                      bool
	IncludeExperienceInsights             bool
	IncludeDiagnostics                    bool
	IncludeFeedbackDiagnostics            bool
	FeedbackAwareRanking                  bool
	UseProjections                        bool
	retrievalPlannerDiagnosticsAuthorized bool
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
	if i.CharacterBudget < 0 {
		return fmt.Errorf("character_budget must be greater than or equal to zero")
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

	// Chunk is intentionally not part of the public response shape.  It lets
	// context assembly account for derived evidence while citations continue to
	// point at the canonical parent or immutable source.
	Chunk *memory.MemoryChunk `json:"-"`
}

// TemporalSelection is the bounded record of how one search resolved fact-valid
// time.
//
// It names the selector that was applied and how many candidates the valid-time
// predicate removed, and nothing else: no intervals, no identities, no content,
// and no per-hit detail. Per-hit version identity stays behind the history and
// provenance endpoints, which are the authorized surfaces for it.
type TemporalSelection struct {
	Mode string `json:"mode"`
	// Omitted counts candidates the valid-time predicate removed. It is a bare
	// count so a caller can see the selector bit without learning which
	// memories, or which facts, were involved.
	Omitted int `json:"omitted"`
}

type SearchResult struct {
	Hits        []SearchHit         `json:"hits"`
	Diagnostics []ContextDiagnostic `json:"diagnostics,omitempty"`
	// Temporal is present only when the caller supplied an explicit valid-time
	// selector or when the predicate actually removed a candidate. An ordinary
	// current search therefore keeps its exact pre-temporal response shape.
	Temporal           *TemporalSelection `json:"temporal,omitempty"`
	plannerDiagnostics []RetrievalPlannerDiagnostics

	// fusionChannelAvailability and fusionStrategy are evaluation-only state.
	// They remain unexported so ordinary API responses cannot expose internal
	// recall execution details.
	fusionChannelAvailability map[FusionChannel]fusionChannelAvailability
	fusionStrategy            FusionStrategy
	retrievalPlan             *RetrievalPlan
	retrievalPassObservations []RetrievalPassObservation
	rerankerObservation       RetrievalRerankerObservation
	// temporalOmissions reports, by bounded category, how many candidates the
	// valid-time predicate removed. It stays unexported so ordinary responses
	// carry no temporal detail; authorized diagnostics read it explicitly.
	temporalOmissions TemporalOmissionReport
}

// TemporalOmissionReport exposes the bounded omission categories recorded while
// answering one search. It is intentionally a value copy so a caller cannot
// mutate the result's internal accounting.
func (result SearchResult) TemporalOmissionReport() TemporalOmissionReport {
	return result.temporalOmissions.Normalize()
}

type RetrievalPassObservation struct {
	Pass             int
	CandidateCount   int
	VisibleMemoryIDs []string
	Evidence         EvidenceAssessment
	Latency          time.Duration
}

type RetrievalRerankerObservation struct {
	Attempted        bool
	Used             bool
	Safe             bool
	FallbackCategory string
}

type fusionChannelAvailability string

const (
	fusionChannelAvailable   fusionChannelAvailability = "available"
	fusionChannelUnavailable fusionChannelAvailability = "unavailable"
)

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
	// Query-analysis fields are bounded aggregate diagnostics. They are only
	// populated for explicitly requested diagnostic/evaluation paths; no query
	// text, plans, identifiers, scope values, or raw scores are carried here.
	PolicyVersion       QueryAnalysisPolicyVersion     `json:"policy_version,omitempty"`
	LimitsVersion       QueryAnalysisLimitsVersion     `json:"limits_version,omitempty"`
	OriginalRetained    bool                           `json:"original_retained,omitempty"`
	HintCount           int                            `json:"hint_count,omitempty"`
	SignalCount         int                            `json:"signal_count,omitempty"`
	SubqueryCount       int                            `json:"subquery_count,omitempty"`
	CandidateCount      int                            `json:"candidate_count,omitempty"`
	ElapsedNS           int64                          `json:"elapsed_ns,omitempty"`
	Fallback            QueryAnalysisFallbackCategory  `json:"fallback,omitempty"`
	Disposition         QueryAnalysisDisposition       `json:"disposition,omitempty"`
	Categories          []QueryAnalysisDiagnosticCount `json:"categories,omitempty"`
	NormalizationStatus string                         `json:"normalization_status,omitempty"`
	TimeStatus          string                         `json:"time_status,omitempty"`
	RolloutStage        string                         `json:"rollout_stage,omitempty"`
}

type AssembledContext struct {
	Profile            []SearchHit                `json:"profile"`
	RecentSession      []SearchHit                `json:"recent_session"`
	RecentEpisodes     []SearchHit                `json:"recent_episodes"`
	RelevantSummaries  []SearchHit                `json:"relevant_summaries"`
	RelatedEntities    []SearchHit                `json:"related_entities"`
	Citations          []Citation                 `json:"citations"`
	KnownFailures      []ExperienceInsightContext `json:"known_failures,omitempty"`
	ExperienceLessons  []ExperienceInsightContext `json:"experience_lessons,omitempty"`
	Diagnostics        []ContextDiagnostic        `json:"diagnostics,omitempty"`
	plannerDiagnostics []ContextDiagnostic
}

type ScoredMemory struct {
	Memory                  memory.CanonicalMemory
	LexicalScore            float64
	SemanticScore           float64
	RelationScore           float64
	SourceEventID           string
	ParentMemoryID          string
	EmbeddingRevision       string
	EmbeddingRevisionActive bool
	Embedding               []float32
	SessionID               string
	EntityKey               string
	TimeSlice               string
	Citations               []Citation
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

// ChunkSearchInput is deliberately separate from SearchInput. Chunk stores may
// use a different physical representation, but must receive the exact resolved
// scope and the same query constraints as canonical retrieval.
type ChunkSearchInput struct {
	Scope          memory.Scope
	Query          string
	QueryEmbedding []float32
	Classes        []memory.MemoryClass
	TopK           int
}

// ChunkCandidate is a derived retrieval candidate. Parent is the validated
// canonical representation used to preserve the existing public hit contract;
// the chunk itself remains internal evidence metadata.
type ChunkCandidate struct {
	Chunk     memory.MemoryChunk
	Parent    memory.CanonicalMemory
	Score     ScoreBreakdown
	Citations []Citation
}

// ChunkCandidateSearcher returns only candidates it can prove are visible.
// Service repeats the scope and lifecycle checks, so an implementation error
// cannot widen ordinary retrieval.
type ChunkCandidateSearcher interface {
	SearchChunks(ctx context.Context, input ChunkSearchInput) ([]ChunkCandidate, error)
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

type ContextCalibrationSummaryReader interface {
	ReadContextCalibrationSummary(ctx context.Context, input memory.ReadContextCalibrationSummaryInput) (memory.ContextCalibrationSummary, error)
}

// ContextCalibrationLimits are deployment-owned ceilings and floors. A rollout
// payload is deliberately not allowed to make calibration broader than these
// values; invalid or absent limits fail closed at the request boundary.
type ContextCalibrationLimits struct {
	MaxSummaryAge       time.Duration
	MinimumEvidence     int
	ConfidenceThreshold float64
	DecayWindow         time.Duration
	ContributionCap     float64
	MaxCandidates       int
	MaxContextItems     int
	MaxElapsed          time.Duration
}

func (l ContextCalibrationLimits) valid() bool {
	return l.MaxSummaryAge > 0 && l.MinimumEvidence > 0 && l.ConfidenceThreshold >= 0 && l.ConfidenceThreshold <= 1 &&
		l.DecayWindow > 0 && l.ContributionCap > 0 && l.ContributionCap <= 1 && l.MaxCandidates > 0 && l.MaxContextItems > 0 && l.MaxElapsed > 0
}

// ContextProjectionReader is optional so existing deployments can roll back
// projection consumption without changing the live retrieval path.
type ContextProjectionReader interface {
	ReadLatestContextProjection(ctx context.Context, scope memory.Scope, kind memory.ContextProjectionKind) (memory.ContextProjection, error)
}

type rankingRolloutMetricObserver interface {
	RecordRankingRollout(ctx context.Context, event telemetry.RankingRolloutEvent)
}

type retrievalRerankMetricObserver interface {
	RecordRetrievalRerank(ctx context.Context, event telemetry.RetrievalRerankEvent)
}

type MemorySearcher interface {
	Search(ctx context.Context, input SearchInput) (SearchResult, error)
}

type ContextAssembler interface {
	AssembleContext(ctx context.Context, input AssembleContextInput) (AssembledContext, error)
}

type ServiceDependencies struct {
	Lexical                         LexicalSearcher
	Semantic                        SemanticSearcher
	Relations                       RelationSearcher
	GraphTraversal                  GraphTraversalSearcher
	Citations                       CitationLister
	Insights                        DerivedInsightLister
	UsefulnessSummarizer            UsefulnessSummarizer
	TaskEvaluationSummarizer        TaskEvaluationSummarizer
	RankingRolloutPolicyReader      RankingRolloutPolicyReader
	ContextCalibrationSummaryReader ContextCalibrationSummaryReader
	// ContextCalibrationEnabled is an operational hard gate. An exact-scope
	// rollout policy can narrow deployment limits but cannot bypass a disabled
	// deployment.
	ContextCalibrationEnabled    bool
	ContextCalibrationLimits     ContextCalibrationLimits
	Projections                  ContextProjectionReader
	ProjectionConsumptionEnabled bool
	Chunks                       ChunkCandidateSearcher
	// FusionStrategy defaults to the versioned RRF baseline when omitted.
	FusionStrategy FusionStrategy
	// ChunkRollout defaults to default_off. Shadow evaluates chunk candidates
	// only for authorized diagnostics; Active permits them to influence results.
	ChunkRollout         memory.ChunkRolloutMode
	QueryAnalyzer        QueryAnalyzer
	QueryAnalysisLimits  QueryAnalysisLimits
	Reranker             Reranker
	RerankerMode         RerankerMode
	RerankerProvider     string
	RerankerVersion      string
	QualityBounds        QualityAdjustmentBounds
	RetrievalPlanPolicy  RetrievalPlanPolicy
	GraphTraversalLimits GraphTraversalLimits
}

type QueryAnalyzer interface {
	Analyze(input QueryAnalysisInput) (QueryAnalysisResult, error)
}

type effectiveQueryAnalysisPolicyReader interface {
	ReadEffectiveQueryAnalysisRolloutPolicy(ctx context.Context, input memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
}

type effectiveRetrievalPlannerPolicyReader interface {
	ReadEffectiveRetrievalPlannerRolloutPolicy(ctx context.Context, input memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
}

const (
	retrievalPlannerRankingVersion  = "quality-feature-v1"
	retrievalPlannerRendererVersion = "context-renderer-v1"
)

type Service struct {
	lexical                         LexicalSearcher
	semantic                        SemanticSearcher
	relations                       RelationSearcher
	graphTraversal                  GraphTraversalSearcher
	citations                       CitationLister
	insights                        DerivedInsightLister
	usefulnessSummarizer            UsefulnessSummarizer
	taskEvaluationSummarizer        TaskEvaluationSummarizer
	rankingRolloutPolicyReader      RankingRolloutPolicyReader
	contextCalibrationSummaryReader ContextCalibrationSummaryReader
	contextCalibrationEnabled       bool
	contextCalibrationLimits        ContextCalibrationLimits
	projections                     ContextProjectionReader
	projectionConsumptionEnabled    bool
	chunks                          ChunkCandidateSearcher
	fusionStrategy                  FusionStrategy
	chunkRollout                    memory.ChunkRolloutMode
	queryAnalyzer                   QueryAnalyzer
	queryAnalysisLimits             QueryAnalysisLimits
	reranker                        Reranker
	rerankerMode                    RerankerMode
	rerankerProvider                string
	rerankerVersion                 string
	qualityBounds                   QualityAdjustmentBounds
	retrievalPlanPolicy             RetrievalPlanPolicy
	graphTraversalLimits            GraphTraversalLimits
	// now supplies the evaluation clock for valid-time predicates. It defaults to
	// UTC wall time and exists so tests can pin one instant deterministically.
	now      func() time.Time
	observer telemetry.Observer
}

func (s *Service) evaluationClock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now().UTC()
}

func NewService(deps ServiceDependencies, observers ...telemetry.Observer) *Service {
	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}

	chunkRollout := deps.ChunkRollout
	if chunkRollout == "" {
		chunkRollout = memory.ChunkRolloutModeDefaultOff
	}
	fusionStrategy := deps.FusionStrategy
	if fusionStrategy.Name == "" {
		fusionStrategy = DefaultRRFStrategy()
	}
	rerankerMode := deps.RerankerMode
	if rerankerMode == "" {
		rerankerMode = RerankerModeDisabled
	}
	if !rerankerMode.Valid() {
		rerankerMode = RerankerModeDisabled
	}
	qualityBounds := deps.QualityBounds
	if qualityBounds.PerFeature <= 0 {
		qualityBounds.PerFeature = 0.05
	}
	if qualityBounds.Total <= 0 {
		qualityBounds.Total = 0.25
	}
	retrievalPlanPolicy := deps.RetrievalPlanPolicy
	if retrievalPlanPolicy.PlannerVersion == "" && retrievalPlanPolicy.Version == "" && retrievalPlanPolicy.Templates == nil {
		retrievalPlanPolicy = DefaultRetrievalPlanPolicy()
	}
	graphTraversalLimits := deps.GraphTraversalLimits
	if graphTraversalLimits == (GraphTraversalLimits{}) {
		graphTraversalLimits = DefaultGraphTraversalLimits()
	}
	if graphTraversalLimits.Validate() != nil {
		graphTraversalLimits = DefaultGraphTraversalLimits()
	}
	return &Service{
		lexical:                         deps.Lexical,
		semantic:                        deps.Semantic,
		relations:                       deps.Relations,
		graphTraversal:                  deps.GraphTraversal,
		citations:                       deps.Citations,
		insights:                        deps.Insights,
		usefulnessSummarizer:            deps.UsefulnessSummarizer,
		taskEvaluationSummarizer:        deps.TaskEvaluationSummarizer,
		rankingRolloutPolicyReader:      deps.RankingRolloutPolicyReader,
		contextCalibrationSummaryReader: deps.ContextCalibrationSummaryReader,
		contextCalibrationEnabled:       deps.ContextCalibrationEnabled,
		contextCalibrationLimits:        deps.ContextCalibrationLimits,
		projections:                     deps.Projections,
		projectionConsumptionEnabled:    deps.ProjectionConsumptionEnabled,
		chunks:                          deps.Chunks,
		fusionStrategy:                  fusionStrategy,
		chunkRollout:                    chunkRollout,
		queryAnalyzer:                   deps.QueryAnalyzer,
		queryAnalysisLimits:             deps.QueryAnalysisLimits,
		reranker:                        deps.Reranker,
		rerankerMode:                    rerankerMode,
		rerankerProvider:                strings.TrimSpace(deps.RerankerProvider),
		rerankerVersion:                 strings.TrimSpace(deps.RerankerVersion),
		qualityBounds:                   qualityBounds,
		retrievalPlanPolicy:             retrievalPlanPolicy,
		graphTraversalLimits:            graphTraversalLimits,
		observer:                        observer,
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
	// Capture one evaluation instant per request so every valid-time predicate in
	// this search observes the same clock, even across follow-up passes.
	evaluationInstant := s.evaluationClock()
	temporalConstraint := input.TemporalConstraint
	if temporalConstraint.Mode == "" {
		temporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
	}
	surface := input.rankingSurface
	if surface == "" {
		surface = memory.RankingRolloutSurfaceSearch
	}
	var activeRankingPolicy *memory.RankingRolloutPolicy
	var queryAnalysisPolicy *memory.RankingRolloutPolicy
	if s.rankingRolloutPolicyReader != nil && !input.rankingPolicyDisabled {
		policy, policyErr := s.readActiveRankingPolicy(ctx, input.Scope, surface)
		if policyErr != nil {
			return SearchResult{}, policyErr
		}
		activeRankingPolicy = policy
	}
	// Effective query-analysis policy includes diagnostics/shadow stages that
	// are intentionally excluded from the ranking active-policy reader.
	if s.queryAnalyzer != nil && !input.queryAnalysisPolicyDisabled {
		if effectiveReader, ok := s.rankingRolloutPolicyReader.(effectiveQueryAnalysisPolicyReader); ok {
			policy, policyErr := effectiveReader.ReadEffectiveQueryAnalysisRolloutPolicy(ctx, memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: input.Scope, Surface: surface, SessionID: input.SessionID, UserID: input.UserID})
			if policyErr == nil {
				queryAnalysisPolicy = &policy
			} // malformed/foreign/expired QA policy fails closed to original-only
		}
	}
	fusionStrategy := s.fusionStrategy
	if input.fusionStrategyOverride != nil {
		fusionStrategy = *input.fusionStrategyOverride
	}
	if activeRankingPolicy != nil {
		configured, ok, fusionErr := fusionStrategyFromPolicy(*activeRankingPolicy)
		if fusionErr != nil {
			return SearchResult{}, fusionErr
		}
		if ok {
			fusionStrategy = configured
		}
	}
	baselineFusionStrategy := fusionStrategy

	// Query understanding is strictly rollout-gated. The immutable original
	// signal is always retained; malformed/unavailable analysis falls back to it.
	recallInputs, analysisDiagnostics := s.queryRecallInputs(ctx, input, queryAnalysisPolicy, surface)
	qaResolution := memory.ResolveQueryAnalysisRollout(queryAnalysisPolicy, memory.ResolveQueryAnalysisRolloutInput{Scope: input.Scope, Surface: surface, SessionID: input.SessionID, UserID: input.UserID, Now: time.Now().UTC()})
	analysis := originalQueryAnalysis(input)
	if qaResolution.DerivedSignalsAffectResults && len(recallInputs) > 0 && recallInputs[0].queryAnalysis != nil {
		analysis = *recallInputs[0].queryAnalysis
	}
	plannerExecution := retrievalPlannerExecution{stage: memory.RetrievalPlannerRolloutStageBaseline}
	if !input.retrievalPlannerDisabled {
		plannerExecution = s.resolveRetrievalPlannerExecution(ctx, input, surface, analysis, time.Now().UTC())
	}
	if plannerExecution.active() {
		if !s.canExecuteRetrievalPlan(input, *plannerExecution.plan) || !s.canRetainPlannerBaseline(input, *plannerExecution.plan) {
			fallback := input
			fallback.retrievalPlannerDisabled = true
			return s.Search(ctx, fallback)
		}
		fusionStrategy = plannerExecution.plan.Fusion
	}

	channelCandidates := make([]FusionChannelCandidates, 0, 4)
	baselineChannelCandidates := make([]FusionChannelCandidates, 0, 4)
	plannerFailed := false
	diversityMetadata := make(map[string]ScoredMemory)
	channelAvailability := defaultFusionChannelAvailability()
	chunkCitationMap := map[string][]Citation{}
	chunkByMemoryID := map[string]*memory.MemoryChunk{}
	fusionDiagnostics := []ContextDiagnostic(nil)
	fusionDiagnostics = append(fusionDiagnostics, analysisDiagnostics...)
	analysisLimits := s.queryAnalysisLimits
	if analysisLimits.Version == "" || analysisLimits.Validate() != nil {
		analysisLimits = DefaultQueryAnalysisLimits()
	}
	// Candidate limits affect result-producing retrieval only. Diagnostics and
	// shadow analysis may use their own analyzer bounds, but must not silently
	// cap the canonical recall result until an exact-scope active rollout is
	// authorized.
	if qaResolution.DerivedSignalsAffectResults && queryAnalysisPolicy != nil && queryAnalysisPolicy.QueryAnalysis != nil {
		qa := queryAnalysisPolicy.QueryAnalysis
		analysisLimits = QueryAnalysisLimits{Version: QueryAnalysisLimitsVersionV1, MaxQueryBytes: qa.MaxQueryBytes, MaxHints: qa.MaxHints, MaxSignals: qa.MaxSignals, MaxSubqueries: qa.MaxSubqueries, MaxTermBytes: qa.MaxTermBytes, MaxSubqueryBytes: qa.MaxSubqueryBytes, MaxAnalysisWork: qa.MaxAnalysisWork, MaxCandidatesPerSignal: qa.MaxCandidatesPerSignal, MaxAggregateCandidates: qa.MaxAggregateCandidates, MaxElapsed: qa.MaxElapsed}
		if analysisLimits.Validate() != nil {
			analysisLimits = DefaultQueryAnalysisLimits()
		}
	}
	queryAnalysisShadowLimits := analysisLimits
	resultRecallInputs := make([]SearchInput, 0, len(recallInputs))
	queryAnalysisShadowInputs := make([]SearchInput, 0, len(recallInputs))
	for _, recallInput := range recallInputs {
		if recallInput.queryAnalysisObserveOnly {
			queryAnalysisShadowInputs = append(queryAnalysisShadowInputs, recallInput)
			continue
		}
		resultRecallInputs = append(resultRecallInputs, recallInput)
	}
	recallInputs = resultRecallInputs
	if plannerExecution.active() && !plannerFailed {
		analysisLimits.MaxCandidatesPerSignal = plannerExecution.plan.TotalCandidates
		analysisLimits.MaxAggregateCandidates = plannerExecution.plan.TotalCandidates
	}
	aggregateCandidates := 0
	recalledCandidates := 0
	filteredCandidates := 0
	expiredCandidates := 0
	var temporalOmissions TemporalOmissionReport
	var graphTraversalDiagnostics *GraphTraversalDiagnostics
	requiredChannelUnavailable := false
	shadowCandidates := 0
	plannerEvidence := EvidenceAssessment{}
	plannerEvidenceSet := false
	plannerPassCount := 1
	plannerCandidateCount := 0
	plannerChannelAvailability := plannerNotEvaluatedChannels(plannerExecution.plan)
	plannerChangedRankCount := 0
	plannerChangedRankObserved := false
	var shadowPlannedOrder []string
	plannerPassObservations := make([]RetrievalPassObservation, 0, 2)
	reserveRecall := func(channel FusionChannel, recallInput SearchInput) (SearchInput, bool, bool, error) {
		if !plannerExecution.active() {
			return recallInput, true, true, nil
		}
		if plannerExecution.declaresChannel(channel) {
			callInput, execute, err := plannerExecution.reserveChannelRequest(1, channel, recallInput)
			return callInput, execute, true, err
		}
		callInput, execute, err := plannerExecution.reserveBaselineRequest(channel, recallInput)
		return callInput, execute, false, err
	}

	filterChannel := func(pass int, channel FusionChannel, hits []ScoredMemory, includePlanned, includeBaseline bool) error {
		if includePlanned {
			recalledCandidates += len(hits)
		}
		if analysisLimits.MaxCandidatesPerSignal > 0 && len(hits) > analysisLimits.MaxCandidatesPerSignal {
			if includePlanned {
				filteredCandidates += len(hits) - analysisLimits.MaxCandidatesPerSignal
			}
			hits = hits[:analysisLimits.MaxCandidatesPerSignal]
		}
		if plannerExecution.active() && includePlanned {
			remaining := plannerExecution.ledger.UnacceptedChannelRequestsForPass(pass, channel)
			if len(hits) > remaining {
				filteredCandidates += len(hits) - remaining
				hits = hits[:remaining]
			}
		}
		visible := make([]ScoredMemory, 0, len(hits))
		for _, hit := range hits {
			if hit.Memory.Scope.Normalized() != input.Scope.Normalized() || hit.Memory.State != memory.MemoryStateActive {
				if includePlanned {
					filteredCandidates++
				}
				continue
			}
			// Fact-valid time is evaluated before ranking so an expired version
			// can never outrank the current one on similarity alone.
			if !temporalConstraint.Matches(hit.Memory.TemporalValidity, evaluationInstant) {
				if includePlanned {
					filteredCandidates++
				}
				expiredCandidates++
				// Record a bounded reason. Only the class and the shape of the
				// validity snapshot are inspected, so the category can never
				// carry content, identity, or interval detail.
				temporalOmissions.Add(classifyTemporalOmission(temporalConstraint, hit.Memory.Class, hit.Memory.TemporalValidity))
				continue
			}
			if !matchClassFilter(hit.Memory.Class, input.Classes) {
				if includePlanned {
					filteredCandidates++
				}
				continue
			}
			if !input.IncludeSummaries && hit.Memory.Class == memory.MemoryClassSummary {
				if includePlanned {
					filteredCandidates++
				}
				continue
			}
			if !input.IncludeRelations && hit.Memory.Class == memory.MemoryClassRelation {
				if includePlanned {
					filteredCandidates++
				}
				continue
			}
			visible = append(visible, hit)
		}
		planned := visible
		if includePlanned && analysisLimits.MaxAggregateCandidates > 0 {
			remaining := maxInt(analysisLimits.MaxAggregateCandidates-aggregateCandidates, 0)
			if len(planned) > remaining {
				filteredCandidates += len(planned) - remaining
				planned = planned[:remaining]
			}
		}
		if plannerExecution.active() && includePlanned && len(planned) > 0 {
			if consumeErr := plannerExecution.ledger.RecordChannelAccepted(pass, channel, len(planned), time.Now().UTC()); consumeErr != nil {
				filteredCandidates += len(planned)
				if includeBaseline && len(visible) > 0 {
					baselineChannelCandidates = append(baselineChannelCandidates, FusionChannelCandidates{Channel: channel, Candidates: visible})
				}
				return consumeErr
			}
		}
		metadataHits := planned
		if includeBaseline {
			metadataHits = visible
		}
		for _, hit := range metadataHits {
			if existing, exists := diversityMetadata[hit.Memory.ID]; !exists {
				diversityMetadata[hit.Memory.ID] = hit
			} else {
				diversityMetadata[hit.Memory.ID] = mergeDiversityScoredMemory(existing, hit)
			}
		}
		if includePlanned && len(planned) > 0 {
			aggregateCandidates += len(planned)
			channelCandidates = append(channelCandidates, FusionChannelCandidates{Channel: channel, Candidates: planned})
		}
		if includeBaseline && len(visible) > 0 {
			baselineChannelCandidates = append(baselineChannelCandidates, FusionChannelCandidates{Channel: channel, Candidates: visible})
		}
		return nil
	}

	// Process each signal across all physical channels before moving to the
	// next signal. This reserves aggregate candidate capacity for the immutable
	// original query even when a derived lexical channel is noisy.
	for signalIndex, recallInput := range recallInputs {
		if s.lexical != nil && plannerExecution.channelEnabled(FusionChannelLexical) {
			callInput, execute, plannedChannel, reserveErr := reserveRecall(FusionChannelLexical, recallInput)
			if reserveErr != nil {
				plannerFailed = plannerExecution.active()
				execute = false
			}
			if execute {
				callCtx, cancel := ctx, func() {}
				if plannerExecution.active() {
					var ok bool
					callCtx, cancel, ok = plannerExecution.callContext(ctx)
					if !ok {
						plannerFailed = true
						execute = false
					}
				}
				if !execute {
					cancel()
					continue
				}
				hits, recallErr := s.lexical.SearchLexical(callCtx, callInput)
				cancel()
				if recallErr != nil {
					if signalIndex == 0 {
						return SearchResult{}, recallErr
					}
					plannerFailed = plannerExecution.active()
					if input.IncludeFeedbackDiagnostics {
						fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "query_analysis", Status: "derived_signal_unavailable", Reason: "derived lexical recall failed closed"})
					}
				} else {
					channelAvailability[FusionChannelLexical] = fusionChannelAvailable
					if filterErr := filterChannel(1, FusionChannelLexical, hits, plannedChannel, plannerExecution.active()); filterErr != nil {
						plannerFailed = plannerExecution.active()
					}
				}
			}
		}
		if s.semantic != nil && plannerExecution.channelEnabled(FusionChannelSemantic) {
			callInput, execute, plannedChannel, reserveErr := reserveRecall(FusionChannelSemantic, recallInput)
			if reserveErr != nil {
				plannerFailed = plannerExecution.active()
				execute = false
			}
			if execute {
				callCtx, cancel := ctx, func() {}
				if plannerExecution.active() {
					var ok bool
					callCtx, cancel, ok = plannerExecution.callContext(ctx)
					if !ok {
						plannerFailed = true
						execute = false
					}
				}
				if !execute {
					cancel()
					continue
				}
				hits, recallErr := s.semantic.SearchSemantic(callCtx, callInput)
				cancel()
				if recallErr != nil {
					if plannerExecution.active() {
						plannerFailed = true
					}
					if input.IncludeFeedbackDiagnostics {
						fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "fusion", Status: "optional_channel_unavailable", Reason: "semantic recall failed closed"})
					}
				} else {
					channelAvailability[FusionChannelSemantic] = fusionChannelAvailable
					if filterErr := filterChannel(1, FusionChannelSemantic, hits, plannedChannel, plannerExecution.active()); filterErr != nil {
						plannerFailed = plannerExecution.active()
					}
				}
			}
		}
		if input.IncludeRelations && s.relations != nil && plannerExecution.channelEnabled(FusionChannelRelation) {
			callInput, execute, plannedChannel, reserveErr := reserveRecall(FusionChannelRelation, recallInput)
			if reserveErr != nil {
				plannerFailed = plannerExecution.active()
				execute = false
			}
			if execute {
				callCtx, cancel := ctx, func() {}
				if plannerExecution.active() {
					var ok bool
					callCtx, cancel, ok = plannerExecution.callContext(ctx)
					if !ok {
						plannerFailed = true
						execute = false
					}
				}
				if !execute {
					cancel()
					continue
				}
				hits, recallErr := s.relations.SearchRelations(callCtx, callInput)
				cancel()
				if recallErr != nil {
					if plannerExecution.active() {
						plannerFailed = true
					}
					requiredChannelUnavailable = true
					if input.IncludeFeedbackDiagnostics {
						fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "fusion", Status: "optional_channel_unavailable", Reason: "relation recall failed closed"})
					}
				} else {
					channelAvailability[FusionChannelRelation] = fusionChannelAvailable
					// Graph traversal is an optional relation-channel enrichment. It
					// is gated by an active exact-scope plan; any failure omits only
					// graph evidence and retains the already-approved relation hits.
					if (plannerExecution.active() || plannerExecution.shadow() || plannerExecution.stage == memory.RetrievalPlannerRolloutStageDiagnosticsOnly) && plannerExecution.plan.GraphTraversalPolicy != nil && plannerExecution.plan.GraphTraversalLimits.MaxHops > 0 && s.graphTraversal != nil {
						seedIDs := make([]string, 0, len(hits))
						for _, hit := range hits {
							if hit.Memory.Scope.Normalized() == input.Scope.Normalized() && hit.Memory.State == memory.MemoryStateActive && temporalConstraint.Matches(hit.Memory.TemporalValidity, evaluationInstant) {
								seedIDs = append(seedIDs, hit.Memory.ID)
							}
						}
						if len(seedIDs) > plannerExecution.plan.GraphTraversalLimits.MaxSeeds {
							seedIDs = seedIDs[:plannerExecution.plan.GraphTraversalLimits.MaxSeeds]
						}
						if len(seedIDs) > 0 {
							graphCtx, graphCancel := context.WithTimeout(ctx, plannerExecution.plan.GraphTraversalLimits.MaxElapsed)
							graphInput := GraphTraversalInput{Scope: input.Scope, SeedMemoryIDs: seedIDs, TemporalConstraint: temporalConstraint, TimeFrom: input.TimeFrom, TimeTo: input.TimeTo, TopK: callInput.TopK, Limits: plannerExecution.plan.GraphTraversalLimits}
							var graphResult GraphTraversalResult
							var graphErr error
							if outcomeSearcher, ok := s.graphTraversal.(GraphTraversalOutcomeSearcher); ok {
								graphResult, graphErr = outcomeSearcher.ExpandGraphResult(graphCtx, graphInput)
							} else {
								graphResult.Candidates, graphErr = s.graphTraversal.ExpandGraph(graphCtx, graphInput)
								graphResult = graphResult.Normalize()
							}
							graphCancel()
							if graphErr == nil {
								graphTraversalDiagnostics = &GraphTraversalDiagnostics{PolicyVersion: plannerExecution.plan.GraphTraversalPolicy.PolicyVersion, HopBucket: graphHopBucket(plannerExecution.plan.GraphTraversalLimits.MaxHops), PathBucket: graphPathBucket(len(graphResult.Candidates), plannerExecution.plan.GraphTraversalLimits), Truncation: string(graphResult.Truncation), Failure: "none"}
							}
							if graphErr == nil && plannerExecution.active() {
								for _, path := range graphResult.Candidates {
									path.Proof.PolicyVersion = plannerExecution.plan.GraphTraversalPolicy.PolicyVersion
									if path.Proof.Validate() != nil {
										continue
									}
									candidate := ScoredMemory{Memory: path.Memory, RelationScore: path.RelationConfidence}
									hits = append(hits, candidate)
								}
							}
							if graphErr != nil {
								graphTraversalDiagnostics = &GraphTraversalDiagnostics{PolicyVersion: plannerExecution.plan.GraphTraversalPolicy.PolicyVersion, HopBucket: graphHopBucket(plannerExecution.plan.GraphTraversalLimits.MaxHops), PathBucket: "0", Truncation: string(graphResult.Truncation), Failure: graphFailureCategory(graphErr)}
							}
						}
					}
					if filterErr := filterChannel(1, FusionChannelRelation, hits, plannedChannel, plannerExecution.active()); filterErr != nil {
						plannerFailed = plannerExecution.active()
					}
				}
			}
		}
	}

	// Derived chunks are strictly opt-in. In shadow mode we execute the search
	// only to produce diagnostics for explicitly requested evaluation callers;
	// ordinary responses remain byte-for-byte compatible with canonical retrieval.
	if s.chunks != nil && s.chunkRollout != memory.ChunkRolloutModeDefaultOff && plannerExecution.channelEnabled(FusionChannelChunk) {
		allChunkCandidates := make([]ChunkCandidate, 0)
		var chunkErr error
		chunkPlannedChannel := !plannerExecution.active() || plannerExecution.declaresChannel(FusionChannelChunk)
		for _, recallInput := range recallInputs {
			if analysisLimits.MaxAggregateCandidates > 0 && aggregateCandidates >= analysisLimits.MaxAggregateCandidates {
				break
			}
			callInput, execute, _, reserveErr := reserveRecall(FusionChannelChunk, recallInput)
			if reserveErr != nil {
				plannerFailed = plannerExecution.active()
				execute = false
			}
			if !execute {
				continue
			}
			callCtx, cancel := ctx, func() {}
			if plannerExecution.active() {
				var ok bool
				callCtx, cancel, ok = plannerExecution.callContext(ctx)
				if !ok {
					plannerFailed = true
					cancel()
					break
				}
			}
			chunkCandidates, err := s.chunks.SearchChunks(callCtx, ChunkSearchInput{Scope: callInput.Scope, Query: callInput.Query, QueryEmbedding: callInput.QueryEmbedding, Classes: callInput.Classes, TopK: callInput.TopK})
			cancel()
			if err != nil {
				if plannerExecution.active() {
					plannerFailed = true
				}
				chunkErr = err
				continue
			}
			if analysisLimits.MaxCandidatesPerSignal > 0 && len(chunkCandidates) > analysisLimits.MaxCandidatesPerSignal {
				chunkCandidates = chunkCandidates[:analysisLimits.MaxCandidatesPerSignal]
			}
			if analysisLimits.MaxAggregateCandidates > 0 {
				remaining := analysisLimits.MaxAggregateCandidates - aggregateCandidates - len(allChunkCandidates)
				if remaining <= 0 {
					break
				}
				if len(chunkCandidates) > remaining {
					chunkCandidates = chunkCandidates[:remaining]
				}
			}
			allChunkCandidates = append(allChunkCandidates, chunkCandidates...)
		}
		chunkCandidates := allChunkCandidates
		if chunkErr != nil {
			requiredChannelUnavailable = true
			if s.chunkRollout == memory.ChunkRolloutModeActive {
				if input.IncludeFeedbackDiagnostics {
					fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "fusion", Status: "optional_channel_unavailable", Reason: "chunk recall failed closed"})
				}
			} else if input.IncludeFeedbackDiagnostics {
				fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "chunk_retrieval", Status: "shadow_unavailable", Reason: "chunk candidate evaluation failed closed"})
			}
		} else {
			if s.chunkRollout == memory.ChunkRolloutModeActive {
				channelAvailability[FusionChannelChunk] = fusionChannelAvailable
			}
			accepted := 0
			omitted := 0
			chunkHits := make([]ScoredMemory, 0, len(chunkCandidates))
			for _, candidate := range chunkCandidates {
				if err := candidate.Chunk.Validate(); err != nil || candidate.Chunk.Scope.Normalized() != input.Scope.Normalized() || candidate.Parent.Scope.Normalized() != input.Scope.Normalized() || candidate.Parent.State != memory.MemoryStateActive {
					omitted++
					continue
				}
				if !matchClassFilter(candidate.Chunk.Class, input.Classes) || (!input.IncludeSummaries && candidate.Chunk.Class == memory.MemoryClassSummary) || (!input.IncludeRelations && candidate.Chunk.Class == memory.MemoryClassRelation) {
					omitted++
					continue
				}
				accepted++
				if s.chunkRollout == memory.ChunkRolloutModeActive {
					// Parent memory is the public result. Keep chunk metadata private.
					hit := ScoredMemory{Memory: candidate.Parent, LexicalScore: candidate.Score.Lexical, SemanticScore: candidate.Score.Semantic, RelationScore: candidate.Score.Relation}
					chunkHits = append(chunkHits, hit)
					if len(candidate.Citations) > 0 {
						chunkCitationMap[candidate.Parent.ID] = append(chunkCitationMap[candidate.Parent.ID], candidate.Citations...)
					}
					chunkCopy := candidate.Chunk
					chunkByMemoryID[candidate.Parent.ID] = &chunkCopy
				}
			}
			if len(chunkHits) > 0 {
				if filterErr := filterChannel(1, FusionChannelChunk, chunkHits, chunkPlannedChannel, plannerExecution.active()); filterErr != nil {
					plannerFailed = plannerExecution.active()
				}
			}
			if input.IncludeFeedbackDiagnostics && s.chunkRollout == memory.ChunkRolloutModeShadow {
				fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "chunk_retrieval", Status: "shadow_evaluated", Available: len(chunkCandidates), Included: accepted, Omitted: omitted})
			}
		}
	}

	if plannerExecution.active() {
		minimumVisible := 1
		if plannerExecution.plan.FollowUp.Enabled {
			minimumVisible = plannerExecution.plan.FollowUp.MinimumVisible
		}
		plannerEvidence, err = assessPlannedEvidence(input.Scope, fusionStrategy, channelCandidates, diversityMetadata, EvidenceAssessmentInput{
			Pass: 1, MinimumVisible: minimumVisible, RecalledCandidates: recalledCandidates,
			FilteredCandidates: filteredCandidates, RequiredChannelUnavailable: requiredChannelUnavailable,
			RemainingCandidates: plannerExecution.ledger.RemainingForRetrieval(),
		})
		plannerEvidenceSet = err == nil
		passOneCandidates := aggregateCandidates
		passOneOrder, orderErr := plannedCandidateOrder(input.Scope, fusionStrategy, channelCandidates, diversityMetadata, 0)
		if plannerEvidenceSet && orderErr == nil {
			plannerPassObservations = append(plannerPassObservations, RetrievalPassObservation{Pass: 1, CandidateCount: passOneCandidates, VisibleMemoryIDs: passOneOrder, Evidence: plannerEvidence, Latency: time.Since(plannerExecution.startedAt)})
		}
		if plannerEvidenceSet && plannerExecution.plan.FollowUp.Enabled && plannerEvidence.FollowUpEligible {
			followUpStarted := time.Now()
			followUpExecuted, followUpErr := s.executeRetrievalFollowUp(ctx, input, plannerExecution, func(pass int, channel FusionChannel, hits []ScoredMemory) error {
				return filterChannel(pass, channel, hits, true, false)
			})
			// Follow-up is optional. Accounting or provider failure preserves the
			// successful first pass and never starts a new baseline provider pass.
			if followUpErr != nil {
				followUpExecuted = false
			}
			if followUpExecuted {
				plannerPassCount = 2
				plannerEvidence, err = assessPlannedEvidence(input.Scope, fusionStrategy, channelCandidates, diversityMetadata, EvidenceAssessmentInput{
					Pass: 2, MinimumVisible: minimumVisible, RecalledCandidates: recalledCandidates,
					FilteredCandidates: filteredCandidates, RequiredChannelUnavailable: requiredChannelUnavailable,
					RemainingCandidates: plannerExecution.ledger.RemainingForRetrieval(),
				})
				plannerEvidenceSet = err == nil
				passTwoOrder, orderErr := plannedCandidateOrder(input.Scope, fusionStrategy, channelCandidates, diversityMetadata, 0)
				if plannerEvidenceSet && orderErr == nil {
					plannerPassObservations = append(plannerPassObservations, RetrievalPassObservation{Pass: 2, CandidateCount: aggregateCandidates - passOneCandidates, VisibleMemoryIDs: passTwoOrder, Evidence: plannerEvidence, Latency: time.Since(followUpStarted)})
				}
			}
		}
		plannerCandidateCount = aggregateCandidates
	} else if plannerExecution.shadow() {
		plannerEvidence, plannerEvidenceSet, plannerCandidateCount, plannerChannelAvailability, shadowPlannedOrder = s.executeShadowRetrievalComparison(ctx, input, plannerExecution)
		if plannerEvidenceSet {
			plannerPassObservations = append(plannerPassObservations, RetrievalPassObservation{Pass: 1, CandidateCount: plannerCandidateCount, VisibleMemoryIDs: append([]string(nil), shadowPlannedOrder...), Evidence: plannerEvidence, Latency: time.Since(plannerExecution.startedAt)})
		}
	}
	if plannerFailed {
		channelCandidates = baselineChannelCandidates
		fusionStrategy = baselineFusionStrategy
		plannerExecution = retrievalPlannerExecution{stage: memory.RetrievalPlannerRolloutStageBaseline}
		plannerEvidence = EvidenceAssessment{}
		plannerEvidenceSet = false
		plannerPassObservations = nil
	}

	channelCandidates = mergeSignalChannels(channelCandidates)
	fused, err := FuseCandidates(fusionStrategy, channelCandidates)
	if err != nil {
		return SearchResult{}, err
	}
	if input.IncludeFeedbackDiagnostics {
		fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "fusion", Status: "strategy_applied", Reason: string(fusionStrategy.Name) + ":" + fusionStrategy.Version, Available: len(channelCandidates), Included: len(fused)})
	}
	s.recordFusionTelemetry(ctx, fusionStrategy, channelCandidates)
	// Identity/lineage deduplication is always applied after stable fusion. An
	// active diversity rollout may additionally perform bounded semantic
	// selection; malformed or absent policy configuration safely retains this
	// deterministic deduplicated baseline.
	diversityPolicy, diversityEnabled := diversityPolicyFromRankingRollout(activeRankingPolicy)
	diversityCandidates := make([]DiversityCandidate, 0, len(fused))
	for _, candidate := range fused {
		metadata := diversityMetadata[candidate.Memory.ID]
		diversityCandidates = append(diversityCandidates, DiversityCandidate{FusedCandidate: candidate, SourceEventID: metadata.SourceEventID, ParentMemoryID: metadata.ParentMemoryID, EmbeddingRevision: metadata.EmbeddingRevision, EmbeddingRevisionActive: metadata.EmbeddingRevisionActive, Embedding: append([]float32(nil), metadata.Embedding...), SessionID: metadata.SessionID, EntityKey: metadata.EntityKey, TimeSlice: metadata.TimeSlice, Citations: append([]Citation(nil), metadata.Citations...)})
	}
	var diversitySelection DiversitySelection
	if diversityEnabled {
		diversitySelection, err = SelectDiverseCandidates(input.Scope, diversityPolicy, diversityCandidates, len(diversityCandidates))
	} else {
		diversitySelection = DeduplicateDiversityCandidates(input.Scope, diversityCandidates)
	}
	if err != nil {
		return SearchResult{}, err
	}
	fused = make([]FusedCandidate, 0, len(diversitySelection.Candidates))
	for _, candidate := range diversitySelection.Candidates {
		fused = append(fused, candidate.FusedCandidate)
	}
	if input.IncludeFeedbackDiagnostics && diversityEnabled {
		fusionDiagnostics = append(fusionDiagnostics, ContextDiagnostic{Section: "diversity", Status: "policy_applied", Reason: diversityPolicy.Name + ":" + diversityPolicy.Version, Included: len(diversitySelection.Candidates), Omitted: diversitySelection.Dispositions.Duplicate + diversitySelection.Dispositions.Diversity})
	}
	scored := make([]SearchHit, 0, len(fused))
	memoryIDs := make([]string, 0, len(fused))
	for candidateIndex, candidate := range fused {
		memoryIDs = append(memoryIDs, candidate.Memory.ID)
		searchHit := SearchHit{
			Memory: candidate.Memory,
			Score: ScoreBreakdown{
				Overall:  candidate.Score,
				Lexical:  candidate.ChannelScores.Lexical,
				Semantic: candidate.ChannelScores.Semantic,
				Relation: candidate.ChannelScores.Relation,
			},
			Citations: append([]Citation(nil), diversitySelection.Candidates[candidateIndex].Citations...),
		}
		if chunk := chunkByMemoryID[candidate.Memory.ID]; chunk != nil {
			searchHit.Chunk = chunk
		}
		scored = append(scored, searchHit)
	}

	diagnostics, err := s.applyUsefulnessFeedbackSignals(ctx, input, scored)
	if err != nil {
		return SearchResult{}, err
	}
	rerankerObservation := RetrievalRerankerObservation{Safe: true}
	if rerankDiagnostics, observation := s.applyOptionalReranker(ctx, input, scored, activeRankingPolicy, plannerExecution); len(rerankDiagnostics) > 0 {
		diagnostics = append(diagnostics, rerankDiagnostics...)
		rerankerObservation = observation
	}

	if !input.rankingPolicyDisabled {
		adjusted, policyDiagnostics, err := s.applyRankingRolloutPolicy(ctx, input.Scope, surface, input.FeedbackAwareRanking, scored, activeRankingPolicy)
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
	if plannerExecution.shadow() && plannerEvidenceSet {
		baselineOrder := make([]string, 0, len(scored))
		for _, hit := range scored {
			baselineOrder = append(baselineOrder, hit.Memory.ID)
		}
		plannerChangedRankCount = changedRankCount(baselineOrder, shadowPlannedOrder)
		plannerChangedRankObserved = true
	}
	shadowCandidates = s.observeQueryAnalysisShadow(ctx, input, queryAnalysisShadowInputs, queryAnalysisShadowLimits)

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
			scored[i].Citations = append(scored[i].Citations, chunkCitationMap[scored[i].Memory.ID]...)
		}
	} else {
		for i := range scored {
			scored[i].Citations = append(scored[i].Citations, chunkCitationMap[scored[i].Memory.ID]...)
		}
	}

	if len(fusionDiagnostics) > 0 {
		for i := range fusionDiagnostics {
			if fusionDiagnostics[i].Section == "query_analysis" {
				if fusionDiagnostics[i].RolloutStage == string(memory.QueryAnalysisRolloutStageShadow) {
					fusionDiagnostics[i].Status = "shadow_evaluated"
					fusionDiagnostics[i].CandidateCount = shadowCandidates
				} else {
					fusionDiagnostics[i].CandidateCount = aggregateCandidates
				}
			}
		}
		diagnostics = append(diagnostics, fusionDiagnostics...)
	}
	if !input.queryAnalysisDiagnosticsAuthorized {
		diagnostics = filterQueryAnalysisDiagnostics(diagnostics)
	}
	if plannerExecution.plan != nil && !plannerEvidenceSet {
		plannerCandidateCount = aggregateCandidates
		plannerEvidence, err = AssessRetrievalEvidence(EvidenceAssessmentInput{
			Pass: 1, VisibleCandidates: len(scored), MinimumVisible: 1,
			RecalledCandidates: recalledCandidates, FilteredCandidates: filteredCandidates,
			DuplicateCandidates: maxInt(aggregateCandidates-len(scored), 0),
			RemainingCandidates: plannerExecution.ledgerRemaining(),
		})
		plannerEvidenceSet = err == nil
	}
	if plannerExecution.active() {
		plannerChannelAvailability = plannerObservedChannels(plannerExecution.plan, channelAvailability)
	}
	plannerElapsed := plannerDiagnosticElapsed(plannerExecution.startedAt, time.Now())
	var plannerDiagnostics []RetrievalPlannerDiagnostics
	if input.retrievalPlannerDiagnosticsAuthorized && plannerExecution.plan != nil && plannerEvidenceSet {
		plannerDiagnostics = s.retrievalPlannerDiagnostics(ctx, RetrievalPlannerDiagnosticsInput{Plan: *plannerExecution.plan, RolloutStage: plannerExecution.stage, Evidence: plannerEvidence, PassCount: plannerPassCount, CandidateCount: plannerCandidateCount, Elapsed: plannerElapsed, ChannelAvailability: plannerChannelAvailability, ChangedRankCount: plannerChangedRankCount, ChangedRankObserved: plannerChangedRankObserved, TemporalOmissions: temporalOmissions, GraphTraversal: graphTraversalDiagnostics})
	}
	s.recordRetrievalPlannerTelemetry(ctx, plannerExecution, plannerEvidence, plannerEvidenceSet, plannerPassCount, plannerCandidateCount, plannerElapsed, plannerChannelAvailability, plannerChangedRankCount, plannerChangedRankObserved, graphTraversalDiagnostics)
	result = SearchResult{
		Hits:                      scored,
		Diagnostics:               diagnostics,
		plannerDiagnostics:        plannerDiagnostics,
		fusionChannelAvailability: channelAvailability,
		fusionStrategy:            fusionStrategy,
		retrievalPlan: func() *RetrievalPlan {
			if plannerExecution.active() {
				return plannerExecution.plan
			}
			return nil
		}(),
		retrievalPassObservations: plannerPassObservations,
		rerankerObservation:       rerankerObservation,
		temporalOmissions:         temporalOmissions,
		Temporal:                  temporalSelectionSummary(input.TemporalConstraint, temporalOmissions),
	}
	return result, nil
}

func graphHopBucket(hops int) string {
	switch hops {
	case 0:
		return "zero"
	case 1:
		return "one"
	case 2:
		return "two"
	default:
		return "three"
	}
}

func graphPathBucket(count int, limits GraphTraversalLimits) string {
	if count <= 0 {
		return "0"
	}
	if count <= 10 {
		return "1_10"
	}
	if count <= 50 {
		return "11_50"
	}
	return "51_plus"
}

func graphFailureCategory(err error) string {
	if err == nil {
		return "none"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authorization"):
		return "authorization"
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline"):
		return "timeout"
	case strings.Contains(message, "policy"):
		return "policy_rejected"
	case strings.Contains(message, "unavailable"), strings.Contains(message, "no rows"):
		return "unavailable"
	default:
		return "repository"
	}
}

// temporalSelectionSummary projects the request's valid-time selection into the
// bounded response block. It is nil for an ordinary current search that removed
// nothing, which is what keeps the ordinary response shape unchanged: the key
// only appears when the caller asked for history or when the selector actually
// excluded something.
func temporalSelectionSummary(constraint memory.TemporalConstraint, omissions TemporalOmissionReport) *TemporalSelection {
	explicit := constraint.Mode != "" && constraint.Mode != memory.TemporalSelectionCurrent
	if !explicit && omissions.Total == 0 {
		return nil
	}
	mode := string(constraint.Mode)
	if mode == "" {
		mode = string(memory.TemporalSelectionCurrent)
	}
	return &TemporalSelection{Mode: mode, Omitted: omissions.Total}
}

func (s *Service) observeQueryAnalysisShadow(ctx context.Context, original SearchInput, inputs []SearchInput, limits QueryAnalysisLimits) int {
	if len(inputs) == 0 {
		return 0
	}
	perSignal := limits.MaxCandidatesPerSignal
	aggregate := limits.MaxAggregateCandidates
	if perSignal <= 0 || aggregate <= 0 {
		defaults := DefaultQueryAnalysisLimits()
		if perSignal <= 0 {
			perSignal = defaults.MaxCandidatesPerSignal
		}
		if aggregate <= 0 {
			aggregate = defaults.MaxAggregateCandidates
		}
	}
	observed := 0
	record := func(hits []ScoredMemory) {
		if len(hits) > perSignal {
			hits = hits[:perSignal]
		}
		for _, hit := range hits {
			if observed >= aggregate {
				return
			}
			if hit.Memory.Scope.Normalized() != original.Scope.Normalized() || hit.Memory.State != memory.MemoryStateActive || !matchClassFilter(hit.Memory.Class, original.Classes) || (!original.IncludeSummaries && hit.Memory.Class == memory.MemoryClassSummary) || (!original.IncludeRelations && hit.Memory.Class == memory.MemoryClassRelation) {
				continue
			}
			observed++
		}
	}
	for _, shadowInput := range inputs {
		if observed >= aggregate {
			break
		}
		remaining := aggregate - observed
		shadowInput.TopK = minInt(perSignal, remaining)
		if s.lexical != nil {
			if hits, err := s.lexical.SearchLexical(ctx, shadowInput); err == nil {
				record(hits)
			}
		}
		if observed >= aggregate {
			break
		}
		if s.semantic != nil {
			if hits, err := s.semantic.SearchSemantic(ctx, shadowInput); err == nil {
				record(hits)
			}
		}
		if observed >= aggregate {
			break
		}
		if original.IncludeRelations && s.relations != nil {
			if hits, err := s.relations.SearchRelations(ctx, shadowInput); err == nil {
				record(hits)
			}
		}
		if observed >= aggregate {
			break
		}
		if s.chunks != nil && s.chunkRollout != memory.ChunkRolloutModeDefaultOff {
			chunks, err := s.chunks.SearchChunks(ctx, ChunkSearchInput{Scope: shadowInput.Scope, Query: shadowInput.Query, QueryEmbedding: shadowInput.QueryEmbedding, Classes: shadowInput.Classes, TopK: shadowInput.TopK})
			if err == nil {
				hits := make([]ScoredMemory, 0, len(chunks))
				for _, candidate := range chunks {
					hits = append(hits, ScoredMemory{Memory: candidate.Parent})
				}
				record(hits)
			}
		}
	}
	return observed
}

type retrievalPlannerMetricObserver interface {
	RecordRetrievalPlanner(context.Context, telemetry.RetrievalPlannerEvent)
}

type retrievalPlannerChannelMetricObserver interface {
	RecordRetrievalPlannerChannel(context.Context, telemetry.RetrievalPlannerChannelEvent)
}

type retrievalPlannerChangedRankMetricObserver interface {
	RecordRetrievalPlannerChangedRank(context.Context, telemetry.RetrievalPlannerChangedRankEvent)
}

type retrievalPlannerDiagnosticMetricObserver interface {
	RecordRetrievalPlannerDiagnostic(context.Context, telemetry.RetrievalPlannerDiagnosticEvent)
}

func (s *Service) recordRetrievalPlannerTelemetry(ctx context.Context, execution retrievalPlannerExecution, evidence EvidenceAssessment, evidenceSet bool, pass, recalled int, elapsed time.Duration, channels []RetrievalPlannerChannelAvailability, changedRanks int, changedRankObserved bool, graph *GraphTraversalDiagnostics) {
	observer, ok := s.observer.(retrievalPlannerMetricObserver)
	if !ok || observer == nil || execution.plan == nil || !evidenceSet {
		return
	}
	reranker := "ineligible"
	if execution.plan.RerankerEligible && execution.plan.RerankerHeadroom > 0 {
		reranker = "eligible"
	}
	plannerVersion := string(execution.plan.Identity.PlannerVersion)
	policyVersion := string(execution.plan.Identity.PolicyVersion)
	family := string(execution.plan.Family)
	stage := string(execution.stage)
	event := telemetry.RetrievalPlannerEvent{
		PlannerVersion: plannerVersion, PolicyVersion: policyVersion,
		Family: family, Stage: stage, Disposition: string(execution.plan.Disposition), Pass: pass,
		BudgetBucket: plannerCandidateBucket(recalled), Evidence: string(evidence.Disposition), Fallback: string(execution.plan.Fallback),
		LatencyBucket: plannerLatencyBucket(elapsed), Reranker: reranker,
	}
	if graph != nil {
		event.GraphHopBucket = graph.HopBucket
		event.GraphPathBucket = graph.PathBucket
		event.GraphTruncation = graph.Truncation
		event.GraphFailure = graph.Failure
	}
	observer.RecordRetrievalPlanner(ctx, event)
	if channelObserver, ok := s.observer.(retrievalPlannerChannelMetricObserver); ok {
		for _, channel := range channels {
			channelObserver.RecordRetrievalPlannerChannel(ctx, telemetry.RetrievalPlannerChannelEvent{
				PlannerVersion: plannerVersion, PolicyVersion: policyVersion, Family: family, Stage: stage,
				Channel: string(channel.Channel), Availability: channel.Availability,
			})
		}
	}
	if changedRankObserved {
		if changedRankObserver, ok := s.observer.(retrievalPlannerChangedRankMetricObserver); ok {
			changedRankObserver.RecordRetrievalPlannerChangedRank(ctx, telemetry.RetrievalPlannerChangedRankEvent{
				PlannerVersion: plannerVersion, PolicyVersion: policyVersion, Family: family, Stage: stage,
				Bucket: plannerChangedRankBucket(changedRanks), Count: changedRanks,
			})
		}
	}
}

func (s *Service) recordRetrievalPlannerDiagnosticFailure(ctx context.Context, failure string) {
	observer, ok := s.observer.(retrievalPlannerDiagnosticMetricObserver)
	if !ok || observer == nil {
		return
	}
	observer.RecordRetrievalPlannerDiagnostic(ctx, telemetry.RetrievalPlannerDiagnosticEvent{FailureCategory: failure})
}

func (s *Service) retrievalPlannerDiagnostics(ctx context.Context, input RetrievalPlannerDiagnosticsInput) []RetrievalPlannerDiagnostics {
	diagnostic, failure := buildRetrievalPlannerDiagnostics(input)
	if failure != "" {
		s.recordRetrievalPlannerDiagnosticFailure(ctx, failure)
		return nil
	}
	return []RetrievalPlannerDiagnostics{diagnostic}
}

func (execution retrievalPlannerExecution) ledgerRemaining() int {
	if execution.ledger == nil {
		return 0
	}
	return execution.ledger.RemainingForRetrieval()
}

func filterQueryAnalysisDiagnostics(diagnostics []ContextDiagnostic) []ContextDiagnostic {
	filtered := diagnostics[:0]
	for _, diagnostic := range diagnostics {
		if diagnostic.Section != "query_analysis" {
			filtered = append(filtered, diagnostic)
		}
	}
	return filtered
}

func (s *Service) canExecuteRetrievalPlan(input SearchInput, plan RetrievalPlan) bool {
	for _, channel := range plan.Channels {
		switch channel {
		case FusionChannelLexical:
			if s.lexical == nil {
				return false
			}
		case FusionChannelSemantic:
			if s.semantic == nil {
				return false
			}
		case FusionChannelRelation:
			if s.relations == nil || !input.IncludeRelations {
				return false
			}
		case FusionChannelChunk:
			if s.chunks == nil || s.chunkRollout != memory.ChunkRolloutModeActive {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (s *Service) canRetainPlannerBaseline(input SearchInput, plan RetrievalPlan) bool {
	required := make(map[FusionChannel]bool, 4)
	if s.lexical != nil {
		required[FusionChannelLexical] = true
	}
	if s.semantic != nil {
		required[FusionChannelSemantic] = true
	}
	if input.IncludeRelations && s.relations != nil {
		required[FusionChannelRelation] = true
	}
	if s.chunks != nil && s.chunkRollout == memory.ChunkRolloutModeActive {
		required[FusionChannelChunk] = true
	}
	planned := make(map[FusionChannel]bool, len(plan.Channels))
	for _, channel := range plan.Channels {
		planned[channel] = true
	}
	for channel := range required {
		if !planned[channel] && plan.FallbackChannelCandidates[channel] <= 0 {
			return false
		}
	}
	for channel := range plan.FallbackChannelCandidates {
		if !required[channel] || planned[channel] {
			return false
		}
	}
	return true
}

func assessPlannedEvidence(scope memory.Scope, strategy FusionStrategy, channels []FusionChannelCandidates, metadata map[string]ScoredMemory, input EvidenceAssessmentInput) (EvidenceAssessment, error) {
	merged := mergeSignalChannels(channels)
	fused, err := FuseCandidates(strategy, merged)
	if err != nil {
		return EvidenceAssessment{}, err
	}
	candidates := make([]DiversityCandidate, 0, len(fused))
	for _, candidate := range fused {
		item := metadata[candidate.Memory.ID]
		candidates = append(candidates, DiversityCandidate{FusedCandidate: candidate, SourceEventID: item.SourceEventID, ParentMemoryID: item.ParentMemoryID, EmbeddingRevision: item.EmbeddingRevision, EmbeddingRevisionActive: item.EmbeddingRevisionActive})
	}
	selection := DeduplicateDiversityCandidates(scope, candidates)
	input.VisibleCandidates = len(selection.Candidates)
	accepted := input.RecalledCandidates - input.FilteredCandidates
	input.DuplicateCandidates = maxInt(accepted-input.VisibleCandidates, 0)
	return AssessRetrievalEvidence(input)
}

func (s *Service) executeRetrievalFollowUp(ctx context.Context, input SearchInput, execution retrievalPlannerExecution, filter func(int, FusionChannel, []ScoredMemory) error) (bool, error) {
	if !execution.active() || !execution.plan.FollowUp.Enabled || len(execution.plan.FollowUp.Channels) == 0 {
		return false, nil
	}
	executed := false
	remainingAllocation := execution.plan.FollowUp.CandidateAllocation
	for _, channel := range execution.plan.FollowUp.Channels {
		if !execution.channelEnabled(channel) || execution.ledger.RemainingForRetrieval() <= 0 || remainingAllocation <= 0 {
			continue
		}
		limit := minInt(execution.ledger.UnusedChannelAllocation(channel), remainingAllocation)
		limit = minInt(limit, execution.ledger.RemainingForRetrieval())
		if limit <= 0 {
			continue
		}
		followUpInput := input
		followUpInput.TopK = limit
		callCtx, cancel, ok := execution.callContext(ctx)
		if !ok {
			break
		}
		if err := execution.ledger.ReserveChannelRequest(2, channel, limit, time.Now().UTC()); err != nil {
			cancel()
			break
		}
		remainingAllocation -= limit
		executed = true
		var hits []ScoredMemory
		var err error
		switch channel {
		case FusionChannelLexical:
			if s.lexical != nil {
				hits, err = s.lexical.SearchLexical(callCtx, followUpInput)
			}
		case FusionChannelSemantic:
			if s.semantic != nil {
				hits, err = s.semantic.SearchSemantic(callCtx, followUpInput)
			}
		case FusionChannelRelation:
			if s.relations != nil && input.IncludeRelations {
				hits, err = s.relations.SearchRelations(callCtx, followUpInput)
			}
		}
		cancel()
		if err != nil || len(hits) == 0 {
			continue
		}
		if len(hits) > limit {
			hits = hits[:limit]
		}
		if filterErr := filter(2, channel, hits); filterErr != nil {
			return executed, filterErr
		}
	}
	return executed, nil
}

func (s *Service) executeShadowRetrievalComparison(ctx context.Context, input SearchInput, execution retrievalPlannerExecution) (EvidenceAssessment, bool, int, []RetrievalPlannerChannelAvailability, []string) {
	if !execution.shadow() || !s.canExecuteRetrievalPlan(input, *execution.plan) {
		return EvidenceAssessment{}, false, 0, plannerNotEvaluatedChannels(execution.plan), nil
	}
	channels := make([]FusionChannelCandidates, 0, len(execution.plan.Channels))
	metadata := make(map[string]ScoredMemory)
	recalled, filtered, accepted := 0, 0, 0
	unavailable := false
	availability := make([]RetrievalPlannerChannelAvailability, 0, len(execution.plan.Channels))
	for _, channel := range execution.plan.Channels {
		channelObservation := RetrievalPlannerChannelAvailability{Channel: channel, Availability: "available"}
		limit := minInt(execution.ledger.UnusedChannelAllocation(channel), execution.ledger.RemainingForRetrieval())
		if limit <= 0 {
			continue
		}
		callInput := input
		callInput.TopK = limit
		callCtx, cancel, ok := execution.callContext(ctx)
		if !ok {
			unavailable = true
			channelObservation.Availability = "unavailable"
			availability = append(availability, channelObservation)
			break
		}
		if err := execution.ledger.ReserveChannelRequest(1, channel, limit, time.Now().UTC()); err != nil {
			cancel()
			unavailable = true
			channelObservation.Availability = "unavailable"
			availability = append(availability, channelObservation)
			continue
		}
		var hits []ScoredMemory
		var callErr error
		switch channel {
		case FusionChannelLexical:
			hits, callErr = s.lexical.SearchLexical(callCtx, callInput)
		case FusionChannelSemantic:
			hits, callErr = s.semantic.SearchSemantic(callCtx, callInput)
		case FusionChannelRelation:
			hits, callErr = s.relations.SearchRelations(callCtx, callInput)
		case FusionChannelChunk:
			chunkHits, err := s.chunks.SearchChunks(callCtx, ChunkSearchInput{Scope: callInput.Scope, Query: callInput.Query, QueryEmbedding: callInput.QueryEmbedding, Classes: callInput.Classes, TopK: limit})
			callErr = err
			for _, candidate := range chunkHits {
				hits = append(hits, ScoredMemory{Memory: candidate.Parent, LexicalScore: candidate.Score.Lexical, SemanticScore: candidate.Score.Semantic, RelationScore: candidate.Score.Relation})
			}
		}
		cancel()
		if callErr != nil {
			unavailable = true
			channelObservation.Availability = "unavailable"
			availability = append(availability, channelObservation)
			continue
		}
		recalled += len(hits)
		if len(hits) > limit {
			filtered += len(hits) - limit
			hits = hits[:limit]
		}
		visible := hits[:0]
		for _, hit := range hits {
			if hit.Memory.Scope.Normalized() != input.Scope.Normalized() || hit.Memory.State != memory.MemoryStateActive || !matchClassFilter(hit.Memory.Class, input.Classes) || (!input.IncludeSummaries && hit.Memory.Class == memory.MemoryClassSummary) || (!input.IncludeRelations && hit.Memory.Class == memory.MemoryClassRelation) {
				filtered++
				continue
			}
			visible = append(visible, hit)
			metadata[hit.Memory.ID] = hit
		}
		if len(visible) > 0 {
			if err := execution.ledger.RecordChannelAccepted(1, channel, len(visible), time.Now().UTC()); err != nil {
				unavailable = true
				continue
			}
			accepted += len(visible)
			channels = append(channels, FusionChannelCandidates{Channel: channel, Candidates: visible})
		}
		availability = append(availability, channelObservation)
	}
	availability = completePlannerChannelAvailability(execution.plan, availability, "unavailable")
	minimumVisible := 1
	if execution.plan.FollowUp.Enabled {
		minimumVisible = execution.plan.FollowUp.MinimumVisible
	}
	assessment, err := assessPlannedEvidence(input.Scope, execution.plan.Fusion, channels, metadata, EvidenceAssessmentInput{Pass: 1, MinimumVisible: minimumVisible, RecalledCandidates: recalled, FilteredCandidates: filtered, RequiredChannelUnavailable: unavailable, RemainingCandidates: execution.ledger.RemainingForRetrieval()})
	if err != nil {
		return EvidenceAssessment{}, false, accepted, availability, nil
	}
	plannedOrder, err := plannedCandidateOrder(input.Scope, execution.plan.Fusion, channels, metadata, input.TopK)
	if err != nil {
		return EvidenceAssessment{}, false, accepted, availability, nil
	}
	return assessment, true, accepted, availability, plannedOrder
}

func plannedCandidateOrder(scope memory.Scope, strategy FusionStrategy, channels []FusionChannelCandidates, metadata map[string]ScoredMemory, topK int) ([]string, error) {
	fused, err := FuseCandidates(strategy, mergeSignalChannels(channels))
	if err != nil {
		return nil, err
	}
	candidates := make([]DiversityCandidate, 0, len(fused))
	for _, candidate := range fused {
		item := metadata[candidate.Memory.ID]
		candidates = append(candidates, DiversityCandidate{FusedCandidate: candidate, SourceEventID: item.SourceEventID, ParentMemoryID: item.ParentMemoryID, EmbeddingRevision: item.EmbeddingRevision, EmbeddingRevisionActive: item.EmbeddingRevisionActive})
	}
	selection := DeduplicateDiversityCandidates(scope, candidates)
	if topK > 0 && len(selection.Candidates) > topK {
		selection.Candidates = selection.Candidates[:topK]
	}
	result := make([]string, 0, len(selection.Candidates))
	for _, candidate := range selection.Candidates {
		result = append(result, candidate.Memory.ID)
	}
	return result, nil
}

func changedRankCount(baseline, planned []string) int {
	count, size := 0, maxInt(len(baseline), len(planned))
	for i := 0; i < size; i++ {
		var left, right string
		if i < len(baseline) {
			left = baseline[i]
		}
		if i < len(planned) {
			right = planned[i]
		}
		if left != right {
			count++
		}
	}
	return count
}

func plannerNotEvaluatedChannels(plan *RetrievalPlan) []RetrievalPlannerChannelAvailability {
	if plan == nil {
		return nil
	}
	result := make([]RetrievalPlannerChannelAvailability, 0, len(plan.Channels))
	for _, channel := range plan.Channels {
		result = append(result, RetrievalPlannerChannelAvailability{Channel: channel, Availability: "not_evaluated"})
	}
	return result
}

func plannerObservedChannels(plan *RetrievalPlan, availability map[FusionChannel]fusionChannelAvailability) []RetrievalPlannerChannelAvailability {
	if plan == nil {
		return nil
	}
	result := make([]RetrievalPlannerChannelAvailability, 0, len(plan.Channels))
	for _, channel := range plan.Channels {
		result = append(result, RetrievalPlannerChannelAvailability{Channel: channel, Availability: string(availability[channel])})
	}
	return result
}

func completePlannerChannelAvailability(plan *RetrievalPlan, observations []RetrievalPlannerChannelAvailability, missing string) []RetrievalPlannerChannelAvailability {
	if plan == nil {
		return nil
	}
	byChannel := make(map[FusionChannel]string, len(observations))
	for _, observation := range observations {
		byChannel[observation.Channel] = observation.Availability
	}
	result := make([]RetrievalPlannerChannelAvailability, 0, len(plan.Channels))
	for _, channel := range plan.Channels {
		availability := byChannel[channel]
		if availability == "" {
			availability = missing
		}
		result = append(result, RetrievalPlannerChannelAvailability{Channel: channel, Availability: availability})
	}
	return result
}

// mergeSignalChannels converts per-signal recall streams into one stable rank
// stream per physical channel. Candidate identity is kept once at its earliest
// rank, so repeated aliases cannot add incomparable raw scores.
func mergeSignalChannels(inputs []FusionChannelCandidates) []FusionChannelCandidates {
	order := []FusionChannel{FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk}
	byChannel := make(map[FusionChannel][]ScoredMemory, len(order))
	seen := make(map[FusionChannel]map[string]struct{}, len(order))
	for _, input := range inputs {
		if seen[input.Channel] == nil {
			seen[input.Channel] = make(map[string]struct{})
		}
		for _, candidate := range input.Candidates {
			if _, ok := seen[input.Channel][candidate.Memory.ID]; ok {
				continue
			}
			seen[input.Channel][candidate.Memory.ID] = struct{}{}
			byChannel[input.Channel] = append(byChannel[input.Channel], candidate)
		}
	}
	result := make([]FusionChannelCandidates, 0, len(byChannel))
	for _, channel := range order {
		if candidates := byChannel[channel]; len(candidates) > 0 {
			result = append(result, FusionChannelCandidates{Channel: channel, Candidates: candidates})
		}
	}
	return result
}

// queryRecallInputs returns the original input first and, only for an
// authorized active query-analysis rollout, bounded derived signals. Every
// derived input copies the caller's narrowing constraints verbatim.
func (s *Service) queryRecallInputs(ctx context.Context, input SearchInput, rankingPolicy *memory.RankingRolloutPolicy, surface memory.RankingRolloutSurface) ([]SearchInput, []ContextDiagnostic) {
	original := input
	inputs := []SearchInput{original}
	if s.queryAnalyzer == nil {
		return inputs, nil
	}
	limits := s.queryAnalysisLimits
	if limits.Version == "" {
		limits = DefaultQueryAnalysisLimits()
	}
	if rankingPolicy != nil && rankingPolicy.QueryAnalysis != nil {
		qa := rankingPolicy.QueryAnalysis
		limits = QueryAnalysisLimits{Version: QueryAnalysisLimitsVersionV1, MaxQueryBytes: qa.MaxQueryBytes, MaxHints: qa.MaxHints, MaxSignals: qa.MaxSignals, MaxSubqueries: qa.MaxSubqueries, MaxTermBytes: qa.MaxTermBytes, MaxSubqueryBytes: qa.MaxSubqueryBytes, MaxAnalysisWork: qa.MaxAnalysisWork, MaxCandidatesPerSignal: qa.MaxCandidatesPerSignal, MaxAggregateCandidates: qa.MaxAggregateCandidates, MaxElapsed: qa.MaxElapsed}
	}
	analysisInput := QueryAnalysisInput{AcceptedQuery: input.Query, PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits, Constraints: QueryAnalysisConstraints{Classes: append([]memory.MemoryClass(nil), input.Classes...), TimeFrom: input.TimeFrom, TimeTo: input.TimeTo}}
	analysisLimit := limits.MaxElapsed
	if analysisLimit <= 0 {
		analysisLimit = DefaultQueryAnalysisLimits().MaxElapsed
	}
	type analysisOutcome struct {
		result QueryAnalysisResult
		err    error
	}
	outcomes := make(chan analysisOutcome, 1)
	analysisStarted := time.Now()
	go func() {
		result, err := s.queryAnalyzer.Analyze(analysisInput)
		outcomes <- analysisOutcome{result: result, err: err}
	}()
	var analysis QueryAnalysisResult
	var err error
	select {
	case outcome := <-outcomes:
		analysis, err = outcome.result, outcome.err
	case <-time.After(analysisLimit):
		if input.IncludeFeedbackDiagnostics {
			return inputs, []ContextDiagnostic{{Section: "query_analysis", Status: "original_only", Reason: "analysis exceeded elapsed budget", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: limits.Version, OriginalRetained: true, Fallback: QueryAnalysisFallbackUnavailable, Disposition: QueryAnalysisDispositionOriginalOnly, Categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticUnavailable, Count: 1}}, RolloutStage: string(memory.QueryAnalysisRolloutStageOriginalOnly), ElapsedNS: boundedAnalysisElapsedNS(time.Since(analysisStarted), limits.MaxElapsed)}}
		}
		return inputs, nil
	case <-ctx.Done():
		return inputs, nil
	}
	diagnostics := make([]ContextDiagnostic, 0, 1)
	if err != nil {
		if input.IncludeFeedbackDiagnostics {
			diagnostics = append(diagnostics, ContextDiagnostic{Section: "query_analysis", Status: "original_only", Reason: "analysis unavailable; original query retained", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: limits.Version, OriginalRetained: true, Fallback: QueryAnalysisFallbackUnavailable, Disposition: QueryAnalysisDispositionOriginalOnly, Categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticUnavailable, Count: 1}}, RolloutStage: string(memory.QueryAnalysisRolloutStageOriginalOnly), ElapsedNS: boundedAnalysisElapsedNS(time.Since(analysisStarted), limits.MaxElapsed)})
		}
		return inputs, diagnostics
	}
	if err := analysis.Validate(analysisInput); err != nil {
		if input.IncludeFeedbackDiagnostics {
			diagnostics = append(diagnostics, ContextDiagnostic{Section: "query_analysis", Status: "original_only", Reason: "analysis rejected; original query retained", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: limits.Version, OriginalRetained: true, Fallback: QueryAnalysisFallbackMalformed, Disposition: QueryAnalysisDispositionOriginalOnly, Categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticMalformed, Count: 1}}, RolloutStage: string(memory.QueryAnalysisRolloutStageOriginalOnly), ElapsedNS: boundedAnalysisElapsedNS(time.Since(analysisStarted), limits.MaxElapsed)})
		}
		return inputs, diagnostics
	}
	analysisCopy := analysis
	inputs[0].queryAnalysis = &analysisCopy
	input.queryAnalysis = &analysisCopy
	resolution := memory.ResolveQueryAnalysisRollout(rankingPolicy, memory.ResolveQueryAnalysisRolloutInput{Scope: input.Scope, Surface: surface, SessionID: input.SessionID, UserID: input.UserID, Now: time.Now().UTC()})
	fallback := queryAnalysisFallbackForResult(analysis)
	if !resolution.DerivedSignalsAffectResults {
		if resolution.Stage == memory.QueryAnalysisRolloutStageShadow {
			for _, signal := range analysis.Signals[1:] {
				if len(inputs) >= limits.MaxSignals {
					break
				}
				derived := input
				derived.Query = signal.Text
				derived.QueryEmbedding = nil
				derived.queryAnalysisObserveOnly = true
				inputs = append(inputs, derived)
			}
		}
		if input.IncludeFeedbackDiagnostics {
			diagnostic := queryAnalysisDiagnostic(analysis, limits, resolution.Stage, fallback, time.Since(analysisStarted))
			diagnostic.Status, diagnostic.Reason = string(resolution.Stage), "rollout does not permit derived signals"
			diagnostics = append(diagnostics, diagnostic)
		}
		return inputs, diagnostics
	}
	for _, signal := range analysis.Signals[1:] {
		if len(inputs) >= limits.MaxSignals {
			break
		}
		derived := input
		derived.Query = signal.Text
		derived.QueryEmbedding = nil // embeddings are authoritative only for original query
		inputs = append(inputs, derived)
	}
	if input.IncludeFeedbackDiagnostics {
		diagnostic := queryAnalysisDiagnostic(analysis, limits, resolution.Stage, fallback, time.Since(analysisStarted))
		diagnostic.Status, diagnostic.Reason, diagnostic.Included = "active", "bounded derived signals enabled", len(inputs)-1
		diagnostics = append(diagnostics, diagnostic)
	}
	return inputs, diagnostics
}

func queryAnalysisFallbackForResult(result QueryAnalysisResult) QueryAnalysisFallbackCategory {
	if result.Disposition != QueryAnalysisDispositionOriginalOnly {
		return QueryAnalysisFallbackNone
	}
	for _, category := range result.Categories {
		switch category.Category {
		case QueryAnalysisDiagnosticAdversarial:
			return QueryAnalysisFallbackAdversarial
		case QueryAnalysisDiagnosticMalformed:
			return QueryAnalysisFallbackMalformed
		case QueryAnalysisDiagnosticUnavailable:
			return QueryAnalysisFallbackUnavailable
		case QueryAnalysisDiagnosticOverBudget:
			return QueryAnalysisFallbackOverBudget
		case QueryAnalysisDiagnosticDuplicate:
			return QueryAnalysisFallbackDuplicate
		}
	}
	return QueryAnalysisFallbackNone
}

func boundedAnalysisElapsedNS(elapsed, max time.Duration) int64 {
	if elapsed < 0 {
		return 0
	}
	if max > 0 && elapsed > max {
		elapsed = max
	}
	return elapsed.Nanoseconds()
}

func countAnalysisSubqueries(result QueryAnalysisResult) int {
	count := 0
	for index, signal := range result.Signals {
		if index > 0 && signal.Kind == QueryAnalysisSignalSubquery {
			count++
		}
	}
	return count
}

// queryAnalysisDiagnostic converts the validated analyzer result into the
// allowlisted API diagnostic shape. It intentionally carries aggregate values
// only; callers may fill CandidateCount after recall completes.
func queryAnalysisDiagnostic(result QueryAnalysisResult, limits QueryAnalysisLimits, stage memory.QueryAnalysisRolloutStage, fallback QueryAnalysisFallbackCategory, elapsed time.Duration) ContextDiagnostic {
	diagnostic, err := QueryAnalysisDiagnosticsFromResult(result, limits, fallback, elapsed, 0, string(stage))
	if err != nil {
		return ContextDiagnostic{Section: "query_analysis", Status: "original_only", OriginalRetained: true, Fallback: QueryAnalysisFallbackMalformed, Disposition: QueryAnalysisDispositionOriginalOnly, RolloutStage: string(memory.QueryAnalysisRolloutStageOriginalOnly)}
	}
	return ContextDiagnostic{Section: "query_analysis", Status: string(stage), PolicyVersion: diagnostic.PolicyVersion, LimitsVersion: diagnostic.LimitsVersion, OriginalRetained: diagnostic.OriginalRetained, HintCount: diagnostic.HintCount, SignalCount: diagnostic.SignalCount, SubqueryCount: diagnostic.SubqueryCount, CandidateCount: diagnostic.CandidateCount, ElapsedNS: diagnostic.Elapsed.Nanoseconds(), Fallback: diagnostic.Fallback, Disposition: diagnostic.Disposition, Categories: append([]QueryAnalysisDiagnosticCount(nil), diagnostic.Categories...), RolloutStage: diagnostic.RolloutStage, NormalizationStatus: diagnostic.NormalizationStatus, TimeStatus: diagnostic.TimeStatus}
}

func mergeDiversityScoredMemory(left, right ScoredMemory) ScoredMemory {
	merged := left
	if merged.SourceEventID == "" {
		merged.SourceEventID = right.SourceEventID
	}
	if merged.ParentMemoryID == "" {
		merged.ParentMemoryID = right.ParentMemoryID
	}
	if merged.EmbeddingRevision == "" {
		merged.EmbeddingRevision = right.EmbeddingRevision
	}
	if !merged.EmbeddingRevisionActive {
		merged.EmbeddingRevisionActive = right.EmbeddingRevisionActive
	}
	if len(merged.Embedding) == 0 {
		merged.Embedding = append([]float32(nil), right.Embedding...)
	}
	if merged.SessionID == "" {
		merged.SessionID = right.SessionID
	}
	if merged.EntityKey == "" {
		merged.EntityKey = right.EntityKey
	}
	if merged.TimeSlice == "" {
		merged.TimeSlice = right.TimeSlice
	}
	if len(merged.Citations) == 0 {
		merged.Citations = append([]Citation(nil), right.Citations...)
	}
	return merged
}

func defaultFusionChannelAvailability() map[FusionChannel]fusionChannelAvailability {
	return map[FusionChannel]fusionChannelAvailability{
		FusionChannelLexical:  fusionChannelUnavailable,
		FusionChannelSemantic: fusionChannelUnavailable,
		FusionChannelRelation: fusionChannelUnavailable,
		FusionChannelChunk:    fusionChannelUnavailable,
	}
}

type retrievalFusionMetricObserver interface {
	RecordRetrievalFusion(context.Context, telemetry.RetrievalFusionEvent)
}

func (s *Service) recordFusionTelemetry(ctx context.Context, strategy FusionStrategy, channels []FusionChannelCandidates) {
	observer, ok := s.observer.(retrievalFusionMetricObserver)
	if !ok || observer == nil {
		return
	}
	seen := make(map[FusionChannel]struct{}, len(channels))
	for _, candidates := range channels {
		seen[candidates.Channel] = struct{}{}
		observer.RecordRetrievalFusion(ctx, telemetry.RetrievalFusionEvent{
			Strategy: string(strategy.Name), Version: strategy.Version, Channel: string(candidates.Channel),
			Availability: "available", CandidateCount: len(candidates.Candidates), Outcome: "fused",
		})
	}
	for _, channel := range []FusionChannel{FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk} {
		if _, present := seen[channel]; present {
			continue
		}
		observer.RecordRetrievalFusion(ctx, telemetry.RetrievalFusionEvent{
			Strategy: string(strategy.Name), Version: strategy.Version, Channel: string(channel),
			Availability: "unavailable", CandidateCount: 0, Outcome: "fallback",
		})
	}
}

func (s *Service) applyRankingRolloutPolicy(ctx context.Context, scope memory.Scope, surface memory.RankingRolloutSurface, feedbackAwareRanking bool, scored []SearchHit, policy *memory.RankingRolloutPolicy) (bool, []ContextDiagnostic, error) {
	if s.rankingRolloutPolicyReader == nil {
		if feedbackAwareRanking {
			applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
			return true, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
		}
		return false, nil, nil
	}

	if policy == nil {
		if feedbackAwareRanking {
			applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
			s.recordRankingRolloutPolicyEvaluation(ctx, "request_hint", surface, memory.RankingRolloutPolicy{}, "no_active_policy")
			return true, []ContextDiagnostic{{Section: "search_feedback", Status: "request_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
		}
		s.recordRankingRolloutPolicyEvaluation(ctx, "not_applied", surface, memory.RankingRolloutPolicy{}, "no_active_policy")
		return false, nil, nil
	}

	applied := policy.Status == memory.RankingRolloutPolicyStatusActiveForScope && policy.Mode == memory.RankingRolloutModeActiveForScope
	if applied {
		for i := range scored {
			adjustment, err := s.rankingRolloutAdjustmentForHit(ctx, scope, *policy, scored[i].Memory.ID)
			if err != nil {
				return false, nil, err
			}
			scored[i].Score.Overall += adjustment
		}
		s.recordRankingRolloutPolicyEvaluation(ctx, "applied", surface, *policy, "active_policy")
		return true, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_policy_applied", Reason: "active scoped ranking rollout policy applied"}}, nil
	}

	if feedbackAwareRanking {
		applyFeedbackAwareRankingHint(scored, s.usefulnessSummarizer, ctx, scope)
		s.recordRankingRolloutPolicyEvaluation(ctx, "request_hint", surface, *policy, "ineligible_policy")
		return true, []ContextDiagnostic{{Section: "search_feedback", Status: "request_hint_applied", Reason: "explicit per-request feedback-aware ranking hint applied"}}, nil
	}

	s.recordRankingRolloutPolicyEvaluation(ctx, "not_applied", surface, *policy, "ineligible_policy")
	return false, []ContextDiagnostic{{Section: "search_feedback", Status: "ranking_policy_skipped", Reason: "active scoped ranking rollout policy is not eligible for default ranking"}}, nil
}

func (s *Service) readActiveRankingPolicy(ctx context.Context, scope memory.Scope, surface memory.RankingRolloutSurface) (*memory.RankingRolloutPolicy, error) {
	policy, err := s.rankingRolloutPolicyReader.ReadActiveRankingRolloutPolicy(ctx, memory.ReadActiveRankingRolloutPolicyInput{Scope: scope, Surface: surface})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if policy.Scope.Normalized() != scope.Normalized() {
		return nil, fmt.Errorf("active ranking rollout policy scope does not match retrieval scope")
	}
	return &policy, nil
}

func fusionStrategyFromPolicy(policy memory.RankingRolloutPolicy) (FusionStrategy, bool, error) {
	if policy.Status != memory.RankingRolloutPolicyStatusActiveForScope || policy.Mode != memory.RankingRolloutModeActiveForScope {
		return FusionStrategy{}, false, nil
	}
	if policy.FusionStrategy == "" {
		return FusionStrategy{}, false, nil
	}
	weights := make(map[FusionChannel]float64, len(policy.FusionChannelWeights))
	for channel, weight := range policy.FusionChannelWeights {
		weights[FusionChannel(channel)] = weight
	}
	strategy := FusionStrategy{
		Name:                FusionStrategyName(policy.FusionStrategy),
		Version:             policy.FusionVersion,
		RankConstant:        policy.FusionRankConstant,
		ChannelWeights:      weights,
		PerChannelCandidate: policy.FusionPerChannelCandidate,
		TotalCandidates:     policy.FusionTotalCandidates,
	}
	if err := strategy.Validate(); err != nil {
		return FusionStrategy{}, false, fmt.Errorf("invalid active fusion strategy: %w", err)
	}
	return strategy, true, nil
}

func diversityPolicyFromRankingRollout(policy *memory.RankingRolloutPolicy) (DiversityPolicy, bool) {
	if policy == nil || policy.Status != memory.RankingRolloutPolicyStatusActiveForScope || policy.Mode != memory.RankingRolloutModeActiveForScope || strings.TrimSpace(policy.DiversityPolicyName) == "" {
		return DiversityPolicy{}, false
	}
	weights := DiversityCoverageWeights{
		MemoryClass: policy.DiversityCoverageWeights["memory_class"],
		Session:     policy.DiversityCoverageWeights["session"],
		Entity:      policy.DiversityCoverageWeights["entity"],
		TimeSlice:   policy.DiversityCoverageWeights["time_slice"],
		Unknown:     policy.DiversityCoverageWeights["unknown"],
	}
	converted := DiversityPolicy{
		Name: policy.DiversityPolicyName, Version: policy.DiversityPolicyVersion,
		MMRLambda: policy.DiversityMMRLambda, SemanticThreshold: policy.DiversitySemanticThreshold,
		MaxCandidates: policy.DiversityMaxCandidates, MaxPairwiseComparisons: policy.DiversityMaxPairwiseComparisons,
		MaxEmbeddingDimensions: policy.DiversityMaxEmbeddingDimensions, MaxCitationsPerCandidate: policy.DiversityMaxCitationsPerCandidate,
		CoverageWeights: weights,
	}
	if err := converted.Validate(); err != nil {
		return DiversityPolicy{}, false
	}
	return converted, true
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

func (s *Service) applyOptionalReranker(ctx context.Context, input SearchInput, scored []SearchHit, policy *memory.RankingRolloutPolicy, planner retrievalPlannerExecution) ([]ContextDiagnostic, RetrievalRerankerObservation) {
	observation := RetrievalRerankerObservation{Safe: true}
	if s.reranker == nil || s.rerankerMode == RerankerModeDisabled || len(scored) == 0 {
		return nil, observation
	}
	rerankerPolicy := policy
	if s.rerankerMode == RerankerModeShadow {
		rerankerPolicy = planner.rolloutPolicy
	}
	if rerankerPolicy == nil || !rerankerAllowedForPolicy(*rerankerPolicy, input.Scope, s.rerankerMode, s.rerankerProvider, s.rerankerVersion) {
		s.recordRerankTelemetry(ctx, "fallback", "no_matching_policy", len(scored))
		observation.FallbackCategory = "no_matching_policy"
		if rerankerPolicy != nil || planner.planned() {
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
		}
		return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "no matching scoped reranker policy"}}, observation
	}
	if planner.plan != nil && !planner.plan.RerankerEligible {
		observation.FallbackCategory = "planner_ineligible_or_no_headroom"
		return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
	}
	if planner.planned() {
		if !time.Now().Before(planner.deadline) {
			observation.FallbackCategory = "planner_latency_exhausted"
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
		}
		if !planner.plan.RerankerEligible || planner.ledger.RemainingForReranker() < len(scored) {
			observation.FallbackCategory = "planner_ineligible_or_no_headroom"
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
		}
		if err := planner.ledger.ConsumeReranker(len(scored), time.Now().UTC()); err != nil {
			observation.FallbackCategory = "planner_headroom_exhausted"
			if !time.Now().Before(planner.deadline) {
				observation.FallbackCategory = "planner_latency_exhausted"
			}
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
		}
	}
	candidates := make([]RerankCandidate, 0, len(scored))
	for _, hit := range scored {
		if hit.Memory.Scope.Normalized() != input.Scope.Normalized() || hit.Memory.State != memory.MemoryStateActive {
			s.recordRerankTelemetry(ctx, "fallback", "visibility_validation", len(scored))
			observation.Safe = false
			observation.FallbackCategory = "visibility_validation"
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "candidate visibility validation failed"}}, observation
		}
		candidates = append(candidates, RerankCandidate{ID: hit.Memory.ID, Text: hit.Memory.Content})
	}
	observation.Attempted = true
	rerankCtx, cancel := ctx, func() {}
	if planner.planned() {
		var ok bool
		rerankCtx, cancel, ok = planner.callContext(ctx)
		if !ok {
			observation.Attempted = false
			observation.FallbackCategory = "planner_latency_exhausted"
			return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker not applied"}}, observation
		}
	}
	scores, err := s.reranker.Rerank(rerankCtx, RerankRequest{Query: input.Query, Candidates: candidates})
	cancel()
	if err != nil {
		s.recordRerankTelemetry(ctx, "fallback", "provider_unavailable", len(scored))
		observation.FallbackCategory = "provider_unavailable"
		return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker unavailable"}}, observation
	}
	if err := ValidateRerankResponse(candidates, scores); err != nil {
		s.recordRerankTelemetry(ctx, "fallback", "invalid_response", len(scored))
		observation.Safe = false
		observation.FallbackCategory = "invalid_response"
		return []ContextDiagnostic{{Section: "rerank", Status: "fallback", Reason: "optional reranker response invalid"}}, observation
	}
	if s.rerankerMode != RerankerModeActive {
		s.recordRerankTelemetry(ctx, "shadow", "", len(scored))
		return []ContextDiagnostic{{Section: "rerank", Status: "shadow_evaluated", Reason: "reranker executed without changing ordinary ordering"}}, observation
	}
	byID := make(map[string]float64, len(scores))
	for _, score := range scores {
		byID[score.ID] = score.Score
	}
	for i := range scored {
		feature, featureErr := s.qualityFeaturesForHit(ctx, input.Scope, scored[i])
		if featureErr != nil {
			feature = QualityFeatureVector{Version: "quality-v1"}
		}
		adjustment, _ := ComputeQualityAdjustment(feature, s.qualityBounds)
		// Provider scores are normalized to a bounded hint around the fused baseline.
		if score, ok := byID[scored[i].Memory.ID]; ok {
			if score > 1 {
				score = 1
			}
			if score < -1 {
				score = -1
			}
			scored[i].Score.Overall += score * s.qualityBounds.Total
		}
		scored[i].Score.Overall += adjustment
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score.Overall == scored[j].Score.Overall {
			if !scored[i].Memory.ModifiedAt.Equal(scored[j].Memory.ModifiedAt) {
				return scored[i].Memory.ModifiedAt.After(scored[j].Memory.ModifiedAt)
			}
			return scored[i].Memory.ID < scored[j].Memory.ID
		}
		return scored[i].Score.Overall > scored[j].Score.Overall
	})
	s.recordRerankTelemetry(ctx, "applied", "", len(scored))
	observation.Used = true
	return []ContextDiagnostic{{Section: "rerank", Status: "applied", Reason: "active scoped reranker applied"}}, observation
}

func (s *Service) qualityFeaturesForHit(ctx context.Context, scope memory.Scope, hit SearchHit) (QualityFeatureVector, error) {
	features := QualityFeatureVector{Version: "quality-v1"}
	if !hit.Memory.ModifiedAt.IsZero() {
		age := time.Since(hit.Memory.ModifiedAt)
		switch {
		case age <= 24*time.Hour:
			features.Freshness = 1
		case age >= 365*24*time.Hour:
			features.Freshness = -1
		default:
			features.Freshness = 1 - 2*float64(age-24*time.Hour)/float64(364*24*time.Hour)
		}
	}
	if len(hit.Citations) > 0 {
		features.EvidenceCoverage = minQualityFeature(1, float64(len(hit.Citations))/3)
	}
	if s.usefulnessSummarizer != nil {
		summary, err := s.usefulnessSummaryForHit(ctx, scope, hit.Memory.ID)
		if err != nil {
			return features, err
		}
		switch summary.EffectiveQuality {
		case memory.UsefulnessQualityPositive:
			features.Usefulness = 1
		case memory.UsefulnessQualityNegative:
			features.Usefulness = -1
		case memory.UsefulnessQualityMixed:
			features.Usefulness = -0.5
		case memory.UsefulnessQualityNeedsReview:
			features.Usefulness = -1
		}
	}
	if s.taskEvaluationSummarizer != nil {
		summary, err := s.taskEvaluationSummarizer.SummarizeTaskEvaluations(ctx, memory.SummarizeTaskEvaluationsInput{Scope: scope, EvidenceTargetKind: memory.TaskEvidenceTargetMemory, EvidenceTargetID: hit.Memory.ID})
		if err != nil {
			return features, err
		}
		if summary.ActiveEvaluations > 0 {
			succeeded := summary.VerdictCounts[memory.TaskEvaluationVerdictSucceeded]
			failed := summary.VerdictCounts[memory.TaskEvaluationVerdictFailed] + summary.VerdictCounts[memory.TaskEvaluationVerdictPartial]
			features.TaskSuccess = clampQualityFeature(float64(succeeded-failed) / float64(summary.ActiveEvaluations))
		}
	}
	return features, nil
}

func minQualityFeature(limit, value float64) float64 {
	if value < limit {
		return value
	}
	return limit
}

func clampQualityFeature(value float64) float64 {
	if value < -1 {
		return -1
	}
	if value > 1 {
		return 1
	}
	return value
}

func (s *Service) recordRerankTelemetry(ctx context.Context, outcome, fallbackCategory string, candidateCount int) {
	observer, ok := s.observer.(retrievalRerankMetricObserver)
	if !ok || observer == nil {
		return
	}
	observer.RecordRetrievalRerank(ctx, telemetry.RetrievalRerankEvent{
		Provider: s.rerankerProvider, Mode: string(s.rerankerMode), Outcome: outcome,
		FallbackCategory: fallbackCategory, CandidateCount: candidateCount,
	})
}

func rerankerAllowedForPolicy(policy memory.RankingRolloutPolicy, scope memory.Scope, runtimeMode RerankerMode, runtimeProvider, runtimeVersion string) bool {
	if policy.Scope.Normalized() != scope.Normalized() {
		return false
	}
	switch runtimeMode {
	case RerankerModeShadow:
		if policy.Status != memory.RankingRolloutPolicyStatusDryRun || policy.Mode != memory.RankingRolloutModeDryRun || policy.RerankerMode != string(runtimeMode) || strings.TrimSpace(policy.RerankerProvider) == "" || policy.RerankerProvider != strings.TrimSpace(runtimeProvider) || strings.TrimSpace(policy.RerankerVersion) == "" || policy.RerankerVersion != strings.TrimSpace(runtimeVersion) {
			return false
		}
		return true
	case RerankerModeActive:
		if policy.Status != memory.RankingRolloutPolicyStatusActiveForScope || policy.Mode != memory.RankingRolloutModeActiveForScope {
			return false
		}
	default:
		return false
	}
	if policy.ThresholdStatus != memory.RankingRolloutThresholdStatusSatisfied {
		return false
	}
	if policy.LatestDryRunStatus != memory.RankingRolloutThresholdStatusSatisfied {
		return false
	}
	if strings.TrimSpace(policy.LatestDryRunID) == "" {
		return false
	}
	if policy.RerankerMode != "" && policy.RerankerMode != string(runtimeMode) {
		return false
	}
	if strings.TrimSpace(policy.RerankerProvider) != "" && policy.RerankerProvider != strings.TrimSpace(runtimeProvider) {
		return false
	}
	if strings.TrimSpace(policy.RerankerVersion) != "" && policy.RerankerVersion != strings.TrimSpace(runtimeVersion) {
		return false
	}
	return true
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

	var contextPolicy *memory.RankingRolloutPolicy
	if s.rankingRolloutPolicyReader != nil {
		contextPolicy, err = s.readActiveRankingPolicy(ctx, input.Scope, memory.RankingRolloutSurfaceContext)
		if err != nil {
			return AssembledContext{}, err
		}
	}
	var contextFusion *FusionStrategy
	if contextPolicy != nil {
		configured, ok, fusionErr := fusionStrategyFromPolicy(*contextPolicy)
		if fusionErr != nil {
			return AssembledContext{}, fusionErr
		}
		if ok {
			contextFusion = &configured
		}
	}
	result, err := s.Search(ctx, SearchInput{
		Scope:                      input.Scope,
		Query:                      input.Query,
		SessionID:                  input.SessionID,
		UserID:                     input.UserID,
		TopK:                       maxInt(input.Budget*3, input.Budget),
		IncludeSummaries:           true,
		IncludeRelations:           input.IncludeRelations,
		IncludeFeedbackDiagnostics: input.IncludeDiagnostics && input.IncludeFeedbackDiagnostics,
		FeedbackAwareRanking:       false,
		rankingSurface:             memory.RankingRolloutSurfaceContext,
		rankingPolicyDisabled:      true,
		fusionStrategyOverride:     contextFusion,
	})
	if err != nil {
		return AssembledContext{}, err
	}
	projectionHits, projectionDiagnostics := s.readProjectionHits(ctx, input)
	if len(projectionHits) > 0 {
		result.Hits = append(projectionHits, result.Hits...)
	}
	chunkContextDiagnostics := []ContextDiagnostic(nil)
	result.Hits, chunkContextDiagnostics = s.boundChunkContextEvidence(input, result.Hits)
	adjusted, policyDiagnostics, err := s.applyRankingRolloutPolicy(ctx, input.Scope, memory.RankingRolloutSurfaceContext, input.FeedbackAwareRanking, result.Hits, contextPolicy)
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
		output.Diagnostics = append(output.Diagnostics, projectionDiagnostics...)
		output.Diagnostics = append(output.Diagnostics, chunkContextDiagnostics...)
	}
	calibrationDiagnostics := []ContextDiagnostic(nil)

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

	}
	if contextDiversityPolicy, enabled := diversityPolicyFromRankingRollout(contextPolicy); enabled {
		omitted := 0
		var sectionOmitted int
		profiles, sectionOmitted = applyContextDiversity(input.Scope, contextDiversityPolicy, profiles)
		omitted += sectionOmitted
		summaries, sectionOmitted = applyContextDiversity(input.Scope, contextDiversityPolicy, summaries)
		omitted += sectionOmitted
		relations, sectionOmitted = applyContextDiversity(input.Scope, contextDiversityPolicy, relations)
		omitted += sectionOmitted
		episodes, sectionOmitted = applyContextDiversity(input.Scope, contextDiversityPolicy, episodes)
		omitted += sectionOmitted
		others, sectionOmitted = applyContextDiversity(input.Scope, contextDiversityPolicy, others)
		omitted += sectionOmitted
		if input.IncludeDiagnostics && omitted > 0 {
			output.Diagnostics = append(output.Diagnostics, ContextDiagnostic{Section: "diversity", Status: "section_selection_applied", Omitted: omitted})
		}
	}
	calibrationCandidates := make([]SearchHit, 0, len(profiles)+len(summaries)+len(relations)+len(episodes)+len(others))
	calibrationCandidates = append(calibrationCandidates, profiles...)
	calibrationCandidates = append(calibrationCandidates, summaries...)
	calibrationCandidates = append(calibrationCandidates, relations...)
	calibrationCandidates = append(calibrationCandidates, episodes...)
	calibrationCandidates = append(calibrationCandidates, others...)
	calibrated, calibrationDiagnostics := s.applyContextCalibrationRollout(ctx, input, calibrationCandidates, contextPolicy)
	if len(calibrationDiagnostics) > 0 {
		rank := make(map[string]int, len(calibrated))
		for index, hit := range calibrated {
			rank[hit.Memory.ID] = index
		}
		reorder := func(section []SearchHit) []SearchHit {
			sort.SliceStable(section, func(i, j int) bool { return rank[section[i].Memory.ID] < rank[section[j].Memory.ID] })
			return section
		}
		profiles, summaries, relations, episodes, others = reorder(filterCalibratedHits(profiles, rank)), reorder(filterCalibratedHits(summaries, rank)), reorder(filterCalibratedHits(relations, rank)), reorder(filterCalibratedHits(episodes, rank)), reorder(filterCalibratedHits(others, rank))
	}
	if input.IncludeDiagnostics {
		output.Diagnostics = append(output.Diagnostics, calibrationDiagnostics...)
	}
	for _, section := range [][]SearchHit{profiles, summaries, relations, episodes, others} {
		for _, hit := range section {
			for _, citation := range hit.Citations {
				key := citation.MemoryID + ":" + citation.RawEventID + ":" + citation.Operation
				if _, ok := citationSeen[key]; ok {
					continue
				}
				citationSeen[key] = struct{}{}
				output.Citations = append(output.Citations, citation)
			}
		}
	}

	if result.retrievalPlan != nil {
		remaining := minInt(input.Budget, result.retrievalPlan.ContextItems)
		if remaining < 0 {
			remaining = 0
		}
		classHits := map[memory.MemoryClass][]SearchHit{
			memory.MemoryClassProfile:    profiles,
			memory.MemoryClassSummary:    summaries,
			memory.MemoryClassRelation:   relations,
			memory.MemoryClassEpisodic:   episodes,
			memory.MemoryClassProcedural: others,
		}
		seenClasses := make(map[memory.MemoryClass]struct{}, len(result.retrievalPlan.ContextPriorities))
		availableContextItems := len(profiles) + len(summaries) + len(episodes) + len(others)
		if input.IncludeRelations {
			availableContextItems += len(relations)
		}
		includedContextItems := 0
		priorities := append([]memory.MemoryClass(nil), result.retrievalPlan.ContextPriorities...)
		for _, class := range []memory.MemoryClass{memory.MemoryClassProfile, memory.MemoryClassSummary, memory.MemoryClassRelation, memory.MemoryClassEpisodic, memory.MemoryClassProcedural} {
			if _, seen := seenClasses[class]; !seen {
				priorities = append(priorities, class)
			}
		}
		priorities = preserveContextSummaryPreference(priorities)
		for _, class := range priorities {
			if remaining <= 0 {
				break
			}
			if _, duplicate := seenClasses[class]; duplicate {
				continue
			}
			seenClasses[class] = struct{}{}
			candidates := classHits[class]
			if class == memory.MemoryClassRelation && !input.IncludeRelations {
				continue
			}
			if quota := result.retrievalPlan.MemoryClassQuotas[class]; quota > 0 && len(candidates) > quota {
				candidates = candidates[:quota]
			}
			take := minInt(len(candidates), remaining)
			switch class {
			case memory.MemoryClassProfile:
				output.Profile = append(output.Profile, candidates[:take]...)
			case memory.MemoryClassSummary:
				output.RelevantSummaries = append(output.RelevantSummaries, candidates[:take]...)
			case memory.MemoryClassRelation:
				output.RelatedEntities = append(output.RelatedEntities, candidates[:take]...)
			case memory.MemoryClassEpisodic:
				output.RecentSession = append(output.RecentSession, candidates[:take]...)
			case memory.MemoryClassProcedural:
				output.RecentEpisodes = append(output.RecentEpisodes, candidates[:take]...)
			}
			remaining -= take
			includedContextItems += take
		}
		if input.retrievalPlannerDiagnosticsAuthorized && availableContextItems > includedContextItems {
			output.plannerDiagnostics = append(output.plannerDiagnostics, ContextDiagnostic{Section: "context_planner", Status: "omitted_by_quota_or_budget", Reason: "planned context quota or caller budget omitted visible items", Omitted: availableContextItems - includedContextItems})
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

func applyContextDiversity(scope memory.Scope, policy DiversityPolicy, hits []SearchHit) ([]SearchHit, int) {
	if len(hits) <= 1 {
		return hits, 0
	}
	candidates := make([]DiversityCandidate, 0, len(hits))
	for _, hit := range hits {
		candidates = append(candidates, DiversityCandidate{FusedCandidate: FusedCandidate{Memory: hit.Memory, Score: hit.Score.Overall}, Citations: append([]Citation(nil), hit.Citations...)})
	}
	selection, err := SelectDiverseCandidates(scope, policy, candidates, len(candidates))
	if err != nil {
		return hits, 0
	}
	byID := make(map[string]SearchHit, len(hits))
	for _, hit := range hits {
		byID[hit.Memory.ID] = hit
	}
	result := make([]SearchHit, 0, len(selection.Candidates))
	for _, candidate := range selection.Candidates {
		hit := byID[candidate.Memory.ID]
		hit.Citations = candidate.Citations
		result = append(result, hit)
	}
	return result, selection.Dispositions.Duplicate + selection.Dispositions.Diversity
}

// boundChunkContextEvidence keeps chunk-derived evidence within the existing
// context character budget. Parent memories remain the public identity; the
// chunk text is used only as the bounded evidence payload when it is visible.
func (s *Service) boundChunkContextEvidence(input AssembleContextInput, hits []SearchHit) ([]SearchHit, []ContextDiagnostic) {
	if len(hits) == 0 {
		return hits, nil
	}
	used := 0
	kept := make([]SearchHit, 0, len(hits))
	omitted := 0
	for _, hit := range hits {
		if hit.Chunk == nil {
			kept = append(kept, hit)
			continue
		}
		if hit.Chunk.Scope.Normalized() != input.Scope.Normalized() || hit.Chunk.LifecycleState != memory.MemoryStateActive || hit.Memory.Scope.Normalized() != input.Scope.Normalized() || hit.Memory.State != memory.MemoryStateActive {
			omitted++
			continue
		}
		chunkBytes := len([]byte(hit.Chunk.Content))
		if input.CharacterBudget > 0 && used+chunkBytes > input.CharacterBudget {
			omitted++
			continue
		}
		used += chunkBytes
		// Preserve canonical identity and citations while allowing the derived
		// bounded text to be assembled into an existing section.
		hit.Memory.Content = hit.Chunk.Content
		kept = append(kept, hit)
	}
	if omitted == 0 || !input.IncludeDiagnostics {
		return kept, nil
	}
	return kept, []ContextDiagnostic{{Section: "chunk_context", Status: "omitted_by_budget_or_validation", Reason: "chunk evidence exceeded bounded context budget or failed lifecycle/scope validation", Omitted: omitted}}
}

func (s *Service) readProjectionHits(ctx context.Context, input AssembleContextInput) ([]SearchHit, []ContextDiagnostic) {
	if (!input.UseProjections && !s.projectionConsumptionEnabled) || s.projections == nil {
		return nil, nil
	}
	hits := make([]SearchHit, 0)
	diagnostics := make([]ContextDiagnostic, 0, 2)
	for _, kind := range []memory.ContextProjectionKind{memory.ContextProjectionKindAlwaysVisible, memory.ContextProjectionKindSession} {
		projection, err := s.projections.ReadLatestContextProjection(ctx, input.Scope, kind)
		section := "profile"
		if kind == memory.ContextProjectionKindSession {
			section = "recent_session"
		}
		if err != nil {
			diagnostics = append(diagnostics, ContextDiagnostic{Section: section, Status: "projection_unavailable", Reason: "projection read failed closed"})
			continue
		}
		included, omitted, budgetOmitted := 0, 0, 0
		usedBytes := 0
		for _, item := range projection.Items {
			if item.Source.Scope.Normalized() != input.Scope.Normalized() || item.LifecycleState != memory.MemoryStateActive {
				omitted++
				continue
			}
			itemBytes := len([]byte(item.Text))
			if itemBytes == 0 || (input.CharacterBudget > 0 && usedBytes+itemBytes > input.CharacterBudget) {
				budgetOmitted++
				continue
			}
			memoryID := item.Source.MemoryID
			if memoryID == "" && item.Source.Kind == memory.ContextProjectionSourceCanonicalVersion {
				memoryID = item.Source.ID
			}
			hit := SearchHit{Memory: memory.CanonicalMemory{ID: memoryID, Scope: input.Scope, Class: item.Class, State: memory.MemoryStateActive, Content: item.Text}, Citations: []Citation{{MemoryID: item.Citation.MemoryID, RawEventID: item.Citation.RawEventID, Operation: item.Citation.Operation}}}
			hits = append(hits, hit)
			included++
			usedBytes += itemBytes
		}
		status := "no_visible_items"
		if included > 0 {
			status = "included"
		}
		diagnostics = append(diagnostics, ContextDiagnostic{Section: section, Status: status, Included: included, Omitted: omitted})
		if budgetOmitted > 0 {
			diagnostics = append(diagnostics, ContextDiagnostic{Section: section, Status: "omitted_by_budget", Reason: "projection item exceeds remaining context character budget", Omitted: budgetOmitted})
		}
	}
	return hits, diagnostics
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

func preserveContextSummaryPreference(priorities []memory.MemoryClass) []memory.MemoryClass {
	summaryIndex, episodicIndex := -1, -1
	for index, class := range priorities {
		if class == memory.MemoryClassSummary && summaryIndex < 0 {
			summaryIndex = index
		}
		if class == memory.MemoryClassEpisodic && episodicIndex < 0 {
			episodicIndex = index
		}
	}
	if summaryIndex >= 0 && episodicIndex >= 0 && episodicIndex < summaryIndex {
		priorities[summaryIndex], priorities[episodicIndex] = priorities[episodicIndex], priorities[summaryIndex]
	}
	return priorities
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
