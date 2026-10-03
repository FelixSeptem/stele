package memory

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// OperationPrecedenceVersion identifies the wire-compatible ordering of
// governed-operation gates. It is persisted with durable work so a restarted
// worker cannot silently reinterpret an old decision.
const OperationPrecedenceVersion = "governed-operation-precedence-v1"

type OperationPrecedenceStage string

const (
	OperationStageScope     OperationPrecedenceStage = "scope"
	OperationStageLifecycle OperationPrecedenceStage = "lifecycle"
	OperationStageGrant     OperationPrecedenceStage = "grant"
	OperationStageApproval  OperationPrecedenceStage = "approval"
	OperationStageReplay    OperationPrecedenceStage = "replay"
	OperationStageHandoff   OperationPrecedenceStage = "handoff"
	OperationStageMutation  OperationPrecedenceStage = "mutation"
)

func (s OperationPrecedenceStage) Valid() bool {
	switch s {
	case OperationStageScope, OperationStageLifecycle, OperationStageGrant, OperationStageApproval, OperationStageReplay, OperationStageHandoff, OperationStageMutation:
		return true
	default:
		return false
	}
}

type OperationPrecedenceOutcome string

const (
	OperationOutcomeAccepted        OperationPrecedenceOutcome = "accepted"
	OperationOutcomeDenied          OperationPrecedenceOutcome = "denied"
	OperationOutcomeReplayed        OperationPrecedenceOutcome = "replayed"
	OperationOutcomeConflict        OperationPrecedenceOutcome = "conflict"
	OperationOutcomeIncompatible    OperationPrecedenceOutcome = "incompatible"
	OperationOutcomePolicyDisabled  OperationPrecedenceOutcome = "policy_disabled"
	OperationOutcomeLifecycleDenied OperationPrecedenceOutcome = "lifecycle_denied"
	OperationOutcomeGrantDenied     OperationPrecedenceOutcome = "grant_denied"
	OperationOutcomeScopeDenied     OperationPrecedenceOutcome = "scope_denied"
	OperationOutcomeHandoffDenied   OperationPrecedenceOutcome = "handoff_denied"
	OperationOutcomeMutationDenied  OperationPrecedenceOutcome = "mutation_denied"
	OperationOutcomeRetryable       OperationPrecedenceOutcome = "retryable"
)

func (o OperationPrecedenceOutcome) Valid() bool {
	switch o {
	case OperationOutcomeAccepted, OperationOutcomeDenied, OperationOutcomeReplayed, OperationOutcomeConflict,
		OperationOutcomeIncompatible, OperationOutcomePolicyDisabled, OperationOutcomeLifecycleDenied,
		OperationOutcomeGrantDenied, OperationOutcomeScopeDenied, OperationOutcomeHandoffDenied,
		OperationOutcomeMutationDenied, OperationOutcomeRetryable:
		return true
	default:
		return false
	}
}

// OperationPrecedenceInput is intentionally metadata-only. Payloads and raw
// errors never enter the evaluator or its returned decision.
type OperationPrecedenceInput struct {
	Version               string
	Operation             string
	Scope                 Scope
	GrantedScope          Scope
	PrincipalID           string
	GrantID               string
	LifecycleVisible      bool
	LifecycleChecked      bool
	PrincipalGranted      bool
	ApprovalRequired      bool
	ApprovalEnabled       bool
	PolicyVersion         string
	ExpectedPolicyVersion string
	RequireIdempotency    bool
	IdempotencyKey        string
	RequestFingerprint    string
	ExistingFingerprint   string
	ExistingOutcome       OperationPrecedenceOutcome
	HandoffAllowed        bool
	MutationAllowed       bool
}

type OperationPrecedenceDecision struct {
	Version            string                     `json:"precedence_version"`
	Operation          string                     `json:"operation"`
	ScopeProof         string                     `json:"scope_proof"`
	PrincipalID        string                     `json:"principal_id,omitempty"`
	GrantID            string                     `json:"grant_id,omitempty"`
	PolicyVersion      string                     `json:"policy_version,omitempty"`
	RequestID          string                     `json:"request_id,omitempty"`
	OperationID        string                     `json:"operation_id,omitempty"`
	IdempotencyKey     string                     `json:"idempotency_key,omitempty"`
	RequestFingerprint string                     `json:"request_fingerprint,omitempty"`
	Stage              OperationPrecedenceStage   `json:"stage"`
	Outcome            OperationPrecedenceOutcome `json:"outcome"`
	Replayed           bool                       `json:"replayed,omitempty"`
}

func (d OperationPrecedenceDecision) Validate() error {
	if d.Version != OperationPrecedenceVersion || strings.TrimSpace(d.Operation) == "" || strings.TrimSpace(d.ScopeProof) == "" {
		return fmt.Errorf("precedence decision metadata is invalid")
	}
	if !d.Stage.Valid() || !d.Outcome.Valid() {
		return fmt.Errorf("precedence decision stage or outcome is invalid")
	}
	for name, value := range map[string]string{
		"operation": d.Operation, "scope proof": d.ScopeProof, "principal id": d.PrincipalID,
		"grant id": d.GrantID, "policy version": d.PolicyVersion, "request id": d.RequestID,
		"operation id": d.OperationID, "idempotency key": d.IdempotencyKey, "request fingerprint": d.RequestFingerprint,
	} {
		if len(value) > 256 {
			return fmt.Errorf("%s is too large", name)
		}
	}
	if d.Replayed && d.Outcome != OperationOutcomeReplayed {
		return fmt.Errorf("replayed decision must use replayed outcome")
	}
	return nil
}

// EvaluateOperationPrecedence applies the contract in strict order. It does
// not query storage and it never returns payloads, scope values, or raw errors.
func EvaluateOperationPrecedence(input OperationPrecedenceInput) (OperationPrecedenceDecision, error) {
	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = OperationPrecedenceVersion
	}
	d := OperationPrecedenceDecision{Version: version, Operation: strings.TrimSpace(input.Operation), PolicyVersion: strings.TrimSpace(input.PolicyVersion), PrincipalID: strings.TrimSpace(input.PrincipalID), GrantID: strings.TrimSpace(input.GrantID), IdempotencyKey: strings.TrimSpace(input.IdempotencyKey), RequestFingerprint: strings.TrimSpace(input.RequestFingerprint)}
	if version != OperationPrecedenceVersion {
		return precedenceDeny(d, OperationStageScope, OperationOutcomeIncompatible), nil
	}
	if err := input.Scope.Validate(); err != nil {
		return precedenceDeny(d, OperationStageScope, OperationOutcomeScopeDenied), nil
	}
	if err := input.GrantedScope.Validate(); err != nil || input.Scope.Normalized() != input.GrantedScope.Normalized() {
		return precedenceDeny(d, OperationStageScope, OperationOutcomeScopeDenied), nil
	}
	d.ScopeProof = scopeProof(input.Scope)
	if strings.TrimSpace(input.Operation) == "" {
		return OperationPrecedenceDecision{}, fmt.Errorf("operation is required")
	}
	if !input.LifecycleChecked || !input.LifecycleVisible {
		return precedenceDeny(d, OperationStageLifecycle, OperationOutcomeLifecycleDenied), nil
	}
	if !input.PrincipalGranted || strings.TrimSpace(input.PrincipalID) == "" {
		return precedenceDeny(d, OperationStageGrant, OperationOutcomeGrantDenied), nil
	}
	if input.ApprovalRequired {
		if !input.ApprovalEnabled {
			return precedenceDeny(d, OperationStageApproval, OperationOutcomePolicyDisabled), nil
		}
		if strings.TrimSpace(input.ExpectedPolicyVersion) != "" && strings.TrimSpace(input.PolicyVersion) != strings.TrimSpace(input.ExpectedPolicyVersion) {
			return precedenceDeny(d, OperationStageApproval, OperationOutcomeIncompatible), nil
		}
		if strings.TrimSpace(input.PolicyVersion) == "" {
			return precedenceDeny(d, OperationStageApproval, OperationOutcomeIncompatible), nil
		}
	}
	if input.RequireIdempotency && strings.TrimSpace(input.IdempotencyKey) == "" {
		return precedenceDeny(d, OperationStageReplay, OperationOutcomeDenied), nil
	}
	if strings.TrimSpace(input.ExistingFingerprint) != "" {
		if strings.TrimSpace(input.ExistingFingerprint) != strings.TrimSpace(input.RequestFingerprint) {
			return precedenceDeny(d, OperationStageReplay, OperationOutcomeConflict), nil
		}
		d.Replayed = true
		d.Outcome = OperationOutcomeReplayed
		d.Stage = OperationStageReplay
		return d, nil
	}
	if !input.HandoffAllowed {
		return precedenceDeny(d, OperationStageHandoff, OperationOutcomeHandoffDenied), nil
	}
	if !input.MutationAllowed {
		return precedenceDeny(d, OperationStageMutation, OperationOutcomeMutationDenied), nil
	}
	d.Stage = OperationStageMutation
	d.Outcome = OperationOutcomeAccepted
	return d, nil
}

func precedenceDeny(d OperationPrecedenceDecision, stage OperationPrecedenceStage, outcome OperationPrecedenceOutcome) OperationPrecedenceDecision {
	d.Stage = stage
	d.Outcome = outcome
	if d.Version == "" {
		d.Version = OperationPrecedenceVersion
	}
	if d.ScopeProof == "" {
		d.ScopeProof = "redacted"
	}
	return d
}

func scopeProof(scope Scope) string {
	// A proof is an opaque marker. It is deliberately not the scope itself.
	return fmt.Sprintf("scope:%x", stableScopeDigest(scope.Normalized()))
}

func stableScopeDigest(scope Scope) [32]byte {
	// Keep the proof deterministic without exposing the source values in the
	// decision. The digest implementation is isolated for easy versioning.
	return sha256.Sum256([]byte(scope.Tenant + "\x00" + scope.Project + "\x00" + scope.Namespace))
}
