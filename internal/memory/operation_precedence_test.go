package memory

import "testing"

func precedenceInput() OperationPrecedenceInput {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	return OperationPrecedenceInput{
		Operation: "intent.submit", Scope: scope, GrantedScope: scope, PrincipalID: "principal-a", GrantID: "grant-a",
		LifecycleChecked: true, LifecycleVisible: true, PrincipalGranted: true,
		ApprovalRequired: true, ApprovalEnabled: true, PolicyVersion: "policy-v1", ExpectedPolicyVersion: "policy-v1",
		RequireIdempotency: true, IdempotencyKey: "idem-a", RequestFingerprint: "fingerprint-a",
		HandoffAllowed: true, MutationAllowed: true,
	}
}

func TestEvaluateOperationPrecedenceStopsAtFirstFailedGate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*OperationPrecedenceInput)
		stage  OperationPrecedenceStage
		want   OperationPrecedenceOutcome
	}{
		{"scope", func(i *OperationPrecedenceInput) { i.GrantedScope.Namespace = "foreign" }, OperationStageScope, OperationOutcomeScopeDenied},
		{"lifecycle", func(i *OperationPrecedenceInput) { i.LifecycleVisible = false }, OperationStageLifecycle, OperationOutcomeLifecycleDenied},
		{"grant", func(i *OperationPrecedenceInput) { i.PrincipalGranted = false }, OperationStageGrant, OperationOutcomeGrantDenied},
		{"approval", func(i *OperationPrecedenceInput) { i.ApprovalEnabled = false }, OperationStageApproval, OperationOutcomePolicyDisabled},
		{"replay conflict", func(i *OperationPrecedenceInput) { i.ExistingFingerprint = "other" }, OperationStageReplay, OperationOutcomeConflict},
		{"handoff", func(i *OperationPrecedenceInput) { i.HandoffAllowed = false }, OperationStageHandoff, OperationOutcomeHandoffDenied},
		{"mutation", func(i *OperationPrecedenceInput) { i.MutationAllowed = false }, OperationStageMutation, OperationOutcomeMutationDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := precedenceInput()
			tt.mutate(&input)
			got, err := EvaluateOperationPrecedence(input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Stage != tt.stage || got.Outcome != tt.want {
				t.Fatalf("decision = %+v, want %s/%s", got, tt.stage, tt.want)
			}
		})
	}
}

func TestEvaluateOperationPrecedenceReplayAndVersion(t *testing.T) {
	input := precedenceInput()
	input.ExistingFingerprint = input.RequestFingerprint
	got, err := EvaluateOperationPrecedence(input)
	if err != nil || !got.Replayed || got.Outcome != OperationOutcomeReplayed || got.Stage != OperationStageReplay {
		t.Fatalf("replay decision = %+v, %v", got, err)
	}
	input = precedenceInput()
	input.Version = "precedence-v0"
	got, err = EvaluateOperationPrecedence(input)
	if err != nil || got.Stage != OperationStageScope || got.Outcome != OperationOutcomeIncompatible {
		t.Fatalf("incompatible decision = %+v, %v", got, err)
	}
}

func TestOperationPrecedenceDecisionValidationRejectsUnboundedMetadata(t *testing.T) {
	d := OperationPrecedenceDecision{Version: OperationPrecedenceVersion, Operation: "op", ScopeProof: "proof", Stage: OperationStageScope, Outcome: OperationOutcomeDenied, RequestID: string(make([]byte, 257))}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want bounded metadata failure")
	}
}
