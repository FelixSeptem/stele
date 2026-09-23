package retrieval

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

// EvaluationFixture is repository-owned test data for deterministic retrieval replay.
// Source content is only used to seed an explicitly owned test scope and is never
// included in diagnostics or reports.
type EvaluationFixture struct {
	Version string           `json:"version"`
	Cases   []EvaluationCase `json:"cases"`
}

const (
	evaluationFixtureMaxCases           = 1000
	evaluationFixtureMaxSourcesPerCase  = 100
	evaluationFixtureMaxEvidenceGroups  = 16
	evaluationFixtureMaxExcludedAliases = 100
	evaluationFixtureMaxSourceBytes     = 64 * 1024
)

// EvaluationCase defines one scoped retrieval assertion. Evidence aliases are stable
// fixture-local names; database identifiers are resolved only by the fixture seeder.
type EvaluationCase struct {
	ID                     string                         `json:"id"`
	Category               string                         `json:"category"`
	Scope                  memory.Scope                   `json:"scope"`
	Query                  string                         `json:"query"`
	Sources                []EvaluationSource             `json:"sources"`
	ExpectedEvidenceGroups [][]string                     `json:"expected_evidence_groups"`
	ExcludedAliases        []string                       `json:"excluded_aliases,omitempty"`
	ExpectedAnalysis       *EvaluationAnalysisExpectation `json:"expected_analysis,omitempty"`
	Planner                *EvaluationPlannerExpectation  `json:"planner,omitempty"`
	// Temporal declares the fact-valid contract for a case. It is optional so
	// existing pre-temporal fixtures keep their exact meaning.
	Temporal *EvaluationTemporalExpectation `json:"temporal,omitempty"`
}

// EvaluationPlannerExpectation is a bounded fixture contract containing only
// planner identities and aggregate resource expectations.
type EvaluationPlannerExpectation struct {
	PlannerVersion            string                `json:"planner_version,omitempty"`
	PolicyVersion             string                `json:"policy_version,omitempty"`
	Family                    RetrievalQueryFamily  `json:"family"`
	PlanIdentity              string                `json:"plan_identity"`
	Channels                  []FusionChannel       `json:"channels"`
	FallbackChannelCandidates map[FusionChannel]int `json:"fallback_channel_candidates,omitempty"`
	MaxCandidates             int                   `json:"max_candidates"`
	MaxCandidatesPerChannel   int                   `json:"max_candidates_per_channel,omitempty"`
	Protected                 bool                  `json:"protected,omitempty"`
	ExpectedPasses            int                   `json:"expected_passes"`
	FollowUpEligible          bool                  `json:"follow_up_eligible,omitempty"`
	RerankerEligible          bool                  `json:"reranker_eligible,omitempty"`
}

// EvaluationAnalysisExpectation declares only bounded aggregate outcomes. It
// cannot carry normalized text, derived queries, candidates, scores, or IDs.
type EvaluationAnalysisExpectation struct {
	PolicyVersion     QueryAnalysisPolicyVersion        `json:"policy_version"`
	LimitsVersion     QueryAnalysisLimitsVersion        `json:"limits_version"`
	Disposition       QueryAnalysisDisposition          `json:"disposition"`
	Fallback          QueryAnalysisFallbackCategory     `json:"fallback"`
	OriginalRetained  bool                              `json:"original_retained"`
	Categories        []QueryAnalysisDiagnosticCategory `json:"categories,omitempty"`
	MaxSignalCount    int                               `json:"max_signal_count"`
	MaxSubqueryCount  int                               `json:"max_subquery_count"`
	MaxCandidateCount int                               `json:"max_candidate_count"`
}

// EvaluationSource is a controlled source event used by one evaluation case.
type EvaluationSource struct {
	Alias             string             `json:"alias"`
	EventType         string             `json:"event_type"`
	Content           string             `json:"content"`
	Class             memory.MemoryClass `json:"class,omitempty"`
	State             memory.MemoryState `json:"state,omitempty"`
	FactCluster       string             `json:"fact_cluster,omitempty"`
	SourceEventID     string             `json:"source_event_id,omitempty"`
	ParentMemoryID    string             `json:"parent_memory_id,omitempty"`
	EmbeddingRevision string             `json:"embedding_revision,omitempty"`
	SourceTimestamp   time.Time          `json:"source_timestamp,omitempty"`
}

// EvaluationRankingMetadata identifies an evaluation report without exposing
// environment-specific provider settings or database connection information.
type EvaluationRankingMetadata struct {
	FixtureVersion              string           `json:"fixture_version"`
	RepresentationVersion       string           `json:"representation_version"`
	RankingVersion              string           `json:"ranking_version"`
	FusionStrategy              string           `json:"fusion_strategy,omitempty"`
	CompatibleEmbeddingRevision string           `json:"compatible_embedding_revision"`
	EmbeddingProvider           string           `json:"embedding_provider,omitempty"`
	EmbeddingVersion            string           `json:"embedding_version,omitempty"`
	LexicalMatchMode            LexicalMatchMode `json:"lexical_match_mode,omitempty"`
	PolicyVersion               string           `json:"policy_version"`
	AnalysisVersion             string           `json:"analysis_version,omitempty"`
	AnalysisLimitsVersion       string           `json:"analysis_limits_version,omitempty"`
	RolloutDisposition          string           `json:"rollout_disposition,omitempty"`
	QualityFeatureVersion       string           `json:"quality_feature_version,omitempty"`
	RerankerProvider            string           `json:"reranker_provider,omitempty"`
	RerankerVersion             string           `json:"reranker_version,omitempty"`
	RerankerMode                string           `json:"reranker_mode,omitempty"`
	PlannerVersion              string           `json:"planner_version,omitempty"`
	PlannerPolicyVersion        string           `json:"planner_policy_version,omitempty"`
	// TemporalPolicyVersion names the fact-valid policy a replay applied. It is
	// declared together with the coverage counts below: a report that claims a
	// temporal policy without declaring what it measured is not reproducible.
	TemporalPolicyVersion string `json:"temporal_policy_version,omitempty"`
	// TemporalCoverageVersion is the fixture-side version of the temporal
	// coverage contract, so a report can be matched to the fixture that produced it.
	TemporalCoverageVersion   string `json:"temporal_coverage_version,omitempty"`
	ContextEfficiencyVersion  string `json:"context_efficiency_version,omitempty"`
	CalibrationPolicyVersion  string `json:"calibration_policy_version,omitempty"`
	CalibrationSummaryVersion string `json:"calibration_summary_version,omitempty"`
}

// EvaluationSafetyFailureCategory is a stable non-sensitive failure reason.
type EvaluationSafetyFailureCategory string

const (
	EvaluationSafetyFailureInvalidFixtureScope EvaluationSafetyFailureCategory = "invalid_fixture_scope"
	EvaluationSafetyFailureCrossScope          EvaluationSafetyFailureCategory = "cross_scope_result"
	EvaluationSafetyFailureLifecycleVisibility EvaluationSafetyFailureCategory = "lifecycle_visibility"
	EvaluationSafetyFailureUnsafeDiagnostics   EvaluationSafetyFailureCategory = "unsafe_diagnostics"
	// EvaluationSafetyFailureStaleFactWin means a version outside the requested
	// fact-valid selection was served. It is the temporal counterpart of a
	// lifecycle leak: the result exists and is in scope, but it is not the
	// version whose validity the caller asked about.
	EvaluationSafetyFailureStaleFactWin EvaluationSafetyFailureCategory = "stale_fact_win"
	// EvaluationSafetyFailureValidityAmbiguity means the fixture expected one
	// unambiguous selected version but the case cannot distinguish versions.
	EvaluationSafetyFailureValidityAmbiguity EvaluationSafetyFailureCategory = "validity_ambiguity"
	// EvaluationSafetyFailureProvenanceMismatch means a derived artifact was
	// attributed to a source version other than the one it was built from.
	EvaluationSafetyFailureProvenanceMismatch EvaluationSafetyFailureCategory = "provenance_mismatch"
	// EvaluationSafetyFailureHiddenVersionLeakage means a version withheld from
	// current retrieval was nevertheless observable through a derived channel.
	EvaluationSafetyFailureHiddenVersionLeakage EvaluationSafetyFailureCategory = "hidden_version_leakage"
	// EvaluationSafetyFailureResourceOverflow means a temporal replay exceeded
	// its fixed candidate envelope. It is a hard failure even when quality
	// metrics improve, because temporal selection must remain bounded.
	EvaluationSafetyFailureResourceOverflow      EvaluationSafetyFailureCategory = "resource_overflow"
	EvaluationSafetyFailureGraphCycle            EvaluationSafetyFailureCategory = "graph_untruncated_cycle"
	EvaluationSafetyFailureGraphNondeterministic EvaluationSafetyFailureCategory = "graph_nondeterministic_replay"
	// EvaluationSafetyFailureAcrossValidityAmbiguity is the per-case category
	// reported when a temporal expectation contradicts itself.
	EvaluationSafetyFailureAcrossValidityAmbiguity EvaluationSafetyFailureCategory = "temporal_expectation_ambiguous"
)

// EvaluationTemporalCaseKind names the fact-valid scenario a temporal fixture
// case reproduces. Each kind maps to exactly one selector shape, so a fixture
// that declares the wrong selector for its scenario fails before seeding.
type EvaluationTemporalCaseKind string

const (
	// EvaluationTemporalCaseCurrentValid is an ordinary current search: only
	// versions valid at the evaluation instant may surface.
	EvaluationTemporalCaseCurrentValid EvaluationTemporalCaseKind = "current_valid"
	// EvaluationTemporalCaseExpired is a current search where the fact's
	// interval already ended, so the version must not surface.
	EvaluationTemporalCaseExpired EvaluationTemporalCaseKind = "expired"
	// EvaluationTemporalCaseAsOf is a historical point-in-time selection.
	EvaluationTemporalCaseAsOf EvaluationTemporalCaseKind = "as_of"
	// EvaluationTemporalCaseInterval is a half-open valid_during selection.
	EvaluationTemporalCaseInterval EvaluationTemporalCaseKind = "interval"
	// EvaluationTemporalCaseRetroactiveCorrection is an authorized correction
	// that closed a competing interval and named a successor.
	EvaluationTemporalCaseRetroactiveCorrection EvaluationTemporalCaseKind = "retroactive_correction"
	// EvaluationTemporalCaseLegacyCompatible is a row with no recorded validity
	// snapshot, which must stay retrievable as current.
	EvaluationTemporalCaseLegacyCompatible EvaluationTemporalCaseKind = "legacy_compatible"
	// EvaluationTemporalCaseStaleSimilarity is semantically similar but not
	// valid; similarity must not override fact-valid time.
	EvaluationTemporalCaseStaleSimilarity EvaluationTemporalCaseKind = "stale_similarity"
	// EvaluationTemporalCaseTemporalIsolation proves a temporal selector cannot
	// cross a scope boundary to reach a similarly-timed foreign fact.
	EvaluationTemporalCaseTemporalIsolation EvaluationTemporalCaseKind = "temporal_isolation"
)

var evaluationTemporalCaseKinds = []EvaluationTemporalCaseKind{
	EvaluationTemporalCaseCurrentValid,
	EvaluationTemporalCaseExpired,
	EvaluationTemporalCaseAsOf,
	EvaluationTemporalCaseInterval,
	EvaluationTemporalCaseRetroactiveCorrection,
	EvaluationTemporalCaseLegacyCompatible,
	EvaluationTemporalCaseStaleSimilarity,
	EvaluationTemporalCaseTemporalIsolation,
}

func (kind EvaluationTemporalCaseKind) valid() bool {
	for _, candidate := range evaluationTemporalCaseKinds {
		if kind == candidate {
			return true
		}
	}
	return false
}

// requiresSelector reports whether the scenario must carry an explicit non-current
// selector. Current, expired, and legacy cases describe the default selection and
// therefore must not carry one; isolation is expressed as an interval query that
// must not reach a foreign scope.
func (kind EvaluationTemporalCaseKind) requiresSelector() bool {
	switch kind {
	case EvaluationTemporalCaseAsOf, EvaluationTemporalCaseInterval,
		EvaluationTemporalCaseRetroactiveCorrection, EvaluationTemporalCaseTemporalIsolation:
		return true
	default:
		return false
	}
}

// EvaluationTemporalExpectation is the bounded temporal contract of one fixture
// case. It carries only the selector, an aggregate expectation, and aliases; it
// has no field able to hold content, identifiers, or provider payloads.
type EvaluationTemporalExpectation struct {
	Kind EvaluationTemporalCaseKind `json:"kind"`
	// EvaluationInstant anchors a current selection. It is required for current
	// and legacy cases and forbidden for historical ones, whose selector already
	// names its instant.
	EvaluationInstant *time.Time            `json:"evaluation_instant,omitempty"`
	Selector          *TemporalCaseSelector `json:"selector,omitempty"`
	// MinimumSelectedVersions is the number of expected-aliases that must be
	// selected. It is an aggregate count, never a list of identifiers.
	MinimumSelectedVersions int `json:"minimum_selected_versions"`
	// AllowUnselectedExpected marks a case where an expected alias is legitimately
	// absent under a stricter selection. Only temporal_isolation may use it.
	AllowUnselectedExpected bool `json:"allow_unselected_expected,omitempty"`
	// AuthorizedCorrection marks a retroactive correction that was explicitly
	// authorized, so an overlap is resolved rather than withheld.
	AuthorizedCorrection bool `json:"authorized_correction,omitempty"`
	// HiddenAliases name sources that must remain invisible through every
	// channel, including derived ones. They must not also be expected evidence.
	HiddenAliases []string `json:"hidden_aliases,omitempty"`
}

// TemporalCaseSelector is the fixture-level validity selector. It mirrors the
// public request fields rather than the runtime constraint so fixtures stay
// declarative and cannot smuggle an arbitrary evaluation instant.
type TemporalCaseSelector struct {
	Mode      memory.TemporalSelectionMode `json:"mode"`
	AsOf      *time.Time                   `json:"as_of,omitempty"`
	ValidFrom *time.Time                   `json:"valid_from,omitempty"`
	ValidTo   *time.Time                   `json:"valid_to,omitempty"`
}

// Constraint converts the declarative selector into the runtime constraint,
// rejecting a shape that the runtime would refuse anyway.
func (selector TemporalCaseSelector) Constraint() (memory.TemporalConstraint, error) {
	constraint := memory.TemporalConstraint{Mode: selector.Mode, AsOf: selector.AsOf, ValidFrom: selector.ValidFrom, ValidTo: selector.ValidTo}
	if err := constraint.Validate(); err != nil {
		return memory.TemporalConstraint{}, err
	}
	return constraint, nil
}

const (
	evaluationTemporalFixtureMinimum   = 8
	evaluationTemporalMaxHiddenAliases = 100
)

// EvaluationSafetyFailure records an aggregate safety outcome. It deliberately has
// no memory, source, scope, query, database, or error payload fields.
type EvaluationSafetyFailure struct {
	Category EvaluationSafetyFailureCategory `json:"category"`
	Count    int                             `json:"count"`
}

type evaluationFailure struct {
	category EvaluationSafetyFailureCategory
}

func (e evaluationFailure) Error() string {
	return "retrieval evaluation failed: " + string(e.category)
}

// NewEvaluationFailure intentionally drops the cause text. Evaluation failures can
// cross a database or fixture boundary, where a cause may include source content,
// connection details, or hidden identifiers. Callers retain internal causes only in
// their local logs and return this stable category to reports and administrative paths.
func NewEvaluationFailure(category EvaluationSafetyFailureCategory, cause string) error {
	_ = cause
	if !evaluationSafetyFailureCategoryValid(category) {
		category = EvaluationSafetyFailureUnsafeDiagnostics
	}
	return evaluationFailure{category: category}
}

// EvaluationMetricReport holds quality measurements calculated from visible,
// in-scope results only.
type EvaluationMetricReport struct {
	RecallAt1                  float64                   `json:"recall_at_1"`
	RecallAt5                  float64                   `json:"recall_at_5"`
	RecallAt10                 float64                   `json:"recall_at_10"`
	MRR                        float64                   `json:"mrr"`
	NDCGAt1                    float64                   `json:"ndcg_at_1"`
	NDCGAt5                    float64                   `json:"ndcg_at_5"`
	NDCGAt10                   float64                   `json:"ndcg_at_10"`
	MultiHopEvidenceCoverage   float64                   `json:"multi_hop_evidence_coverage"`
	DuplicateRate              float64                   `json:"duplicate_rate"`
	CandidatePoolSize          int                       `json:"candidate_pool_size"`
	P50LatencyMS               float64                   `json:"p50_latency_ms"`
	P95LatencyMS               float64                   `json:"p95_latency_ms"`
	ProtectedRecall            float64                   `json:"protected_recall"`
	EvidenceCoverage           float64                   `json:"evidence_coverage"`
	BudgetOmissionRate         float64                   `json:"budget_omission_rate"`
	TemporalEvidenceCoverage   float64                   `json:"temporal_evidence_coverage"`
	AnalysisSignalCount        int                       `json:"analysis_signal_count"`
	AnalysisSubqueryCount      int                       `json:"analysis_subquery_count"`
	AnalysisCandidateCount     int                       `json:"analysis_candidate_count"`
	FirstPassEvidenceCoverage  float64                   `json:"first_pass_evidence_coverage,omitempty"`
	SecondPassEvidenceCoverage float64                   `json:"second_pass_evidence_coverage,omitempty"`
	SecondPassEvidenceGain     float64                   `json:"second_pass_evidence_gain,omitempty"`
	SecondPassCount            int                       `json:"second_pass_count,omitempty"`
	MaxPassesObserved          int                       `json:"max_passes_observed,omitempty"`
	MaxPlannerCandidates       int                       `json:"max_planner_candidates,omitempty"`
	PlannerFallbackRate        float64                   `json:"planner_fallback_rate,omitempty"`
	PlannerRerankerUseRate     float64                   `json:"planner_reranker_use_rate,omitempty"`
	ContextEfficiency          *ContextEfficiencyMetrics `json:"context_efficiency,omitempty"`
}

// EvaluationCaseReport is a bounded per-case contribution to an evaluation report.
type EvaluationCaseReport struct {
	CaseID                     string                         `json:"case_id"`
	Category                   string                         `json:"category,omitempty"`
	Metrics                    EvaluationMetricReport         `json:"metrics"`
	SafetyFailures             []EvaluationSafetyFailure      `json:"safety_failures,omitempty"`
	CandidatePoolSize          int                            `json:"candidate_pool_size"`
	LatencyMS                  float64                        `json:"latency_ms"`
	ChunkDerivedCount          int                            `json:"chunk_derived_count,omitempty"`
	AnalysisSignalCount        int                            `json:"analysis_signal_count,omitempty"`
	AnalysisSubqueryCount      int                            `json:"analysis_subquery_count,omitempty"`
	AnalysisCandidateCount     int                            `json:"analysis_candidate_count,omitempty"`
	AnalysisOriginalRetained   bool                           `json:"analysis_original_retained,omitempty"`
	AnalysisDisposition        QueryAnalysisDisposition       `json:"analysis_disposition,omitempty"`
	AnalysisFallback           QueryAnalysisFallbackCategory  `json:"analysis_fallback,omitempty"`
	AnalysisCategories         []QueryAnalysisDiagnosticCount `json:"analysis_categories,omitempty"`
	AnalysisElapsedMS          float64                        `json:"analysis_elapsed_ms,omitempty"`
	ChangedRankCount           int                            `json:"changed_rank_count,omitempty"`
	RerankFallback             string                         `json:"rerank_fallback,omitempty"`
	QueryFamily                RetrievalQueryFamily           `json:"query_family,omitempty"`
	PlannerIdentity            string                         `json:"planner_identity,omitempty"`
	PlannerVersion             string                         `json:"planner_version,omitempty"`
	PlannerPolicyVersion       string                         `json:"planner_policy_version,omitempty"`
	PassCount                  int                            `json:"pass_count,omitempty"`
	FirstPassEvidenceCoverage  float64                        `json:"first_pass_evidence_coverage,omitempty"`
	SecondPassEvidenceCoverage float64                        `json:"second_pass_evidence_coverage,omitempty"`
	SecondPassEvidenceGain     float64                        `json:"second_pass_evidence_gain,omitempty"`
	MaxPlannerCandidates       int                            `json:"max_planner_candidates,omitempty"`
	FirstPassCandidates        int                            `json:"first_pass_candidates,omitempty"`
	SecondPassCandidates       int                            `json:"second_pass_candidates,omitempty"`
	FirstPassLatencyMS         float64                        `json:"first_pass_latency_ms,omitempty"`
	SecondPassLatencyMS        float64                        `json:"second_pass_latency_ms,omitempty"`
	PlannerFallbackCategory    string                         `json:"planner_fallback_category,omitempty"`
	PlannerRerankerUsed        bool                           `json:"planner_reranker_used,omitempty"`
	PlannerProtected           bool                           `json:"planner_protected,omitempty"`
	// TemporalKind names the fact-valid scenario this case replayed. It is empty
	// for pre-temporal cases, so an ordinary report is unchanged.
	TemporalKind EvaluationTemporalCaseKind `json:"temporal_kind,omitempty"`
	// TemporalMode and TemporalSelectorKind describe the selection that was
	// actually applied. Only the bounded mode and the selector's coarse shape
	// appear; the instants themselves are not reportable.
	TemporalMode                memory.TemporalSelectionMode `json:"temporal_mode,omitempty"`
	TemporalSelectorKind        string                       `json:"temporal_selector_kind,omitempty"`
	TemporalSelectedVersions    int                          `json:"temporal_selected_versions,omitempty"`
	TemporalHiddenAliasCount    int                          `json:"temporal_hidden_alias_count,omitempty"`
	TemporalStaleVersions       int                          `json:"temporal_stale_versions,omitempty"`
	TemporalAmbiguousVersions   int                          `json:"temporal_ambiguous_versions,omitempty"`
	TemporalConflictDisposition string                       `json:"temporal_conflict_disposition,omitempty"`
	TemporalProvenanceMismatch  int                          `json:"temporal_provenance_mismatch,omitempty"`
	TemporalHiddenVersionLeak   int                          `json:"temporal_hidden_version_leak,omitempty"`
	// TemporalFallbackCategory names the bounded reason a temporal selection was
	// not applied in full. An empty value means the selection was honoured.
	TemporalFallbackCategory string `json:"temporal_fallback_category,omitempty"`
}

// EvaluationTemporalCoverage aggregates the temporal evidence of a replay. It
// carries counts and bounded categories only, so a report can prove which
// scenarios ran without exposing instants, aliases, or identifiers.
type EvaluationTemporalCoverage struct {
	PolicyVersion      string         `json:"policy_version,omitempty"`
	CoverageVersion    string         `json:"coverage_version,omitempty"`
	Cases              int            `json:"cases,omitempty"`
	KindCounts         map[string]int `json:"kind_counts,omitempty"`
	ModeCounts         map[string]int `json:"mode_counts,omitempty"`
	SelectedVersions   int            `json:"selected_versions,omitempty"`
	StaleVersions      int            `json:"stale_versions,omitempty"`
	AmbiguousVersions  int            `json:"ambiguous_versions,omitempty"`
	ProvenanceMismatch int            `json:"provenance_mismatch,omitempty"`
	HiddenVersionLeaks int            `json:"hidden_version_leaks,omitempty"`
	FallbackCounts     map[string]int `json:"fallback_counts,omitempty"`
	DispositionCounts  map[string]int `json:"disposition_counts,omitempty"`
	EvaluationInstants int            `json:"evaluation_instants,omitempty"`
}

// EvaluationReport is the versioned data model rendered by local and CI replay.
type EvaluationReport struct {
	Metadata                   EvaluationRankingMetadata        `json:"metadata"`
	Cases                      []EvaluationCaseReport           `json:"cases"`
	Metrics                    EvaluationMetricReport           `json:"metrics"`
	SafetyFailures             []EvaluationSafetyFailure        `json:"safety_failures,omitempty"`
	DispositionAggregates      map[string]int                   `json:"disposition_aggregates,omitempty"`
	AnalysisFallbackAggregates map[string]int                   `json:"analysis_fallback_aggregates,omitempty"`
	AnalysisCategoryAggregates map[string]int                   `json:"analysis_category_aggregates,omitempty"`
	GeneratedAt                time.Time                        `json:"generated_at"`
	RealStack                  bool                             `json:"real_stack"`
	ReleaseEligible            bool                             `json:"release_eligible"`
	DeterministicReplay        bool                             `json:"deterministic_replay,omitempty"`
	RollbackTested             bool                             `json:"rollback_tested,omitempty"`
	ChangedRankCount           int                              `json:"changed_rank_count,omitempty"`
	RerankFallbackCounts       map[string]int                   `json:"rerank_fallback_counts,omitempty"`
	PlannerEvidence            EvaluationPlannerReleaseEvidence `json:"planner_evidence,omitempty"`
	// TemporalCoverage reports the fact-valid scenarios this replay measured. It
	// is omitted entirely for a report produced from a pre-temporal fixture, so
	// the ordinary report shape is unchanged.
	TemporalCoverage       *EvaluationTemporalCoverage       `json:"temporal_coverage,omitempty"`
	GraphTraversalEvidence *EvaluationGraphTraversalEvidence `json:"graph_traversal_evidence,omitempty"`
}

type EvaluationGraphTraversalEvidence struct {
	CandidateCountByHop  map[string]int `json:"candidate_count_by_hop,omitempty"`
	TruncationRate       float64        `json:"truncation_rate,omitempty"`
	FallbackRate         float64        `json:"fallback_rate,omitempty"`
	CitationCoverage     float64        `json:"citation_coverage,omitempty"`
	EligibleRelationRate float64        `json:"eligible_relation_hit_rate,omitempty"`
	LatencyBucket        string         `json:"latency_bucket,omitempty"`
	QualityDelta         float64        `json:"quality_delta,omitempty"`
}

func (e *EvaluationGraphTraversalEvidence) validate() error {
	if e == nil {
		return nil
	}
	for hop, count := range e.CandidateCountByHop {
		if (hop != "zero" && hop != "one" && hop != "two" && hop != "three") || count < 0 || count > QueryAnalysisHardMaxAggregateCandidates {
			return fmt.Errorf("graph traversal candidate aggregate is invalid")
		}
	}
	for _, value := range []float64{e.TruncationRate, e.FallbackRate, e.CitationCoverage, e.EligibleRelationRate} {
		if !boundedRate(value) {
			return fmt.Errorf("graph traversal evidence rate is invalid")
		}
	}
	if e.QualityDelta < -1 || e.QualityDelta > 1 || !evaluationSafeIdentity(e.LatencyBucket) {
		return fmt.Errorf("graph traversal evidence identity is invalid")
	}
	return nil
}

type EvaluationPlannerReleaseEvidence struct {
	Compatible        bool    `json:"compatible"`
	SafetyFailures    int     `json:"safety_failures"`
	ResourceFailure   bool    `json:"resource_failure,omitempty"`
	MaxPasses         int     `json:"max_passes"`
	MaxCandidateCount int     `json:"max_candidate_count"`
	FallbackRate      float64 `json:"fallback_rate"`
	RerankerSafe      bool    `json:"reranker_safe"`
	RollbackTested    bool    `json:"rollback_tested"`
}

// EvaluationFixtureSeed is the alias-to-record resolution produced by a fixture
// seeder. It intentionally excludes source content and raw event identifiers.
type EvaluationFixtureSeed struct {
	FixtureVersion string                  `json:"fixture_version"`
	Aliases        []EvaluationSeededAlias `json:"aliases"`
}

// EvaluationSeededAlias is safe to use for replay matching and diagnostics.
type EvaluationSeededAlias struct {
	CaseID      string             `json:"case_id"`
	Alias       string             `json:"alias"`
	Scope       memory.Scope       `json:"scope"`
	MemoryID    string             `json:"memory_id"`
	RawEventID  string             `json:"-"`
	State       memory.MemoryState `json:"state"`
	FactCluster string             `json:"fact_cluster,omitempty"`
}

// EvaluationReleasePolicy defines release decisions separately from measured reports.
type EvaluationReleasePolicy struct {
	Version                       string                         `json:"version"`
	ProtectedCutoffs              []int                          `json:"protected_cutoffs"`
	ProtectedCategories           []string                       `json:"protected_categories,omitempty"`
	ProtectedFamilies             []RetrievalQueryFamily         `json:"protected_families,omitempty"`
	MaxRecallRegression           float64                        `json:"max_recall_regression"`
	MaxMultiHopCoverageRegression float64                        `json:"max_multi_hop_coverage_regression"`
	MaxEvidenceCoverageRegression float64                        `json:"max_evidence_coverage_regression,omitempty"`
	MaxBudgetOmissionIncrease     float64                        `json:"max_budget_omission_increase,omitempty"`
	MaxP95LatencyMS               int                            `json:"max_p95_latency_ms"`
	MaxP95LatencyRegressionMS     int                            `json:"max_p95_latency_regression_ms,omitempty"`
	ResourceBudget                EvaluationResourceBudget       `json:"resource_budget,omitempty"`
	Prerequisites                 EvaluationPrerequisites        `json:"prerequisites,omitempty"`
	Rollback                      EvaluationRollbackContract     `json:"rollback,omitempty"`
	Retention                     EvaluationRetentionContract    `json:"retention,omitempty"`
	Planner                       EvaluationPlannerReleasePolicy `json:"planner,omitempty"`
	MaxDuplicateTokenRate         float64                        `json:"max_duplicate_token_rate,omitempty"`
	MaxStaleTokenRate             float64                        `json:"max_stale_token_rate,omitempty"`
	MaxQualityPerBudgetRegression float64                        `json:"max_quality_per_budget_regression,omitempty"`
	RequireDeterministicReplay    bool                           `json:"require_deterministic_replay,omitempty"`
}

type EvaluationPlannerReleasePolicy struct {
	RequireCompatible     bool    `json:"require_compatible,omitempty"`
	MaxPasses             int     `json:"max_passes,omitempty"`
	MaxCandidateCount     int     `json:"max_candidate_count,omitempty"`
	MaxFallbackRate       float64 `json:"max_fallback_rate,omitempty"`
	RequireRollbackTested bool    `json:"require_rollback_tested,omitempty"`
}

// EvaluationResourceBudget bounds work consumed by a release evaluation.
type EvaluationResourceBudget struct {
	MaxCases      int `json:"max_cases,omitempty"`
	MaxCandidates int `json:"max_candidates,omitempty"`
	MaxElapsedMS  int `json:"max_elapsed_ms,omitempty"`
}
type EvaluationPrerequisites struct {
	RequireRealStack bool `json:"require_real_stack,omitempty"`
	RequireOwnedDSN  bool `json:"require_owned_dsn,omitempty"`
}
type EvaluationRollbackContract struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}
type EvaluationRetentionContract struct {
	WindowHours int    `json:"window_hours,omitempty"`
	Owner       string `json:"owner,omitempty"`
}

// EvaluationReleaseDecision is the bounded policy result for a candidate report.
type EvaluationReleaseDecision struct {
	PolicyVersion string   `json:"policy_version"`
	Eligible      bool     `json:"eligible"`
	HardFailures  []string `json:"hard_failures,omitempty"`
	Advisories    []string `json:"advisories,omitempty"`
}

// MarshalEvaluationReport is the machine-readable report boundary. Its model only
// contains version metadata, aggregate measurements, categories, and fixture aliases;
// it intentionally has no source payload, DSN, credential, or database-error field.
func MarshalEvaluationReport(report EvaluationReport) ([]byte, error) {
	if err := report.Metadata.Validate(); err != nil {
		return nil, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if err := report.validateSafeOutput(); err != nil {
		return nil, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	return json.Marshal(report)
}

func (m EvaluationRankingMetadata) Validate() error {
	if strings.TrimSpace(m.FixtureVersion) == "" {
		return fmt.Errorf("fixture version is required")
	}
	if strings.TrimSpace(m.RepresentationVersion) == "" {
		return fmt.Errorf("representation version is required")
	}
	if strings.TrimSpace(m.RankingVersion) == "" {
		return fmt.Errorf("ranking version is required")
	}
	if !evaluationSafeIdentity(m.FusionStrategy) {
		return fmt.Errorf("fusion strategy identity is invalid")
	}
	if strings.TrimSpace(m.CompatibleEmbeddingRevision) == "" {
		return fmt.Errorf("compatible embedding revision is required")
	}
	if m.LexicalMatchMode != "" && m.LexicalMatchMode != LexicalMatchAllTerms && m.LexicalMatchMode != LexicalMatchAnyTerms {
		return fmt.Errorf("unsupported lexical match mode %q", m.LexicalMatchMode)
	}
	if strings.TrimSpace(m.PolicyVersion) == "" {
		return fmt.Errorf("policy version is required")
	}
	if m.AnalysisVersion != "" && !evaluationSafeIdentity(m.AnalysisVersion) {
		return fmt.Errorf("analysis version is invalid")
	}
	if m.AnalysisLimitsVersion != "" && !evaluationSafeIdentity(m.AnalysisLimitsVersion) {
		return fmt.Errorf("analysis limits version is invalid")
	}
	if m.RolloutDisposition != "" && !evaluationSafeIdentity(m.RolloutDisposition) {
		return fmt.Errorf("rollout disposition is invalid")
	}
	if m.RolloutDisposition != "" && !evaluationRolloutDispositionValid(m.RolloutDisposition) {
		return fmt.Errorf("rollout disposition is unsupported")
	}
	for name, value := range map[string]string{"quality feature version": m.QualityFeatureVersion, "reranker provider": m.RerankerProvider, "reranker version": m.RerankerVersion, "reranker mode": m.RerankerMode} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	for name, value := range map[string]string{"planner version": m.PlannerVersion, "planner policy version": m.PlannerPolicyVersion} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	if (m.PlannerVersion == "") != (m.PlannerPolicyVersion == "") {
		return fmt.Errorf("planner version and policy version must be declared together")
	}
	// A temporal policy claim without a coverage contract would let a report
	// appear temporal-aware while measuring nothing about fact validity.
	if (m.TemporalPolicyVersion == "") != (m.TemporalCoverageVersion == "") {
		return fmt.Errorf("temporal policy version and coverage version must be declared together")
	}
	for name, value := range map[string]string{"temporal policy version": m.TemporalPolicyVersion, "temporal coverage version": m.TemporalCoverageVersion} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	for name, value := range map[string]string{"context efficiency version": m.ContextEfficiencyVersion, "calibration policy version": m.CalibrationPolicyVersion, "calibration summary version": m.CalibrationSummaryVersion} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	for name, value := range map[string]string{"embedding provider": m.EmbeddingProvider, "embedding version": m.EmbeddingVersion} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	if m.RerankerMode != "" && m.RerankerMode != "disabled" && m.RerankerMode != "diagnostics_only" && m.RerankerMode != "shadow" && m.RerankerMode != "active_for_scope" {
		return fmt.Errorf("reranker mode is invalid")
	}
	return nil
}

func evaluationRolloutDispositionValid(disposition string) bool {
	switch disposition {
	case "original_only", "diagnostics_only", "shadow", "active", "active_for_scope", "disabled", "rollback":
		return true
	default:
		return false
	}
}

func (report EvaluationReport) validateSafeOutput() error {
	if err := validateEvaluationSafetyFailures(report.SafetyFailures); err != nil {
		return err
	}
	if report.Metrics.ContextEfficiency != nil {
		if err := report.Metrics.ContextEfficiency.Validate(); err != nil {
			return err
		}
	}
	if err := report.GraphTraversalEvidence.validate(); err != nil {
		return err
	}
	if report.PlannerEvidence.SafetyFailures < 0 || report.PlannerEvidence.SafetyFailures > QueryAnalysisHardMaxDiagnosticCount ||
		report.PlannerEvidence.MaxPasses < 0 || report.PlannerEvidence.MaxPasses > 2 ||
		report.PlannerEvidence.MaxCandidateCount < 0 || report.PlannerEvidence.MaxCandidateCount > 5000 ||
		!boundedRate(report.PlannerEvidence.FallbackRate) {
		return fmt.Errorf("evaluation planner release evidence is invalid")
	}
	for name, value := range map[string]string{"quality feature version": report.Metadata.QualityFeatureVersion, "reranker provider": report.Metadata.RerankerProvider, "reranker version": report.Metadata.RerankerVersion, "reranker mode": report.Metadata.RerankerMode} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	for name, value := range map[string]string{"embedding provider": report.Metadata.EmbeddingProvider, "embedding version": report.Metadata.EmbeddingVersion} {
		if !evaluationSafeIdentity(value) {
			return fmt.Errorf("%s identity is invalid", name)
		}
	}
	if report.Metadata.RerankerMode != "" && report.Metadata.RerankerMode != "disabled" && report.Metadata.RerankerMode != "diagnostics_only" && report.Metadata.RerankerMode != "shadow" && report.Metadata.RerankerMode != "active_for_scope" {
		return fmt.Errorf("reranker mode is invalid")
	}
	for _, item := range report.Cases {
		if !evaluationSafeIdentity(item.CaseID) || !evaluationSafeIdentity(item.Category) {
			return fmt.Errorf("evaluation case identity is invalid")
		}
		if item.CandidatePoolSize < 0 || item.CandidatePoolSize > QueryAnalysisHardMaxAggregateCandidates ||
			item.AnalysisSignalCount < 0 || item.AnalysisSignalCount > QueryAnalysisHardMaxSignals ||
			item.AnalysisSubqueryCount < 0 || item.AnalysisSubqueryCount > QueryAnalysisHardMaxSubqueries ||
			item.AnalysisCandidateCount < 0 || item.AnalysisCandidateCount > QueryAnalysisHardMaxAggregateCandidates {
			return fmt.Errorf("evaluation case count is outside its bound")
		}
		if item.QueryFamily != "" && !item.QueryFamily.valid() {
			return fmt.Errorf("evaluation query family is invalid")
		}
		for _, value := range []string{item.PlannerIdentity, item.PlannerVersion, item.PlannerPolicyVersion, item.PlannerFallbackCategory} {
			if !evaluationSafeIdentity(value) {
				return fmt.Errorf("evaluation planner identity is invalid")
			}
		}
		if item.PassCount < 0 || item.PassCount > 2 || item.MaxPlannerCandidates < 0 || item.MaxPlannerCandidates > 5000 || item.FirstPassCandidates < 0 || item.SecondPassCandidates < 0 || item.FirstPassCandidates+item.SecondPassCandidates > 5000 || item.FirstPassLatencyMS < 0 || item.SecondPassLatencyMS < 0 ||
			!boundedRate(item.FirstPassEvidenceCoverage) || !boundedRate(item.SecondPassEvidenceCoverage) || item.SecondPassEvidenceGain < 0 || item.SecondPassEvidenceGain > 1 {
			return fmt.Errorf("evaluation planner metrics are invalid")
		}
		if err := validateEvaluationSafetyFailures(item.SafetyFailures); err != nil {
			return err
		}
		if item.AnalysisDisposition != "" && !item.AnalysisDisposition.valid() {
			return fmt.Errorf("evaluation analysis disposition is invalid")
		}
		if item.AnalysisFallback != "" && !item.AnalysisFallback.valid() {
			return fmt.Errorf("evaluation analysis fallback is invalid")
		}
		for _, count := range item.AnalysisCategories {
			if !count.Category.valid() || count.Count < 0 || count.Count > QueryAnalysisHardMaxDiagnosticCount {
				return fmt.Errorf("evaluation analysis category is invalid")
			}
		}
	}
	for key, count := range report.DispositionAggregates {
		if !evaluationCandidateDispositionValid(EvaluationCandidateDisposition(key)) || count < 0 || count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("evaluation disposition aggregate is invalid")
		}
	}
	for key, count := range report.AnalysisFallbackAggregates {
		if !QueryAnalysisFallbackCategory(key).valid() || count < 0 || count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("evaluation fallback aggregate is invalid")
		}
	}
	for key, count := range report.AnalysisCategoryAggregates {
		if !QueryAnalysisDiagnosticCategory(key).valid() || count < 0 || count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("evaluation category aggregate is invalid")
		}
	}
	if err := validateEvaluationTemporalCoverage(report.TemporalCoverage); err != nil {
		return err
	}
	// A case that declares a temporal kind must also be counted by the coverage
	// aggregate, otherwise a report could claim a scenario ran while the summary
	// silently omits it.
	declared := 0
	for _, item := range report.Cases {
		if item.TemporalKind != "" {
			declared++
			if !item.TemporalKind.valid() {
				return fmt.Errorf("evaluation temporal kind is invalid")
			}
			if item.TemporalMode != "" && !evaluationTemporalModeValid(item.TemporalMode) {
				return fmt.Errorf("evaluation temporal mode is invalid")
			}
			if item.TemporalSelectorKind != "" && !evaluationSafeIdentity(item.TemporalSelectorKind) {
				return fmt.Errorf("evaluation temporal selector kind is invalid")
			}
			if !evaluationSafeIdentity(item.TemporalFallbackCategory) || !evaluationSafeIdentity(item.TemporalConflictDisposition) {
				return fmt.Errorf("evaluation temporal category is invalid")
			}
			for _, value := range []int{item.TemporalSelectedVersions, item.TemporalHiddenAliasCount, item.TemporalStaleVersions, item.TemporalAmbiguousVersions, item.TemporalProvenanceMismatch, item.TemporalHiddenVersionLeak} {
				if value < 0 || value > QueryAnalysisHardMaxAggregateCandidates {
					return fmt.Errorf("evaluation temporal count is outside its bound")
				}
			}
		}
	}
	if declared > 0 && report.TemporalCoverage == nil {
		return fmt.Errorf("evaluation temporal coverage is required when temporal cases are present")
	}
	if report.TemporalCoverage != nil && report.TemporalCoverage.Cases != declared {
		return fmt.Errorf("evaluation temporal coverage disagrees with its cases")
	}
	return nil
}

// validateEvaluationTemporalCoverage keeps the aggregate bounded and free of
// anything that could carry an instant, alias, scope, or identifier.
func validateEvaluationTemporalCoverage(coverage *EvaluationTemporalCoverage) error {
	if coverage == nil {
		return nil
	}
	if coverage.Cases <= 0 || coverage.Cases > evaluationFixtureMaxCases {
		return fmt.Errorf("evaluation temporal coverage case count is invalid")
	}
	if !evaluationSafeIdentity(coverage.PolicyVersion) || !evaluationSafeIdentity(coverage.CoverageVersion) {
		return fmt.Errorf("evaluation temporal coverage identity is invalid")
	}
	for name, count := range coverage.KindCounts {
		if !evaluationTemporalCaseKindNameValid(name) {
			return fmt.Errorf("evaluation temporal kind is unsupported")
		}
		if count <= 0 || count > evaluationFixtureMaxCases {
			return fmt.Errorf("evaluation temporal kind count is invalid")
		}
	}
	for name, count := range coverage.ModeCounts {
		if !evaluationTemporalModeValid(memory.TemporalSelectionMode(name)) || count <= 0 || count > evaluationFixtureMaxCases {
			return fmt.Errorf("evaluation temporal mode count is invalid")
		}
	}
	for name, count := range coverage.FallbackCounts {
		if !evaluationSafeIdentity(name) || count <= 0 || count > evaluationFixtureMaxCases {
			return fmt.Errorf("evaluation temporal fallback count is invalid")
		}
	}
	for name, count := range coverage.DispositionCounts {
		if !memory.TemporalConflictDisposition(name).Valid() || count <= 0 || count > evaluationFixtureMaxCases {
			return fmt.Errorf("evaluation temporal disposition count is invalid")
		}
	}
	for _, value := range []int{coverage.SelectedVersions, coverage.StaleVersions, coverage.AmbiguousVersions, coverage.ProvenanceMismatch, coverage.HiddenVersionLeaks, coverage.EvaluationInstants} {
		if value < 0 || value > QueryAnalysisHardMaxAggregateCandidates {
			return fmt.Errorf("evaluation temporal coverage count is outside its bound")
		}
	}
	return nil
}

func evaluationTemporalModeValid(mode memory.TemporalSelectionMode) bool {
	switch mode {
	case memory.TemporalSelectionCurrent, memory.TemporalSelectionAsOf, memory.TemporalSelectionDuring:
		return true
	default:
		return false
	}
}

func evaluationTemporalCaseKindNameValid(name string) bool {
	return EvaluationTemporalCaseKind(name).valid()
}

func validateEvaluationSafetyFailures(failures []EvaluationSafetyFailure) error {
	for _, failure := range failures {
		if !evaluationSafetyFailureCategoryValid(failure.Category) || failure.Count <= 0 || failure.Count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("evaluation safety failure is invalid")
		}
	}
	return nil
}

func evaluationCandidateDispositionValid(disposition EvaluationCandidateDisposition) bool {
	switch disposition {
	case EvaluationCandidateDispositionReturned, EvaluationCandidateDispositionNotReturned:
		return true
	default:
		return false
	}
}

func evaluationSafeIdentity(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 128 {
		return false
	}
	if strings.Contains(value, "..") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-' || character == '.' || character == ':' || character == '/' || character == '=' || character == ',' {
			continue
		}
		return false
	}
	return true
}

func (f EvaluationFixture) Validate() error {
	if strings.TrimSpace(f.Version) == "" {
		return fmt.Errorf("fixture version is required")
	}
	if len(f.Cases) == 0 {
		return fmt.Errorf("fixture must include at least one case")
	}
	if len(f.Cases) > evaluationFixtureMaxCases {
		return fmt.Errorf("fixture case count exceeds the safe bound")
	}

	caseIDs := make(map[string]struct{}, len(f.Cases))
	temporalKinds := make(map[EvaluationTemporalCaseKind]struct{}, len(evaluationTemporalCaseKinds))
	temporalCases := 0
	for _, item := range f.Cases {
		if err := item.validate(); err != nil {
			return err
		}
		id := strings.TrimSpace(item.ID)
		if _, exists := caseIDs[id]; exists {
			return fmt.Errorf("duplicate case id")
		}
		caseIDs[id] = struct{}{}
		if item.Temporal != nil {
			temporalCases++
			if _, duplicate := temporalKinds[item.Temporal.Kind]; duplicate {
				return fmt.Errorf("duplicate temporal fixture kind")
			}
			temporalKinds[item.Temporal.Kind] = struct{}{}
		}
	}
	// A temporal fixture is only meaningful if it covers every safe/unsafe
	// scenario the release gate reasons about. A partial fixture would let a
	// release pass while an untested scenario remains unmeasured.
	if temporalCases > 0 {
		if temporalCases < evaluationTemporalFixtureMinimum {
			return fmt.Errorf("temporal fixture must cover every temporal kind")
		}
		for _, kind := range evaluationTemporalCaseKinds {
			if _, covered := temporalKinds[kind]; !covered {
				return fmt.Errorf("temporal fixture is missing a required temporal kind")
			}
		}
	}
	return nil
}

func (c EvaluationCase) validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("case id is required")
	}
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Query) == "" {
		return fmt.Errorf("fixture query is required")
	}
	if !utf8.ValidString(c.Query) || len(c.Query) > QueryAnalysisHardMaxQueryBytes {
		return fmt.Errorf("fixture query exceeds the safe input bound")
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("fixture case must include at least one source")
	}
	if len(c.Sources) > evaluationFixtureMaxSourcesPerCase {
		return fmt.Errorf("fixture source count exceeds the safe bound")
	}

	aliases := make(map[string]struct{}, len(c.Sources))
	for _, source := range c.Sources {
		alias := strings.TrimSpace(source.Alias)
		if alias == "" {
			return fmt.Errorf("source alias is required")
		}
		if !evaluationSafeIdentity(alias) {
			return fmt.Errorf("source alias is invalid")
		}
		if _, exists := aliases[alias]; exists {
			return fmt.Errorf("duplicate source alias")
		}
		aliases[alias] = struct{}{}
		if strings.TrimSpace(source.EventType) == "" {
			return fmt.Errorf("source event type is required")
		}
		if !evaluationSafeIdentity(strings.TrimSpace(source.EventType)) {
			return fmt.Errorf("source event type is invalid")
		}
		if strings.TrimSpace(source.Content) == "" {
			return fmt.Errorf("source content is required")
		}
		if !utf8.ValidString(source.Content) || len(source.Content) > evaluationFixtureMaxSourceBytes {
			return fmt.Errorf("source content exceeds the safe input bound")
		}
		if !evaluationMemoryClassValid(source.Class) {
			return fmt.Errorf("invalid source memory class")
		}
		if !evaluationMemoryStateValid(source.State) {
			return fmt.Errorf("invalid source memory state")
		}
	}

	if len(c.ExpectedEvidenceGroups) == 0 {
		return fmt.Errorf("fixture case must include expected evidence groups")
	}
	if len(c.ExpectedEvidenceGroups) > evaluationFixtureMaxEvidenceGroups {
		return fmt.Errorf("expected evidence group count exceeds the safe bound")
	}
	expectedAliases := make(map[string]struct{})
	for _, group := range c.ExpectedEvidenceGroups {
		if len(group) == 0 {
			return fmt.Errorf("expected evidence group is required")
		}
		for _, alias := range group {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				return fmt.Errorf("expected evidence alias is required")
			}
			if _, exists := aliases[alias]; !exists {
				return fmt.Errorf("unknown evidence alias")
			}
			expectedAliases[alias] = struct{}{}
		}
	}

	if len(c.ExcludedAliases) > evaluationFixtureMaxExcludedAliases {
		return fmt.Errorf("excluded alias count exceeds the safe bound")
	}
	exclusions := make(map[string]struct{}, len(c.ExcludedAliases))
	for _, alias := range c.ExcludedAliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			return fmt.Errorf("excluded alias is required")
		}
		if _, exists := exclusions[alias]; exists {
			return fmt.Errorf("duplicate excluded alias")
		}
		if !evaluationSafeIdentity(alias) {
			return fmt.Errorf("invalid excluded alias")
		}
		if _, expected := expectedAliases[alias]; expected {
			return fmt.Errorf("excluded alias cannot also be expected evidence")
		}
		exclusions[alias] = struct{}{}
	}
	if c.ExpectedAnalysis != nil {
		if err := c.ExpectedAnalysis.validate(); err != nil {
			return err
		}
	} else if evaluationCategoryRequiresAnalysis(c.Category) {
		return fmt.Errorf("analysis expectation is required for category")
	}
	if c.Planner != nil {
		if err := c.Planner.validate(); err != nil {
			return err
		}
	}
	if c.Temporal != nil {
		if err := c.Temporal.validate(aliases, expectedAliases); err != nil {
			return err
		}
	}
	return nil
}

// validate checks one temporal expectation against the aliases the case actually
// declares. A temporal fixture that cannot be evaluated unambiguously is refused
// before seeding, because an ambiguous fixture silently degrades into a test
// that passes for the wrong reason.
func (expectation EvaluationTemporalExpectation) validate(aliases map[string]struct{}, expectedAliases map[string]struct{}) error {
	if !expectation.Kind.valid() {
		return fmt.Errorf("temporal fixture kind is invalid")
	}
	if expectation.MinimumSelectedVersions <= 0 || expectation.MinimumSelectedVersions > evaluationFixtureMaxSourcesPerCase {
		return fmt.Errorf("temporal fixture selected-version bound is invalid")
	}
	if expectation.MinimumSelectedVersions > len(expectedAliases) {
		return fmt.Errorf("temporal fixture expects more selected versions than it declares evidence for")
	}

	selectorPresent := expectation.Selector != nil
	switch {
	case expectation.Kind.requiresSelector() && !selectorPresent:
		return fmt.Errorf("temporal fixture kind requires an explicit selector")
	case !expectation.Kind.requiresSelector() && selectorPresent:
		return fmt.Errorf("temporal fixture current-class kind must not declare a selector")
	}

	if selectorPresent {
		constraint, err := expectation.Selector.Constraint()
		if err != nil {
			return fmt.Errorf("temporal fixture selector is invalid: %w", err)
		}
		if constraint.Mode == memory.TemporalSelectionCurrent {
			return fmt.Errorf("temporal fixture selector must not restate the current mode")
		}
	}
	if expectation.Kind.requiresSelector() && expectation.EvaluationInstant != nil {
		return fmt.Errorf("temporal fixture historical kind must not declare an evaluation instant")
	}
	if !expectation.Kind.requiresSelector() && expectation.EvaluationInstant == nil {
		return fmt.Errorf("temporal fixture current-class kind requires an evaluation instant")
	}
	if expectation.EvaluationInstant != nil && expectation.EvaluationInstant.IsZero() {
		return fmt.Errorf("temporal fixture evaluation instant is invalid")
	}

	// A retroactive correction is only resolved when it is authorized. Without
	// authorization the correct outcome is a withheld fact, not a silent
	// replacement, so the flag must match the declared scenario exactly.
	if expectation.AuthorizedCorrection != (expectation.Kind == EvaluationTemporalCaseRetroactiveCorrection) {
		return fmt.Errorf("temporal fixture correction authorization does not match its kind")
	}
	// Only an isolation case may tolerate an expected alias being absent, since
	// the point of that scenario is that a foreign fact stays out.
	if expectation.AllowUnselectedExpected && expectation.Kind != EvaluationTemporalCaseTemporalIsolation {
		return fmt.Errorf("temporal fixture may only tolerate unselected evidence for an isolation case")
	}

	if len(expectation.HiddenAliases) > evaluationTemporalMaxHiddenAliases {
		return fmt.Errorf("temporal fixture hidden alias count exceeds the safe bound")
	}
	hidden := make(map[string]struct{}, len(expectation.HiddenAliases))
	for _, alias := range expectation.HiddenAliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || !evaluationSafeIdentity(alias) {
			return fmt.Errorf("temporal fixture hidden alias is invalid")
		}
		if _, duplicate := hidden[alias]; duplicate {
			return fmt.Errorf("temporal fixture hidden alias is duplicated")
		}
		if _, declared := aliases[alias]; !declared {
			return fmt.Errorf("temporal fixture hidden alias is unknown")
		}
		// A hidden source that is also expected evidence is a contradiction: the
		// case would demand both that the version surface and that it not.
		if _, expected := expectedAliases[alias]; expected {
			return fmt.Errorf("temporal fixture hidden alias cannot also be expected evidence")
		}
		hidden[alias] = struct{}{}
	}
	return nil
}

func (expectation EvaluationPlannerExpectation) validate() error {
	if !expectation.Family.valid() || !evaluationSafeIdentity(expectation.PlanIdentity) || strings.TrimSpace(expectation.PlanIdentity) == "" {
		return fmt.Errorf("planner fixture identity is invalid")
	}
	if expectation.PlannerVersion != string(RetrievalPlannerVersionV1) {
		return fmt.Errorf("planner fixture version is unsupported")
	}
	if expectation.PolicyVersion != string(RetrievalPlanPolicyVersionV1) {
		return fmt.Errorf("planner fixture policy version is unsupported")
	}
	if len(expectation.Channels) == 0 || len(expectation.Channels) > 4 {
		return fmt.Errorf("planner fixture channels are invalid")
	}
	seen := make(map[FusionChannel]struct{}, len(expectation.Channels))
	for _, channel := range expectation.Channels {
		if !channel.valid() {
			return fmt.Errorf("planner fixture channel is invalid")
		}
		if _, exists := seen[channel]; exists {
			return fmt.Errorf("planner fixture channel is duplicated")
		}
		seen[channel] = struct{}{}
	}
	if expectation.MaxCandidates <= 0 || expectation.MaxCandidates > 5000 || expectation.MaxCandidatesPerChannel < 0 || expectation.MaxCandidatesPerChannel > expectation.MaxCandidates {
		return fmt.Errorf("planner fixture candidate budget is invalid")
	}
	fallbackAllocated := 0
	for channel, count := range expectation.FallbackChannelCandidates {
		if !channel.valid() || count <= 0 || count > expectation.MaxCandidates {
			return fmt.Errorf("planner fixture fallback candidate budget is invalid")
		}
		if expectation.MaxCandidatesPerChannel > 0 && count > expectation.MaxCandidatesPerChannel {
			return fmt.Errorf("planner fixture fallback channel budget exceeds per-channel bound")
		}
		if _, planned := seen[channel]; planned {
			return fmt.Errorf("planner fixture fallback channel overlaps planned channels")
		}
		fallbackAllocated += count
	}
	if fallbackAllocated >= expectation.MaxCandidates {
		return fmt.Errorf("planner fixture fallback reserve exhausts candidate budget")
	}
	wantIdentity := RetrievalPlanIdentity{
		PlannerVersion:            RetrievalPlannerVersion(expectation.PlannerVersion),
		PolicyVersion:             RetrievalPlanPolicyVersion(expectation.PolicyVersion),
		Family:                    expectation.Family,
		FallbackChannelCandidates: expectation.FallbackChannelCandidates,
	}.String()
	if expectation.PlanIdentity != wantIdentity {
		return fmt.Errorf("planner fixture plan identity is incompatible")
	}
	if expectation.ExpectedPasses < 1 || expectation.ExpectedPasses > 2 || (expectation.FollowUpEligible && expectation.ExpectedPasses != 2) {
		return fmt.Errorf("planner fixture pass expectation is invalid")
	}
	return nil
}

func (expectation EvaluationAnalysisExpectation) validate() error {
	if !expectation.PolicyVersion.valid() {
		return fmt.Errorf("invalid analysis policy version")
	}
	if !expectation.LimitsVersion.valid() {
		return fmt.Errorf("invalid analysis limits version")
	}
	if !expectation.Disposition.valid() {
		return fmt.Errorf("invalid analysis disposition")
	}
	if !expectation.Fallback.valid() {
		return fmt.Errorf("invalid analysis fallback")
	}
	if !expectation.OriginalRetained {
		return fmt.Errorf("analysis expectation must retain original query")
	}
	seen := make(map[QueryAnalysisDiagnosticCategory]struct{}, len(expectation.Categories))
	for _, category := range expectation.Categories {
		if !category.valid() {
			return fmt.Errorf("invalid analysis category")
		}
		if _, duplicate := seen[category]; duplicate {
			return fmt.Errorf("duplicate analysis category")
		}
		seen[category] = struct{}{}
	}
	if expectation.MaxSignalCount < 1 || expectation.MaxSignalCount > QueryAnalysisHardMaxSignals {
		return fmt.Errorf("analysis signal bound is invalid")
	}
	if expectation.MaxSubqueryCount < 0 || expectation.MaxSubqueryCount > QueryAnalysisHardMaxSubqueries || expectation.MaxSubqueryCount >= expectation.MaxSignalCount {
		return fmt.Errorf("analysis subquery bound is invalid")
	}
	if expectation.MaxCandidateCount < 0 || expectation.MaxCandidateCount > QueryAnalysisHardMaxAggregateCandidates {
		return fmt.Errorf("analysis candidate bound is invalid")
	}
	if expectation.Disposition == QueryAnalysisDispositionOriginalOnly && expectation.MaxSignalCount != 1 {
		return fmt.Errorf("original-only analysis expectation must allow exactly one signal")
	}
	if expectation.Disposition != QueryAnalysisDispositionOriginalOnly && expectation.Fallback != QueryAnalysisFallbackNone {
		return fmt.Errorf("non-original-only analysis expectation cannot require fallback")
	}
	if expectation.Fallback != QueryAnalysisFallbackNone {
		category := QueryAnalysisDiagnosticCategory(expectation.Fallback)
		if _, exists := seen[category]; !exists {
			return fmt.Errorf("analysis fallback category must be expected")
		}
	}
	return nil
}

func evaluationCategoryRequiresAnalysis(category string) bool {
	switch strings.TrimSpace(category) {
	case "mixed-language", "ambiguous", "malformed", "adversarial", "analyzer-unavailable":
		return true
	default:
		return false
	}
}

func (p EvaluationReleasePolicy) Validate() error {
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("release policy version is required")
	}
	if len(p.ProtectedCutoffs) == 0 {
		return fmt.Errorf("protected cutoffs are required")
	}
	previous := 0
	for _, cutoff := range p.ProtectedCutoffs {
		if cutoff <= 0 || cutoff <= previous {
			return fmt.Errorf("protected cutoffs must be strictly increasing positive values")
		}
		previous = cutoff
	}
	if p.MaxP95LatencyMS <= 0 {
		return fmt.Errorf("max p95 latency must be greater than zero")
	}
	if p.MaxRecallRegression < 0 || p.MaxMultiHopCoverageRegression < 0 || p.MaxEvidenceCoverageRegression < 0 || p.MaxBudgetOmissionIncrease < 0 || p.MaxP95LatencyRegressionMS < 0 || p.MaxDuplicateTokenRate < 0 || p.MaxDuplicateTokenRate > 1 || p.MaxStaleTokenRate < 0 || p.MaxStaleTokenRate > 1 || p.MaxQualityPerBudgetRegression < 0 || p.MaxQualityPerBudgetRegression > 1 {
		return fmt.Errorf("quality regression tolerances must be greater than or equal to zero")
	}
	if p.ResourceBudget.MaxCases < 0 || p.ResourceBudget.MaxCandidates < 0 || p.ResourceBudget.MaxElapsedMS < 0 {
		return fmt.Errorf("resource budget values must be non-negative")
	}
	if p.ResourceBudget.MaxElapsedMS > 0 && p.ResourceBudget.MaxElapsedMS > 24*60*60*1000 {
		return fmt.Errorf("resource budget elapsed bound is excessive")
	}
	if p.Rollback.Strategy != "" && !evaluationSafeIdentity(p.Rollback.Strategy) {
		return fmt.Errorf("rollback strategy is invalid")
	}
	if p.Retention.WindowHours < 0 || p.Retention.WindowHours > 24*365*10 {
		return fmt.Errorf("retention window is invalid")
	}
	if p.Retention.Owner != "" && !evaluationSafeIdentity(p.Retention.Owner) {
		return fmt.Errorf("retention owner is invalid")
	}
	if p.Planner.MaxPasses < 0 || p.Planner.MaxPasses > 2 || p.Planner.MaxCandidateCount < 0 || p.Planner.MaxCandidateCount > 5000 || !boundedRate(p.Planner.MaxFallbackRate) {
		return fmt.Errorf("planner release policy is invalid")
	}
	seenCategories := make(map[string]struct{}, len(p.ProtectedCategories))
	for _, category := range p.ProtectedCategories {
		category = strings.TrimSpace(category)
		if category == "" {
			return fmt.Errorf("protected category is required")
		}
		if _, exists := seenCategories[category]; exists {
			return fmt.Errorf("duplicate protected category")
		}
		seenCategories[category] = struct{}{}
	}
	seenFamilies := make(map[RetrievalQueryFamily]struct{}, len(p.ProtectedFamilies))
	for _, family := range p.ProtectedFamilies {
		if !family.valid() {
			return fmt.Errorf("protected query family is invalid")
		}
		if _, exists := seenFamilies[family]; exists {
			return fmt.Errorf("duplicate protected query family")
		}
		seenFamilies[family] = struct{}{}
	}
	return nil
}

func evaluationMemoryClassValid(class memory.MemoryClass) bool {
	switch class {
	case "", memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassRelation:
		return true
	default:
		return false
	}
}

func evaluationSafetyFailureCategoryValid(category EvaluationSafetyFailureCategory) bool {
	switch category {
	case EvaluationSafetyFailureInvalidFixtureScope,
		EvaluationSafetyFailureCrossScope,
		EvaluationSafetyFailureLifecycleVisibility,
		EvaluationSafetyFailureUnsafeDiagnostics,
		EvaluationSafetyFailureStaleFactWin,
		EvaluationSafetyFailureValidityAmbiguity,
		EvaluationSafetyFailureProvenanceMismatch,
		EvaluationSafetyFailureHiddenVersionLeakage,
		EvaluationSafetyFailureResourceOverflow,
		EvaluationSafetyFailureGraphCycle,
		EvaluationSafetyFailureGraphNondeterministic:
		return true
	default:
		return false
	}
}

func evaluationMemoryStateValid(state memory.MemoryState) bool {
	switch state {
	case "", memory.MemoryStateCandidate, memory.MemoryStateActive, memory.MemoryStateSuppressed, memory.MemoryStateForgotten, memory.MemoryStateDeleted:
		return true
	default:
		return false
	}
}
