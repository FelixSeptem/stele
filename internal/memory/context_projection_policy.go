package memory

import "fmt"

type ContextProjectionOmissionReason string

const (
	ContextProjectionOmissionLifecycle ContextProjectionOmissionReason = "lifecycle"
	ContextProjectionOmissionPolicy    ContextProjectionOmissionReason = "policy"
	ContextProjectionOmissionClass     ContextProjectionOmissionReason = "class"
	ContextProjectionOmissionBudget    ContextProjectionOmissionReason = "budget"
	ContextProjectionOmissionScope     ContextProjectionOmissionReason = "scope"
	ContextProjectionOmissionStale     ContextProjectionOmissionReason = "stale"
)

type ContextProjectionPolicy struct {
	Version              string
	MinProfileConfidence float64
	MaxItemTextBytes     int
}

func DefaultContextProjectionPolicy(version string) ContextProjectionPolicy {
	if version == "" {
		version = "policy-v1"
	}
	return ContextProjectionPolicy{Version: version, MinProfileConfidence: 0.7, MaxItemTextBytes: MaxContextProjectionItemTextBytes}
}

type ContextProjectionPolicyDecision struct {
	Include bool
	Reason  ContextProjectionOmissionReason
}

func ResolveContextProjectionPolicy(policy ContextProjectionPolicy, kind ContextProjectionKind, class MemoryClass, state MemoryState, confidence float64, textBytes int, rawEvidence bool) ContextProjectionPolicyDecision {
	if !kind.Valid() {
		return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionPolicy}
	}
	if state != MemoryStateActive {
		return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionLifecycle}
	}
	maxText := policy.MaxItemTextBytes
	if maxText <= 0 {
		maxText = MaxContextProjectionItemTextBytes
	}
	if textBytes <= 0 || textBytes > maxText {
		return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionPolicy}
	}
	if kind == ContextProjectionKindArchivalHistory {
		if rawEvidence || class != MemoryClassProfile {
			return ContextProjectionPolicyDecision{Include: true}
		}
		return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionClass}
	}
	switch kind {
	case ContextProjectionKindAlwaysVisible:
		if class != MemoryClassProfile {
			return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionClass}
		}
		minConfidence := policy.MinProfileConfidence
		if minConfidence <= 0 {
			minConfidence = 0.7
		}
		if confidence < minConfidence {
			return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionPolicy}
		}
	case ContextProjectionKindSession:
		if class != MemoryClassSummary {
			return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionClass}
		}
	case ContextProjectionKindRetrieval:
		if !validMemoryClass(class) {
			return ContextProjectionPolicyDecision{Reason: ContextProjectionOmissionClass}
		}
	}
	return ContextProjectionPolicyDecision{Include: true}
}

func (r ContextProjectionOmissionReason) String() string { return string(r) }

func (p ContextProjectionPolicy) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("projection policy version is required")
	}
	if p.MinProfileConfidence < 0 || p.MinProfileConfidence > 1 {
		return fmt.Errorf("projection profile confidence must be between zero and one")
	}
	if p.MaxItemTextBytes <= 0 || p.MaxItemTextBytes > MaxContextProjectionItemTextBytes {
		return fmt.Errorf("projection max item text bytes must be between one and %d", MaxContextProjectionItemTextBytes)
	}
	return nil
}
