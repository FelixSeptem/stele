package benchmark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

const (
	RepresentationFixtureSchemaVersion = "representation-fixture-v1"
	maxRepresentationFixtureCases      = 64
	maxRepresentationFixtureAliases    = 32
	maxRepresentationFixtureBytes      = 1 << 20
)

// RepresentationFixture is repository-owned input for EV3 comparisons. Source
// aliases are fixture-local and never become report fields or database IDs.
type RepresentationFixture struct {
	SchemaVersion  string                      `json:"schema_version"`
	SourceChecksum string                      `json:"source_checksum"`
	QRELChecksum   string                      `json:"qrels_checksum"`
	Cases          []RepresentationFixtureCase `json:"cases"`
}

type RepresentationFixtureCase struct {
	ID                    string                                   `json:"id"`
	Family                retrieval.RepresentationTaskFamily       `json:"family"`
	Candidate             retrieval.RepresentationCandidate        `json:"candidate"`
	Scope                 memory.Scope                             `json:"scope"`
	Query                 string                                   `json:"query"`
	Sources               []retrieval.EvaluationSource             `json:"sources"`
	CriticalFacts         []retrieval.RepresentationCriticalFact   `json:"critical_facts"`
	CriticalAlias         string                                   `json:"critical_alias"`
	RequiredSourceAliases []string                                 `json:"required_source_aliases"`
	ExcludedSourceAliases []string                                 `json:"excluded_source_aliases,omitempty"`
	BaselineLimits        retrieval.RepresentationEvaluationLimits `json:"baseline_limits"`
	CandidateLimits       retrieval.RepresentationEvaluationLimits `json:"candidate_limits"`
}

// RepresentationObservation is an evaluation-only aggregate returned by an
// existing authorized retrieval/projection adapter. It deliberately cannot
// carry a query, source text, IDs, scores, or an agent/model outcome.
type RepresentationObservation struct {
	// Evidence and references stay inside the offline runner. Aggregates are
	// derived from these records, never from caller-supplied success counts.
	Evidence              []retrieval.ProgressiveContextEvidence
	SearchEvidence        []retrieval.ProgressiveContextEvidence
	SourceEvidence        []retrieval.ProgressiveContextEvidence
	SourceWatermark       string
	RealStack             bool
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
	SafetyFailures        []retrieval.RepresentationEvaluationFailure
	DeterministicReplay   bool
	TaskObservation       *bool
	MaintenanceCost       *int64
	RepeatedErrorCount    *int
}

// RepresentationObserver is implemented by the offline/shadow adapter that
// invokes existing exact-scope retrieval/projection paths. It must never be
// wired into ordinary public retrieval or Provider/MCP request handling.
type RepresentationObserver interface {
	ObserveRepresentation(context.Context, RepresentationFixtureCase, string) (RepresentationObservation, error)
}

// ReplayRepresentationFixture converts locked fixture cases into the common
// shadow-only representation evaluator. It does not mutate memory, select a
// strategy, or infer task outcomes.
func ReplayRepresentationFixture(ctx context.Context, fixture RepresentationFixture, rendererVersion string, evaluatedAt time.Time, observer RepresentationObserver) ([]retrieval.RepresentationEvaluationReport, error) {
	if observer == nil {
		return nil, fmt.Errorf("representation observer is required")
	}
	if err := fixture.validate(); err != nil {
		return nil, err
	}
	qrelChecksum, sourceChecksum, err := RepresentationFixtureLocks(fixture)
	if err != nil {
		return nil, err
	}
	if fixture.QRELChecksum != qrelChecksum || fixture.SourceChecksum != sourceChecksum {
		return nil, fmt.Errorf("representation fixture checksum does not match its lock")
	}
	if strings.TrimSpace(rendererVersion) == "" || strings.ContainsAny(rendererVersion, " \t\r\n\\") {
		return nil, fmt.Errorf("representation renderer version is invalid")
	}
	if evaluatedAt.IsZero() {
		return nil, fmt.Errorf("representation evaluation time is required")
	}
	reports := make([]retrieval.RepresentationEvaluationReport, 0, len(fixture.Cases))
	watermark := ""
	for _, item := range fixture.Cases {
		baseline, err := observer.ObserveRepresentation(ctx, item, retrieval.ParentFirstFlatStrategy)
		if err != nil {
			return nil, fmt.Errorf("observe baseline: %w", err)
		}
		observation, err := observer.ObserveRepresentation(ctx, item, string(item.Candidate))
		if err != nil {
			return nil, fmt.Errorf("observe representation %s: %w", item.ID, err)
		}
		repeated, err := observer.ObserveRepresentation(ctx, item, string(item.Candidate))
		if err != nil {
			return nil, fmt.Errorf("repeat candidate: %w", err)
		}
		rollback, err := observer.ObserveRepresentation(ctx, item, retrieval.ParentFirstFlatStrategy)
		if err != nil {
			return nil, fmt.Errorf("rollback baseline: %w", err)
		}
		if watermark == "" {
			watermark = baseline.SourceWatermark
		}
		evidenceDigest := representationEvidenceDigest(baseline, observation, repeated, rollback)
		baseline = measureRepresentationObservation(item, baseline, false, evaluatedAt)
		observation = measureRepresentationObservation(item, observation, true, evaluatedAt)
		if watermark == "" || baseline.SourceWatermark != watermark || observation.SourceWatermark != watermark || repeated.SourceWatermark != watermark || rollback.SourceWatermark != watermark {
			observation.SafetyFailures = append(observation.SafetyFailures, retrieval.RepresentationFailureFreshness)
		}
		observation.DeterministicReplay = sameRepresentationReplay(observation, measureRepresentationObservation(item, repeated, true, evaluatedAt))
		rollbackTested := sameRepresentationReplay(baseline, measureRepresentationObservation(item, rollback, false, evaluatedAt))
		if !rollbackTested {
			observation.DeterministicReplay = false
		}
		baselineMetrics := representationComparisonMetrics(item, baseline)
		report, err := retrieval.EvaluateRepresentation(retrieval.RepresentationEvaluationInput{
			Baseline:       &baselineMetrics,
			EvidenceDigest: evidenceDigest,
			RealStack:      baseline.RealStack && observation.RealStack && repeated.RealStack && rollback.RealStack,
			RollbackTested: rollbackTested,
			Scope:          item.Scope,
			Policy:         retrieval.RepresentationEvaluationPolicy{Version: "representation-evaluation-v1", Baseline: "flat-fusion-v1", Candidate: item.Candidate, Mode: retrieval.RepresentationEvaluationModeShadow, Limits: item.CandidateLimits},
			Family:         item.Family, FixtureVersion: fixture.SchemaVersion, SourceChecksum: sourceChecksum, QRELChecksum: qrelChecksum,
			SourceWatermark: watermark, RendererVersion: rendererVersion, EvaluatedAt: evaluatedAt,
			CriticalFactsExpected: len(item.CriticalFacts), CriticalFactsReturned: observation.CriticalFactsReturned,
			ExpectedSources: len(item.RequiredSourceAliases), ReturnedSources: observation.ReturnedSources,
			ExpectedCitations: len(item.RequiredSourceAliases), ReturnedCitations: observation.ReturnedCitations,
			CandidateCount: observation.CandidateCount, CharacterCount: observation.CharacterCount, TokenCount: observation.TokenCount,
			LatencyMS: observation.LatencyMS, SourceSearchRetained: observation.SourceSearchRetained, BrokenReferences: observation.BrokenReferences,
			StaleOrNoisyEvidence: observation.StaleOrNoisyEvidence, SafetyFailures: observation.SafetyFailures,
			DeterministicReplay: observation.DeterministicReplay, TaskObservation: observation.TaskObservation, MaintenanceCost: observation.MaintenanceCost, RepeatedErrorCount: observation.RepeatedErrorCount,
		})
		if err != nil {
			return nil, fmt.Errorf("evaluate representation %s: %w", item.ID, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func LoadRepresentationFixture(source []byte) (RepresentationFixture, error) {
	if len(source) == 0 || len(source) > maxRepresentationFixtureBytes {
		return RepresentationFixture{}, &StatusError{Status: StatusInvalidManifest, Message: "representation fixture is empty or exceeds the byte limit"}
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var fixture RepresentationFixture
	if err := decoder.Decode(&fixture); err != nil {
		return RepresentationFixture{}, &StatusError{Status: StatusInvalidManifest, Message: "decode representation fixture", Cause: err}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return RepresentationFixture{}, &StatusError{Status: StatusInvalidManifest, Message: "decode representation fixture", Cause: err}
	}
	if err := fixture.validate(); err != nil {
		return RepresentationFixture{}, &StatusError{Status: StatusInvalidManifest, Message: "validate representation fixture", Cause: err}
	}
	qrel, sourceLock, err := RepresentationFixtureLocks(fixture)
	if err != nil {
		return RepresentationFixture{}, err
	}
	if !strings.EqualFold(fixture.QRELChecksum, qrel) || !strings.EqualFold(fixture.SourceChecksum, sourceLock) {
		return RepresentationFixture{}, &StatusError{Status: StatusChecksumMismatch, Message: "representation fixture checksum does not match its lock"}
	}
	return fixture, nil
}

// RepresentationFixtureLocks produces separate canonical qrel and source
// locks. The source lock includes the qrel lock while clearing itself.
func RepresentationFixtureLocks(fixture RepresentationFixture) (qrelChecksum, sourceChecksum string, err error) {
	if err := fixture.validate(); err != nil {
		return "", "", err
	}
	qrelInput := struct {
		SchemaVersion string                      `json:"schema_version"`
		Cases         []RepresentationFixtureCase `json:"cases"`
	}{SchemaVersion: fixture.SchemaVersion, Cases: fixture.Cases}
	qrelPayload, err := json.Marshal(qrelInput)
	if err != nil {
		return "", "", fmt.Errorf("marshal representation qrel checksum input: %w", err)
	}
	qrelChecksum = checksumBytes(qrelPayload)
	fixture.QRELChecksum = qrelChecksum
	fixture.SourceChecksum = ""
	payload, err := json.Marshal(fixture)
	if err != nil {
		return "", "", fmt.Errorf("marshal representation source checksum input: %w", err)
	}
	sourceChecksum = checksumBytes(payload)
	return qrelChecksum, sourceChecksum, nil
}

func (fixture RepresentationFixture) validate() error {
	if fixture.SchemaVersion != RepresentationFixtureSchemaVersion || len(fixture.Cases) != 4 || len(fixture.Cases) > maxRepresentationFixtureCases {
		return fmt.Errorf("representation fixture schema or case count is invalid")
	}
	seenIDs := make(map[string]struct{}, len(fixture.Cases))
	seenFamilies := make(map[retrieval.RepresentationTaskFamily]struct{}, len(fixture.Cases))
	for _, item := range fixture.Cases {
		if err := item.validate(); err != nil {
			return err
		}
		if _, exists := seenIDs[item.ID]; exists {
			return fmt.Errorf("duplicate representation fixture case id")
		}
		seenIDs[item.ID] = struct{}{}
		if _, exists := seenFamilies[item.Family]; exists {
			return fmt.Errorf("duplicate representation fixture task family")
		}
		seenFamilies[item.Family] = struct{}{}
	}
	for _, family := range []retrieval.RepresentationTaskFamily{
		retrieval.RepresentationTaskFamilyExactFacts,
		retrieval.RepresentationTaskFamilyCrossDocument,
		retrieval.RepresentationTaskFamilyProfilePreference,
		retrieval.RepresentationTaskFamilyProceduralLesson,
	} {
		if _, exists := seenFamilies[family]; !exists {
			return fmt.Errorf("representation fixture is missing required task family")
		}
	}
	return nil
}

func (item RepresentationFixtureCase) validate() error {
	if !representationFixtureSafeIdentity(item.ID) || !representationFixtureSafeIdentity(item.CriticalAlias) || !retrieval.RepresentationCandidateSupportsTaskFamily(item.Family, item.Candidate) {
		return fmt.Errorf("representation fixture case identity is invalid")
	}
	if err := item.Scope.Validate(); err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(item.Scope.Project), "benchmark-") {
		return fmt.Errorf("representation fixture scope must use benchmark project")
	}
	if strings.TrimSpace(item.Query) == "" || len(item.Query) > 2048 || len(item.Sources) == 0 || len(item.Sources) > maxRepresentationFixtureAliases || len(item.CriticalFacts) == 0 || len(item.CriticalFacts) > maxRepresentationFixtureAliases {
		return fmt.Errorf("representation fixture query, sources or facts are invalid")
	}
	sources := make(map[string]retrieval.EvaluationSource)
	for _, source := range item.Sources {
		if !representationFixtureSafeIdentity(source.Alias) || source.Content == "" || len(source.Content) > 65536 {
			return fmt.Errorf("invalid representation source")
		}
		if _, exists := sources[source.Alias]; exists {
			return fmt.Errorf("duplicate representation source")
		}
		sources[source.Alias] = source
	}
	if err := validateRepresentationLimits(item.BaselineLimits); err != nil {
		return err
	}
	if item.CandidateLimits != item.BaselineLimits {
		return fmt.Errorf("representation fixture baseline and candidate budgets must match")
	}
	if len(item.RequiredSourceAliases) == 0 || len(item.RequiredSourceAliases) > maxRepresentationFixtureAliases || len(item.ExcludedSourceAliases) > maxRepresentationFixtureAliases {
		return fmt.Errorf("representation fixture aliases are invalid")
	}
	aliases := make(map[string]struct{}, len(item.RequiredSourceAliases))
	criticalFound := false
	for _, alias := range item.RequiredSourceAliases {
		if !representationFixtureSafeIdentity(alias) {
			return fmt.Errorf("representation fixture required alias is invalid")
		}
		if _, exists := aliases[alias]; exists {
			return fmt.Errorf("duplicate representation fixture required alias")
		}
		aliases[alias] = struct{}{}
		if source, exists := sources[alias]; !exists || source.State != memory.MemoryStateActive {
			return fmt.Errorf("required source missing or hidden")
		}
		criticalFound = criticalFound || alias == item.CriticalAlias
	}
	if !criticalFound {
		return fmt.Errorf("representation fixture critical alias is not required")
	}
	for _, alias := range item.ExcludedSourceAliases {
		if !representationFixtureSafeIdentity(alias) {
			return fmt.Errorf("representation fixture excluded alias is invalid")
		}
		if _, required := aliases[alias]; required {
			return fmt.Errorf("representation fixture excluded alias is required")
		}
		aliases[alias] = struct{}{}
		if _, exists := sources[alias]; !exists {
			return fmt.Errorf("excluded source missing")
		}
	}
	for _, fact := range item.CriticalFacts {
		source, exists := sources[fact.Alias]
		if !exists || fact.Value == "" || !strings.Contains(source.Content, fact.Value) {
			return fmt.Errorf("critical fact missing from source")
		}
		found := false
		for _, alias := range item.RequiredSourceAliases {
			found = found || alias == fact.Alias
		}
		if !found {
			return fmt.Errorf("critical fact source is not required")
		}
	}
	return nil
}

func measureRepresentationObservation(item RepresentationFixtureCase, observation RepresentationObservation, candidate bool, evaluatedAt time.Time) RepresentationObservation {
	observation.CriticalFactsReturned, observation.ReturnedSources, observation.ReturnedCitations = 0, 0, 0
	observation.CandidateCount, observation.CharacterCount, observation.TokenCount = 0, 0, 0
	references := map[string]retrieval.ProgressiveContextEvidence{}
	for _, reference := range observation.SourceEvidence {
		references[reference.Alias] = reference
	}
	for _, source := range item.Sources {
		reference, ok := references[source.Alias]
		expectedContent := source.Content
		if source.State == memory.MemoryStateDeleted {
			expectedContent = ""
		}
		if !ok || reference.Memory.Content != expectedContent || reference.Memory.State != source.State || reference.Memory.Scope != item.Scope || reference.Memory.SourceVersion <= 0 {
			observation.SafetyFailures = append(observation.SafetyFailures, retrieval.RepresentationFailureFreshness)
		}
	}
	valid := map[string]retrieval.ProgressiveContextEvidence{}
	inspect := func(evidence []retrieval.ProgressiveContextEvidence, measured bool) {
		for _, entry := range evidence {
			observation.CandidateCount++
			observation.CharacterCount += len([]rune(entry.Memory.Content))
			observation.TokenCount += len(strings.Fields(entry.Memory.Content))
			if entry.Memory.Scope != item.Scope {
				observation.SafetyFailures = append(observation.SafetyFailures, retrieval.RepresentationFailureScope)
				continue
			}
			if entry.Memory.State != memory.MemoryStateActive {
				observation.SafetyFailures = append(observation.SafetyFailures, retrieval.RepresentationFailureLifecycle)
				continue
			}
			ref, exists := references[entry.Alias]
			if !exists || ref.Memory.ID != entry.Memory.ID || ref.Memory.SourceVersion != entry.Memory.SourceVersion || strings.TrimSpace(entry.Memory.Content) == "" || !strings.Contains(ref.Memory.Content, entry.Memory.Content) {
				observation.BrokenReferences++
				continue
			}
			current := memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
			if !current.Matches(entry.Memory.TemporalValidity, evaluatedAt) || !current.Matches(ref.Memory.TemporalValidity, evaluatedAt) {
				observation.SafetyFailures = append(observation.SafetyFailures, retrieval.RepresentationFailureFreshness)
				observation.StaleOrNoisyEvidence++
				continue
			}
			excluded := false
			for _, alias := range item.ExcludedSourceAliases {
				excluded = excluded || alias == entry.Alias
			}
			if excluded {
				observation.StaleOrNoisyEvidence++
				continue
			}
			cited := false
			allCitationsValid := true
			for _, citation := range entry.Citations {
				resolved := false
				for _, expected := range ref.Citations {
					resolved = resolved || (citation.MemoryID == ref.Memory.ID && citation.RawEventID != "" && citation.RawEventID == expected.RawEventID)
				}
				if !resolved {
					observation.BrokenReferences++
					allCitationsValid = false
				}
				cited = cited || resolved
			}
			if !cited || !allCitationsValid {
				observation.BrokenReferences++
				continue
			}
			if measured {
				valid[entry.Alias] = entry
			}
		}
	}
	inspect(observation.Evidence, true)
	if candidate {
		inspect(observation.SearchEvidence, false)
		observation.SourceSearchRetained = len(observation.SearchEvidence) > 0
	}
	for _, alias := range item.RequiredSourceAliases {
		if _, ok := valid[alias]; ok {
			observation.ReturnedSources++
			observation.ReturnedCitations++
		}
	}
	for _, fact := range item.CriticalFacts {
		if entry, ok := valid[fact.Alias]; ok && strings.Contains(entry.Memory.Content, fact.Value) {
			observation.CriticalFactsReturned++
		}
	}
	return observation
}

func sameRepresentationReplay(a, b RepresentationObservation) bool {
	// Latency is observational; result contents/references and hard failures must
	// replay byte for byte. No timing noise changes deterministic replay status.
	a.LatencyMS, b.LatencyMS = 0, 0
	a.DeterministicReplay, b.DeterministicReplay = false, false
	return representationEvidenceDigest(a) == representationEvidenceDigest(b)
}

func representationEvidenceDigest(observations ...RepresentationObservation) string {
	type versionBinding struct {
		ID      string
		Version int64
	}
	var versions []versionBinding
	for i := range observations {
		observations[i].LatencyMS = 0
		for _, group := range [][]retrieval.ProgressiveContextEvidence{observations[i].Evidence, observations[i].SearchEvidence, observations[i].SourceEvidence} {
			for _, entry := range group {
				versions = append(versions, versionBinding{entry.Memory.ID, entry.Memory.SourceVersion})
			}
		}
	}
	payload, _ := json.Marshal(struct {
		Observations []RepresentationObservation
		Versions     []versionBinding
	}{observations, versions})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func representationComparisonMetrics(item RepresentationFixtureCase, observation RepresentationObservation) retrieval.RepresentationComparisonMetrics {
	rate := func(n, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(n) / float64(total)
	}
	m := retrieval.RepresentationComparisonMetrics{CriticalFactRetention: rate(observation.CriticalFactsReturned, len(item.CriticalFacts)), SourceCoverage: rate(observation.ReturnedSources, len(item.RequiredSourceAliases)), CitationCoverage: rate(observation.ReturnedCitations, len(item.RequiredSourceAliases)), CandidateCount: observation.CandidateCount, CharacterCount: observation.CharacterCount, TokenCount: observation.TokenCount, LatencyMS: observation.LatencyMS, BrokenReferenceCount: observation.BrokenReferences, StaleOrNoisyCount: observation.StaleOrNoisyEvidence}
	if len(observation.SafetyFailures) > 0 {
		m.StaleOrNoisyCount++
	}
	if observation.TaskObservation != nil {
		m.TaskObserved = true
		m.TaskSucceeded = *observation.TaskObservation
	}
	if observation.MaintenanceCost != nil {
		m.MaintenanceObserved = true
		m.MaintenanceCostTotal = *observation.MaintenanceCost
	}
	if observation.RepeatedErrorCount != nil {
		m.RepeatedErrorObserved = true
		m.RepeatedErrorCount = *observation.RepeatedErrorCount
	}
	return m
}

func validateRepresentationLimits(limits retrieval.RepresentationEvaluationLimits) error {
	if limits.MaxCandidates <= 0 || limits.MaxCandidates > 1000 || limits.MaxCharacters <= 0 || limits.MaxCharacters > 1_000_000 || limits.MaxTokens <= 0 || limits.MaxTokens > 100_000 || limits.MaxLatencyMS <= 0 || limits.MaxLatencyMS > 300_000 {
		return fmt.Errorf("representation fixture limits are invalid")
	}
	return nil
}

func representationFixtureSafeIdentity(value string) bool {
	if value == "" || len(value) > 128 || strings.ContainsAny(value, " \t\r\n\\") || strings.Contains(value, "..") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' || character == '.' || character == ':' || character == '/' || character == '=' || character == ',' {
			continue
		}
		return false
	}
	return true
}
