package telemetry

import (
	"context"
	"strings"
	"testing"
)

func TestMetricsObserverExportsProceduralVerificationSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordProceduralVerification(context.Background(), ProceduralVerificationEvent{
		Operation: "record", Result: "accepted", Disposition: "refuted", Trigger: "administrator", Queue: "durable", Recovery: "none", FailureCategory: "none",
	})
	observer.RecordProceduralVerification(context.Background(), ProceduralVerificationEvent{
		Operation: "tenant-secret", Result: "memory-123", Disposition: "actual claim text", Trigger: "source-99", Queue: "postgres://secret", Recovery: "run-99", FailureCategory: "boom detail",
	})
	metrics := observer.RenderPrometheus()
	want := `stele_procedural_verification_total{disposition="refuted",failure_category="none",operation="record",queue="durable",recovery="none",result="accepted",trigger="administrator"} 1`
	if !strings.Contains(metrics, want) {
		t.Fatalf("procedural verification metric missing:\n%s", metrics)
	}
	for _, forbidden := range []string{"tenant-secret", "memory-123", "actual claim text", "source-99", "postgres://secret", "run-99", "boom detail"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics leaked high-cardinality value %q:\n%s", forbidden, metrics)
		}
	}
}
