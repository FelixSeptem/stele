package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestSchedulerRunCategoriesAreBounded(t *testing.T) {
	for _, state := range []SchedulerRunState{
		SchedulerRunPending, SchedulerRunRunning, SchedulerRunRetrying,
		SchedulerRunCompleted, SchedulerRunFailed, SchedulerRunDuplicate,
		SchedulerRunSkipped, SchedulerRunExhausted, SchedulerRunCancelled,
		SchedulerRunRecovered,
	} {
		if !state.Valid() {
			t.Fatalf("state %q should be valid", state)
		}
	}
	if SchedulerRunState("raw-error").Valid() {
		t.Fatal("unbounded scheduler state must be rejected")
	}
	for _, disposition := range []MaintenanceExecutionDisposition{
		MaintenanceDispositionCompleted, MaintenanceDispositionDuplicate,
		MaintenanceDispositionSkipped, MaintenanceDispositionExhausted,
		MaintenanceDispositionCancelled, MaintenanceDispositionManualReview,
	} {
		if !disposition.Valid() {
			t.Fatalf("disposition %q should be valid", disposition)
		}
	}
}

type schedulerHistoryStoreStub struct {
	durableStoreStub
	attempts []SchedulerRunAttempt
}

func (s *schedulerHistoryStoreStub) RecordSchedulerRunAttempt(_ context.Context, attempt SchedulerRunAttempt) error {
	s.attempts = append(s.attempts, attempt)
	return nil
}

func (s *schedulerHistoryStoreStub) RecordSchedulerRunSummary(context.Context, SchedulerRunSummary) error {
	return nil
}

func TestDurableMaintenanceJobAppendsRunHistoryForDuplicateAndTerminal(t *testing.T) {
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	store := &schedulerHistoryStoreStub{durableStoreStub: durableStoreStub{acquired: true}}
	job := DurableMaintenanceJob{
		Job:      maintenanceJobFunc(func(context.Context) (int, error) { return 2, nil }),
		Store:    store,
		Scope:    memory.Scope{Tenant: "t", Project: "p", Namespace: "n"},
		WorkerID: "worker-1",
		Cadence:  time.Hour,
		Now:      func() time.Time { return now },
	}
	if _, err := job.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	store.acquired = false
	if _, err := job.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.attempts) != 3 {
		t.Fatalf("history attempts = %d, want running, completed, duplicate", len(store.attempts))
	}
	if store.attempts[0].State != SchedulerRunRunning || store.attempts[1].State != SchedulerRunCompleted || store.attempts[2].Disposition != MaintenanceDispositionDuplicate {
		t.Fatalf("history = %+v", store.attempts)
	}
	if store.attempts[0].Scope != (memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}) {
		t.Fatalf("history scope = %+v", store.attempts[0].Scope)
	}
}

type schedulerRetentionStoreStub struct {
	cutoff time.Time
	limit  int
}

func (s *schedulerRetentionStoreStub) PruneSchedulerRunAttemptDetails(_ context.Context, cutoff time.Time, limit int) (int, error) {
	s.cutoff, s.limit = cutoff, limit
	return 3, nil
}

func TestSchedulerRunHistoryRetentionIsBoundedAndIdempotentFriendly(t *testing.T) {
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	store := &schedulerRetentionStoreStub{}
	job := SchedulerRunHistoryRetentionJob{Store: store, Now: func() time.Time { return now }, RetentionWindow: 24 * time.Hour, Limit: 7}
	deleted, err := job.Run(context.Background())
	if err != nil || deleted != 3 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if !store.cutoff.Equal(now.Add(-24*time.Hour)) || store.limit != 7 {
		t.Fatalf("retention input cutoff=%v limit=%d", store.cutoff, store.limit)
	}
}

func TestSchedulerRunSummaryConformanceEligibilityFailsClosed(t *testing.T) {
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	summary := SchedulerRunSummary{
		RunKey: "maintenance:run-1", JobClass: "projection_refresh",
		Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"},
		State: SchedulerRunCompleted, SourceWatermark: "wm-1", Freshness: "fresh",
		SLO: "within_budget", ObservedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute),
	}
	if !summary.EligibleForConformance(now, time.Hour, "wm-1") {
		t.Fatal("fresh matching summary should be eligible")
	}
	for _, mutate := range []func(*SchedulerRunSummary){
		func(s *SchedulerRunSummary) { s.Freshness = "stale" },
		func(s *SchedulerRunSummary) { s.SourceWatermark = "wm-2" },
		func(s *SchedulerRunSummary) { s.RetryExhausted = true },
		func(s *SchedulerRunSummary) { s.State = SchedulerRunFailed },
	} {
		candidate := summary
		mutate(&candidate)
		if candidate.EligibleForConformance(now, time.Hour, "wm-1") {
			t.Fatalf("summary %+v should fail closed", candidate)
		}
	}
}

var _ telemetry.Observer = (*maintenanceObserverStub)(nil)
