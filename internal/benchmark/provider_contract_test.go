package benchmark

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestReplayContractRejectsMalformedForeignAndHiddenOperations(t *testing.T) {
	runScope := memory.Scope{Tenant: "benchmark", Project: "contracts", Namespace: "run-1"}
	fixture := ContractFixture{SchemaVersion: SchemaVersion, Subset: "memory_kv", Operations: []ContractOperation{
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "name"}, Scope: runScope, Relevant: true},
		{Name: "memory_kv.get", Arguments: nil, Scope: runScope, Relevant: true},
		{Name: "memory_vector.search", Arguments: map[string]any{"query": "hidden"}, Scope: memory.Scope{Tenant: "other", Project: "contracts", Namespace: "run-1"}, Relevant: true},
		{Name: "memory_kv.forget", Arguments: map[string]any{"key": "forgotten"}, Scope: runScope, Relevant: true, TargetState: memory.MemoryStateForgotten},
	}}
	report := ReplayContract(fixture, runScope)
	if report.OperationAccuracy != 0.25 || report.MalformedCallRate != 0.25 || report.ScopeSafetyFailures != 1 || report.LifecycleSafetyFailures != 0 {
		t.Fatalf("unexpected contract report: %#v", report)
	}
	if !report.Outcomes[3].Refused || !report.Outcomes[3].LifecycleSafe {
		t.Fatalf("expected hidden target to be refused safely: %#v", report.Outcomes[3])
	}
}

func TestValidateContractOperationRequiresKnownArgumentsAndObjectResultShape(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "contracts", Namespace: "run-1"}
	valid := ContractOperation{Name: "memory_vector.search", Scope: scope, Arguments: map[string]any{"query": "find a fact"}, Expected: map[string]any{"results": []any{}}}
	if err := ValidateContractOperation(valid); err != nil {
		t.Fatalf("valid contract operation rejected: %v", err)
	}
	missingQuery := valid
	missingQuery.Arguments = map[string]any{}
	if err := ValidateContractOperation(missingQuery); err == nil {
		t.Fatal("expected missing required argument to be rejected")
	}
	malformedResult := valid
	malformedResult.Expected = "not an object"
	if err := ValidateContractOperation(malformedResult); err == nil {
		t.Fatal("expected malformed result shape to be rejected")
	}
}

func TestReplayContractScopedRejectsForeignSessionAndRefusesHiddenMemory(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "contracts", Namespace: "run-1"}
	fixture := ContractFixture{SchemaVersion: SchemaVersion, Subset: "memory_rec_sum", Operations: []ContractOperation{
		{Name: "memory_rec_sum.search", Arguments: map[string]any{"query": "known"}, Scope: scope, SessionID: "foreign-session", Relevant: true},
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "forgotten"}, Scope: scope, SessionID: "allowed-session", Relevant: true, TargetState: memory.MemoryStateForgotten},
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "irrelevant"}, Scope: scope, SessionID: "allowed-session", Relevant: false},
	}}
	report := ReplayContractScoped(fixture, ContractReplayScope{Scope: scope, SessionID: "allowed-session"})
	if report.ScopeSafetyFailures != 1 || report.LifecycleSafetyFailures != 0 {
		t.Fatalf("expected foreign session detection with safe hidden-memory refusal: %#v", report)
	}
	if !report.Outcomes[1].Refused || !report.Outcomes[2].Refused {
		t.Fatalf("expected hidden and irrelevant calls to be refused: %#v", report.Outcomes)
	}
}

func TestReplayContractReportsZeroMustNotReturnViolationsForAllHiddenStates(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "contracts", Namespace: "run-1"}
	fixture := ContractFixture{SchemaVersion: SchemaVersion, Subset: "memory_kv", Operations: []ContractOperation{
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "suppressed"}, Scope: scope, Relevant: true, TargetState: memory.MemoryStateSuppressed},
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "forgotten"}, Scope: scope, Relevant: true, TargetState: memory.MemoryStateForgotten},
		{Name: "memory_kv.get", Arguments: map[string]any{"key": "deleted"}, Scope: scope, Relevant: true, TargetState: memory.MemoryStateDeleted},
	}}
	report := ReplayContract(fixture, scope)
	if report.MustNotReturnViolations != 0 {
		t.Fatalf("hidden memory must not leak: %#v", report)
	}
	for _, outcome := range report.Outcomes {
		if !outcome.Refused {
			t.Fatalf("hidden lifecycle target was not refused: %#v", outcome)
		}
	}
}
