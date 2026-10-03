package telemetry

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

func TestRetrievalEvaluationTelemetryIsLowCardinalityAndRedacted(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordRetrievalEvaluation(context.Background(), RetrievalEvaluationEvent{
		Status: "decision", FixtureVersion: "fixture-v1", RankingVersion: "baseline-v1",
		PolicyVersion: "release-policy-v1", FailureCategory: "none", Decision: "accepted",
	})
	output := observer.RenderPrometheus()
	if !strings.Contains(output, "stele_retrieval_evaluation_total") {
		t.Fatalf("metrics output = %s", output)
	}
	for _, forbidden := range []string{"tenant", "query", "memory_id", "credential", "postgres://"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("metrics output contains forbidden field %q: %s", forbidden, output)
		}
	}
}

func TestReleaseEvidenceOperationalTelemetryUsesBoundedCategories(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordReleaseEvidenceOperational(context.Background(), ReleaseEvidenceOperationalEvent{
		Operation: "attestation", Result: "mismatch", State: "completed",
		Cleanup: "complete", Freshness: "stale", Rollback: "failed", DurationBucket: "1s_10s",
	})
	output := observer.RenderPrometheus()
	if !strings.Contains(output, "stele_retrieval_release_evidence_operational_total") || !strings.Contains(output, "attestation") {
		t.Fatalf("metrics output = %s", output)
	}
	var logs bytes.Buffer
	LogReleaseEvidenceOperationalLifecycle(log.New(&logs, "", 0), ReleaseEvidenceOperationalEvent{Operation: "rollback", Result: "failed", State: "completed", Cleanup: "complete", Rollback: "failed"})
	if strings.Contains(strings.ToLower(logs.String()), "tenant") || !strings.Contains(logs.String(), "component=retrieval_release_evidence") {
		t.Fatalf("logs=%s", logs.String())
	}
}

func TestReleaseEvidenceReconciliationTelemetryUsesBoundedCategories(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordReleaseEvidenceReconciliation(context.Background(), ReleaseEvidenceReconciliationEvent{
		Outcome: "revoked", Reason: "watermark_mismatch", Freshness: "stale", SLO: "within_budget",
	})
	output := observer.RenderPrometheus()
	if !strings.Contains(output, "stele_retrieval_release_evidence_reconciliation_total") || !strings.Contains(output, "watermark_mismatch") {
		t.Fatalf("metrics output = %s", output)
	}
	if strings.Contains(strings.ToLower(output), "tenant") || strings.Contains(strings.ToLower(output), "handoff") {
		t.Fatalf("metrics output contains sensitive labels: %s", output)
	}
}
