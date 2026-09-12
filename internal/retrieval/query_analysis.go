package retrieval

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

type QueryAnalysisPolicyVersion string

const QueryAnalysisPolicyVersionV1 QueryAnalysisPolicyVersion = "query-analysis-v1"

func (version QueryAnalysisPolicyVersion) valid() bool {
	return version == QueryAnalysisPolicyVersionV1
}

type QueryAnalysisLimitsVersion string

const QueryAnalysisLimitsVersionV1 QueryAnalysisLimitsVersion = "query-analysis-limits-v1"

func (version QueryAnalysisLimitsVersion) valid() bool {
	return version == QueryAnalysisLimitsVersionV1
}

const (
	// QueryAnalysisStageCount is the fixed number of bounded v1 analyzer
	// stages: normalization, aliases, four hint extractors, and decomposition.
	QueryAnalysisStageCount                 = 7
	QueryAnalysisHardMaxQueryBytes          = 16 * 1024
	QueryAnalysisHardMaxHints               = 4
	QueryAnalysisHardMaxSignals             = 16
	QueryAnalysisHardMaxSubqueries          = 8
	QueryAnalysisHardMaxTermBytes           = 1024
	QueryAnalysisHardMaxSubqueryBytes       = 4096
	QueryAnalysisHardMaxAnalysisWork        = QueryAnalysisStageCount
	QueryAnalysisHardMaxCandidatesPerSignal = 100
	QueryAnalysisHardMaxAggregateCandidates = 1000
	QueryAnalysisHardMaxDiagnosticCount     = 10000
	QueryAnalysisHardMaxElapsed             = 5 * time.Second
)

// QueryAnalysisLimits bounds every value that query analysis may add to the
// mandatory original-query retrieval path. MaxSignals includes the original
// signal, so it must be at least one.
type QueryAnalysisLimits struct {
	Version                QueryAnalysisLimitsVersion
	MaxQueryBytes          int
	MaxHints               int
	MaxSignals             int
	MaxSubqueries          int
	MaxTermBytes           int
	MaxSubqueryBytes       int
	MaxAnalysisWork        int
	MaxCandidatesPerSignal int
	MaxAggregateCandidates int
	MaxElapsed             time.Duration
}

func DefaultQueryAnalysisLimits() QueryAnalysisLimits {
	return QueryAnalysisLimits{
		Version:                QueryAnalysisLimitsVersionV1,
		MaxQueryBytes:          4096,
		MaxHints:               QueryAnalysisHardMaxHints,
		MaxSignals:             8,
		MaxSubqueries:          4,
		MaxTermBytes:           256,
		MaxSubqueryBytes:       1024,
		MaxAnalysisWork:        QueryAnalysisStageCount,
		MaxCandidatesPerSignal: 50,
		MaxAggregateCandidates: 200,
		MaxElapsed:             250 * time.Millisecond,
	}
}

func (limits QueryAnalysisLimits) Validate() error {
	if !limits.Version.valid() {
		return fmt.Errorf("unsupported query-analysis limits version")
	}
	checks := []struct {
		name string
		got  int
		min  int
		max  int
	}{
		{name: "max query bytes", got: limits.MaxQueryBytes, min: 1, max: QueryAnalysisHardMaxQueryBytes},
		{name: "max hints", got: limits.MaxHints, min: 0, max: QueryAnalysisHardMaxHints},
		{name: "max signals", got: limits.MaxSignals, min: 1, max: QueryAnalysisHardMaxSignals},
		{name: "max subqueries", got: limits.MaxSubqueries, min: 0, max: QueryAnalysisHardMaxSubqueries},
		{name: "max term bytes", got: limits.MaxTermBytes, min: 1, max: QueryAnalysisHardMaxTermBytes},
		{name: "max subquery bytes", got: limits.MaxSubqueryBytes, min: 1, max: QueryAnalysisHardMaxSubqueryBytes},
		{name: "max analysis work", got: limits.MaxAnalysisWork, min: 1, max: QueryAnalysisHardMaxAnalysisWork},
		{name: "max candidates per signal", got: limits.MaxCandidatesPerSignal, min: 1, max: QueryAnalysisHardMaxCandidatesPerSignal},
		{name: "max aggregate candidates", got: limits.MaxAggregateCandidates, min: 1, max: QueryAnalysisHardMaxAggregateCandidates},
	}
	for _, check := range checks {
		if check.got < check.min || check.got > check.max {
			return fmt.Errorf("query-analysis %s must be between %d and %d", check.name, check.min, check.max)
		}
	}
	if limits.MaxSubqueries >= limits.MaxSignals {
		return fmt.Errorf("query-analysis max subqueries must leave capacity for the original signal")
	}
	if limits.MaxAggregateCandidates < limits.MaxCandidatesPerSignal {
		return fmt.Errorf("query-analysis max aggregate candidates must cover candidates per signal")
	}
	if limits.MaxElapsed <= 0 || limits.MaxElapsed > QueryAnalysisHardMaxElapsed {
		return fmt.Errorf("query-analysis max elapsed must be between 1ns and %s", QueryAnalysisHardMaxElapsed)
	}
	return nil
}

// QueryAnalysisInput is a pure, provider-independent input. It intentionally
// contains no persistence, network, provider, or scope-discovery capability.
type QueryAnalysisInput struct {
	AcceptedQuery string
	PolicyVersion QueryAnalysisPolicyVersion
	Limits        QueryAnalysisLimits
	Constraints   QueryAnalysisConstraints
}

// QueryAnalysisConstraints copies only already-authorized narrowing constraints.
// It deliberately carries no tenant, project, namespace, session, or user values.
type QueryAnalysisConstraints struct {
	Classes  []memory.MemoryClass
	TimeFrom time.Time
	TimeTo   time.Time
}

func (constraints QueryAnalysisConstraints) Validate() error {
	seen := make(map[memory.MemoryClass]struct{}, len(constraints.Classes))
	for _, class := range constraints.Classes {
		switch class {
		case memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassProcedural,
			memory.MemoryClassSummary, memory.MemoryClassRelation:
		default:
			return fmt.Errorf("query-analysis constraint contains unsupported memory class")
		}
		if _, duplicate := seen[class]; duplicate {
			return fmt.Errorf("query-analysis constraint contains duplicate memory class")
		}
		seen[class] = struct{}{}
	}
	if !constraints.TimeFrom.IsZero() && !constraints.TimeTo.IsZero() && constraints.TimeFrom.After(constraints.TimeTo) {
		return fmt.Errorf("query-analysis time constraint must be ordered")
	}
	return nil
}

func (input QueryAnalysisInput) Validate() error {
	if input.AcceptedQuery == "" {
		return fmt.Errorf("query-analysis accepted query is required")
	}
	if !utf8.ValidString(input.AcceptedQuery) {
		return fmt.Errorf("query-analysis accepted query must be valid UTF-8")
	}
	if !input.PolicyVersion.valid() {
		return fmt.Errorf("unsupported query-analysis policy version")
	}
	if err := input.Limits.Validate(); err != nil {
		return err
	}
	if err := input.Constraints.Validate(); err != nil {
		return err
	}
	if len(input.AcceptedQuery) > input.Limits.MaxQueryBytes {
		return fmt.Errorf("query-analysis accepted query exceeds max query bytes")
	}
	return nil
}

type QueryAnalysisIdentity struct {
	PolicyVersion QueryAnalysisPolicyVersion `json:"policy_version"`
	LimitsVersion QueryAnalysisLimitsVersion `json:"limits_version"`
}

type QueryAnalysisDisposition string

const (
	QueryAnalysisDispositionComplete     QueryAnalysisDisposition = "complete"
	QueryAnalysisDispositionPartial      QueryAnalysisDisposition = "partial"
	QueryAnalysisDispositionOriginalOnly QueryAnalysisDisposition = "original_only"
)

func (disposition QueryAnalysisDisposition) valid() bool {
	switch disposition {
	case QueryAnalysisDispositionComplete, QueryAnalysisDispositionPartial, QueryAnalysisDispositionOriginalOnly:
		return true
	default:
		return false
	}
}

type QueryAnalysisHintKind string

const (
	QueryAnalysisHintEntity      QueryAnalysisHintKind = "entity"
	QueryAnalysisHintTemporal    QueryAnalysisHintKind = "temporal"
	QueryAnalysisHintMemoryClass QueryAnalysisHintKind = "memory_class"
	QueryAnalysisHintIntent      QueryAnalysisHintKind = "intent"
)

func (kind QueryAnalysisHintKind) valid() bool {
	switch kind {
	case QueryAnalysisHintEntity, QueryAnalysisHintTemporal, QueryAnalysisHintMemoryClass, QueryAnalysisHintIntent:
		return true
	default:
		return false
	}
}

type QueryAnalysisHintDisposition string

const (
	QueryAnalysisHintPresent QueryAnalysisHintDisposition = "present"
	QueryAnalysisHintAbsent  QueryAnalysisHintDisposition = "absent"
	QueryAnalysisHintUnknown QueryAnalysisHintDisposition = "unknown"
)

func (disposition QueryAnalysisHintDisposition) valid() bool {
	switch disposition {
	case QueryAnalysisHintPresent, QueryAnalysisHintAbsent, QueryAnalysisHintUnknown:
		return true
	default:
		return false
	}
}

type QueryAnalysisHint struct {
	Kind        QueryAnalysisHintKind        `json:"kind"`
	Disposition QueryAnalysisHintDisposition `json:"disposition"`
	Value       string                       `json:"value,omitempty"`
}

type QueryAnalysisSignalKind string

const (
	QueryAnalysisSignalOriginal   QueryAnalysisSignalKind = "original"
	QueryAnalysisSignalNormalized QueryAnalysisSignalKind = "normalized"
	QueryAnalysisSignalTerm       QueryAnalysisSignalKind = "term"
	QueryAnalysisSignalSubquery   QueryAnalysisSignalKind = "subquery"
)

func (kind QueryAnalysisSignalKind) valid() bool {
	switch kind {
	case QueryAnalysisSignalOriginal, QueryAnalysisSignalNormalized, QueryAnalysisSignalTerm, QueryAnalysisSignalSubquery:
		return true
	default:
		return false
	}
}

type QueryAnalysisSignal struct {
	Kind      QueryAnalysisSignalKind `json:"kind"`
	Text      string                  `json:"text"`
	Mandatory bool                    `json:"mandatory"`
}

type QueryAnalysisResult struct {
	Identity    QueryAnalysisIdentity    `json:"identity"`
	Disposition QueryAnalysisDisposition `json:"disposition"`
	Hints       []QueryAnalysisHint      `json:"hints"`
	Signals     []QueryAnalysisSignal    `json:"signals"`
	WorkUnits   int                      `json:"work_units"`
	// Categories contains bounded internal aggregate outcomes for later
	// authorized diagnostics conversion. It is never serialized with a result.
	Categories []QueryAnalysisDiagnosticCount `json:"-"`
}

func NewQueryAnalysisResult(input QueryAnalysisInput, disposition QueryAnalysisDisposition, hints []QueryAnalysisHint, derived []QueryAnalysisSignal, categories ...QueryAnalysisDiagnosticCount) (QueryAnalysisResult, error) {
	result := QueryAnalysisResult{
		Identity: QueryAnalysisIdentity{
			PolicyVersion: input.PolicyVersion,
			LimitsVersion: input.Limits.Version,
		},
		Disposition: disposition,
		Hints:       append([]QueryAnalysisHint(nil), hints...),
		Categories:  append([]QueryAnalysisDiagnosticCount(nil), categories...),
		Signals: []QueryAnalysisSignal{{
			Kind:      QueryAnalysisSignalOriginal,
			Text:      input.AcceptedQuery,
			Mandatory: true,
		}},
	}
	result.Signals = append(result.Signals, derived...)
	sort.Slice(result.Categories, func(i, j int) bool {
		return result.Categories[i].Category < result.Categories[j].Category
	})
	if err := result.Validate(input); err != nil {
		return QueryAnalysisResult{}, err
	}
	return result, nil
}

func (result QueryAnalysisResult) Validate(input QueryAnalysisInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if !result.Identity.PolicyVersion.valid() || result.Identity.PolicyVersion != input.PolicyVersion {
		return fmt.Errorf("query-analysis result policy version does not match input")
	}
	if !result.Identity.LimitsVersion.valid() || result.Identity.LimitsVersion != input.Limits.Version {
		return fmt.Errorf("query-analysis result limits version does not match input")
	}
	if !result.Disposition.valid() {
		return fmt.Errorf("unknown query-analysis disposition")
	}
	if result.WorkUnits < 0 || result.WorkUnits > input.Limits.MaxAnalysisWork {
		return fmt.Errorf("query-analysis work units exceed max analysis work")
	}
	seenCategories := make(map[QueryAnalysisDiagnosticCategory]struct{}, len(result.Categories))
	for _, count := range result.Categories {
		if !count.Category.valid() {
			return fmt.Errorf("unknown query-analysis diagnostic category")
		}
		if _, duplicate := seenCategories[count.Category]; duplicate {
			return fmt.Errorf("duplicate query-analysis diagnostic category")
		}
		seenCategories[count.Category] = struct{}{}
		if count.Count <= 0 || count.Count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("query-analysis diagnostic count is outside its bound")
		}
		if count.Category == QueryAnalysisDiagnosticOverBudget && result.Disposition != QueryAnalysisDispositionPartial {
			return fmt.Errorf("query-analysis over-budget category requires partial disposition")
		}
		if count.Category == QueryAnalysisDiagnosticAdversarial && result.Disposition != QueryAnalysisDispositionOriginalOnly {
			return fmt.Errorf("query-analysis adversarial category requires original-only disposition")
		}
	}
	if len(result.Hints) > input.Limits.MaxHints {
		return fmt.Errorf("query-analysis hint count exceeds max hints")
	}
	seenHints := make(map[QueryAnalysisHintKind]struct{}, len(result.Hints))
	for _, hint := range result.Hints {
		if !hint.Kind.valid() {
			return fmt.Errorf("unknown query-analysis hint kind")
		}
		if !hint.Disposition.valid() {
			return fmt.Errorf("unknown query-analysis hint disposition")
		}
		if _, duplicate := seenHints[hint.Kind]; duplicate {
			return fmt.Errorf("duplicate query-analysis hint kind")
		}
		seenHints[hint.Kind] = struct{}{}
		if hint.Disposition == QueryAnalysisHintPresent {
			if hint.Value == "" || !utf8.ValidString(hint.Value) || len(hint.Value) > input.Limits.MaxTermBytes {
				return fmt.Errorf("present query-analysis hint requires a bounded UTF-8 value")
			}
		} else if hint.Value != "" {
			return fmt.Errorf("absent or unknown query-analysis hint must not carry a value")
		}
	}

	if len(result.Signals) == 0 || len(result.Signals) > input.Limits.MaxSignals {
		return fmt.Errorf("query-analysis signal count must retain the original within max signals")
	}
	first := result.Signals[0]
	if first.Kind != QueryAnalysisSignalOriginal || first.Text != input.AcceptedQuery || !first.Mandatory {
		return fmt.Errorf("query-analysis first signal must be the exact mandatory original query")
	}
	seenSignals := map[string]struct{}{first.Text: {}}
	subqueries := 0
	for index, signal := range result.Signals {
		if !signal.Kind.valid() {
			return fmt.Errorf("unknown query-analysis signal kind")
		}
		if signal.Text == "" || !utf8.ValidString(signal.Text) {
			return fmt.Errorf("query-analysis signal text must be non-empty valid UTF-8")
		}
		if index == 0 {
			continue
		}
		if signal.Kind == QueryAnalysisSignalOriginal || signal.Mandatory {
			return fmt.Errorf("query-analysis derived signal cannot be original or mandatory")
		}
		maxBytes := input.Limits.MaxTermBytes
		if signal.Kind == QueryAnalysisSignalSubquery {
			maxBytes = input.Limits.MaxSubqueryBytes
			subqueries++
		}
		if len(signal.Text) > maxBytes {
			return fmt.Errorf("query-analysis signal text exceeds its configured bound")
		}
		if _, duplicate := seenSignals[signal.Text]; duplicate {
			return fmt.Errorf("query-analysis duplicate signal")
		}
		seenSignals[signal.Text] = struct{}{}
	}
	if subqueries > input.Limits.MaxSubqueries {
		return fmt.Errorf("query-analysis subquery count exceeds max subqueries")
	}
	if result.Disposition == QueryAnalysisDispositionOriginalOnly && len(result.Signals) != 1 {
		return fmt.Errorf("original-only query-analysis result cannot contain derived signals")
	}
	return nil
}

type QueryAnalysisFallbackCategory string

const (
	QueryAnalysisFallbackNone        QueryAnalysisFallbackCategory = "none"
	QueryAnalysisFallbackUnavailable QueryAnalysisFallbackCategory = "unavailable"
	QueryAnalysisFallbackMalformed   QueryAnalysisFallbackCategory = "malformed"
	QueryAnalysisFallbackAdversarial QueryAnalysisFallbackCategory = "adversarial"
	QueryAnalysisFallbackDuplicate   QueryAnalysisFallbackCategory = "duplicate"
	QueryAnalysisFallbackOverBudget  QueryAnalysisFallbackCategory = "over_budget"
)

func (category QueryAnalysisFallbackCategory) valid() bool {
	switch category {
	case QueryAnalysisFallbackNone, QueryAnalysisFallbackUnavailable, QueryAnalysisFallbackMalformed,
		QueryAnalysisFallbackAdversarial, QueryAnalysisFallbackDuplicate, QueryAnalysisFallbackOverBudget:
		return true
	default:
		return false
	}
}

type QueryAnalysisDiagnosticCategory string

const (
	QueryAnalysisDiagnosticUnavailable QueryAnalysisDiagnosticCategory = "unavailable"
	QueryAnalysisDiagnosticMalformed   QueryAnalysisDiagnosticCategory = "malformed"
	QueryAnalysisDiagnosticAdversarial QueryAnalysisDiagnosticCategory = "adversarial"
	QueryAnalysisDiagnosticDuplicate   QueryAnalysisDiagnosticCategory = "duplicate"
	QueryAnalysisDiagnosticOverBudget  QueryAnalysisDiagnosticCategory = "over_budget"
)

func (category QueryAnalysisDiagnosticCategory) valid() bool {
	switch category {
	case QueryAnalysisDiagnosticUnavailable, QueryAnalysisDiagnosticMalformed, QueryAnalysisDiagnosticAdversarial,
		QueryAnalysisDiagnosticDuplicate, QueryAnalysisDiagnosticOverBudget:
		return true
	default:
		return false
	}
}

type QueryAnalysisDiagnosticCount struct {
	Category QueryAnalysisDiagnosticCategory `json:"category"`
	Count    int                             `json:"count"`
}

// QueryAnalysisDiagnostics is an allowlisted aggregate representation. It has
// no field capable of carrying query, normalized, subquery, candidate, score,
// provider-reasoning, identifier, or scope text.
type QueryAnalysisDiagnostics struct {
	PolicyVersion       QueryAnalysisPolicyVersion     `json:"policy_version"`
	LimitsVersion       QueryAnalysisLimitsVersion     `json:"limits_version"`
	Disposition         QueryAnalysisDisposition       `json:"disposition"`
	Fallback            QueryAnalysisFallbackCategory  `json:"fallback"`
	OriginalRetained    bool                           `json:"original_retained"`
	Categories          []QueryAnalysisDiagnosticCount `json:"categories"`
	HintCount           int                            `json:"hint_count"`
	SignalCount         int                            `json:"signal_count"`
	SubqueryCount       int                            `json:"subquery_count"`
	CandidateCount      int                            `json:"candidate_count"`
	Elapsed             time.Duration                  `json:"elapsed_ns"`
	RolloutStage        string                         `json:"rollout_stage,omitempty"`
	NormalizationStatus string                         `json:"normalization_status,omitempty"`
	TimeStatus          string                         `json:"time_status,omitempty"`
}

func (diagnostics QueryAnalysisDiagnostics) Validate(limits QueryAnalysisLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if !diagnostics.PolicyVersion.valid() {
		return fmt.Errorf("unsupported query-analysis diagnostic policy version")
	}
	if !diagnostics.LimitsVersion.valid() || diagnostics.LimitsVersion != limits.Version {
		return fmt.Errorf("query-analysis diagnostic limits version does not match limits")
	}
	if !diagnostics.Disposition.valid() {
		return fmt.Errorf("unknown query-analysis diagnostic disposition")
	}
	if !diagnostics.Fallback.valid() {
		return fmt.Errorf("unknown query-analysis fallback category")
	}
	if !diagnostics.OriginalRetained {
		return fmt.Errorf("query-analysis diagnostics must record original retained")
	}
	seen := make(map[QueryAnalysisDiagnosticCategory]struct{}, len(diagnostics.Categories))
	for _, count := range diagnostics.Categories {
		if !count.Category.valid() {
			return fmt.Errorf("unknown query-analysis diagnostic category")
		}
		if _, duplicate := seen[count.Category]; duplicate {
			return fmt.Errorf("duplicate diagnostic category")
		}
		seen[count.Category] = struct{}{}
		if count.Count < 0 || count.Count > QueryAnalysisHardMaxDiagnosticCount {
			return fmt.Errorf("query-analysis diagnostic count is outside its bound")
		}
	}
	if diagnostics.HintCount < 0 || diagnostics.HintCount > limits.MaxHints {
		return fmt.Errorf("query-analysis hint count is outside its bound")
	}
	if diagnostics.SignalCount < 1 || diagnostics.SignalCount > limits.MaxSignals {
		return fmt.Errorf("query-analysis signal count is outside its bound")
	}
	if diagnostics.SubqueryCount < 0 || diagnostics.SubqueryCount > limits.MaxSubqueries {
		return fmt.Errorf("query-analysis subquery count is outside its bound")
	}
	if diagnostics.SubqueryCount > diagnostics.SignalCount-1 {
		return fmt.Errorf("query-analysis subquery count exceeds derived signal count")
	}
	if diagnostics.Disposition == QueryAnalysisDispositionOriginalOnly && diagnostics.SignalCount != 1 {
		return fmt.Errorf("query-analysis original-only diagnostics must contain exactly one signal")
	}
	if diagnostics.CandidateCount < 0 || diagnostics.CandidateCount > limits.MaxAggregateCandidates {
		return fmt.Errorf("query-analysis candidate count is outside its bound")
	}
	if diagnostics.Elapsed < 0 || diagnostics.Elapsed > limits.MaxElapsed {
		return fmt.Errorf("query-analysis elapsed duration is outside its bound")
	}
	return nil
}

func MarshalQueryAnalysisDiagnostics(diagnostics QueryAnalysisDiagnostics, limits QueryAnalysisLimits) ([]byte, error) {
	if err := diagnostics.Validate(limits); err != nil {
		return nil, err
	}
	stable := diagnostics
	stable.Categories = append([]QueryAnalysisDiagnosticCount(nil), diagnostics.Categories...)
	sort.Slice(stable.Categories, func(i, j int) bool {
		return stable.Categories[i].Category < stable.Categories[j].Category
	})
	return json.Marshal(stable)
}

// QueryAnalysisDiagnosticsFromResult converts a validated analyzer result into
// the bounded, allowlisted diagnostic representation. It deliberately copies
// only versions, dispositions, categories, and aggregate counts; signal text,
// hints, scopes, identifiers, and scores are never retained.
func QueryAnalysisDiagnosticsFromResult(result QueryAnalysisResult, limits QueryAnalysisLimits, fallback QueryAnalysisFallbackCategory, elapsed time.Duration, candidateCount int, rolloutStage string) (QueryAnalysisDiagnostics, error) {
	if err := limits.Validate(); err != nil {
		return QueryAnalysisDiagnostics{}, err
	}
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > limits.MaxElapsed {
		elapsed = limits.MaxElapsed
	}
	if candidateCount < 0 {
		candidateCount = 0
	}
	if candidateCount > limits.MaxAggregateCandidates {
		candidateCount = limits.MaxAggregateCandidates
	}
	diagnostics := QueryAnalysisDiagnostics{
		PolicyVersion:       result.Identity.PolicyVersion,
		LimitsVersion:       result.Identity.LimitsVersion,
		Disposition:         result.Disposition,
		Fallback:            fallback,
		OriginalRetained:    len(result.Signals) > 0 && result.Signals[0].Kind == QueryAnalysisSignalOriginal && result.Signals[0].Mandatory,
		HintCount:           len(result.Hints),
		SignalCount:         len(result.Signals),
		SubqueryCount:       0,
		CandidateCount:      candidateCount,
		Elapsed:             elapsed,
		RolloutStage:        rolloutStage,
		NormalizationStatus: queryAnalysisNormalizationStatus(result),
		TimeStatus:          queryAnalysisTimeStatus(result),
		Categories:          append([]QueryAnalysisDiagnosticCount(nil), result.Categories...),
	}
	for i, signal := range result.Signals {
		if i > 0 && signal.Kind == QueryAnalysisSignalSubquery {
			diagnostics.SubqueryCount++
		}
	}
	if err := diagnostics.Validate(limits); err != nil {
		return QueryAnalysisDiagnostics{}, err
	}
	return diagnostics, nil
}

func queryAnalysisNormalizationStatus(result QueryAnalysisResult) string {
	for _, signal := range result.Signals {
		if signal.Kind == QueryAnalysisSignalNormalized {
			return "normalized"
		}
	}
	return "unchanged"
}

func queryAnalysisTimeStatus(result QueryAnalysisResult) string {
	for _, hint := range result.Hints {
		if hint.Kind == QueryAnalysisHintTemporal {
			return string(hint.Disposition)
		}
	}
	return "not_detected"
}
