package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/diagnostics"
)

func TestMetricsObserverExportsOperationsBacklogsAndAdmission(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()
	observer.RecordOperation(ctx, OperationEvent{
		Mode:      "scheduler",
		Component: "embedding_rebuild_job",
		Operation: "embedding_rebuild",
		Status:    "ok",
		Count:     3,
		Duration:  2 * time.Second,
	})
	observer.RecordBacklog(ctx, BacklogEvent{
		Mode:      "scheduler",
		Component: "embedding_rebuild_job",
		Queue:     "embedding_rebuilds",
		Status:    "ok",
		Pending:   5,
	})
	observer.RecordAdmission(ctx, AdmissionEvent{
		Component: "embedding_cutover",
		Operation: "preflight",
		Decision:  "deny",
		Blockers: []diagnostics.Finding{
			{Code: "zero_eligible_memory"},
		},
		Warnings: []diagnostics.Finding{
			{Code: "many_waves"},
		},
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_operations_total{component="embedding_rebuild_job",mode="scheduler",operation="embedding_rebuild",status="ok"} 1`,
		`stele_operation_processed_total{component="embedding_rebuild_job",mode="scheduler",operation="embedding_rebuild",status="ok"} 3`,
		`stele_backlog_pending{component="embedding_rebuild_job",mode="scheduler",queue="embedding_rebuilds",status="ok"} 5`,
		`stele_admission_decisions_total{component="embedding_cutover",decision="deny",operation="preflight"} 1`,
		`stele_admission_findings_total{code="zero_eligible_memory",component="embedding_cutover",operation="preflight",severity="blocker"} 1`,
		`stele_admission_findings_total{code="many_waves",component="embedding_cutover",operation="preflight",severity="warning"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
}

func TestMetricsObserverExportsEmbeddingCutoverAndProviderReadinessSignals(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordCutoverPlanState(ctx, CutoverPlanStateEvent{
		Status: "active",
		Count:  2,
	})
	observer.RecordCutoverItemState(ctx, CutoverItemStateEvent{
		Status: "queued",
		Count:  7,
	})
	observer.RecordProviderProbe(ctx, ProviderProbeEvent{
		Mode:     "scheduler",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Result:   "failure",
	})
	observer.RecordCutoverWaveDispatch(ctx, CutoverWaveDispatchEvent{
		Result:     "ok",
		Dispatched: 3,
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_embedding_cutover_plans{status="active"} 2`,
		`stele_embedding_cutover_items{status="queued"} 7`,
		`stele_embedding_provider_probe_total{mode="scheduler",model="text-embedding-3-small",provider="openai",result="failure"} 1`,
		`stele_embedding_cutover_wave_dispatch_total{result="ok"} 1`,
		`stele_embedding_cutover_wave_dispatched_total{result="ok"} 3`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
}
