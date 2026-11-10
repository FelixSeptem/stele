package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// RepresentationTaskFamily describes a bounded fixture category. It never
// carries the task's text, answer, or source material.
type RepresentationTaskFamily string

const (
	RepresentationTaskFamilyExactFacts        RepresentationTaskFamily = "exact_facts"
	RepresentationTaskFamilyCrossDocument     RepresentationTaskFamily = "cross_document"
	RepresentationTaskFamilyProfilePreference RepresentationTaskFamily = "profile_preference"
	RepresentationTaskFamilyProceduralLesson  RepresentationTaskFamily = "procedural_lesson"
)

func (f RepresentationTaskFamily) valid() bool {
	switch f {
	case RepresentationTaskFamilyExactFacts, RepresentationTaskFamilyCrossDocument, RepresentationTaskFamilyProfilePreference, RepresentationTaskFamilyProceduralLesson:
		return true
	default:
		return false
	}
}

// RepresentationCandidate identifies an experimental derived representation.
// All candidates retain authorized source search; none is an active strategy.
type RepresentationCandidate string

const (
	RepresentationCandidateExactExcerptWithSearch    RepresentationCandidate = "exact-excerpt-with-search-v1"
	RepresentationCandidateTopicProjectionWithSearch RepresentationCandidate = "topic-projection-with-search-v1"
	RepresentationCandidateHierarchicalWithSearch    RepresentationCandidate = "hierarchical-search-v1"
)

func (c RepresentationCandidate) valid() bool {
	switch c {
	case RepresentationCandidateExactExcerptWithSearch, RepresentationCandidateTopicProjectionWithSearch, RepresentationCandidateHierarchicalWithSearch:
		return true
	default:
		return false
	}
}

func representationCandidateCompatible(family RepresentationTaskFamily, candidate RepresentationCandidate) bool {
	switch family {
	case RepresentationTaskFamilyExactFacts:
		return candidate == RepresentationCandidateExactExcerptWithSearch
	case RepresentationTaskFamilyCrossDocument:
		return candidate == RepresentationCandidateTopicProjectionWithSearch || candidate == RepresentationCandidateHierarchicalWithSearch
	case RepresentationTaskFamilyProfilePreference:
		return candidate == RepresentationCandidateTopicProjectionWithSearch
	case RepresentationTaskFamilyProceduralLesson:
		return candidate == RepresentationCandidateHierarchicalWithSearch
	default:
		return false
	}
}

// RepresentationCandidateSupportsTaskFamily reports fixture compatibility for
// an evaluation-only candidate. It does not select or activate a strategy.
func RepresentationCandidateSupportsTaskFamily(family RepresentationTaskFamily, candidate RepresentationCandidate) bool {
	return representationCandidateCompatible(family, candidate)
}

type RepresentationEvaluationMode string

const (
	RepresentationEvaluationModeOffline RepresentationEvaluationMode = "offline"
	RepresentationEvaluationModeShadow  RepresentationEvaluationMode = "shadow"
)

type RepresentationEvaluationVerdict string

const (
	RepresentationEvaluationShadow  RepresentationEvaluationVerdict = "shadow"
	RepresentationEvaluationNonPass RepresentationEvaluationVerdict = "non_pass"
)

// RepresentationEvaluationFailure is deliberately a small, report-safe
// vocabulary. It must not encode scope values, IDs, or free-form errors.
type RepresentationEvaluationFailure string

const (
	RepresentationFailureIncompatible        RepresentationEvaluationFailure = "incompatible"
	RepresentationFailureSourceSearchMissing RepresentationEvaluationFailure = "source_search_missing"
	RepresentationFailureCriticalFactLoss    RepresentationEvaluationFailure = "critical_fact_loss"
	RepresentationFailureScope               RepresentationEvaluationFailure = "scope"
	RepresentationFailureLifecycle           RepresentationEvaluationFailure = "lifecycle"
	RepresentationFailureFreshness           RepresentationEvaluationFailure = "freshness"
	RepresentationFailureBrokenReference     RepresentationEvaluationFailure = "broken_reference"
	RepresentationFailureBudget              RepresentationEvaluationFailure = "budget"
	RepresentationFailureLatency             RepresentationEvaluationFailure = "latency"
	RepresentationFailureNondeterministic    RepresentationEvaluationFailure = "nondeterministic"
	RepresentationFailureMissingBaseline     RepresentationEvaluationFailure = "missing_baseline"
)

func (f RepresentationEvaluationFailure) valid() bool {
	switch f {
	case RepresentationFailureIncompatible, RepresentationFailureSourceSearchMissing, RepresentationFailureCriticalFactLoss,
		RepresentationFailureScope, RepresentationFailureLifecycle, RepresentationFailureFreshness, RepresentationFailureBrokenReference,
		RepresentationFailureBudget, RepresentationFailureLatency, RepresentationFailureNondeterministic, RepresentationFailureMissingBaseline:
		return true
	default:
		return false
	}
}

type RepresentationEvaluationLimits struct {
	MaxCandidates int `json:"max_candidates"`
	MaxCharacters int `json:"max_characters"`
	MaxTokens     int `json:"max_tokens"`
	MaxLatencyMS  int `json:"max_latency_ms"`
}

func (l RepresentationEvaluationLimits) validate() error {
	if l.MaxCandidates <= 0 || l.MaxCandidates > 1000 || l.MaxCharacters <= 0 || l.MaxCharacters > 1_000_000 || l.MaxTokens <= 0 || l.MaxTokens > 100_000 || l.MaxLatencyMS <= 0 || l.MaxLatencyMS > 300_000 {
		return fmt.Errorf("representation evaluation limits are invalid")
	}
	return nil
}

type RepresentationEvaluationPolicy struct {
	Version   string                         `json:"version"`
	Baseline  string                         `json:"baseline"`
	Candidate RepresentationCandidate        `json:"candidate"`
	Mode      RepresentationEvaluationMode   `json:"mode"`
	Limits    RepresentationEvaluationLimits `json:"limits"`
}

func (p RepresentationEvaluationPolicy) validate() error {
	if !representationSafeIdentity(p.Version) || p.Baseline != ParentFirstFlatStrategy || !p.Candidate.valid() {
		return fmt.Errorf("representation evaluation policy identities are invalid")
	}
	if p.Mode != RepresentationEvaluationModeOffline && p.Mode != RepresentationEvaluationModeShadow {
		return fmt.Errorf("representation evaluation mode must be offline or shadow")
	}
	return p.Limits.validate()
}

// RepresentationEvaluationInput consists only of pre-aggregated counts and
// identities from an evaluation fixture/adapter. Raw tasks, queries, sources,
// and candidate payloads never enter this boundary.
type RepresentationEvaluationInput struct {
	// These observations are supplied only by an owned-stack harness after
	// real retrieval and baseline-equivalence checks. Offline fixtures leave
	// them false; a configured release DSN cannot upgrade synthetic evidence.
	RealStack             bool
	RollbackTested        bool
	Scope                 memory.Scope
	Policy                RepresentationEvaluationPolicy
	Family                RepresentationTaskFamily
	FixtureVersion        string
	SourceChecksum        string
	EvidenceDigest        string
	QRELChecksum          string
	SourceWatermark       string
	RendererVersion       string
	EvaluatedAt           time.Time
	CriticalFactsExpected int
	CriticalFactsReturned int
	ExpectedSources       int
	ReturnedSources       int
	ExpectedCitations     int
	ReturnedCitations     int
	CandidateCount        int
	CharacterCount        int
	TokenCount            int
	LatencyMS             int
	SourceSearchRetained  bool
	BrokenReferences      int
	StaleOrNoisyEvidence  int
	SafetyFailures        []RepresentationEvaluationFailure
	DeterministicReplay   bool
	TaskObservation       *bool
	RepeatedErrorCount    *int
	MaintenanceCost       *int64
	Baseline              *RepresentationComparisonMetrics
}

// RepresentationComparisonMetrics records measured values and distinguishes
// missing external observations from an observed zero or failed task.
type RepresentationComparisonMetrics struct {
	CriticalFactRetention float64 `json:"critical_fact_retention"`
	SourceCoverage        float64 `json:"source_coverage"`
	CitationCoverage      float64 `json:"citation_coverage"`
	CandidateCount        int     `json:"candidate_count"`
	CharacterCount        int     `json:"character_count"`
	TokenCount            int     `json:"token_count"`
	LatencyMS             int     `json:"latency_ms"`
	BrokenReferenceCount  int     `json:"broken_reference_count"`
	StaleOrNoisyCount     int     `json:"stale_or_noisy_count"`
	TaskObserved          bool    `json:"task_observed"`
	TaskSucceeded         bool    `json:"task_succeeded"`
	RepeatedErrorObserved bool    `json:"repeated_error_observed"`
	RepeatedErrorCount    int     `json:"repeated_error_count"`
	MaintenanceObserved   bool    `json:"maintenance_observed"`
	MaintenanceCostTotal  int64   `json:"maintenance_cost_total"`
}

// RepresentationComparisonDelta is candidate minus baseline. External outcome
// deltas stay absent until both sides observed the corresponding outcome.
type RepresentationComparisonDelta struct {
	CriticalFactRetention float64 `json:"critical_fact_retention"`
	SourceCoverage        float64 `json:"source_coverage"`
	CitationCoverage      float64 `json:"citation_coverage"`
	CandidateCount        int     `json:"candidate_count"`
	CharacterCount        int     `json:"character_count"`
	TokenCount            int     `json:"token_count"`
	LatencyMS             int     `json:"latency_ms"`
	BrokenReferenceCount  int     `json:"broken_reference_count"`
	StaleOrNoisyCount     int     `json:"stale_or_noisy_count"`
	TaskSucceeded         *int    `json:"task_succeeded,omitempty"`
	RepeatedErrorCount    *int    `json:"repeated_error_count,omitempty"`
	MaintenanceCostTotal  *int64  `json:"maintenance_cost_total,omitempty"`
}

func (m RepresentationComparisonMetrics) validate() error {
	if !boundedRate(m.CriticalFactRetention) || !boundedRate(m.SourceCoverage) || !boundedRate(m.CitationCoverage) ||
		m.CandidateCount < 0 || m.CharacterCount < 0 || m.TokenCount < 0 || m.LatencyMS < 0 || m.BrokenReferenceCount < 0 || m.StaleOrNoisyCount < 0 || m.RepeatedErrorCount < 0 || m.MaintenanceCostTotal < 0 ||
		(!m.TaskObserved && m.TaskSucceeded) || (!m.RepeatedErrorObserved && m.RepeatedErrorCount != 0) || (!m.MaintenanceObserved && m.MaintenanceCostTotal != 0) {
		return fmt.Errorf("representation comparison metrics are invalid")
	}
	return nil
}

// RepresentationSourceSearchObservation is a bounded aggregate emitted by the
// existing authorized source-search path. It intentionally cannot contain
// result IDs, source text, queries, or scores.
type RepresentationSourceSearchObservation struct {
	Scope             memory.Scope
	Active            bool
	ReturnedSources   int
	ReturnedCitations int
	CandidateCount    int
	CharacterCount    int
	TokenCount        int
	LatencyMS         int
}

// RepresentationPathAdapterInput connects EV3 evaluation to existing derived
// evaluators without adding a public retrieval surface. Exactly one candidate
// material path is inspected according to Candidate; authorized source search
// is always inspected independently and must remain retained.
type RepresentationPathAdapterInput struct {
	CriticalFacts           []RepresentationCriticalFact
	SourceEvidence          []ProgressiveContextEvidence
	ExpectedWatermarkHash   string
	ExpectedPolicyVersion   string
	ExpectedRendererVersion string
	Scope                   memory.Scope
	Family                  RepresentationTaskFamily
	Candidate               RepresentationCandidate
	Limits                  RepresentationEvaluationLimits
	EvaluatedAt             time.Time
	SourceSearch            RepresentationSourceSearchObservation
	ExactEvidence           []ProgressiveContextEvidence
	TopicProjection         *memory.ContextProjection
	ParentFirst             ParentFirstEvaluationInput
	ExpectedEvidence        int
}

// RepresentationCriticalFact is fixture-only material. Its alias and value
// never enter aggregate reports or persisted experiment JSON.
type RepresentationCriticalFact struct{ Alias, Value string }

// RepresentationPathObservation is a bounded adapter result suitable for
// RepresentationEvaluationInput. It never crosses a public/API boundary.
type RepresentationPathObservation struct {
	CriticalFactsReturned int
	ReturnedSources       int
	ReturnedCitations     int
	CandidateCount        int
	CharacterCount        int
	TokenCount            int
	LatencyMS             int
	SourceSearchRetained  bool
	BrokenReferences      int
	StaleOrNoisyEvidence  int
	SafetyFailures        []RepresentationEvaluationFailure
	DeterministicReplay   bool
}

// ObserveTaskAppropriateRepresentation runs only the existing bounded
// progressive/parent-first evaluators. It does not query a public route, alter
// a selected retrieval strategy, or construct an agent prompt.
func ObserveTaskAppropriateRepresentation(in RepresentationPathAdapterInput) (RepresentationPathObservation, error) {
	if err := in.Scope.Validate(); err != nil {
		return RepresentationPathObservation{}, err
	}
	if !in.Family.valid() || !in.Candidate.valid() || !representationCandidateCompatible(in.Family, in.Candidate) {
		return RepresentationPathObservation{}, fmt.Errorf("representation path candidate is incompatible")
	}
	if err := in.Limits.validate(); err != nil {
		return RepresentationPathObservation{}, err
	}
	if in.ExpectedEvidence <= 0 || in.ExpectedEvidence > 1000 {
		return RepresentationPathObservation{}, fmt.Errorf("representation path expected evidence is invalid")
	}
	result := RepresentationPathObservation{
		ReturnedSources: in.SourceSearch.ReturnedSources, ReturnedCitations: in.SourceSearch.ReturnedCitations,
		CandidateCount: in.SourceSearch.CandidateCount, CharacterCount: in.SourceSearch.CharacterCount,
		TokenCount: in.SourceSearch.TokenCount, LatencyMS: in.SourceSearch.LatencyMS,
		DeterministicReplay: true,
	}
	if in.SourceSearch.Scope != in.Scope {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
	} else if !in.SourceSearch.Active {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
	} else {
		result.SourceSearchRetained = true
	}
	now := in.EvaluatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	references := make(map[string]ProgressiveContextEvidence, len(in.SourceEvidence))
	for _, evidence := range in.SourceEvidence {
		if evidence.Alias == "" || evidence.Memory.ID == "" || evidence.Memory.SourceVersion <= 0 {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
		}
		if evidence.Memory.Scope != in.Scope {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
		}
		if evidence.Memory.State != memory.MemoryStateActive {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
		}
		if !(memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}).Matches(evidence.Memory.TemporalValidity, now) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
		}
		if _, duplicate := references[evidence.Alias]; duplicate {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
		}
		references[evidence.Alias] = evidence
	}
	if len(references) == 0 {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
	}
	texts := make(map[string][]string)
	switch in.Candidate {
	case RepresentationCandidateExactExcerptWithSearch:
		levels, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: in.Scope, EvaluatedAt: now, BaselineIdentity: ParentFirstFlatStrategy, Levels: []ProgressiveContextLevelInput{{Identity: "representation-exact-v1", Kind: ProgressiveContextLevelCanonicalChunk, Evidence: in.ExactEvidence, ExpectedEvidence: in.ExpectedEvidence, CharacterBudget: in.Limits.MaxCharacters, TokenBudget: in.Limits.MaxTokens}}})
		if err != nil {
			return RepresentationPathObservation{}, err
		}
		applyRepresentationLevelObservation(&result, levels.Levels[0], in.ExpectedEvidence)
		observeRepresentationEvidence(&result, in.Scope, in.ExactEvidence, references, texts, now)
	case RepresentationCandidateTopicProjectionWithSearch:
		if in.TopicProjection == nil {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
			break
		}
		if in.TopicProjection.Scope != in.Scope {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
			break
		}
		if in.TopicProjection.UpdatedAt.IsZero() || in.TopicProjection.UpdatedAt.After(now) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
		}
		observeRepresentationProjection(&result, in, references, texts, now)
		levels, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: in.Scope, EvaluatedAt: now, BaselineIdentity: ParentFirstFlatStrategy, Levels: []ProgressiveContextLevelInput{{Identity: "representation-topic-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: in.TopicProjection, ExpectedWatermarkHash: in.ExpectedWatermarkHash, MaxAge: 24 * time.Hour, ExpectedEvidence: in.ExpectedEvidence, CharacterBudget: in.Limits.MaxCharacters, TokenBudget: in.Limits.MaxTokens}}})
		if err != nil {
			return RepresentationPathObservation{}, err
		}
		applyRepresentationLevelObservation(&result, levels.Levels[0], in.ExpectedEvidence)
	case RepresentationCandidateHierarchicalWithSearch:
		parent := in.ParentFirst
		if parent.Scope == (memory.Scope{}) {
			parent.Scope = in.Scope
		}
		if parent.StrategyIdentity == "" {
			parent.StrategyIdentity = "representation-parent-first-v1"
		}
		if parent.Mode == "" {
			parent.Mode = ParentFirstModeShadow
		}
		if parent.Limits.MaxCandidates == 0 {
			parent.Limits.MaxCandidates = in.Limits.MaxCandidates
		}
		if parent.Scope != in.Scope {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
			break
		}
		if parent.Mode != ParentFirstModeShadow {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureIncompatible)
			break
		}
		observeRepresentationParents(&result, in, parent, references, now)
		parentReport, err := EvaluateParentFirst(parent)
		if err != nil {
			return RepresentationPathObservation{}, err
		}
		if !parentReport.Eligible {
			result.SafetyFailures = append(result.SafetyFailures, mapParentFirstRepresentationFailures(parentReport.HardFailures)...)
		}
		levels, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: in.Scope, EvaluatedAt: now, BaselineIdentity: ParentFirstFlatStrategy, Levels: []ProgressiveContextLevelInput{{Identity: "representation-hierarchy-v1", Kind: ProgressiveContextLevelCanonicalChunk, Evidence: parentReport.Expanded, ExpectedEvidence: in.ExpectedEvidence, CharacterBudget: in.Limits.MaxCharacters, TokenBudget: in.Limits.MaxTokens}}})
		if err != nil {
			return RepresentationPathObservation{}, err
		}
		applyRepresentationLevelObservation(&result, levels.Levels[0], in.ExpectedEvidence)
		if !containsRepresentationFailureValue(result.SafetyFailures, RepresentationFailureFreshness) {
			observeRepresentationEvidence(&result, in.Scope, parentReport.Expanded, references, texts, now)
		}
	}
	for _, fact := range in.CriticalFacts {
		reference, exists := references[fact.Alias]
		if !exists || fact.Value == "" || !strings.Contains(reference.Memory.Content, fact.Value) {
			continue
		}
		for _, text := range texts[fact.Alias] {
			if strings.Contains(text, fact.Value) {
				result.CriticalFactsReturned++
				break
			}
		}
	}
	if len(in.CriticalFacts) == 0 || result.CriticalFactsReturned != len(in.CriticalFacts) {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureCriticalFactLoss)
	}
	result.SafetyFailures = canonicalRepresentationFailures(result.SafetyFailures)
	if containsRepresentationFailureValue(result.SafetyFailures, RepresentationFailureBrokenReference) && result.BrokenReferences == 0 {
		result.BrokenReferences = 1
	}
	if containsRepresentationFailureValue(result.SafetyFailures, RepresentationFailureFreshness) && result.StaleOrNoisyEvidence == 0 {
		result.StaleOrNoisyEvidence = 1
	}
	if len(result.SafetyFailures) > 0 {
		result.DeterministicReplay = false
	}
	return result, nil
}

func applyRepresentationLevelObservation(result *RepresentationPathObservation, level ProgressiveContextLevelReport, expected int) {
	result.CandidateCount += int(math.Ceil(level.EvidenceCoverage * float64(expected)))
	result.CharacterCount += level.CharacterCount
	result.TokenCount += level.TokenCount
	for _, failure := range level.FailureReasons {
		switch failure {
		case ProgressiveContextFailureIsolation:
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
		case ProgressiveContextFailureLifecycle:
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
		case ProgressiveContextFailureStale, ProgressiveContextFailureWatermark:
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
		case ProgressiveContextFailureBudget:
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBudget)
		case ProgressiveContextFailureCitation:
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
		}
	}
}

func observeRepresentationEvidence(result *RepresentationPathObservation, scope memory.Scope, evidence []ProgressiveContextEvidence, references map[string]ProgressiveContextEvidence, texts map[string][]string, now time.Time) {
	for _, candidate := range evidence {
		reference, exists := references[candidate.Alias]
		if !exists || candidate.Memory.ID == "" || candidate.Memory.ID != reference.Memory.ID {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
			result.BrokenReferences++
			continue
		}
		if candidate.Memory.Scope != scope || reference.Memory.Scope != scope {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
			continue
		}
		if candidate.Memory.State != memory.MemoryStateActive || reference.Memory.State != memory.MemoryStateActive {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
			continue
		}
		if candidate.Memory.SourceVersion <= 0 || candidate.Memory.SourceVersion != reference.Memory.SourceVersion {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			result.StaleOrNoisyEvidence++
			continue
		}
		current := memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
		if !current.Matches(candidate.Memory.TemporalValidity, now) || !current.Matches(reference.Memory.TemporalValidity, now) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			result.StaleOrNoisyEvidence++
			continue
		}
		validCitation := len(candidate.Citations) > 0
		for _, citation := range candidate.Citations {
			if !representationCitationResolves(citation.MemoryID, citation.RawEventID, reference) {
				validCitation = false
			}
		}
		if !validCitation || strings.TrimSpace(candidate.Memory.Content) == "" || !strings.Contains(reference.Memory.Content, candidate.Memory.Content) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
			result.BrokenReferences++
			continue
		}
		texts[candidate.Alias] = append(texts[candidate.Alias], candidate.Memory.Content)
	}
}

func representationCitationResolves(memoryID, rawEventID string, reference ProgressiveContextEvidence) bool {
	if memoryID == "" || memoryID != reference.Memory.ID {
		return false
	}
	if rawEventID == "" {
		return true
	}
	for _, source := range reference.Citations {
		if source.MemoryID == memoryID && source.RawEventID == rawEventID {
			return true
		}
	}
	return false
}

func observeRepresentationProjection(result *RepresentationPathObservation, in RepresentationPathAdapterInput, references map[string]ProgressiveContextEvidence, texts map[string][]string, now time.Time) {
	p := in.TopicProjection
	if p == nil {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
		return
	}
	if p.Scope != in.Scope {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
	}
	if p.Status != memory.ContextProjectionStatusActive {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
	}
	if p.UpdatedAt.IsZero() || p.UpdatedAt.After(now) || now.Sub(p.UpdatedAt) > 24*time.Hour ||
		in.ExpectedWatermarkHash == "" || in.ExpectedWatermarkHash != p.SourceWatermarkHash() ||
		in.ExpectedPolicyVersion == "" || in.ExpectedPolicyVersion != p.PolicyVersion ||
		in.ExpectedRendererVersion == "" || in.ExpectedRendererVersion != p.RendererVersion {
		result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
	}
	for _, item := range p.Items {
		var reference ProgressiveContextEvidence
		found := false
		for _, source := range references {
			if source.Memory.ID == item.Source.MemoryID {
				reference, found = source, true
				break
			}
		}
		if !found || !representationCitationResolves(item.Citation.MemoryID, item.Citation.RawEventID, reference) || item.Source.Kind != memory.ContextProjectionSourceCanonicalVersion ||
			strings.TrimSpace(item.Text) == "" || !strings.Contains(reference.Memory.Content, item.Text) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
			result.BrokenReferences++
			continue
		}
		if item.Source.Scope != in.Scope || reference.Memory.Scope != in.Scope {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureScope)
			continue
		}
		if item.LifecycleState != memory.MemoryStateActive || (item.Source.LifecycleState != "" && item.Source.LifecycleState != memory.MemoryStateActive) || reference.Memory.State != memory.MemoryStateActive {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureLifecycle)
			continue
		}
		watermarked := false
		for _, versionID := range p.SourceWatermark.CanonicalVersionIDs {
			if versionID == item.Source.ID && versionID != "" {
				watermarked = true
				break
			}
		}
		if !watermarked || reference.Memory.SourceVersion <= 0 || item.Source.Version != reference.Memory.SourceVersion || item.Citation.SourceVersion != reference.Memory.SourceVersion || p.UpdatedAt.Before(reference.Memory.ModifiedAt) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			result.StaleOrNoisyEvidence++
			continue
		}
		current := memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
		if !current.Matches(item.TemporalValidity, now) || !current.Matches(reference.Memory.TemporalValidity, now) {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			result.StaleOrNoisyEvidence++
			continue
		}
		texts[reference.Alias] = append(texts[reference.Alias], item.Text)
	}
}

func observeRepresentationParents(result *RepresentationPathObservation, in RepresentationPathAdapterInput, parent ParentFirstEvaluationInput, references map[string]ProgressiveContextEvidence, now time.Time) {
	for _, candidate := range parent.Parents {
		if candidate.UpdatedAt.IsZero() || candidate.UpdatedAt.After(now) || now.Sub(candidate.UpdatedAt) > 24*time.Hour {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
		}
		if in.TopicProjection != nil && candidate.ID == in.TopicProjection.ID {
			observeRepresentationProjection(result, in, references, map[string][]string{}, now)
			if !candidate.UpdatedAt.Equal(in.TopicProjection.UpdatedAt) {
				result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			}
			for _, child := range parent.Children {
				if child.ParentID != candidate.ID {
					continue
				}
				bound := false
				for _, item := range in.TopicProjection.Items {
					if item.Source.MemoryID == child.Evidence.Memory.ID && item.Source.Version == child.Evidence.Memory.SourceVersion {
						bound = true
						break
					}
				}
				if !bound {
					result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
				}
			}
			continue
		}
		bound := false
		for _, reference := range references {
			if candidate.ID != reference.Memory.ID {
				continue
			}
			bound = true
			if reference.Memory.ModifiedAt.IsZero() || candidate.UpdatedAt.Before(reference.Memory.ModifiedAt) {
				result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureFreshness)
			}
			for _, child := range parent.Children {
				if child.ParentID == candidate.ID && child.Evidence.Memory.ID != reference.Memory.ID {
					result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
				}
			}
		}
		if !bound {
			result.SafetyFailures = append(result.SafetyFailures, RepresentationFailureBrokenReference)
		}
	}
}

func mapParentFirstRepresentationFailures(failures []ParentFirstFailureReason) []RepresentationEvaluationFailure {
	result := make([]RepresentationEvaluationFailure, 0, len(failures))
	for _, failure := range failures {
		switch failure {
		case ParentFirstFailureIsolation:
			result = append(result, RepresentationFailureScope)
		case ParentFirstFailureLifecycle:
			result = append(result, RepresentationFailureLifecycle)
		case ParentFirstFailureCandidateBudget:
			result = append(result, RepresentationFailureBudget)
		case ParentFirstFailureLatency:
			result = append(result, RepresentationFailureLatency)
		default:
			result = append(result, RepresentationFailureBrokenReference)
		}
	}
	return result
}

func maxRepresentationInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func (in RepresentationEvaluationInput) validate() error {
	if err := in.Scope.Validate(); err != nil {
		return err
	}
	if err := in.Policy.validate(); err != nil {
		return err
	}
	if !in.Family.valid() || !representationSafeIdentity(in.FixtureVersion) || !validRepresentationChecksum(in.SourceChecksum) || !validRepresentationChecksum(in.QRELChecksum) || !representationSafeIdentity(in.SourceWatermark) || !representationSafeIdentity(in.RendererVersion) {
		return fmt.Errorf("representation evaluation fixture identity is invalid")
	}
	if in.EvaluatedAt.IsZero() {
		return fmt.Errorf("representation evaluation timestamp is required")
	}
	if in.EvidenceDigest != "" && !validRepresentationChecksum(in.EvidenceDigest) {
		return fmt.Errorf("representation evidence digest is invalid")
	}
	if in.CriticalFactsExpected <= 0 || in.CriticalFactsReturned < 0 || in.ExpectedSources <= 0 || in.ReturnedSources < 0 || in.ExpectedCitations <= 0 || in.ReturnedCitations < 0 || in.CandidateCount < 0 || in.CharacterCount < 0 || in.TokenCount < 0 || in.LatencyMS < 0 || in.BrokenReferences < 0 || in.StaleOrNoisyEvidence < 0 {
		return fmt.Errorf("representation evaluation metrics are invalid")
	}
	if in.MaintenanceCost != nil && *in.MaintenanceCost < 0 {
		return fmt.Errorf("representation maintenance cost is invalid")
	}
	if in.RepeatedErrorCount != nil && *in.RepeatedErrorCount < 0 {
		return fmt.Errorf("representation repeated error count is invalid")
	}
	if in.Baseline != nil {
		if err := in.Baseline.validate(); err != nil {
			return err
		}
	}
	for _, failure := range in.SafetyFailures {
		if !failure.valid() {
			return fmt.Errorf("representation evaluation failure is invalid")
		}
	}
	return nil
}

func validRepresentationChecksum(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func representationSafeIdentity(value string) bool {
	if strings.TrimSpace(value) == "" || !evaluationSafeIdentity(value) || strings.TrimSpace(value) != value {
		return false
	}
	return !strings.ContainsAny(value, " \t\r\n")
}

type RepresentationEvaluationReport struct {
	EvidenceDigest string `json:"evidence_digest,omitempty"`
	// PersistenceExpiresAt is trusted storage metadata, outside the hashed JSON.
	PersistenceExpiresAt    *time.Time                        `json:"-"`
	BaselineMetrics         *RepresentationComparisonMetrics  `json:"baseline_metrics,omitempty"`
	ComparisonDelta         *RepresentationComparisonDelta    `json:"comparison_delta,omitempty"`
	TaskObserved            bool                              `json:"task_observed"`
	TaskSucceeded           bool                              `json:"task_succeeded"`
	RepeatedErrorObserved   bool                              `json:"repeated_error_observed"`
	RepeatedErrorCount      int                               `json:"repeated_error_count"`
	MaintenanceObserved     bool                              `json:"maintenance_observed"`
	RealStack               bool                              `json:"real_stack"`
	RollbackTested          bool                              `json:"rollback_tested"`
	PolicyVersion           string                            `json:"policy_version"`
	FixtureVersion          string                            `json:"fixture_version"`
	Limits                  RepresentationEvaluationLimits    `json:"limits"`
	RunIdentity             string                            `json:"run_identity"`
	PolicyIdentity          string                            `json:"policy_identity"`
	ScopeHash               string                            `json:"scope_hash"`
	FixtureIdentity         string                            `json:"fixture_identity"`
	SourceChecksum          string                            `json:"source_checksum"`
	QRELChecksum            string                            `json:"qrels_checksum"`
	SourceWatermark         string                            `json:"source_watermark"`
	RendererVersion         string                            `json:"renderer_version"`
	Family                  RepresentationTaskFamily          `json:"family"`
	Baseline                string                            `json:"baseline"`
	Candidate               RepresentationCandidate           `json:"candidate"`
	Mode                    RepresentationEvaluationMode      `json:"mode"`
	SelectedStrategy        string                            `json:"selected_strategy"`
	Comparable              bool                              `json:"comparable"`
	ActivationEligible      bool                              `json:"activation_eligible"`
	Verdict                 RepresentationEvaluationVerdict   `json:"verdict"`
	RollbackRequired        bool                              `json:"rollback_required"`
	Failures                []RepresentationEvaluationFailure `json:"failures,omitempty"`
	CriticalFactRetention   float64                           `json:"critical_fact_retention"`
	SourceCoverage          float64                           `json:"source_coverage"`
	CitationCoverage        float64                           `json:"citation_coverage"`
	CandidateCount          int                               `json:"candidate_count"`
	CharacterCount          int                               `json:"character_count"`
	TokenCount              int                               `json:"token_count"`
	LatencyMS               int                               `json:"latency_ms"`
	SourceSearchRetained    bool                              `json:"source_search_retained"`
	BrokenReferenceCount    int                               `json:"broken_reference_count"`
	StaleOrNoisyCount       int                               `json:"stale_or_noisy_count"`
	TaskObservationCoverage float64                           `json:"task_observation_coverage"`
	MaintenanceCoverage     float64                           `json:"maintenance_coverage"`
	MaintenanceCostTotal    int64                             `json:"maintenance_cost_total"`
	EvaluatedAt             time.Time                         `json:"evaluated_at"`
}

// EvaluateRepresentation evaluates one offline/shadow candidate without
// routing production retrieval. A passing comparison is intentionally never
// activation eligible; a future, separately authorized release gate may consume
// it as one prerequisite.
func EvaluateRepresentation(in RepresentationEvaluationInput) (RepresentationEvaluationReport, error) {
	if err := in.validate(); err != nil {
		return RepresentationEvaluationReport{}, err
	}
	r := RepresentationEvaluationReport{
		EvidenceDigest: in.EvidenceDigest,
		RealStack:      in.RealStack, RollbackTested: in.RollbackTested,
		PolicyVersion: in.Policy.Version, FixtureVersion: in.FixtureVersion, Limits: in.Policy.Limits,
		PolicyIdentity: representationPolicyIdentity(in.Policy), ScopeHash: scopeHash(in.Scope), FixtureIdentity: representationFixtureIdentity(in),
		SourceChecksum: in.SourceChecksum, QRELChecksum: in.QRELChecksum, SourceWatermark: in.SourceWatermark, RendererVersion: in.RendererVersion,
		Family: in.Family, Baseline: in.Policy.Baseline, Candidate: in.Policy.Candidate, Mode: in.Policy.Mode,
		SelectedStrategy: ParentFirstFlatStrategy, Comparable: true, ActivationEligible: false, Verdict: RepresentationEvaluationShadow,
		CriticalFactRetention: ratio(in.CriticalFactsReturned, in.CriticalFactsExpected), SourceCoverage: ratio(in.ReturnedSources, in.ExpectedSources),
		CitationCoverage: ratio(in.ReturnedCitations, in.ExpectedCitations), CandidateCount: in.CandidateCount, CharacterCount: in.CharacterCount,
		TokenCount: in.TokenCount, LatencyMS: in.LatencyMS, SourceSearchRetained: in.SourceSearchRetained,
		BrokenReferenceCount: in.BrokenReferences, StaleOrNoisyCount: in.StaleOrNoisyEvidence,
		EvaluatedAt: in.EvaluatedAt.UTC(),
	}
	if in.TaskObservation != nil {
		r.TaskObservationCoverage = 1
		r.TaskObserved, r.TaskSucceeded = true, *in.TaskObservation
	}
	if in.RepeatedErrorCount != nil {
		r.RepeatedErrorObserved, r.RepeatedErrorCount = true, *in.RepeatedErrorCount
	}
	if in.MaintenanceCost != nil {
		r.MaintenanceObserved = true
		r.MaintenanceCoverage = 1
		r.MaintenanceCostTotal = *in.MaintenanceCost
	}
	failures := append([]RepresentationEvaluationFailure(nil), in.SafetyFailures...)
	if in.RealStack && !validRepresentationChecksum(in.EvidenceDigest) {
		failures = append(failures, RepresentationFailureIncompatible)
	}
	if in.Baseline == nil {
		failures = append(failures, RepresentationFailureMissingBaseline)
	} else {
		baseline := *in.Baseline
		r.BaselineMetrics = &baseline
		delta := representationComparisonDelta(representationReportMetrics(r), baseline)
		r.ComparisonDelta = &delta
		failures = append(failures, representationMetricsSafetyFailures(baseline, in.Policy.Limits)...)
	}
	if !representationCandidateCompatible(in.Family, in.Policy.Candidate) {
		failures = append(failures, RepresentationFailureIncompatible)
	}
	if !in.SourceSearchRetained {
		failures = append(failures, RepresentationFailureSourceSearchMissing)
	}
	if in.CriticalFactsReturned < in.CriticalFactsExpected {
		failures = append(failures, RepresentationFailureCriticalFactLoss)
	}
	if in.BrokenReferences > 0 {
		failures = append(failures, RepresentationFailureBrokenReference)
	}
	if in.ReturnedSources < in.ExpectedSources || in.ReturnedCitations < in.ExpectedCitations {
		failures = append(failures, RepresentationFailureBrokenReference)
	}
	if in.StaleOrNoisyEvidence > 0 {
		failures = append(failures, RepresentationFailureFreshness)
	}
	if in.CandidateCount > in.Policy.Limits.MaxCandidates || in.CharacterCount > in.Policy.Limits.MaxCharacters || in.TokenCount > in.Policy.Limits.MaxTokens {
		failures = append(failures, RepresentationFailureBudget)
	}
	if in.LatencyMS > in.Policy.Limits.MaxLatencyMS {
		failures = append(failures, RepresentationFailureLatency)
	}
	if !in.DeterministicReplay {
		failures = append(failures, RepresentationFailureNondeterministic)
	}
	r.Failures = canonicalRepresentationFailures(failures)
	if len(r.Failures) != 0 {
		r.Comparable = false
		r.Verdict = RepresentationEvaluationNonPass
		r.RollbackRequired = true
	}
	r.RunIdentity = representationRunIdentity(r, in.EvaluatedAt)
	return r, nil
}

func ratio(returned, expected int) float64 {
	if expected <= 0 {
		return 0
	}
	value := float64(returned) / float64(expected)
	return math.Min(1, value)
}

func representationReportMetrics(r RepresentationEvaluationReport) RepresentationComparisonMetrics {
	return RepresentationComparisonMetrics{
		CriticalFactRetention: r.CriticalFactRetention, SourceCoverage: r.SourceCoverage, CitationCoverage: r.CitationCoverage,
		CandidateCount: r.CandidateCount, CharacterCount: r.CharacterCount, TokenCount: r.TokenCount, LatencyMS: r.LatencyMS,
		BrokenReferenceCount: r.BrokenReferenceCount, StaleOrNoisyCount: r.StaleOrNoisyCount,
		TaskObserved: r.TaskObserved, TaskSucceeded: r.TaskSucceeded, RepeatedErrorObserved: r.RepeatedErrorObserved,
		RepeatedErrorCount: r.RepeatedErrorCount, MaintenanceObserved: r.MaintenanceObserved, MaintenanceCostTotal: r.MaintenanceCostTotal,
	}
}

func representationMetricsSafetyFailures(metrics RepresentationComparisonMetrics, limits RepresentationEvaluationLimits) []RepresentationEvaluationFailure {
	var failures []RepresentationEvaluationFailure
	if metrics.CriticalFactRetention != 1 {
		failures = append(failures, RepresentationFailureCriticalFactLoss)
	}
	if metrics.SourceCoverage != 1 || metrics.CitationCoverage != 1 || metrics.BrokenReferenceCount != 0 {
		failures = append(failures, RepresentationFailureBrokenReference)
	}
	if metrics.StaleOrNoisyCount != 0 {
		failures = append(failures, RepresentationFailureFreshness)
	}
	if metrics.CandidateCount > limits.MaxCandidates || metrics.CharacterCount > limits.MaxCharacters || metrics.TokenCount > limits.MaxTokens {
		failures = append(failures, RepresentationFailureBudget)
	}
	if metrics.LatencyMS > limits.MaxLatencyMS {
		failures = append(failures, RepresentationFailureLatency)
	}
	return failures
}

func representationComparisonDelta(candidate, baseline RepresentationComparisonMetrics) RepresentationComparisonDelta {
	d := RepresentationComparisonDelta{
		CriticalFactRetention: candidate.CriticalFactRetention - baseline.CriticalFactRetention,
		SourceCoverage:        candidate.SourceCoverage - baseline.SourceCoverage, CitationCoverage: candidate.CitationCoverage - baseline.CitationCoverage,
		CandidateCount: candidate.CandidateCount - baseline.CandidateCount, CharacterCount: candidate.CharacterCount - baseline.CharacterCount,
		TokenCount: candidate.TokenCount - baseline.TokenCount, LatencyMS: candidate.LatencyMS - baseline.LatencyMS,
		BrokenReferenceCount: candidate.BrokenReferenceCount - baseline.BrokenReferenceCount, StaleOrNoisyCount: candidate.StaleOrNoisyCount - baseline.StaleOrNoisyCount,
	}
	if candidate.TaskObserved && baseline.TaskObserved {
		outcome := 0
		if candidate.TaskSucceeded {
			outcome++
		}
		if baseline.TaskSucceeded {
			outcome--
		}
		d.TaskSucceeded = &outcome
	}
	if candidate.RepeatedErrorObserved && baseline.RepeatedErrorObserved {
		count := candidate.RepeatedErrorCount - baseline.RepeatedErrorCount
		d.RepeatedErrorCount = &count
	}
	if candidate.MaintenanceObserved && baseline.MaintenanceObserved {
		cost := candidate.MaintenanceCostTotal - baseline.MaintenanceCostTotal
		d.MaintenanceCostTotal = &cost
	}
	return d
}

func canonicalRepresentationFailures(failures []RepresentationEvaluationFailure) []RepresentationEvaluationFailure {
	seen := make(map[RepresentationEvaluationFailure]struct{}, len(failures))
	result := make([]RepresentationEvaluationFailure, 0, len(failures))
	for _, failure := range failures {
		if _, exists := seen[failure]; exists {
			continue
		}
		seen[failure] = struct{}{}
		result = append(result, failure)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func representationPolicyIdentity(policy RepresentationEvaluationPolicy) string {
	encoded, _ := json.Marshal(policy)
	sum := sha256.Sum256(encoded)
	return "representation-policy:" + hex.EncodeToString(sum[:])
}

func representationFixtureIdentity(in RepresentationEvaluationInput) string {
	encoded, _ := json.Marshal(struct {
		Family, Fixture, Source, QREL, Watermark, Renderer string
	}{string(in.Family), in.FixtureVersion, in.SourceChecksum, in.QRELChecksum, in.SourceWatermark, in.RendererVersion})
	sum := sha256.Sum256(encoded)
	return "representation-fixture:" + hex.EncodeToString(sum[:])
}

func representationRunIdentity(report RepresentationEvaluationReport, evaluatedAt time.Time) string {
	report.RunIdentity = ""
	report.EvaluatedAt = evaluatedAt.UTC()
	encoded, _ := json.Marshal(report)
	sum := sha256.Sum256(encoded)
	return "representation-experiment:" + hex.EncodeToString(sum[:])
}

// MarshalRepresentationEvaluationReport is the redacted serialization
// boundary. It rejects invalid or potentially non-report-safe reports.
func MarshalRepresentationEvaluationReport(report RepresentationEvaluationReport) ([]byte, error) {
	if (report.EvidenceDigest != "" && !validRepresentationChecksum(report.EvidenceDigest)) || (report.RealStack && report.Comparable && !validRepresentationChecksum(report.EvidenceDigest)) {
		return nil, fmt.Errorf("representation evidence digest is invalid or missing")
	}
	if !strings.HasPrefix(report.RunIdentity, "representation-experiment:") || len(report.RunIdentity) != len("representation-experiment:")+64 ||
		!strings.HasPrefix(report.PolicyIdentity, "representation-policy:") || len(report.PolicyIdentity) != len("representation-policy:")+64 ||
		!strings.HasPrefix(report.FixtureIdentity, "representation-fixture:") || len(report.FixtureIdentity) != len("representation-fixture:")+64 ||
		!strings.HasPrefix(report.ScopeHash, "scope:") || len(report.ScopeHash) != len("scope:")+64 ||
		!report.Family.valid() || !report.Candidate.valid() || report.Baseline != ParentFirstFlatStrategy || !validRepresentationChecksum(report.SourceChecksum) || !validRepresentationChecksum(report.QRELChecksum) ||
		!representationSafeIdentity(report.SourceWatermark) || !representationSafeIdentity(report.RendererVersion) || report.EvaluatedAt.IsZero() {
		return nil, fmt.Errorf("representation evaluation report identities are invalid")
	}
	for _, identity := range []struct{ value, prefix string }{
		{report.RunIdentity, "representation-experiment:"}, {report.PolicyIdentity, "representation-policy:"},
		{report.FixtureIdentity, "representation-fixture:"}, {report.ScopeHash, "scope:"},
	} {
		if !validRepresentationChecksum(strings.TrimPrefix(identity.value, identity.prefix)) {
			return nil, fmt.Errorf("representation evaluation report hash is invalid")
		}
	}
	policy := RepresentationEvaluationPolicy{Version: report.PolicyVersion, Baseline: report.Baseline, Candidate: report.Candidate, Mode: report.Mode, Limits: report.Limits}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	if !representationSafeIdentity(report.FixtureVersion) || representationPolicyIdentity(policy) != report.PolicyIdentity ||
		representationFixtureIdentity(RepresentationEvaluationInput{Family: report.Family, FixtureVersion: report.FixtureVersion, SourceChecksum: report.SourceChecksum, QRELChecksum: report.QRELChecksum, SourceWatermark: report.SourceWatermark, RendererVersion: report.RendererVersion}) != report.FixtureIdentity ||
		representationRunIdentity(report, report.EvaluatedAt) != report.RunIdentity {
		return nil, fmt.Errorf("representation evaluation report identity mismatch")
	}
	if report.CandidateCount < 0 || report.CharacterCount < 0 || report.TokenCount < 0 || report.LatencyMS < 0 || report.BrokenReferenceCount < 0 || report.StaleOrNoisyCount < 0 || report.MaintenanceCostTotal < 0 {
		return nil, fmt.Errorf("representation evaluation report counts are invalid")
	}
	if err := representationReportMetrics(report).validate(); err != nil {
		return nil, err
	}
	if (report.TaskObservationCoverage != 0 && report.TaskObservationCoverage != 1) || (report.MaintenanceCoverage != 0 && report.MaintenanceCoverage != 1) ||
		(report.TaskObservationCoverage == 1) != report.TaskObserved || (report.MaintenanceCoverage == 1) != report.MaintenanceObserved {
		return nil, fmt.Errorf("representation observation coverage is inconsistent")
	}
	if report.BaselineMetrics == nil {
		if report.Comparable || report.ComparisonDelta != nil || !containsRepresentationFailureValue(report.Failures, RepresentationFailureMissingBaseline) {
			return nil, fmt.Errorf("representation comparison baseline is missing")
		}
	} else {
		if err := report.BaselineMetrics.validate(); err != nil {
			return nil, err
		}
		if report.Comparable && len(representationMetricsSafetyFailures(*report.BaselineMetrics, report.Limits)) != 0 {
			return nil, fmt.Errorf("representation comparison baseline is unsafe")
		}
		expected := representationComparisonDelta(representationReportMetrics(report), *report.BaselineMetrics)
		actualJSON, _ := json.Marshal(report.ComparisonDelta)
		expectedJSON, _ := json.Marshal(expected)
		if string(actualJSON) != string(expectedJSON) {
			return nil, fmt.Errorf("representation comparison delta is inconsistent")
		}
	}
	if report.Verdict == RepresentationEvaluationShadow && (!report.Comparable || report.RollbackRequired || len(report.Failures) != 0 || !report.SourceSearchRetained ||
		report.CriticalFactRetention != 1 || report.SourceCoverage != 1 || report.CitationCoverage != 1 || report.BrokenReferenceCount != 0 || report.StaleOrNoisyCount != 0 ||
		report.CandidateCount > report.Limits.MaxCandidates || report.CharacterCount > report.Limits.MaxCharacters || report.TokenCount > report.Limits.MaxTokens || report.LatencyMS > report.Limits.MaxLatencyMS || !representationCandidateCompatible(report.Family, report.Candidate)) {
		return nil, fmt.Errorf("representation evaluation shadow report is inconsistent")
	}
	if report.Verdict == RepresentationEvaluationNonPass && (report.Comparable || !report.RollbackRequired || len(report.Failures) == 0) {
		return nil, fmt.Errorf("representation evaluation non-pass report is inconsistent")
	}
	if report.Verdict != RepresentationEvaluationShadow && report.Verdict != RepresentationEvaluationNonPass {
		return nil, fmt.Errorf("representation evaluation report verdict is invalid")
	}
	if report.ActivationEligible || report.SelectedStrategy != ParentFirstFlatStrategy {
		return nil, fmt.Errorf("representation evaluation report cannot select an active strategy")
	}
	for _, rate := range []float64{report.CriticalFactRetention, report.SourceCoverage, report.CitationCoverage, report.TaskObservationCoverage, report.MaintenanceCoverage} {
		if !boundedRate(rate) {
			return nil, fmt.Errorf("representation evaluation report rate is invalid")
		}
	}
	for _, failure := range report.Failures {
		if !failure.valid() {
			return nil, fmt.Errorf("representation evaluation report failure is invalid")
		}
	}
	return json.Marshal(report)
}

func containsRepresentationFailureValue(failures []RepresentationEvaluationFailure, want RepresentationEvaluationFailure) bool {
	for _, failure := range failures {
		if failure == want {
			return true
		}
	}
	return false
}
