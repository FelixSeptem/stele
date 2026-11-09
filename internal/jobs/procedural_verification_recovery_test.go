package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestProceduralRevalidationDispatchRecoveryRedeliversPendingDurableRun(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &proceduralRevalidationRecoveryStoreStub{runs: []memory.ProceduralRevalidationRun{{
		ID: "run-1", Scope: scope, SourceWatermark: "watermark-1", WorkIdentity: "identity-1", Status: memory.ProceduralRevalidationRunStatusPending,
	}}}
	dispatcher := &proceduralRevalidationRecoveryDispatcherStub{}
	job := ProceduralRevalidationDispatchRecovery{Store: store, Dispatcher: dispatcher, Scope: scope, Now: func() time.Time {
		return time.Date(2026, time.November, 8, 4, 0, 0, 0, time.UTC)
	}}

	dispatched, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 || len(store.inputs) != 1 || len(dispatcher.requests) != 1 {
		t.Fatalf("dispatched=%d list=%+v requests=%+v", dispatched, store.inputs, dispatcher.requests)
	}
	if got := dispatcher.requests[0]; got.Scope != scope || got.RunID != "run-1" || got.Watermark != "watermark-1" || got.Identity != "identity-1" {
		t.Fatalf("redelivery = %+v", got)
	}
}

func TestProceduralRevalidationDispatchRecoveryRecordsBoundedRedeliveryTelemetry(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := telemetry.NewMetricsObserver()
	job := ProceduralRevalidationDispatchRecovery{
		Store: &proceduralRevalidationRecoveryStoreStub{runs: []memory.ProceduralRevalidationRun{{
			ID: "run-1", Scope: scope, SourceWatermark: "watermark-1", WorkIdentity: "identity-1", Status: memory.ProceduralRevalidationRunStatusPending,
		}}},
		Dispatcher: &proceduralRevalidationRecoveryDispatcherStub{}, Scope: scope, Observer: observer,
	}
	if _, err := job.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="recover"`, `result="accepted"`, `queue="recovery"`, `recovery="redelivered"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("redelivery telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func TestProceduralRevalidationDispatchRecoveryRecordsBoundedFailureTelemetry(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := telemetry.NewMetricsObserver()
	job := ProceduralRevalidationDispatchRecovery{
		Store: &proceduralRevalidationRecoveryStoreStub{runs: []memory.ProceduralRevalidationRun{{
			ID: "run-1", Scope: scope, SourceWatermark: "watermark-1", WorkIdentity: "identity-1", Status: memory.ProceduralRevalidationRunStatusPending,
		}}},
		Dispatcher: &proceduralRevalidationRecoveryDispatcherStub{err: errors.New("queue unavailable")}, Scope: scope, Observer: observer,
	}
	if _, err := job.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() accepted failed redispatch")
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="recover"`, `result="failed"`, `queue="recovery"`, `recovery="redelivered"`, `failure_category="dependency"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("recovery failure telemetry missing %s:\n%s", want, metrics)
		}
	}
}

type proceduralRevalidationRecoveryStoreStub struct {
	runs   []memory.ProceduralRevalidationRun
	inputs []memory.ListPendingProceduralRevalidationRunsInput
}

func (s *proceduralRevalidationRecoveryStoreStub) ListPendingProceduralRevalidationRuns(_ context.Context, input memory.ListPendingProceduralRevalidationRunsInput) ([]memory.ProceduralRevalidationRun, error) {
	s.inputs = append(s.inputs, input)
	return s.runs, nil
}

type proceduralRevalidationRecoveryDispatcherStub struct {
	requests []memory.ProceduralRevalidationDispatch
	err      error
}

func (s *proceduralRevalidationRecoveryDispatcherStub) DispatchProceduralRevalidation(_ context.Context, request memory.ProceduralRevalidationDispatch) error {
	s.requests = append(s.requests, request)
	return s.err
}
