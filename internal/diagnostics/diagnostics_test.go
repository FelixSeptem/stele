package diagnostics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdmissionReportDecisionDeniesWhenBlockersExist(t *testing.T) {
	report := NewAdmissionReport("embedding_cutover", time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		Finding{Severity: SeverityWarning, Code: "large_rollout", Message: "many items"},
		Finding{Severity: SeverityBlocker, Code: "target_unresolved", Message: "provider missing"},
	)

	if report.Decision != AdmissionDecisionDeny {
		t.Fatalf("Decision = %q, want %q", report.Decision, AdmissionDecisionDeny)
	}
	if len(report.Blockers) != 1 || report.Blockers[0].Code != "target_unresolved" {
		t.Fatalf("Blockers = %+v, want target_unresolved", report.Blockers)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != "large_rollout" {
		t.Fatalf("Warnings = %+v, want large_rollout", report.Warnings)
	}
}

func TestAdmissionReportDecisionAllowsWarningOnly(t *testing.T) {
	report := NewAdmissionReport("embedding_cutover", time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		Finding{Severity: SeverityWarning, Code: "many_waves", Message: "rollout is large"},
	)

	if report.Decision != AdmissionDecisionAllow {
		t.Fatalf("Decision = %q, want %q", report.Decision, AdmissionDecisionAllow)
	}
}

func TestMetricLabelsRejectHighCardinalityValues(t *testing.T) {
	labels, err := MetricLabels(map[string]string{
		"component": "embedding_cutover",
		"operation": "preflight",
		"code":      "target_unresolved",
	})
	if err != nil {
		t.Fatalf("MetricLabels() error = %v, want nil", err)
	}
	if labels["operation"] != "preflight" {
		t.Fatalf("operation label = %q, want preflight", labels["operation"])
	}

	_, err = MetricLabels(map[string]string{
		"plan_id": "plan_123",
	})
	if err == nil || err.Error() != "metric label \"plan_id\" is not allowed" {
		t.Fatalf("MetricLabels() error = %v, want plan_id rejection", err)
	}

	_, err = MetricLabels(map[string]string{
		"component": "contains spaces",
	})
	if err == nil || err.Error() != "metric label \"component\" has invalid value \"contains spaces\"" {
		t.Fatalf("MetricLabels() error = %v, want value rejection", err)
	}
}

func TestReadinessEvaluatorComposesChecks(t *testing.T) {
	evaluator := ReadinessEvaluator{
		Mode:       RuntimeModeScheduler,
		ObservedAt: time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
		Checks: []ReadinessCheck{
			{Name: "postgres", Required: true, Check: func(ctx context.Context) error { return nil }},
			{Name: "embedding_provider", Required: true, Check: func(ctx context.Context) error { return errors.New("provider unavailable") }},
			{Name: "optional_probe", Required: false, Check: func(ctx context.Context) error { return errors.New("ignored") }},
		},
	}

	result := evaluator.Evaluate(context.Background())
	if result.Status != ReadinessStatusNotReady {
		t.Fatalf("Status = %q, want %q", result.Status, ReadinessStatusNotReady)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("Findings = %+v, want required failure and optional warning", result.Findings)
	}
	if result.Findings[0].Severity != SeverityBlocker || result.Findings[0].Code != "check_failed" {
		t.Fatalf("first finding = %+v, want blocker check_failed", result.Findings[0])
	}
	if result.Findings[1].Severity != SeverityWarning || result.Findings[1].Code != "check_failed" {
		t.Fatalf("second finding = %+v, want warning check_failed", result.Findings[1])
	}
}
