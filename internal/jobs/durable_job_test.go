package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type durableStoreStub struct {
	acquired  bool
	completed int
	failed    int
}

func (s *durableStoreStub) AcquireMaintenanceLease(context.Context, MaintenanceLeaseInput) (bool, error) {
	return s.acquired, nil
}
func (s *durableStoreStub) RenewMaintenanceLease(context.Context, MaintenanceLeaseInput) error {
	return nil
}
func (s *durableStoreStub) ReclaimMaintenanceLease(context.Context, MaintenanceLeaseInput) (bool, error) {
	return false, nil
}
func (s *durableStoreStub) CompleteMaintenanceExecution(context.Context, MaintenanceCompletion) error {
	s.completed++
	return nil
}
func (s *durableStoreStub) FailMaintenanceExecution(context.Context, MaintenanceFailure) error {
	s.failed++
	return nil
}

type durableInnerJob struct{ runs int }

func (j *durableInnerJob) Name() string                     { return "projection_refresh" }
func (j *durableInnerJob) Run(context.Context) (int, error) { j.runs++; return 3, nil }

func TestDurableMaintenanceJobSuppressesDuplicateAndCompletesOwnedRun(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	store := &durableStoreStub{acquired: true}
	inner := &durableInnerJob{}
	job := DurableMaintenanceJob{Job: inner, Store: store, Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkerID: "w", Cadence: time.Hour, Now: func() time.Time { return now }}
	processed, err := job.Run(context.Background())
	if err != nil || processed != 3 || inner.runs != 1 || store.completed != 1 {
		t.Fatalf("processed=%d err=%v runs=%d completed=%d", processed, err, inner.runs, store.completed)
	}
	store.acquired = false
	processed, err = job.Run(context.Background())
	if err != nil || processed != 0 || inner.runs != 1 {
		t.Fatalf("duplicate processed=%d err=%v runs=%d", processed, err, inner.runs)
	}
}
