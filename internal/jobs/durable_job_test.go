package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type durableStoreStub struct {
	acquired  bool
	state     MaintenanceExecutionState
	completed int
	failed    int
	failure   MaintenanceFailure
	renewed   int
	renewErr  error
}

type maintenanceObserverStub struct {
	events int
	last   telemetry.MaintenanceEvent
}

func (o *maintenanceObserverStub) RecordOperation(context.Context, telemetry.OperationEvent) {}
func (o *maintenanceObserverStub) RecordBacklog(context.Context, telemetry.BacklogEvent)     {}
func (o *maintenanceObserverStub) RecordMaintenance(_ context.Context, event telemetry.MaintenanceEvent) {
	o.events++
	o.last = event
}

func (s *durableStoreStub) AcquireMaintenanceLease(context.Context, MaintenanceLeaseInput) (bool, error) {
	return s.acquired, nil
}
func (s *durableStoreStub) ReadOwnedMaintenanceExecution(context.Context, MaintenanceIdentity, string) (MaintenanceExecutionState, error) {
	return s.state, nil
}
func (s *durableStoreStub) RenewMaintenanceLease(context.Context, MaintenanceLeaseInput) error {
	s.renewed++
	return s.renewErr
}

type blockingDurableInnerJob struct {
	started chan struct{}
	release chan struct{}
}

func (j *blockingDurableInnerJob) Name() string { return "projection_refresh" }
func (j *blockingDurableInnerJob) Run(ctx context.Context) (int, error) {
	close(j.started)
	select {
	case <-j.release:
		return 1, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func TestDurableMaintenanceJobRenewsLeaseAndStopsOnConflict(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	store := &durableStoreStub{acquired: true, renewErr: context.Canceled}
	inner := &blockingDurableInnerJob{started: make(chan struct{}), release: make(chan struct{})}
	job := DurableMaintenanceJob{Job: inner, Store: store, Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkerID: "w", Cadence: time.Hour, LeaseDuration: 5 * time.Millisecond, LeaseRenewInterval: time.Millisecond, Now: func() time.Time { return now }}
	done := make(chan error, 1)
	go func() { _, err := job.Run(context.Background()); done <- err }()
	<-inner.started
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("durable job did not stop after lease conflict")
	}
	if store.renewed == 0 {
		t.Fatal("renew was not attempted")
	}
	if store.completed != 0 {
		t.Fatal("conflicted lease must not complete")
	}
}

func TestDurableMaintenanceJobFailurePersistsRetryState(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	store := &durableStoreStub{acquired: true}
	inner := maintenanceJobFunc(func(context.Context) (int, error) { return 0, context.DeadlineExceeded })
	job := DurableMaintenanceJob{Job: inner, Store: store, Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkerID: "w", Cadence: time.Hour, RetryBackoff: 2 * time.Minute, Now: func() time.Time { return now }}
	if _, err := job.Run(context.Background()); err == nil {
		t.Fatal("failed maintenance unexpectedly succeeded")
	}
	if store.failed != 1 {
		t.Fatalf("failed count = %d, want one persisted failure", store.failed)
	}
}

func TestDurableMaintenanceJobResumesCheckpointAndPreservesExhaustedDisposition(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	store := &durableStoreStub{acquired: true, state: MaintenanceExecutionState{Attempt: 2, Checkpoint: "cursor-7", SourceWatermark: "wm-7"}}
	inner := maintenanceJobFunc(func(context.Context) (int, error) { return 0, context.DeadlineExceeded })
	job := DurableMaintenanceJob{Job: inner, Store: store, Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkerID: "w", Cadence: time.Hour, RetryBackoff: time.Minute, MaxAttempts: 2, Now: func() time.Time { return now }}
	if _, err := job.Run(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	if store.failure.Disposition != MaintenanceDispositionExhausted || !store.failure.NextAttemptAt.IsZero() {
		t.Fatalf("failure=%+v, want terminal exhausted without retry", store.failure)
	}
	if store.failure.Checkpoint != "cursor-7" || store.failure.Watermark != "wm-7" {
		t.Fatalf("failure checkpoint=%q watermark=%q", store.failure.Checkpoint, store.failure.Watermark)
	}
}

type maintenanceJobFunc func(context.Context) (int, error)

func (f maintenanceJobFunc) Name() string                         { return "projection_refresh" }
func (f maintenanceJobFunc) Run(ctx context.Context) (int, error) { return f(ctx) }
func (s *durableStoreStub) ReclaimMaintenanceLease(context.Context, MaintenanceLeaseInput) (bool, error) {
	return false, nil
}
func (s *durableStoreStub) CompleteMaintenanceExecution(context.Context, MaintenanceCompletion) error {
	s.completed++
	return nil
}
func (s *durableStoreStub) FailMaintenanceExecution(_ context.Context, input MaintenanceFailure) error {
	s.failed++
	s.failure = input
	return nil
}

type durableInnerJob struct{ runs int }

func (j *durableInnerJob) Name() string                     { return "projection_refresh" }
func (j *durableInnerJob) Run(context.Context) (int, error) { j.runs++; return 3, nil }

func TestDurableMaintenanceJobSuppressesDuplicateAndCompletesOwnedRun(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	store := &durableStoreStub{acquired: true}
	inner := &durableInnerJob{}
	observer := &maintenanceObserverStub{}
	job := DurableMaintenanceJob{Job: inner, Store: store, Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkerID: "w", Cadence: time.Hour, Now: func() time.Time { return now }, Observer: observer}
	processed, err := job.Run(context.Background())
	if err != nil || processed != 3 || inner.runs != 1 || store.completed != 1 {
		t.Fatalf("processed=%d err=%v runs=%d completed=%d", processed, err, inner.runs, store.completed)
	}
	store.acquired = false
	processed, err = job.Run(context.Background())
	if err != nil || processed != 0 || inner.runs != 1 {
		t.Fatalf("duplicate processed=%d err=%v runs=%d", processed, err, inner.runs)
	}
	if observer.events != 2 || observer.last.Outcome != "duplicate" {
		t.Fatalf("maintenance telemetry = %+v events=%d, want success and duplicate", observer.last, observer.events)
	}
}
