package telemetry

import (
	"context"
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
