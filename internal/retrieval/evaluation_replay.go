package retrieval

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

const evaluationReplayTopK = 100

// EvaluationRunner replays a checked-in fixture through the existing scoped search
// interface. It is an internal evaluation path and does not change SearchInput's
// public contract or ordinary SearchResult output.
type EvaluationRunner struct {
	searcher MemorySearcher
	observer telemetry.Observer
}

func NewEvaluationRunner(searcher MemorySearcher, observers ...telemetry.Observer) *EvaluationRunner {
	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}
	return &EvaluationRunner{searcher: searcher, observer: observer}
}

type EvaluationReplay struct {
	Metadata EvaluationRankingMetadata
	Cases    []EvaluationReplayCase
}

type EvaluationReplayCase struct {
	CaseID                 string
	Category               string
	Scope                  memory.Scope
	ExpectedEvidenceGroups [][]string
	ExcludedAliases        []string
	Candidates             []EvaluationReplayCandidate
	Diagnostics            []EvaluationCandidateDiagnostic
	ChannelAvailability    map[string]EvaluationChannelStatus
	CandidatePoolSize      int
	Latency                time.Duration
	AnalysisDiagnostics    *QueryAnalysisDiagnostics
	PlannerFamily          RetrievalQueryFamily
	PlannerIdentity        string
	PlannerVersion         string
	PlannerPolicyVersion   string
	Passes                 []EvaluationReplayPass
	RerankerUsed           bool
	RerankerSafe           bool
	FallbackCategory       string
	RollbackVerified       bool
	PlannerProtected       bool
	// Temporal carries the fact-valid outcome of this case. It stays zero for a
	// pre-temporal fixture, so replay shape is unchanged for existing fixtures.
	TemporalKind                EvaluationTemporalCaseKind
	TemporalMode                memory.TemporalSelectionMode
	TemporalSelectorKind        string
	TemporalSelectedVersions    int
	TemporalHiddenAliasCount    int
	TemporalStaleVersions       int
	TemporalAmbiguousVersions   int
	TemporalConflictDisposition string
	TemporalProvenanceMismatch  int
	TemporalHiddenVersionLeak   int
	TemporalFallbackCategory    string
}

type EvaluationReplayPass struct {
	Pass             int
	CandidateCount   int
	VisibleCount     int
	EvidenceCoverage float64
	Latency          time.Duration
	Evidence         EvidenceAssessment
}

type EvaluationCandidateDisposition string

const (
	EvaluationCandidateDispositionReturned    EvaluationCandidateDisposition = "returned"
	EvaluationCandidateDispositionNotReturned EvaluationCandidateDisposition = "not_returned"
)

type EvaluationChannelStatus string

const (
	EvaluationChannelStatusAvailable   EvaluationChannelStatus = "available"
	EvaluationChannelStatusUnavailable EvaluationChannelStatus = "unavailable"
)

// EvaluationCandidateDiagnostic contains only a fixture alias and bounded rank
// information. Unknown, hidden, and foreign candidates are deliberately omitted from
// this list and represented by aggregate safety categories during evaluation.
type EvaluationCandidateDiagnostic struct {
	Alias          string
	FusionStrategy string
	LexicalRank    int
	SemanticRank   int
	RelationRank   int
	ChunkRank      int
	ChannelStatus  map[string]EvaluationChannelStatus
	FinalRank      int
	Disposition    EvaluationCandidateDisposition
}

// EvaluationReplayCandidate remains internal to replay and metric calculation. It
// is converted to an alias-only report contribution before it crosses any output
// boundary.
type EvaluationReplayCandidate struct {
	Alias         string
	MemoryID      string
	Scope         memory.Scope
	State         memory.MemoryState
	FactCluster   string
	Lexical       bool
	Semantic      bool
	Relation      bool
	Chunk         bool
	FinalRank     int
	Citations     []Citation
	lexicalScore  float64
	semanticScore float64
	relationScore float64
	ChunkDerived  bool
}

// PlannerRollbackEquivalent proves that a rollback execution is planner-free
// and restores the baseline public retrieval result, citations, safety, and
// bounded resource shape. Latency is intentionally excluded because it is
// measured separately across interleaved samples.
func PlannerRollbackEquivalent(baseline, rollback EvaluationReplay) bool {
	if !reflect.DeepEqual(baseline.Metadata, rollback.Metadata) || len(baseline.Cases) != len(rollback.Cases) {
		return false
	}
	for index := range baseline.Cases {
		left, right := baseline.Cases[index], rollback.Cases[index]
		if right.PlannerFamily != "" || right.PlannerIdentity != "" || right.PlannerVersion != "" || right.PlannerPolicyVersion != "" || len(right.Passes) != 0 || right.RerankerUsed || right.FallbackCategory != "" {
			return false
		}
		if len(evaluationReplaySafetyFailures(left)) != 0 || len(evaluationReplaySafetyFailures(right)) != 0 {
			return false
		}
		left.Latency, right.Latency = 0, 0
		left.RollbackVerified, right.RollbackVerified = false, false
		if !reflect.DeepEqual(left, right) {
			return false
		}
	}
	return true
}

// AggregateEvaluationReplaySamples preserves one deterministic replay shape
// while replacing case and pass latency with the median of repeated samples.
// Any non-latency divergence is rejected rather than hidden by aggregation.
func AggregateEvaluationReplaySamples(samples []EvaluationReplay) (EvaluationReplay, error) {
	if len(samples) == 0 {
		return EvaluationReplay{}, fmt.Errorf("evaluation replay samples are required")
	}
	aggregated := samples[0]
	aggregated.Cases = append([]EvaluationReplayCase(nil), samples[0].Cases...)
	caseLatencies := make([][]time.Duration, len(aggregated.Cases))
	passLatencies := make([][][]time.Duration, len(aggregated.Cases))
	for caseIndex := range aggregated.Cases {
		aggregated.Cases[caseIndex].Candidates = append([]EvaluationReplayCandidate(nil), samples[0].Cases[caseIndex].Candidates...)
		aggregated.Cases[caseIndex].Passes = append([]EvaluationReplayPass(nil), samples[0].Cases[caseIndex].Passes...)
		passLatencies[caseIndex] = make([][]time.Duration, len(aggregated.Cases[caseIndex].Passes))
	}
	for _, sample := range samples {
		if !reflect.DeepEqual(samples[0].Metadata, sample.Metadata) || len(sample.Cases) != len(aggregated.Cases) {
			return EvaluationReplay{}, fmt.Errorf("evaluation replay sample shape changed")
		}
		for caseIndex := range aggregated.Cases {
			expected, current := samples[0].Cases[caseIndex], sample.Cases[caseIndex]
			caseLatencies[caseIndex] = append(caseLatencies[caseIndex], current.Latency)
			expected.Latency, current.Latency = 0, 0
			if len(expected.Passes) != len(current.Passes) {
				return EvaluationReplay{}, fmt.Errorf("evaluation replay pass shape changed")
			}
			expected.Passes = append([]EvaluationReplayPass(nil), expected.Passes...)
			current.Passes = append([]EvaluationReplayPass(nil), current.Passes...)
			for passIndex := range current.Passes {
				passLatencies[caseIndex][passIndex] = append(passLatencies[caseIndex][passIndex], current.Passes[passIndex].Latency)
				expected.Passes[passIndex].Latency = 0
				current.Passes[passIndex].Latency = 0
			}
			if !reflect.DeepEqual(expected, current) {
				return EvaluationReplay{}, fmt.Errorf("evaluation replay sample result changed")
			}
		}
	}
	for caseIndex := range aggregated.Cases {
		aggregated.Cases[caseIndex].Latency = medianEvaluationDuration(caseLatencies[caseIndex])
		for passIndex := range aggregated.Cases[caseIndex].Passes {
			aggregated.Cases[caseIndex].Passes[passIndex].Latency = medianEvaluationDuration(passLatencies[caseIndex][passIndex])
		}
	}
	return aggregated, nil
}

func medianEvaluationDuration(values []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered[(len(ordered)-1)/2]
}

// evaluationReplayTemporalConstraint resolves the selection a temporal fixture
// replays. A pre-temporal case yields the zero constraint, which retrieval
// resolves as current, so existing fixtures replay exactly as before.
func evaluationReplayTemporalConstraint(item EvaluationCase) (memory.TemporalConstraint, error) {
	if item.Temporal == nil {
		return memory.TemporalConstraint{}, nil
	}
	if item.Temporal.Selector == nil {
		// A current-class scenario still names the instant it evaluates at, so
		// the replay is anchored rather than read from the wall clock.
		if item.Temporal.EvaluationInstant != nil {
			return memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}, nil
		}
		return memory.TemporalConstraint{}, nil
	}
	return item.Temporal.Selector.Constraint()
}

func evaluationReplayTemporalKind(item EvaluationCase) EvaluationTemporalCaseKind {
	if item.Temporal == nil {
		return ""
	}
	return item.Temporal.Kind
}

func evaluationReplayTemporalMode(item EvaluationCase, constraint memory.TemporalConstraint) memory.TemporalSelectionMode {
	if item.Temporal == nil {
		return ""
	}
	if constraint.Mode != "" {
		return constraint.Mode
	}
	return memory.TemporalSelectionCurrent
}

// evaluationReplaySelectorKind names the coarse shape of the selector. The
// instants themselves are deliberately excluded: a report must not carry the
// evaluation window it used.
func evaluationReplaySelectorKind(item EvaluationCase) string {
	if item.Temporal == nil {
		return ""
	}
	switch item.Temporal.Kind {
	case EvaluationTemporalCaseAsOf, EvaluationTemporalCaseRetroactiveCorrection:
		return "point"
	case EvaluationTemporalCaseInterval, EvaluationTemporalCaseTemporalIsolation:
		return "interval"
	default:
		return "current"
	}
}

func evaluationReplayHiddenAliasCount(item EvaluationCase) int {
	if item.Temporal == nil {
		return 0
	}
	return len(item.Temporal.HiddenAliases)
}

func (r *EvaluationRunner) Replay(ctx context.Context, fixture EvaluationFixture, seed EvaluationFixtureSeed, metadata EvaluationRankingMetadata) (run EvaluationReplay, replayErr error) {
	started := time.Now()
	defer func() {
		if r == nil || r.observer == nil {
			return
		}
		status := "completed"
		failureCategory := ""
		if replayErr != nil {
			status = "failed"
			failureCategory = string(EvaluationSafetyFailureUnsafeDiagnostics)
		}
		r.rankingEvaluationObserver().RecordRetrievalEvaluation(ctx, telemetry.RetrievalEvaluationEvent{
			Status:          status,
			FixtureVersion:  fixture.Version,
			RankingVersion:  metadata.RankingVersion,
			PolicyVersion:   metadata.PolicyVersion,
			FailureCategory: failureCategory,
			CaseCount:       len(fixture.Cases),
			Duration:        time.Since(started),
		})
	}()
	if err := fixture.Validate(); err != nil {
		return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureInvalidFixtureScope, err.Error())
	}
	if metadata.FusionStrategy == "" {
		metadata.FusionStrategy = fusionStrategyIdentity(DefaultRRFStrategy())
	}
	if err := metadata.Validate(); err != nil {
		return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if metadata.FixtureVersion != fixture.Version || seed.FixtureVersion != fixture.Version {
		return EvaluationReplay{}, fmt.Errorf("evaluation fixture version mismatch")
	}
	if r == nil || r.searcher == nil {
		return EvaluationReplay{}, fmt.Errorf("evaluation searcher is not configured")
	}

	aliases := make(map[string]EvaluationSeededAlias, len(seed.Aliases))
	for _, record := range seed.Aliases {
		key := evaluationSeedAliasKey(record.CaseID, record.Alias)
		if _, exists := aliases[key]; exists {
			return EvaluationReplay{}, fmt.Errorf("duplicate seeded evaluation alias")
		}
		aliases[key] = record
	}

	run = EvaluationReplay{Metadata: metadata, Cases: make([]EvaluationReplayCase, 0, len(fixture.Cases))}
	for _, item := range fixture.Cases {
		caseAliases := make(map[string]EvaluationSeededAlias, len(item.Sources))
		activeAliases := make([]string, 0, len(item.Sources))
		for _, source := range item.Sources {
			record, found := aliases[evaluationSeedAliasKey(item.ID, source.Alias)]
			if !found {
				return EvaluationReplay{}, fmt.Errorf("seeded evaluation alias is missing")
			}
			caseAliases[record.MemoryID] = record
			if record.State == memory.MemoryStateActive {
				activeAliases = append(activeAliases, record.Alias)
			}
		}

		// A temporal fixture declares the exact selection it replays. Resolving it
		// here means a replay cannot report a historical selection while running
		// an ordinary current search.
		temporalConstraint, err := evaluationReplayTemporalConstraint(item)
		if err != nil {
			return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureValidityAmbiguity, err.Error())
		}

		started := time.Now()
		searchInput := SearchInput{
			Scope:                                 item.Scope,
			Query:                                 item.Query,
			QueryEmbedding:                        evaluationPlannerQueryEmbedding(item.Planner),
			LexicalMatchMode:                      metadata.LexicalMatchMode,
			TopK:                                  evaluationReplayTopK,
			IncludeSummaries:                      true,
			IncludeRelations:                      true,
			rankingPolicyDisabled:                 metadata.RolloutDisposition == "" || metadata.RolloutDisposition == "original_only",
			queryAnalysisPolicyDisabled:           metadata.RolloutDisposition == "" || metadata.RolloutDisposition == "original_only",
			IncludeFeedbackDiagnostics:            metadata.AnalysisVersion != "",
			queryAnalysisDiagnosticsAuthorized:    metadata.AnalysisVersion != "",
			retrievalPlannerDiagnosticsAuthorized: item.Planner != nil,
			TemporalConstraint:                    temporalConstraint,
		}
		result, err := r.searcher.Search(ctx, searchInput)
		if err != nil {
			return EvaluationReplay{}, fmt.Errorf("execute evaluation retrieval query")
		}

		caseRun := EvaluationReplayCase{
			CaseID:                 item.ID,
			Category:               item.Category,
			Scope:                  item.Scope,
			ExpectedEvidenceGroups: item.ExpectedEvidenceGroups,
			ExcludedAliases:        item.ExcludedAliases,
			Candidates:             make([]EvaluationReplayCandidate, 0, len(result.Hits)),
			CandidatePoolSize:      len(result.Hits),
			Latency:                time.Since(started),
			ChannelAvailability:    evaluationChannelAvailability(result.fusionChannelAvailability),
		}
		caseRun.TemporalKind = evaluationReplayTemporalKind(item)
		caseRun.TemporalMode = evaluationReplayTemporalMode(item, temporalConstraint)
		caseRun.TemporalSelectorKind = evaluationReplaySelectorKind(item)
		caseRun.TemporalHiddenAliasCount = evaluationReplayHiddenAliasCount(item)
		analysisDiagnostics, err := evaluationReplayAnalysisDiagnostics(result.Diagnostics, metadata, item.ExpectedAnalysis)
		if err != nil {
			return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
		}
		caseRun.AnalysisDiagnostics = analysisDiagnostics
		if result.fusionStrategy.Name != "" {
			metadata.FusionStrategy = fusionStrategyIdentity(result.fusionStrategy)
			run.Metadata.FusionStrategy = metadata.FusionStrategy
		}
		for index, hit := range result.Hits {
			record := caseAliases[hit.Memory.ID]
			caseRun.Candidates = append(caseRun.Candidates, EvaluationReplayCandidate{
				Alias:         record.Alias,
				MemoryID:      hit.Memory.ID,
				Scope:         hit.Memory.Scope,
				State:         hit.Memory.State,
				FactCluster:   record.FactCluster,
				Lexical:       hit.Score.Lexical != 0,
				Semantic:      hit.Score.Semantic != 0,
				Relation:      hit.Score.Relation != 0,
				Chunk:         hit.Chunk != nil,
				FinalRank:     index + 1,
				Citations:     append([]Citation(nil), hit.Citations...),
				lexicalScore:  hit.Score.Lexical,
				semanticScore: hit.Score.Semantic,
				relationScore: hit.Score.Relation,
				ChunkDerived:  hit.Chunk != nil,
			})
		}
		caseRun.Diagnostics = evaluationCandidateDiagnostics(caseRun.Candidates, metadata.FusionStrategy)
		caseRun.Diagnostics = append(caseRun.Diagnostics, evaluationMissingCandidateDiagnostics(caseRun.Diagnostics, activeAliases, metadata.FusionStrategy)...)
		if item.Planner != nil {
			if metadata.PlannerVersion != item.Planner.PlannerVersion || metadata.PlannerPolicyVersion != item.Planner.PolicyVersion {
				return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, "planner metadata does not match fixture")
			}
			if err := evaluationReplayPlannerObservation(&caseRun, result, item.Planner); err != nil {
				return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
			}
		}
		run.Cases = append(run.Cases, caseRun)
	}
	return run, nil
}

func evaluationPlannerQueryEmbedding(expectation *EvaluationPlannerExpectation) []float32 {
	if expectation != nil && expectation.Family == RetrievalQueryFamilySemantic {
		return []float32{1, 0, 0}
	}
	return nil
}

func evaluationReplayPlannerObservation(caseRun *EvaluationReplayCase, result SearchResult, expectation *EvaluationPlannerExpectation) error {
	if caseRun == nil || expectation == nil {
		return fmt.Errorf("planner replay expectation is required")
	}
	if err := expectation.validate(); err != nil {
		return err
	}
	if result.retrievalPlan == nil || len(result.plannerDiagnostics) != 1 {
		return fmt.Errorf("planner replay diagnostics are missing")
	}
	plan := result.retrievalPlan
	diagnostic := result.plannerDiagnostics[0]
	if err := diagnostic.Validate(); err != nil {
		return err
	}
	if plan.Identity.String() != expectation.PlanIdentity || plan.Family != expectation.Family ||
		string(plan.Identity.PlannerVersion) != expectation.PlannerVersion || string(plan.Identity.PolicyVersion) != expectation.PolicyVersion ||
		diagnostic.PlannerVersion != plan.Identity.PlannerVersion || diagnostic.PolicyVersion != plan.Identity.PolicyVersion || diagnostic.QueryFamily != plan.Family ||
		!equalFusionChannels(plan.Channels, expectation.Channels) || !channelCandidatesEqual(plan.FallbackChannelCandidates, expectation.FallbackChannelCandidates) ||
		plan.TotalCandidates > expectation.MaxCandidates || diagnostic.PassCount != expectation.ExpectedPasses {
		return fmt.Errorf("planner replay does not match fixture expectation")
	}
	if expectation.MaxCandidatesPerChannel > 0 {
		for _, count := range plan.ChannelCandidates {
			if count > expectation.MaxCandidatesPerChannel {
				return fmt.Errorf("planner replay exceeds fixture channel budget")
			}
		}
	}
	if plan.RerankerEligible != expectation.RerankerEligible {
		return fmt.Errorf("planner replay reranker eligibility mismatch")
	}
	if len(result.retrievalPassObservations) != diagnostic.PassCount {
		return fmt.Errorf("planner replay requires explicit per-pass observations")
	}
	fallback := "none"
	if diagnostic.Disposition == RetrievalPlanDispositionFallback {
		fallback = string(diagnostic.Fallback)
	}
	caseRun.PlannerFamily = plan.Family
	caseRun.PlannerIdentity = plan.Identity.String()
	caseRun.PlannerVersion = string(plan.Identity.PlannerVersion)
	caseRun.PlannerPolicyVersion = string(plan.Identity.PolicyVersion)
	caseRun.FallbackCategory = fallback
	caseRun.PlannerProtected = expectation.Protected
	caseRun.RerankerUsed = result.rerankerObservation.Used
	caseRun.RerankerSafe = result.rerankerObservation.Safe
	caseRun.Passes = make([]EvaluationReplayPass, 0, len(result.retrievalPassObservations))
	for index, observation := range result.retrievalPassObservations {
		if observation.Pass != index+1 || observation.CandidateCount < 0 || observation.Latency < 0 {
			return fmt.Errorf("planner replay pass observation is invalid")
		}
		coverage, visibleCount, err := evaluationPassEvidenceCoverage(*caseRun, observation.VisibleMemoryIDs)
		if err != nil {
			return err
		}
		caseRun.Passes = append(caseRun.Passes, EvaluationReplayPass{
			Pass: observation.Pass, CandidateCount: observation.CandidateCount,
			VisibleCount: visibleCount, EvidenceCoverage: coverage,
			Latency: observation.Latency, Evidence: observation.Evidence,
		})
	}
	return nil
}

func evaluationPassEvidenceCoverage(caseRun EvaluationReplayCase, visibleMemoryIDs []string) (float64, int, error) {
	visible := make(map[string]struct{}, len(visibleMemoryIDs))
	for _, id := range visibleMemoryIDs {
		if strings.TrimSpace(id) == "" {
			return 0, 0, fmt.Errorf("planner replay visible memory identity is invalid")
		}
		visible[id] = struct{}{}
	}
	passCase := caseRun
	passCase.Candidates = make([]EvaluationReplayCandidate, 0, len(visible))
	for _, candidate := range caseRun.Candidates {
		if _, ok := visible[candidate.MemoryID]; ok {
			passCase.Candidates = append(passCase.Candidates, candidate)
		}
	}
	if len(passCase.Candidates) != len(visible) {
		return 0, 0, fmt.Errorf("planner replay pass contains unknown visible memory")
	}
	return calculateCaseEvaluationMetrics(passCase).EvidenceCoverage, len(visible), nil
}

func equalFusionChannels(left, right []FusionChannel) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func evaluationReplayAnalysisDiagnostics(diagnostics []ContextDiagnostic, metadata EvaluationRankingMetadata, expectation *EvaluationAnalysisExpectation) (*QueryAnalysisDiagnostics, error) {
	if metadata.RolloutDisposition == "original_only" {
		for _, diagnostic := range diagnostics {
			if diagnostic.Section == "query_analysis" {
				return nil, fmt.Errorf("original-query baseline contains query-analysis diagnostics")
			}
		}
		return nil, nil
	}
	var result *QueryAnalysisDiagnostics
	for _, diagnostic := range diagnostics {
		if diagnostic.Section != "query_analysis" {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("multiple query-analysis diagnostics")
		}
		bounded := QueryAnalysisDiagnostics{
			PolicyVersion:       diagnostic.PolicyVersion,
			LimitsVersion:       diagnostic.LimitsVersion,
			Disposition:         diagnostic.Disposition,
			Fallback:            diagnostic.Fallback,
			OriginalRetained:    diagnostic.OriginalRetained,
			Categories:          append([]QueryAnalysisDiagnosticCount(nil), diagnostic.Categories...),
			HintCount:           diagnostic.HintCount,
			SignalCount:         diagnostic.SignalCount,
			SubqueryCount:       diagnostic.SubqueryCount,
			CandidateCount:      diagnostic.CandidateCount,
			Elapsed:             time.Duration(diagnostic.ElapsedNS),
			RolloutStage:        diagnostic.RolloutStage,
			NormalizationStatus: diagnostic.NormalizationStatus,
			TimeStatus:          diagnostic.TimeStatus,
		}
		if bounded.OriginalRetained && bounded.SignalCount == 0 {
			bounded.SignalCount = 1
		}
		if err := bounded.Validate(evaluationHardQueryAnalysisLimits()); err != nil {
			return nil, err
		}
		if string(bounded.PolicyVersion) != metadata.AnalysisVersion || string(bounded.LimitsVersion) != metadata.AnalysisLimitsVersion || !evaluationRolloutDiagnosticsMatch(metadata.RolloutDisposition, bounded.RolloutStage) {
			return nil, fmt.Errorf("query-analysis diagnostic identity mismatch")
		}
		if err := evaluationAnalysisMatchesExpectation(bounded, expectation); err != nil {
			return nil, err
		}
		result = &bounded
	}
	if (metadata.AnalysisVersion != "" || expectation != nil) && result == nil {
		return nil, fmt.Errorf("query-analysis diagnostics are missing")
	}
	return result, nil
}

func evaluationAnalysisMatchesExpectation(diagnostic QueryAnalysisDiagnostics, expectation *EvaluationAnalysisExpectation) error {
	if expectation == nil {
		return nil
	}
	if diagnostic.PolicyVersion != expectation.PolicyVersion || diagnostic.LimitsVersion != expectation.LimitsVersion ||
		diagnostic.Disposition != expectation.Disposition || diagnostic.Fallback != expectation.Fallback ||
		diagnostic.OriginalRetained != expectation.OriginalRetained {
		return fmt.Errorf("query-analysis result does not match fixture expectation")
	}
	if diagnostic.SignalCount > expectation.MaxSignalCount || diagnostic.SubqueryCount > expectation.MaxSubqueryCount || diagnostic.CandidateCount > expectation.MaxCandidateCount {
		return fmt.Errorf("query-analysis result exceeds fixture expectation")
	}
	actualCategories := make(map[QueryAnalysisDiagnosticCategory]struct{}, len(diagnostic.Categories))
	for _, count := range diagnostic.Categories {
		if count.Count > 0 {
			actualCategories[count.Category] = struct{}{}
		}
	}
	for _, expected := range expectation.Categories {
		if _, found := actualCategories[expected]; !found {
			return fmt.Errorf("query-analysis result is missing expected category")
		}
	}
	return nil
}

func evaluationRolloutDiagnosticsMatch(disposition, stage string) bool {
	if disposition == stage {
		return true
	}
	if stage == "original_only" {
		return disposition == "diagnostics_only" || disposition == "shadow" || disposition == "active" || disposition == "active_for_scope" || disposition == "disabled" || disposition == "rollback"
	}
	return disposition == "active_for_scope" && stage == "active"
}

func evaluationHardQueryAnalysisLimits() QueryAnalysisLimits {
	return QueryAnalysisLimits{
		Version:                QueryAnalysisLimitsVersionV1,
		MaxQueryBytes:          QueryAnalysisHardMaxQueryBytes,
		MaxHints:               QueryAnalysisHardMaxHints,
		MaxSignals:             QueryAnalysisHardMaxSignals,
		MaxSubqueries:          QueryAnalysisHardMaxSubqueries,
		MaxTermBytes:           QueryAnalysisHardMaxTermBytes,
		MaxSubqueryBytes:       QueryAnalysisHardMaxSubqueryBytes,
		MaxAnalysisWork:        QueryAnalysisHardMaxAnalysisWork,
		MaxCandidatesPerSignal: QueryAnalysisHardMaxCandidatesPerSignal,
		MaxAggregateCandidates: QueryAnalysisHardMaxAggregateCandidates,
		MaxElapsed:             QueryAnalysisHardMaxElapsed,
	}
}

func evaluationMissingCandidateDiagnostics(returned []EvaluationCandidateDiagnostic, activeAliases []string, fusionStrategy string) []EvaluationCandidateDiagnostic {
	returnedAliases := make(map[string]struct{}, len(returned))
	for _, diagnostic := range returned {
		returnedAliases[diagnostic.Alias] = struct{}{}
	}
	aliases := append([]string(nil), activeAliases...)
	sort.Strings(aliases)
	missing := make([]EvaluationCandidateDiagnostic, 0, len(aliases))
	for _, alias := range aliases {
		if _, found := returnedAliases[alias]; found {
			continue
		}
		if len(returned)+len(missing) >= evaluationReplayTopK {
			break
		}
		missing = append(missing, EvaluationCandidateDiagnostic{
			Alias:          alias,
			FusionStrategy: fusionStrategy,
			ChannelStatus: map[string]EvaluationChannelStatus{
				"lexical":  EvaluationChannelStatusUnavailable,
				"semantic": EvaluationChannelStatusUnavailable,
				"relation": EvaluationChannelStatusUnavailable,
				"chunk":    EvaluationChannelStatusUnavailable,
			},
			Disposition: EvaluationCandidateDispositionNotReturned,
		})
	}
	return missing
}

func evaluationChannelAvailability(availability map[FusionChannel]fusionChannelAvailability) map[string]EvaluationChannelStatus {
	result := make(map[string]EvaluationChannelStatus, 4)
	for _, channel := range []FusionChannel{FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk} {
		status := EvaluationChannelStatusUnavailable
		if availability[channel] == fusionChannelAvailable {
			status = EvaluationChannelStatusAvailable
		}
		result[string(channel)] = status
	}
	return result
}

func fusionStrategyIdentity(strategy FusionStrategy) string {
	return string(strategy.Name) + ":" + strategy.Version
}

type retrievalEvaluationObserver interface {
	RecordRetrievalEvaluation(context.Context, telemetry.RetrievalEvaluationEvent)
}

func (r *EvaluationRunner) rankingEvaluationObserver() retrievalEvaluationObserver {
	if observer, ok := r.observer.(retrievalEvaluationObserver); ok {
		return observer
	}
	return evaluationObserverAdapter{}
}

type evaluationObserverAdapter struct{}

func (evaluationObserverAdapter) RecordRetrievalEvaluation(context.Context, telemetry.RetrievalEvaluationEvent) {
}

func evaluationCandidateDiagnostics(candidates []EvaluationReplayCandidate, fusionStrategy string) []EvaluationCandidateDiagnostic {
	lexicalRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.lexicalScore })
	semanticRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.semanticScore })
	relationRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.relationScore })
	chunkRanks := evaluationBooleanChannelRanks(candidates, func(candidate EvaluationReplayCandidate) bool { return candidate.Chunk })
	diagnostics := make([]EvaluationCandidateDiagnostic, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Alias == "" || candidate.State != memory.MemoryStateActive {
			continue
		}
		diagnostics = append(diagnostics, EvaluationCandidateDiagnostic{
			Alias:          candidate.Alias,
			FusionStrategy: fusionStrategy,
			LexicalRank:    lexicalRanks[candidate.FinalRank],
			SemanticRank:   semanticRanks[candidate.FinalRank],
			RelationRank:   relationRanks[candidate.FinalRank],
			ChunkRank:      chunkRanks[candidate.FinalRank],
			ChannelStatus: map[string]EvaluationChannelStatus{
				"lexical":  evaluationChannelStatus(candidate.Lexical),
				"semantic": evaluationChannelStatus(candidate.Semantic),
				"relation": evaluationChannelStatus(candidate.Relation),
				"chunk":    evaluationChannelStatus(candidate.Chunk),
			},
			FinalRank:   candidate.FinalRank,
			Disposition: EvaluationCandidateDispositionReturned,
		})
	}
	return diagnostics
}

func evaluationBooleanChannelRanks(candidates []EvaluationReplayCandidate, included func(EvaluationReplayCandidate) bool) map[int]int {
	ranks := make(map[int]int)
	rank := 0
	for _, candidate := range candidates {
		if !included(candidate) {
			continue
		}
		rank++
		ranks[candidate.FinalRank] = rank
	}
	return ranks
}

func evaluationChannelStatus(included bool) EvaluationChannelStatus {
	if included {
		return EvaluationChannelStatusAvailable
	}
	return EvaluationChannelStatusUnavailable
}

func evaluationChannelRanks(candidates []EvaluationReplayCandidate, score func(EvaluationReplayCandidate) float64) map[int]int {
	indices := make([]int, 0, len(candidates))
	for index, candidate := range candidates {
		if score(candidate) != 0 {
			indices = append(indices, index)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return score(candidates[indices[i]]) > score(candidates[indices[j]])
	})
	ranks := make(map[int]int, len(indices))
	for index, candidateIndex := range indices {
		ranks[candidates[candidateIndex].FinalRank] = index + 1
	}
	return ranks
}

func evaluationSeedAliasKey(caseID, alias string) string {
	return caseID + "\x00" + alias
}
