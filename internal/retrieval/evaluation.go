package retrieval

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// EvaluationFixture is repository-owned test data for deterministic retrieval replay.
// Source content is only used to seed an explicitly owned test scope and is never
// included in diagnostics or reports.
type EvaluationFixture struct {
	Version string           `json:"version"`
	Cases   []EvaluationCase `json:"cases"`
}

// EvaluationCase defines one scoped retrieval assertion. Evidence aliases are stable
// fixture-local names; database identifiers are resolved only by the fixture seeder.
type EvaluationCase struct {
	ID                     string             `json:"id"`
	Category               string             `json:"category"`
	Scope                  memory.Scope       `json:"scope"`
	Query                  string             `json:"query"`
	Sources                []EvaluationSource `json:"sources"`
	ExpectedEvidenceGroups [][]string         `json:"expected_evidence_groups"`
	ExcludedAliases        []string           `json:"excluded_aliases,omitempty"`
}

// EvaluationSource is a controlled source event used by one evaluation case.
type EvaluationSource struct {
	Alias           string             `json:"alias"`
	EventType       string             `json:"event_type"`
	Content         string             `json:"content"`
	Class           memory.MemoryClass `json:"class,omitempty"`
	State           memory.MemoryState `json:"state,omitempty"`
	FactCluster     string             `json:"fact_cluster,omitempty"`
	SourceTimestamp time.Time          `json:"source_timestamp,omitempty"`
}

// EvaluationRankingMetadata identifies an evaluation report without exposing
// environment-specific provider settings or database connection information.
type EvaluationRankingMetadata struct {
	FixtureVersion              string `json:"fixture_version"`
	RepresentationVersion       string `json:"representation_version"`
	RankingVersion              string `json:"ranking_version"`
	CompatibleEmbeddingRevision string `json:"compatible_embedding_revision"`
	PolicyVersion               string `json:"policy_version"`
}

// EvaluationSafetyFailureCategory is a stable non-sensitive failure reason.
type EvaluationSafetyFailureCategory string

const (
	EvaluationSafetyFailureInvalidFixtureScope EvaluationSafetyFailureCategory = "invalid_fixture_scope"
	EvaluationSafetyFailureCrossScope          EvaluationSafetyFailureCategory = "cross_scope_result"
	EvaluationSafetyFailureLifecycleVisibility EvaluationSafetyFailureCategory = "lifecycle_visibility"
	EvaluationSafetyFailureUnsafeDiagnostics   EvaluationSafetyFailureCategory = "unsafe_diagnostics"
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
	RecallAt1                float64 `json:"recall_at_1"`
	RecallAt5                float64 `json:"recall_at_5"`
	RecallAt10               float64 `json:"recall_at_10"`
	MRR                      float64 `json:"mrr"`
	NDCGAt1                  float64 `json:"ndcg_at_1"`
	NDCGAt5                  float64 `json:"ndcg_at_5"`
	NDCGAt10                 float64 `json:"ndcg_at_10"`
	MultiHopEvidenceCoverage float64 `json:"multi_hop_evidence_coverage"`
	DuplicateRate            float64 `json:"duplicate_rate"`
	CandidatePoolSize        int     `json:"candidate_pool_size"`
	P50LatencyMS             float64 `json:"p50_latency_ms"`
	P95LatencyMS             float64 `json:"p95_latency_ms"`
}

// EvaluationCaseReport is a bounded per-case contribution to an evaluation report.
type EvaluationCaseReport struct {
	CaseID            string                    `json:"case_id"`
	Category          string                    `json:"category,omitempty"`
	Metrics           EvaluationMetricReport    `json:"metrics"`
	SafetyFailures    []EvaluationSafetyFailure `json:"safety_failures,omitempty"`
	CandidatePoolSize int                       `json:"candidate_pool_size"`
	LatencyMS         float64                   `json:"latency_ms"`
}

// EvaluationReport is the versioned data model rendered by local and CI replay.
type EvaluationReport struct {
	Metadata       EvaluationRankingMetadata `json:"metadata"`
	Cases          []EvaluationCaseReport    `json:"cases"`
	Metrics        EvaluationMetricReport    `json:"metrics"`
	SafetyFailures []EvaluationSafetyFailure `json:"safety_failures,omitempty"`
	GeneratedAt    time.Time                 `json:"generated_at"`
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
	Version                       string   `json:"version"`
	ProtectedCutoffs              []int    `json:"protected_cutoffs"`
	ProtectedCategories           []string `json:"protected_categories,omitempty"`
	MaxRecallRegression           float64  `json:"max_recall_regression"`
	MaxMultiHopCoverageRegression float64  `json:"max_multi_hop_coverage_regression"`
	MaxP95LatencyMS               int      `json:"max_p95_latency_ms"`
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
	if strings.TrimSpace(m.CompatibleEmbeddingRevision) == "" {
		return fmt.Errorf("compatible embedding revision is required")
	}
	if strings.TrimSpace(m.PolicyVersion) == "" {
		return fmt.Errorf("policy version is required")
	}
	return nil
}

func (f EvaluationFixture) Validate() error {
	if strings.TrimSpace(f.Version) == "" {
		return fmt.Errorf("fixture version is required")
	}
	if len(f.Cases) == 0 {
		return fmt.Errorf("fixture must include at least one case")
	}

	caseIDs := make(map[string]struct{}, len(f.Cases))
	for _, item := range f.Cases {
		if err := item.validate(); err != nil {
			return err
		}
		id := strings.TrimSpace(item.ID)
		if _, exists := caseIDs[id]; exists {
			return fmt.Errorf("duplicate case id")
		}
		caseIDs[id] = struct{}{}
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
	if len(c.Sources) == 0 {
		return fmt.Errorf("fixture case must include at least one source")
	}

	aliases := make(map[string]struct{}, len(c.Sources))
	for _, source := range c.Sources {
		alias := strings.TrimSpace(source.Alias)
		if alias == "" {
			return fmt.Errorf("source alias is required")
		}
		if _, exists := aliases[alias]; exists {
			return fmt.Errorf("duplicate source alias")
		}
		aliases[alias] = struct{}{}
		if strings.TrimSpace(source.EventType) == "" {
			return fmt.Errorf("source event type is required")
		}
		if strings.TrimSpace(source.Content) == "" {
			return fmt.Errorf("source content is required")
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
		}
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
		exclusions[alias] = struct{}{}
	}
	return nil
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
	if p.MaxRecallRegression < 0 || p.MaxMultiHopCoverageRegression < 0 {
		return fmt.Errorf("quality regression tolerances must be greater than or equal to zero")
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
		EvaluationSafetyFailureUnsafeDiagnostics:
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
