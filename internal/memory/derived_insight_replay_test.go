package memory

import (
	"strings"
	"testing"
	"time"
)

func TestDerivedInsightReplayRequestValidateAcceptsBoundedDryRun(t *testing.T) {
	input := validDerivedInsightReplayRequest()
	input.Mode = DerivedInsightReplayModeDryRun

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDerivedInsightReplayRequestValidateRejectsMissingBounds(t *testing.T) {
	input := validDerivedInsightReplayRequest()
	input.EvidenceWindowEnd = time.Time{}

	err := input.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing bound error")
	}
	if !strings.Contains(err.Error(), "evidence window end is required") {
		t.Fatalf("error = %q, want evidence window end", err)
	}
}

func TestDerivedInsightReplayRequestValidateRejectsReservedActiveType(t *testing.T) {
	input := validDerivedInsightReplayRequest()
	input.InsightTypes = []DerivedInsightType{DerivedInsightTypeHypothesis}

	err := input.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unsupported type error")
	}
	if !strings.Contains(err.Error(), "not supported for replay") {
		t.Fatalf("error = %q, want unsupported type", err)
	}
}

func TestDerivedInsightReplayRunValidateRequiresAuditAndStatus(t *testing.T) {
	run := validDerivedInsightReplayRun()

	if err := run.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	run.Actor = ""
	if err := run.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want actor required")
	}
}

func TestDerivedInsightReplayReportValidateCountsDecisions(t *testing.T) {
	report := DerivedInsightReplayReport{
		RunID: "replay_123",
		Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Counters: DerivedInsightReplayCounters{
			EvidenceEvaluated: 2,
			Created:           1,
			Skipped:           1,
		},
		Decisions: []DerivedInsightReplayDecision{
			{
				InsightType:   DerivedInsightTypeFailurePattern,
				Fingerprint:   "failure_pattern:provider_unavailable",
				Decision:      DerivedInsightReplayDecisionCreate,
				Reason:        DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: 2,
			},
			{
				InsightType: DerivedInsightTypeLesson,
				Fingerprint: "lesson:provider_unavailable",
				Decision:    DerivedInsightReplayDecisionSkip,
				Reason:      DerivedInsightReplayReasonInsufficientEvidence,
			},
		},
		GeneratedAt: time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC),
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func validDerivedInsightReplayRequest() DerivedInsightReplayRequest {
	return DerivedInsightReplayRequest{
		Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Mode:  DerivedInsightReplayModeApply,
		InsightTypes: []DerivedInsightType{
			DerivedInsightTypeFailurePattern,
			DerivedInsightTypeLesson,
		},
		EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		EvidenceLimit:       100,
		Actor:               "operator-a",
		Reason:              "backfill after evaluator update",
		IdempotencyKey:      "replay:tenant-a:project-a:namespace-a:2026-07-01",
		RequestedAt:         time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}
}

func validDerivedInsightReplayRun() DerivedInsightReplayRun {
	input := validDerivedInsightReplayRequest()
	return DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     input.Scope,
		Mode:      input.Mode,
		Status:    DerivedInsightReplayStatusPending,
		Request:   input,
		Actor:     input.Actor,
		Reason:    input.Reason,
		CreatedAt: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}
}
