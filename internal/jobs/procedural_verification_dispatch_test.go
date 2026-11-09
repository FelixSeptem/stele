package jobs

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestProceduralRevalidationDispatcherEnqueuesRunReferenceOnlyWhenScopePolicyEnabled(t *testing.T) {
	sink := &dispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	dispatcher := ProceduralRevalidationDispatcher{Queue: queue, Now: func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }, Policy: memory.StaticProceduralVerificationPolicy{Scope: scope, Enabled: true, Version: "policy-v1"}}
	status, err := dispatcher.Dispatch(context.Background(), scope, "run-1", "watermark-1", "identity-1")
	if err != nil || status != workqueue.BufferStatusBuffered {
		t.Fatalf("Dispatch() = %q, %v", status, err)
	}
	if _, err := queue.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.inputs) != 1 || sink.inputs[0].Kind != workqueue.WorkKindProceduralRevalidation || sink.inputs[0].Reference != "run-1" {
		t.Fatalf("dispatched work = %+v", sink.inputs)
	}
}

func TestProceduralRevalidationDispatcherRefusesDisabledScopeBeforeEnqueue(t *testing.T) {
	sink := &dispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	dispatcher := ProceduralRevalidationDispatcher{Queue: queue, Policy: memory.StaticProceduralVerificationPolicy{Scope: scope, Enabled: false, Version: "policy-v1"}}
	if _, err := dispatcher.Dispatch(context.Background(), scope, "run-1", "watermark-1", "identity-1"); err == nil {
		t.Fatal("Dispatch() accepted disabled scope")
	}
	if len(sink.inputs) != 0 {
		t.Fatalf("disabled scope enqueued work: %+v", sink.inputs)
	}
}

func TestProceduralRevalidationDispatcherImplementsMemoryDispatchContract(t *testing.T) {
	sink := &dispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	dispatcher := ProceduralRevalidationDispatcher{Queue: queue, Now: func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }, Policy: memory.StaticProceduralVerificationPolicy{Scope: scope, Enabled: true, Version: "policy-v1"}}
	if err := dispatcher.DispatchProceduralRevalidation(context.Background(), memory.ProceduralRevalidationDispatch{Scope: scope, RunID: "run-1", Watermark: "watermark-1", Identity: "identity-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.inputs) != 1 || sink.inputs[0].Reference != "run-1" || sink.inputs[0].Watermark != "watermark-1" || sink.inputs[0].Idempotency != "identity-1" {
		t.Fatalf("dispatched work = %+v", sink.inputs)
	}
}

func TestProceduralRevalidationDispatcherRecordsBoundedQueueTelemetry(t *testing.T) {
	sink := &dispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	observer := telemetry.NewMetricsObserver()
	dispatcher := ProceduralRevalidationDispatcher{Queue: queue, Now: func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }, Policy: memory.StaticProceduralVerificationPolicy{Scope: scope, Enabled: true, Version: "policy-v1"}, Observer: observer}
	if _, err := dispatcher.Dispatch(context.Background(), scope, "run-1", "watermark-1", "identity-1"); err != nil {
		t.Fatal(err)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="dispatch"`, `result="accepted"`, `queue="buffered"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("dispatch telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func TestProceduralRevalidationDispatcherRecordsDisabledPolicyTelemetry(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	observer := telemetry.NewMetricsObserver()
	dispatcher := ProceduralRevalidationDispatcher{
		Queue: &dispatchQueueStub{}, Policy: memory.StaticProceduralVerificationPolicy{Scope: scope, Enabled: false, Version: "policy-v1"}, Observer: observer,
	}
	if _, err := dispatcher.Dispatch(context.Background(), scope, "run-1", "watermark-1", "identity-1"); err == nil {
		t.Fatal("Dispatch() accepted disabled policy")
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="dispatch"`, `result="disabled"`, `failure_category="policy_disabled"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("disabled dispatch telemetry missing %s:\n%s", want, metrics)
		}
	}
}
