package benchmark

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// BenchmarkFamily keeps reports from unlike evaluation tracks incomparable.
type BenchmarkFamily string

const (
	FamilyMemory               BenchmarkFamily = "memory"
	FamilyProviderContract     BenchmarkFamily = "provider_contract"
	FamilySpecializedRetrieval BenchmarkFamily = "specialized_retrieval"
	FamilyGenericRetrieval     BenchmarkFamily = "generic_retrieval"
	FamilyStress               BenchmarkFamily = "stress"
)

func validBenchmarkFamily(value string) bool {
	switch BenchmarkFamily(value) {
	case FamilyMemory, FamilyProviderContract, FamilySpecializedRetrieval, FamilyGenericRetrieval, FamilyStress:
		return true
	default:
		return false
	}
}

type ContractOperation struct {
	Name       string             `json:"name"`
	Arguments  map[string]any     `json:"arguments,omitempty"`
	Expected   any                `json:"expected,omitempty"`
	Scope      memory.Scope       `json:"scope"`
	SessionID  string             `json:"session_id,omitempty"`
	Lifecycle  memory.MemoryState `json:"lifecycle,omitempty"`
	MustRefuse bool               `json:"must_refuse,omitempty"`
}

type ContractCase struct {
	ID        string            `json:"id"`
	Subset    string            `json:"subset"`
	Operation ContractOperation `json:"operation"`
	Result    any               `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
}

type ContractOutcome struct {
	CaseID           string `json:"case_id"`
	Subset           string `json:"subset"`
	Operation        string `json:"operation"`
	Valid            bool   `json:"valid"`
	Refusal          bool   `json:"refusal"`
	ScopeFailure     bool   `json:"scope_failure"`
	LifecycleFailure bool   `json:"lifecycle_failure"`
	Malformed        bool   `json:"malformed"`
	Reason           string `json:"reason,omitempty"`
}

type ContractReport struct {
	SchemaVersion           string            `json:"schema_version"`
	Family                  BenchmarkFamily   `json:"family"`
	Dataset                 string            `json:"dataset"`
	SubsetCounts            map[string]int    `json:"subset_counts"`
	OperationAccuracy       float64           `json:"operation_accuracy"`
	MalformedRate           float64           `json:"malformed_call_rate"`
	RefusalAccuracy         float64           `json:"refusal_accuracy"`
	ScopeSafetyFailures     int               `json:"scope_safety_failures"`
	LifecycleSafetyFailures int               `json:"lifecycle_safety_failures"`
	Outcomes                []ContractOutcome `json:"outcomes"`
	Status                  Status            `json:"status"`
}

func (c ContractCase) validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Subset) == "" {
		return fmt.Errorf("contract case id and subset are required")
	}
	if err := c.Operation.Scope.Validate(); err != nil {
		return fmt.Errorf("contract scope: %w", err)
	}
	if strings.TrimSpace(c.Operation.Name) == "" {
		return fmt.Errorf("contract operation name is required")
	}
	return nil
}

// ReplayContract validates the provider-facing operation shape without making
// network calls or invoking an LLM. A replay is deliberately deterministic.
func ReplayContract(cases []ContractCase) ContractReport {
	report := ContractReport{SchemaVersion: SchemaVersion, Family: FamilyProviderContract, Dataset: "bfcl-memory", SubsetCounts: map[string]int{}, Status: StatusSuccess}
	for _, item := range cases {
		outcome := ContractOutcome{CaseID: item.ID, Subset: item.Subset, Operation: item.Operation.Name}
		if err := item.validate(); err != nil {
			outcome.Malformed, outcome.Reason = true, err.Error()
			report.Status = StatusQualityGateFailed
			report.Outcomes = append(report.Outcomes, outcome)
			continue
		}
		report.SubsetCounts[item.Subset]++
		known := map[string]bool{"memory_kv": true, "memory_rec_sum": true, "memory_vector": true}
		if !known[item.Subset] {
			outcome.Malformed, outcome.Reason = true, "unsupported contract subset"
		}
		if !knownOperation(item.Operation.Name) {
			outcome.Malformed, outcome.Reason = true, "unsupported memory operation"
		}
		if item.Operation.Scope.Tenant == "foreign" {
			outcome.ScopeFailure, outcome.Reason = true, "foreign tenant denied"
		}
		if item.Operation.Lifecycle == memory.MemoryStateForgotten || item.Operation.Lifecycle == memory.MemoryStateSuppressed || item.Operation.Lifecycle == memory.MemoryStateDeleted {
			outcome.LifecycleFailure = !item.Operation.MustRefuse
			if outcome.LifecycleFailure {
				outcome.Reason = "hidden lifecycle must be refused"
			}
		}
		outcome.Refusal = item.Operation.MustRefuse && (outcome.Malformed || outcome.ScopeFailure || outcome.LifecycleFailure)
		outcome.Valid = !outcome.Malformed && !outcome.ScopeFailure && !outcome.LifecycleFailure && (!item.Operation.MustRefuse || outcome.Refusal)
		if !outcome.Valid || outcome.ScopeFailure || outcome.LifecycleFailure {
			report.Status = StatusQualityGateFailed
		}
		report.Outcomes = append(report.Outcomes, outcome)
	}
	if len(report.Outcomes) == 0 {
		report.Status = StatusPrerequisiteMissing
		return report
	}
	valid, malformed, refusals, refusalCorrect := 0, 0, 0, 0
	for _, o := range report.Outcomes {
		if o.Valid {
			valid++
		}
		if o.Malformed {
			malformed++
		}
		if o.Refusal {
			refusals++
			refusalCorrect++
		}
	}
	n := float64(len(report.Outcomes))
	report.OperationAccuracy = float64(valid) / n
	report.MalformedRate = float64(malformed) / n
	if refusals > 0 {
		report.RefusalAccuracy = float64(refusalCorrect) / float64(refusals)
	}
	for _, o := range report.Outcomes {
		if o.ScopeFailure {
			report.ScopeSafetyFailures++
		}
		if o.LifecycleFailure {
			report.LifecycleSafetyFailures++
		}
	}
	return report
}

func knownOperation(name string) bool {
	switch name {
	case "get", "put", "search", "update", "forget", "summarize", "vector_search":
		return true
	default:
		return false
	}
}

func MarshalContractReport(report ContractReport) ([]byte, error) {
	report.SchemaVersion = SchemaVersion
	return json.Marshal(report)
}

type SpecializedCase struct {
	ID        string              `json:"id"`
	Subfamily string              `json:"subfamily"`
	Query     BenchmarkQuery      `json:"query"`
	Events    []MemoryEventRecord `json:"events"`
	QRELs     []QREL              `json:"qrels"`
}

// BuiltinSpecializedCases is license-safe, repository-owned coverage for the
// profile, temporal, and multi-hop tracks.
func BuiltinSpecializedCases(scope memory.Scope) []SpecializedCase {
	return []SpecializedCase{
		{ID: "profile-preference-v1", Subfamily: "profile", Events: []MemoryEventRecord{{ID: "profile-current", Scope: scope, SessionID: "profile-session-2", SourceTurnID: "profile-turn-2", Class: memory.MemoryClassProfile, Text: "The user prefers concise answers.", ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"fact_version": "2", "source": "repository-fixture"}}, {ID: "profile-old", Scope: scope, SessionID: "profile-session-1", SourceTurnID: "profile-turn-1", Class: memory.MemoryClassProfile, Text: "The user preferred detailed answers.", ExpectedState: memory.MemoryStateSuppressed, Provenance: map[string]string{"fact_version": "1", "source": "repository-fixture"}}}, Query: BenchmarkQuery{ID: "profile-q1", Scope: scope, SessionID: "profile-session-2", Text: "What answer style does the user prefer?", QueryType: "preference", EvidenceGroups: []EvidenceGroup{{ID: "profile-g", EvidenceIDs: []string{"profile-current"}, Required: true}}, MustNotReturnIDs: []string{"profile-old"}}, QRELs: []QREL{{QueryID: "profile-q1", EvidenceID: "profile-current", Grade: 3, Role: "supporting", GroupID: "profile-g"}}},
		{ID: "temporal-update-v1", Subfamily: "temporal", Events: []MemoryEventRecord{{ID: "temporal-old", Scope: scope, Class: memory.MemoryClassEpisodic, Text: "The service used version one.", ObservedAt: "2024-01-01T00:00:00Z", ExpectedState: memory.MemoryStateSuppressed}, {ID: "temporal-new", Scope: scope, Class: memory.MemoryClassEpisodic, Text: "The service uses version two.", ObservedAt: "2025-01-01T00:00:00Z", ExpectedState: memory.MemoryStateActive}}, Query: BenchmarkQuery{ID: "temporal-q1", Scope: scope, Text: "What version does the service use now?", QueryType: "temporal", QuestionDate: "2025-02-01T00:00:00Z", EvidenceGroups: []EvidenceGroup{{ID: "temporal-g", EvidenceIDs: []string{"temporal-new"}, Required: true}}, MustNotReturnIDs: []string{"temporal-old"}}, QRELs: []QREL{{QueryID: "temporal-q1", EvidenceID: "temporal-new", Grade: 3, Role: "supporting", GroupID: "temporal-g"}}},
		{ID: "multi-hop-v1", Subfamily: "multi-hop", Events: []MemoryEventRecord{{ID: "hop-a", Scope: scope, SessionID: "hop-session", SourceTurnID: "hop-turn-a", Class: memory.MemoryClassRelation, Text: "Ada maintains the Stele service.", ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"evidence_role": "bridge"}}, {ID: "hop-b", Scope: scope, SessionID: "hop-session", SourceTurnID: "hop-turn-b", Class: memory.MemoryClassProcedural, Text: "Stele stores memory in PostgreSQL.", ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"evidence_role": "target"}}}, Query: BenchmarkQuery{ID: "multi-hop-q1", Scope: scope, SessionID: "hop-session", Text: "Which database backs Ada's memory service?", QueryType: "multi-hop", EvidenceGroups: []EvidenceGroup{{ID: "hop-g", EvidenceIDs: []string{"hop-a", "hop-b"}, Required: true}}}, QRELs: []QREL{{QueryID: "multi-hop-q1", EvidenceID: "hop-a", Grade: 2, Role: "supporting", GroupID: "hop-g"}, {QueryID: "multi-hop-q1", EvidenceID: "hop-b", Grade: 3, Role: "supporting", GroupID: "hop-g"}}},
	}
}

type StressBudget struct {
	MaxContextTokens int           `json:"max_context_tokens"`
	MaxSamples       int           `json:"max_samples"`
	Timeout          time.Duration `json:"timeout"`
}
type StressCase struct {
	ID            string  `json:"id"`
	ContextTokens int     `json:"context_tokens"`
	NeedleCount   int     `json:"needle_count"`
	Position      float64 `json:"position"`
	Mode          string  `json:"mode"`
}
type StressReport struct {
	SchemaVersion string          `json:"schema_version"`
	Family        BenchmarkFamily `json:"family"`
	Dataset       string          `json:"dataset"`
	NonGating     bool            `json:"non_gating"`
	Status        Status          `json:"status"`
	Buckets       map[string]int  `json:"buckets"`
	Failures      []string        `json:"failures,omitempty"`
}

func EvaluateStress(dataset string, cases []StressCase, budget StressBudget, visualAvailable bool) StressReport {
	r := StressReport{SchemaVersion: SchemaVersion, Family: FamilyStress, Dataset: dataset, NonGating: true, Status: StatusSuccess, Buckets: map[string]int{}}
	for _, c := range cases {
		key := fmt.Sprintf("%d", c.ContextTokens)
		r.Buckets[key]++
		if budget.MaxContextTokens > 0 && c.ContextTokens > budget.MaxContextTokens {
			r.Failures = append(r.Failures, c.ID+": capacity budget")
		}
		if budget.MaxSamples > 0 && len(cases) > budget.MaxSamples {
			r.Failures = append(r.Failures, "sample budget exceeded")
			break
		}
		if c.Mode == "visual" && !visualAvailable {
			r.Failures = append(r.Failures, c.ID+": visual capability missing")
		}
	}
	if len(r.Failures) > 0 {
		r.Status = StatusCapacityRefused
	}
	return r
}

func CanonicalSpecializedCases(cases []SpecializedCase) []SpecializedCase {
	out := append([]SpecializedCase(nil), cases...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
