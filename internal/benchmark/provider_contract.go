package benchmark

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// ContractOperation is a checksum-lockable, offline representation of one
// BFCL memory provider call. It deliberately carries its expected scope so a
// runner cannot accidentally validate a cross-scope success.
type ContractOperation struct {
	Name        string             `json:"name"`
	Arguments   map[string]any     `json:"arguments,omitempty"`
	Expected    any                `json:"expected,omitempty"`
	Scope       memory.Scope       `json:"scope"`
	SessionID   string             `json:"session_id,omitempty"`
	Relevant    bool               `json:"relevant"`
	TargetState memory.MemoryState `json:"target_state,omitempty"`
}

type ContractFixture struct {
	SchemaVersion string              `json:"schema_version"`
	Subset        string              `json:"subset"`
	Operations    []ContractOperation `json:"operations"`
}

type ContractOutcome struct {
	Operation     string `json:"operation"`
	Valid         bool   `json:"valid"`
	Refused       bool   `json:"refused"`
	ScopeSafe     bool   `json:"scope_safe"`
	LifecycleSafe bool   `json:"lifecycle_safe"`
	Reason        string `json:"reason,omitempty"`
}

type ContractReport struct {
	SchemaVersion           string            `json:"schema_version"`
	Family                  string            `json:"family"`
	Subset                  string            `json:"subset"`
	Outcomes                []ContractOutcome `json:"outcomes"`
	OperationAccuracy       float64           `json:"operation_accuracy"`
	MalformedCallRate       float64           `json:"malformed_call_rate"`
	RefusalCorrectness      float64           `json:"refusal_correctness"`
	ScopeSafetyFailures     int               `json:"scope_safety_failures"`
	LifecycleSafetyFailures int               `json:"lifecycle_safety_failures"`
	MustNotReturnViolations int               `json:"must_not_return_violations"`
}

// ContractReplayScope carries the isolation boundary expected by a replayed
// provider call. An empty SessionID permits fixture operations from several
// sessions; a non-empty value requires an exact session match.
type ContractReplayScope struct {
	Scope     memory.Scope `json:"scope"`
	SessionID string       `json:"session_id,omitempty"`
}

var contractOperations = map[string]struct{}{
	"memory_kv.get": {}, "memory_kv.set": {}, "memory_kv.update": {}, "memory_kv.forget": {},
	"memory_rec_sum.search": {}, "memory_rec_sum.summarize": {}, "memory_vector.search": {},
}

var contractRequiredArguments = map[string][]string{
	"memory_kv.get":            {"key"},
	"memory_kv.set":            {"key", "value"},
	"memory_kv.update":         {"key", "value"},
	"memory_kv.forget":         {"key"},
	"memory_rec_sum.search":    {"query"},
	"memory_rec_sum.summarize": {"session_id"},
	"memory_vector.search":     {"query"},
}

// ValidateContractOperation constrains the replay input before a provider is
// called. Expected, when supplied, is a JSON object describing the expected
// result shape; primitive values cannot silently stand in for a provider
// response contract.
func ValidateContractOperation(operation ContractOperation) error {
	required, known := contractRequiredArguments[operation.Name]
	if !known {
		return fmt.Errorf("unknown operation %q", operation.Name)
	}
	if err := operation.Scope.Validate(); err != nil {
		return fmt.Errorf("operation scope: %w", err)
	}
	if operation.Arguments == nil {
		return fmt.Errorf("malformed arguments")
	}
	for _, key := range required {
		value, present := operation.Arguments[key]
		if !present || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("malformed arguments: %s is required", key)
		}
	}
	if operation.Expected != nil {
		if _, ok := operation.Expected.(map[string]any); !ok {
			return fmt.Errorf("malformed expected result shape")
		}
	}
	return nil
}

func LoadContractFixture(data []byte) (ContractFixture, error) {
	var fixture ContractFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return fixture, fmt.Errorf("decode contract fixture: %w", err)
	}
	if fixture.SchemaVersion == "" {
		fixture.SchemaVersion = SchemaVersion
	}
	if fixture.SchemaVersion != SchemaVersion || strings.TrimSpace(fixture.Subset) == "" {
		return fixture, fmt.Errorf("invalid contract fixture schema or subset")
	}
	return fixture, nil
}

// ReplayContract validates call shape and safety without reaching a remote
// model, judge, or provider. It reports contract metrics separately from
// retrieval-ranking metrics.
func ReplayContract(fixture ContractFixture, runScope memory.Scope) ContractReport {
	return ReplayContractScoped(fixture, ContractReplayScope{Scope: runScope})
}

func ReplayContractScoped(fixture ContractFixture, expected ContractReplayScope) ContractReport {
	report := ContractReport{SchemaVersion: SchemaVersion, Family: "provider_contract", Subset: fixture.Subset}
	for _, operation := range fixture.Operations {
		scopeSafe := operation.Scope.Normalized() == expected.Scope.Normalized()
		if expected.SessionID != "" && operation.SessionID != expected.SessionID {
			scopeSafe = false
		}
		outcome := ContractOutcome{Operation: operation.Name, ScopeSafe: scopeSafe, LifecycleSafe: true}
		_, known := contractOperations[operation.Name]
		switch {
		case !known:
			outcome.Reason = "unknown operation"
		case ValidateContractOperation(operation) != nil:
			outcome.Reason = "malformed arguments or result shape"
		case !outcome.ScopeSafe:
			outcome.Reason = "scope violation"
			report.ScopeSafetyFailures++
		case hiddenState(operation.TargetState):
			// A request for a hidden record must be refused; this is a safe
			// outcome rather than a lifecycle leak.
			outcome.Refused = true
		default:
			outcome.Valid = operation.Relevant
			if !operation.Relevant {
				outcome.Refused = true
			}
		}
		report.Outcomes = append(report.Outcomes, outcome)
	}
	if count := len(report.Outcomes); count > 0 {
		valid, malformed, refused := 0, 0, 0
		for _, outcome := range report.Outcomes {
			if outcome.Valid {
				valid++
			}
			if strings.HasPrefix(outcome.Reason, "malformed") {
				malformed++
			}
			if outcome.Refused {
				refused++
			}
		}
		denominator := float64(count)
		report.OperationAccuracy = float64(valid) / denominator
		report.MalformedCallRate = float64(malformed) / denominator
		report.RefusalCorrectness = float64(refused) / denominator
	}
	return report
}

func hiddenState(state memory.MemoryState) bool {
	return state == memory.MemoryStateForgotten || state == memory.MemoryStateSuppressed || state == memory.MemoryStateDeleted
}
