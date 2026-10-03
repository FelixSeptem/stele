package assurance

import (
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// OperationPrecedenceConformanceCase describes one bounded, non-mutating
// contract fixture. It contains no payload, target identifier, or scope value
// in the returned report.
type OperationPrecedenceConformanceCase struct {
	Name          string
	Operation     string
	ExpectedStage memory.OperationPrecedenceStage
	Expected      memory.OperationPrecedenceOutcome
	Mutate        func(*memory.OperationPrecedenceInput)
}

type OperationPrecedenceConformanceResult struct {
	Name    string
	Stage   memory.OperationPrecedenceStage
	Outcome memory.OperationPrecedenceOutcome
	Passed  bool
}

type OperationPrecedenceConformanceReport struct {
	Version string
	Passed  bool
	Results []OperationPrecedenceConformanceResult
}

func (r OperationPrecedenceConformanceReport) Validate() error {
	if r.Version != memory.OperationPrecedenceVersion || len(r.Results) == 0 {
		return fmt.Errorf("precedence conformance report is invalid")
	}
	for _, result := range r.Results {
		if strings.TrimSpace(result.Name) == "" || !result.Stage.Valid() || !result.Outcome.Valid() {
			return fmt.Errorf("precedence conformance result is invalid")
		}
	}
	return nil
}

// RunOperationPrecedenceConformance exercises the shared contract against one
// exact scope. It is diagnostic only: it never reads or mutates storage,
// invokes a provider, queues work, or changes retrieval/context behavior.
func RunOperationPrecedenceConformance(scope memory.Scope) (OperationPrecedenceConformanceReport, error) {
	if err := scope.Validate(); err != nil {
		return OperationPrecedenceConformanceReport{}, err
	}
	base := memory.OperationPrecedenceInput{
		Operation: "conformance", Scope: scope, GrantedScope: scope, PrincipalID: "conformance-principal", GrantID: "conformance-grant",
		LifecycleChecked: true, LifecycleVisible: true, PrincipalGranted: true,
		ApprovalRequired: true, ApprovalEnabled: true, PolicyVersion: "conformance-policy-v1", ExpectedPolicyVersion: "conformance-policy-v1",
		RequireIdempotency: true, IdempotencyKey: "conformance-key", RequestFingerprint: "conformance-fingerprint",
		HandoffAllowed: true, MutationAllowed: true,
	}
	cases := []OperationPrecedenceConformanceCase{
		{Name: "scope", Operation: "intent", ExpectedStage: memory.OperationStageScope, Expected: memory.OperationOutcomeScopeDenied, Mutate: func(i *memory.OperationPrecedenceInput) { i.GrantedScope.Namespace = "foreign" }},
		{Name: "lifecycle", Operation: "insight", ExpectedStage: memory.OperationStageLifecycle, Expected: memory.OperationOutcomeLifecycleDenied, Mutate: func(i *memory.OperationPrecedenceInput) { i.LifecycleVisible = false }},
		{Name: "grant", Operation: "provider", ExpectedStage: memory.OperationStageGrant, Expected: memory.OperationOutcomeGrantDenied, Mutate: func(i *memory.OperationPrecedenceInput) { i.PrincipalGranted = false }},
		{Name: "approval", Operation: "lifecycle", ExpectedStage: memory.OperationStageApproval, Expected: memory.OperationOutcomePolicyDisabled, Mutate: func(i *memory.OperationPrecedenceInput) { i.ApprovalEnabled = false }},
		{Name: "replay", Operation: "intent", ExpectedStage: memory.OperationStageReplay, Expected: memory.OperationOutcomeReplayed, Mutate: func(i *memory.OperationPrecedenceInput) { i.ExistingFingerprint = i.RequestFingerprint }},
		{Name: "conflict", Operation: "provider", ExpectedStage: memory.OperationStageReplay, Expected: memory.OperationOutcomeConflict, Mutate: func(i *memory.OperationPrecedenceInput) { i.ExistingFingerprint = "different" }},
		{Name: "handoff", Operation: "insight", ExpectedStage: memory.OperationStageHandoff, Expected: memory.OperationOutcomeHandoffDenied, Mutate: func(i *memory.OperationPrecedenceInput) { i.HandoffAllowed = false }},
		{Name: "mutation", Operation: "manual", ExpectedStage: memory.OperationStageMutation, Expected: memory.OperationOutcomeMutationDenied, Mutate: func(i *memory.OperationPrecedenceInput) { i.MutationAllowed = false }},
	}
	report := OperationPrecedenceConformanceReport{Version: memory.OperationPrecedenceVersion, Passed: true, Results: make([]OperationPrecedenceConformanceResult, 0, len(cases))}
	for _, fixture := range cases {
		input := base
		input.Operation = fixture.Operation
		fixture.Mutate(&input)
		decision, err := memory.EvaluateOperationPrecedence(input)
		if err != nil {
			return OperationPrecedenceConformanceReport{}, err
		}
		passed := decision.Stage == fixture.ExpectedStage && decision.Outcome == fixture.Expected
		if !passed {
			report.Passed = false
		}
		report.Results = append(report.Results, OperationPrecedenceConformanceResult{Name: fixture.Name, Stage: decision.Stage, Outcome: decision.Outcome, Passed: passed})
	}
	return report, report.Validate()
}
