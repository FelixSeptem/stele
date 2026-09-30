package reasoning

import (
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func validRequest() InvocationRequest {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	return InvocationRequest{
		Metadata:       InvocationMetadata{RequestID: "req-1", OperationID: "op-1", IdempotencyKey: "idem-1", SchemaVersion: SchemaVersionV1, PolicyVersion: "policy-v1", ProviderVersion: "provider-v1"},
		Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Evidence:       []Evidence{{Kind: "memory", Reference: "mem-1", Version: "v1"}},
		Input:          []byte(`{"operation":"derive_failure_pattern","content":"bounded"}`),
		MaxOutputBytes: 1024,
		Deadline:       now.Add(time.Second),
		Now:            now,
	}
}

func TestInvocationRequestValidatesScopeBudgetAndDeadline(t *testing.T) {
	req := validRequest()
	if err := req.Validate(DefaultLimits(), req.Now); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	bad := req
	bad.Scope.Namespace = ""
	if err := bad.Validate(DefaultLimits(), req.Now); err == nil {
		t.Fatal("missing namespace accepted")
	}
	bad = req
	bad.MaxOutputBytes = DefaultLimits().MaxOutputBytes + 1
	if err := bad.Validate(DefaultLimits(), req.Now); err == nil {
		t.Fatal("over-budget request accepted")
	}
	bad = req
	bad.Deadline = req.Now
	if err := bad.Validate(DefaultLimits(), req.Now); err == nil {
		t.Fatal("expired deadline accepted")
	}
}

func TestCandidateRejectsReservedInsightAndDirectMutation(t *testing.T) {
	base := validCandidate()
	if err := base.Validate(DefaultLimits()); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}
	reserved := base
	reserved.InsightType = "hypothesis"
	if err := reserved.Validate(DefaultLimits()); err == nil {
		t.Fatal("reserved insight type accepted")
	}
	mutation := base
	mutation.DirectMutation = true
	if err := mutation.Validate(DefaultLimits()); err == nil {
		t.Fatal("direct mutation candidate accepted")
	}
}

func TestCapabilityDisabledByDefaultIsSafe(t *testing.T) {
	cap := Discover(CapabilityInput{})
	if err := cap.Validate(); err != nil {
		t.Fatalf("disabled capability invalid: %v", err)
	}
	if cap.Enabled || cap.Mode != ModeDisabled {
		t.Fatalf("default capability = %+v, want disabled", cap)
	}
	if len(cap.Operations) == 0 || cap.Limits.MaxInputBytes <= 0 {
		t.Fatalf("disabled capability lacks bounded contract: %+v", cap)
	}
}

func TestErrorAndEvidenceAreBounded(t *testing.T) {
	req := validRequest()
	req.Input = []byte(strings.Repeat("x", DefaultLimits().MaxInputBytes+1))
	if err := req.Validate(DefaultLimits(), req.Now); err == nil {
		t.Fatal("oversized input accepted")
	}
	if err := (ProviderError{Category: ErrorCategoryMalformedOutput, Code: "bad", Message: "bad output", Retryable: false}).Validate(); err != nil {
		t.Fatalf("valid provider error rejected: %v", err)
	}
}

func validCandidate() Candidate {
	return Candidate{
		ID: "cand-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Kind: "derived_insight", InsightType: "failure_pattern", Content: "bounded result",
		Evidence:   []Evidence{{Kind: "memory", Reference: "mem-1", Version: "v1"}},
		Provenance: map[string]string{"source": "reasoning"}, PolicyVersion: "policy-v1", ProviderVersion: "provider-v1", ReplayID: "replay-1",
	}
}
