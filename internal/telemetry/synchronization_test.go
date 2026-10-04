package telemetry

import (
	"context"
	"strings"
	"testing"
)

func TestMetricsObserverExportsBoundedSynchronizationOutcomes(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordSynchronization(context.Background(), SynchronizationEvent{
		Operation: "resume", Result: "failed", Recovery: "resync_required",
		Transport: "openapi_pull", FailureCategory: "retention",
	})
	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, "stele_provider_sync_total") || !strings.Contains(metrics, `operation="resume"`) || !strings.Contains(metrics, `recovery="resync_required"`) {
		t.Fatalf("synchronization metric missing: %s", metrics)
	}
	for _, secret := range []string{"tenant-secret", "cursor-secret", "payload-secret", "SELECT"} {
		observer.RecordSynchronization(context.Background(), SynchronizationEvent{Operation: secret, Result: secret, Recovery: secret, Transport: secret, FailureCategory: secret})
	}
	metrics = observer.RenderPrometheus()
	for _, secret := range []string{"tenant-secret", "cursor-secret", "payload-secret", "SELECT"} {
		if strings.Contains(metrics, secret) {
			t.Fatalf("raw synchronization value leaked in metrics: %q", secret)
		}
	}
}

func TestMetricsObserverExportsSynchronizationHealthWithoutIdentifiers(t *testing.T) {
	observer := NewMetricsObserver()
	observer.RecordSynchronizationHealth(context.Background(), SynchronizationHealthEvent{Retention: "degraded", Cursor: "expiring", Backlog: "active", Pending: 3})
	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, "stele_provider_sync_backlog_pending") || !strings.Contains(metrics, `retention="degraded"`) {
		t.Fatalf("synchronization health metric missing: %s", metrics)
	}
}
