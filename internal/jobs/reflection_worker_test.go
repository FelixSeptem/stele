package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type reflectionStoreStub struct {
	claims      []memory.ReflectionRun
	checkpoints []memory.CheckpointReflectionRunInput
	completed   []memory.CompleteReflectionRunInput
	failed      []memory.FailReflectionRunInput
}

func (s *reflectionStoreStub) CreateReflectionRun(ctx context.Context, in memory.CreateReflectionRunInput) (memory.ReflectionRun, error) {
	return memory.ReflectionRun{}, nil
}
func (s *reflectionStoreStub) ClaimReflectionRuns(ctx context.Context, in memory.ClaimReflectionRunsInput) ([]memory.ReflectionRun, error) {
	return s.claims, nil
}
func (s *reflectionStoreStub) CheckpointReflectionRun(ctx context.Context, in memory.CheckpointReflectionRunInput) error {
	s.checkpoints = append(s.checkpoints, in)
	return nil
}
func (s *reflectionStoreStub) CompleteReflectionRun(ctx context.Context, in memory.CompleteReflectionRunInput) error {
	s.completed = append(s.completed, in)
	return nil
}
func (s *reflectionStoreStub) FailReflectionRun(ctx context.Context, in memory.FailReflectionRunInput) error {
	s.failed = append(s.failed, in)
	return nil
}
func (s *reflectionStoreStub) ReplayReflectionRun(ctx context.Context, in memory.ReplayReflectionRunInput) (memory.ReflectionRun, error) {
	return memory.ReflectionRun{}, nil
}

type reflectionExecutorStub struct{ err error }

func (s reflectionExecutorStub) ExecuteReflection(ctx context.Context, run memory.ReflectionRun, offset int64) (int64, []string, []string, error) {
	if s.err != nil {
		return offset, nil, nil, s.err
	}
	return offset + 1, []string{"candidate-1"}, []string{"event-1"}, nil
}

func TestReflectionRunWorkerCheckpointsAndCompletes(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	store := &reflectionStoreStub{claims: []memory.ReflectionRun{{ID: "run-1", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Status: memory.ReflectionRunRunning, ProcessedOffset: 2, Attempt: 1, MaxAttempts: 3, LeaseOwner: "worker-1"}}}
	w := ReflectionRunWorker{Store: store, Executor: reflectionExecutorStub{}, Scope: store.claims[0].Scope, WorkerID: "worker-1", Now: func() time.Time { return now }}
	if n, err := w.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("RunOnce()=%d,%v", n, err)
	}
	if len(store.checkpoints) != 1 || store.checkpoints[0].ProcessedOffset != 3 {
		t.Fatalf("checkpoint=%+v", store.checkpoints)
	}
	if len(store.completed) != 1 {
		t.Fatalf("completed=%d", len(store.completed))
	}
}

func TestReflectionRunWorkerRecordsBoundedRetryFailure(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	store := &reflectionStoreStub{claims: []memory.ReflectionRun{{ID: "run-1", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Status: memory.ReflectionRunRunning, Attempt: 1, MaxAttempts: 3, LeaseOwner: "worker-1"}}}
	w := ReflectionRunWorker{Store: store, Executor: reflectionExecutorStub{err: errors.New("boom")}, Scope: store.claims[0].Scope, WorkerID: "worker-1", RetryBackoff: time.Minute, Now: func() time.Time { return now }}
	if n, err := w.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatalf("RunOnce()=%d,%v", n, err)
	}
	if len(store.failed) != 1 || store.failed[0].Category != memory.ReflectionFailureTransient || !store.failed[0].RetryAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("failure=%+v", store.failed)
	}
}
