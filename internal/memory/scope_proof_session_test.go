package memory

import (
	"testing"
	"time"
)

func TestCreateScopeProofRunInputValidateRequiresScopeActorAndReason(t *testing.T) {
	input := CreateScopeProofRunInput{
		Scope:       Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Checks:      []ScopeProofCheck{ScopeProofCheckIngestion, ScopeProofCheckContext},
		FixtureMode: ScopeProofFixtureModeSmoke,
		Actor:       "operator-a",
		Reason:      "prove onboarding",
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := input
	invalid.Actor = ""
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want actor validation error")
	}

	invalid = input
	invalid.Checks = []ScopeProofCheck{"agent_runtime"}
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid check validation error")
	}
}

func TestClaimScopeProofStepsInputValidateRequiresLeaseFields(t *testing.T) {
	input := ClaimScopeProofStepsInput{
		Scope:         Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		WorkerID:      "worker-a",
		Now:           time.Date(2026, 7, 11, 16, 0, 0, 0, time.UTC),
		LeaseDuration: time.Minute,
		Limit:         2,
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := input
	invalid.WorkerID = ""
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want worker validation error")
	}
}

func TestMemorySessionInputsValidateServiceSideContract(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	create := CreateMemorySessionInput{
		Scope:  scope,
		Actor:  "agent-a",
		Reason: "serve user turn",
	}
	if err := create.Validate(); err != nil {
		t.Fatalf("CreateMemorySessionInput.Validate() error = %v", err)
	}

	turn := CreateMemorySessionTurnInput{
		Scope:     scope,
		SessionID: "session_1",
		TurnID:    "turn_1",
		Query:     "what should the agent remember?",
	}
	if err := turn.Validate(); err != nil {
		t.Fatalf("CreateMemorySessionTurnInput.Validate() error = %v", err)
	}

	outcome := RecordMemorySessionTurnOutcomeInput{
		Scope:           scope,
		SessionID:       "session_1",
		TurnID:          "turn_1",
		OutcomeEventIDs: []string{"event_1"},
		ExpectedRecall:  []string{"event_1"},
	}
	if err := outcome.Validate(); err != nil {
		t.Fatalf("RecordMemorySessionTurnOutcomeInput.Validate() error = %v", err)
	}

	invalid := turn
	invalid.SessionID = ""
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want session id validation error")
	}
}

func TestMemoryLoopEvidenceLinkValidateRequiresScopedOwnerAndEvidence(t *testing.T) {
	input := CreateMemoryLoopEvidenceLinkInput{
		Scope:        Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		OwnerKind:    MemoryLoopEvidenceOwnerProof,
		OwnerID:      "proof_1",
		EvidenceKind: MemoryLoopEvidenceKindEvent,
		EvidenceID:   "evt_1",
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := input
	invalid.EvidenceID = ""
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want evidence id validation error")
	}
}
