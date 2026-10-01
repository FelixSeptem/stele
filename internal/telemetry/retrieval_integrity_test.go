package telemetry

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

func TestRetrievalIntegrityTelemetryIsBounded(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordRetrievalIntegrity(context.Background(), RetrievalIntegrityEvent{
		Operation: "tenant-a query text", Result: "completed", Component: "trajectory", Verdict: "passed",
		FindingCategory: "missing", ArtifactCategory: "integrity_report",
	})
	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, "stele_retrieval_integrity_total") {
		t.Fatalf("integrity metric missing: %s", metrics)
	}
	for _, forbidden := range []string{"tenant-a", "query text", "memory-id", "provider-payload"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("integrity telemetry leaked %q: %s", forbidden, metrics)
		}
	}
}

func TestRetrievalIntegrityLifecycleLogIsBounded(t *testing.T) {
	var output bytes.Buffer
	LogRetrievalIntegrityLifecycle(log.New(&output, "", 0), RetrievalIntegrityEvent{
		Operation: "replay", Result: "tenant-a raw provider payload", Component: "integrity", Verdict: "rejected",
		FindingCategory: "foreign-scope", ArtifactCategory: "integrity_report",
	})
	entry := output.String()
	for _, required := range []string{"component=retrieval_integrity", "operation=replay", "result=unknown", "finding_category=foreign-scope"} {
		if !strings.Contains(entry, required) {
			t.Fatalf("lifecycle log omits %q: %s", required, entry)
		}
	}
	for _, forbidden := range []string{"tenant-a", "raw provider payload", "query text"} {
		if strings.Contains(entry, forbidden) {
			t.Fatalf("lifecycle log leaked %q: %s", forbidden, entry)
		}
	}
}
