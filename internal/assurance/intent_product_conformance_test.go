package assurance

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluateIntentConformanceProducesConsumableRedactedReport(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report, err := EvaluateIntentConformance(IntentConformanceInput{
		RunID:           "intent-conformance-run-1",
		Scope:           memory.Scope{Tenant: "tenant-secret", Project: "project-secret", Namespace: "namespace-secret"},
		SchemaVersion:   "schema-1",
		ProviderVersion: "provider-1",
		StartedAt:       now,
		FinishedAt:      now.Add(2 * time.Second),
		Phases: []IntentConformancePhaseResult{
			{Phase: IntentConformancePhaseSubmission, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryAccepted, DurationBucket: "lt_1s"},
			{Phase: IntentConformancePhaseReplay, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryReplay, DurationBucket: "lt_1s"},
			{Phase: IntentConformancePhaseCleanup, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryCleanup, DurationBucket: "lt_1s"},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateIntentConformance() error = %v", err)
	}
	if !report.Consumable || report.Result != IntentConformanceResultPass || report.Fingerprint == "" {
		t.Fatalf("report = %+v, want consumable passing report with fingerprint", report)
	}
	redacted := report.Redacted()
	if err := redacted.Validate(); err != nil {
		t.Fatalf("redacted.Validate() error = %v", err)
	}
	b, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(b)
	for _, forbidden := range []string{"tenant-secret", "project-secret", "namespace-secret", "intent-conformance-run-1"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("redacted report leaked %q: %s", forbidden, payload)
		}
	}
	if redacted.ScopeHash == "" || redacted.RunHash == "" {
		t.Fatalf("redacted report = %+v, want scope and run hashes", redacted)
	}
}

func TestEvaluateIntentConformanceRejectsSensitiveOrUnboundedPhaseData(t *testing.T) {
	base := IntentConformanceInput{
		RunID:           "intent-conformance-run-2",
		Scope:           memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		SchemaVersion:   "schema-1",
		ProviderVersion: "provider-1",
		StartedAt:       time.Now().UTC(),
		Phases:          []IntentConformancePhaseResult{{Phase: IntentConformancePhaseSubmission, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryAccepted}},
	}
	cases := []struct {
		name   string
		mutate func(*IntentConformanceInput)
	}{
		{name: "unsupported phase", mutate: func(input *IntentConformanceInput) { input.Phases[0].Phase = "payload" }},
		{name: "unsupported result", mutate: func(input *IntentConformanceInput) { input.Phases[0].Result = "secret-result" }},
		{name: "unbounded category", mutate: func(input *IntentConformanceInput) {
			input.Phases[0].Category = IntentConformanceCategory(strings.Repeat("x", 257))
		}},
		{name: "too many phases", mutate: func(input *IntentConformanceInput) { input.Phases = make([]IntentConformancePhaseResult, 33) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.Phases = append([]IntentConformancePhaseResult(nil), base.Phases...)
			tc.mutate(&input)
			if _, err := EvaluateIntentConformance(input); err == nil {
				t.Fatal("EvaluateIntentConformance() error = nil, want validation failure")
			}
		})
	}
}

func TestReplayIntentConformanceRequiresDeterministicCompatibleEvidence(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	input := IntentConformanceInput{
		RunID: "intent-conformance-run-3", Scope: memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		SchemaVersion: "schema-1", ProviderVersion: "provider-1", StartedAt: now,
		Phases: []IntentConformancePhaseResult{{Phase: IntentConformancePhaseSubmission, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryAccepted}},
	}
	previous, err := EvaluateIntentConformance(input)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := ReplayIntentConformance(previous, input)
	if err != nil {
		t.Fatalf("ReplayIntentConformance() error = %v", err)
	}
	if replayed.Fingerprint != previous.Fingerprint {
		t.Fatalf("replayed fingerprint = %q, previous = %q", replayed.Fingerprint, previous.Fingerprint)
	}
	input.Phases[0].Category = IntentConformanceCategoryConflict
	if _, err := ReplayIntentConformance(previous, input); err == nil {
		t.Fatal("ReplayIntentConformance() error = nil, want nondeterministic failure")
	}
}

func TestEvaluateIntentConformanceUsesFailurePrecedenceAndRejectsUnsafeVersions(t *testing.T) {
	base := IntentConformanceInput{
		RunID:         "intent-conformance-precedence",
		Scope:         memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		SchemaVersion: "schema-1", ProviderVersion: "provider-1", StartedAt: time.Now().UTC(),
		Phases: []IntentConformancePhaseResult{
			{Phase: IntentConformancePhaseSubmission, Result: IntentConformanceResultPass, Category: IntentConformanceCategoryAccepted},
			{Phase: IntentConformancePhaseInspection, Result: IntentConformanceResultSkip, Category: IntentConformanceCategoryValidation},
			{Phase: IntentConformancePhaseReplay, Result: IntentConformanceResultDegraded, Category: IntentConformanceCategoryReplay},
			{Phase: IntentConformancePhaseRollback, Result: IntentConformanceResultFail, Category: IntentConformanceCategoryRollback},
		},
	}
	report, err := EvaluateIntentConformance(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.Result != IntentConformanceResultFail || report.Consumable {
		t.Fatalf("report = %+v, want failed and non-consumable", report)
	}
	for _, version := range []string{"postgres://secret", "dsn@host", "provider secret"} {
		input := base
		input.ProviderVersion = version
		if _, err := EvaluateIntentConformance(input); err == nil {
			t.Fatalf("version %q was accepted, want validation failure", version)
		}
	}
}
