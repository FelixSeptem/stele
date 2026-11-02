package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type derivedWorkStoreStub struct {
	items       []workqueue.DerivedWorkItem
	checkpoints []workqueue.CheckpointInput
	completed   []workqueue.TerminalInput
	retries     []workqueue.RetryInput
}

func (s *derivedWorkStoreStub) ClaimDerivedWork(context.Context, workqueue.ClaimInput) ([]workqueue.DerivedWorkItem, error) {
	return s.items, nil
}
func (s *derivedWorkStoreStub) CheckpointDerivedWork(_ context.Context, in workqueue.CheckpointInput) error {
	s.checkpoints = append(s.checkpoints, in)
	return nil
}
func (s *derivedWorkStoreStub) CompleteDerivedWork(_ context.Context, in workqueue.TerminalInput) error {
	s.completed = append(s.completed, in)
	return nil
}
func (s *derivedWorkStoreStub) RetryDerivedWork(_ context.Context, in workqueue.RetryInput) error {
	s.retries = append(s.retries, in)
	return nil
}

type derivedWorkExecutorStub struct{ err error }

func (e derivedWorkExecutorStub) ExecuteDerivedWork(context.Context, workqueue.DerivedWorkItem) (int64, string, error) {
	return 3, "evidence-1", e.err
}

type exhaustionAwareDerivedWorkExecutorStub struct {
	derivedWorkExecutorStub
	exhausted []workqueue.DerivedWorkItem
}

func (e *exhaustionAwareDerivedWorkExecutorStub) HandleDerivedWorkExhaustion(_ context.Context, item workqueue.DerivedWorkItem) error {
	e.exhausted = append(e.exhausted, item)
	return nil
}

func TestDerivedWorkWorkerUsesCommonCheckpointAndCompletionPath(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &derivedWorkStoreStub{items: []workqueue.DerivedWorkItem{{ID: "work-1", DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Watermark: "wm-1", AttemptCount: 1, MaxAttempts: 3}}}
	w := DerivedWorkWorker{Store: store, Executor: derivedWorkExecutorStub{}, Scope: scope, WorkerID: "worker-1", Now: func() time.Time { return time.Unix(10, 0).UTC() }}
	if n, err := w.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("RunOnce()=%d,%v", n, err)
	}
	if len(store.checkpoints) != 1 || len(store.completed) != 1 || store.completed[0].EvidenceReference != "evidence-1" {
		t.Fatalf("checkpoint=%+v completed=%+v", store.checkpoints, store.completed)
	}
}

func TestDerivedWorkWorkerRefusesMemoryIntentKindAfterRiverCutover(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	kind := workqueue.WorkKindMemoryIntent
	store := &derivedWorkStoreStub{}
	worker := DerivedWorkWorker{Store: store, Executor: derivedWorkExecutorStub{}, Scope: scope, WorkerID: "worker", Kind: &kind}
	if processed, err := worker.RunOnce(context.Background()); err == nil || processed != 0 {
		t.Fatalf("RunOnce() = %d, %v; want memory-intent kind rejected", processed, err)
	}
	if len(store.completed) != 0 || len(store.retries) != 0 || len(store.checkpoints) != 0 {
		t.Fatalf("legacy worker touched memory-intent state: %+v", store)
	}
}

type memoryIntentJobStoreStub struct {
	jobID  int64
	intent memory.MemoryIntentRecord
	called bool
}

func (s *memoryIntentJobStoreStub) ExecuteMemoryIntentJob(ctx context.Context, job *river.Job[MemoryIntentJobArgs], execute func(context.Context, memory.MemoryIntentRecord) error) error {
	s.called = true
	s.jobID = job.ID
	return execute(ctx, s.intent)
}

func TestRiverMemoryIntentWorkerReturnsExecutionFailureForRiverRetry(t *testing.T) {
	store := &memoryIntentJobStoreStub{intent: memory.MemoryIntentRecord{ID: "intent-1"}}
	wantErr := context.DeadlineExceeded
	worker := RiverMemoryIntentWorker{Store: store, Execute: func(context.Context, memory.MemoryIntentRecord) error { return wantErr }}
	job := &river.Job[MemoryIntentJobArgs]{JobRow: &rivertype.JobRow{ID: 42}, Args: MemoryIntentJobArgs{IntentID: "intent-1"}}
	if err := worker.Work(context.Background(), job); !errors.Is(err, wantErr) {
		t.Fatalf("Work() error = %v, want %v", err, wantErr)
	}
	if !store.called || store.jobID != 42 {
		t.Fatalf("store execution = called %t job %d, want River job 42", store.called, store.jobID)
	}
}

func TestDerivedWorkWorkerNotifiesExecutorWhenFinalAttemptFails(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &derivedWorkStoreStub{items: []workqueue.DerivedWorkItem{{ID: "work-1", DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Watermark: "wm-1", AttemptCount: 3, MaxAttempts: 3}}}
	executor := &exhaustionAwareDerivedWorkExecutorStub{derivedWorkExecutorStub: derivedWorkExecutorStub{err: context.DeadlineExceeded}}
	worker := DerivedWorkWorker{Store: store, Executor: executor, Scope: scope, WorkerID: "worker-1", Now: func() time.Time { return time.Unix(10, 0).UTC() }}

	if processed, err := worker.RunOnce(context.Background()); err != nil || processed != 0 {
		t.Fatalf("RunOnce() = %d, %v", processed, err)
	}
	if len(store.retries) != 1 {
		t.Fatalf("retry calls = %d, want 1", len(store.retries))
	}
	if len(executor.exhausted) != 1 || executor.exhausted[0].ID != "work-1" {
		t.Fatalf("exhaustion callbacks = %+v", executor.exhausted)
	}
}
