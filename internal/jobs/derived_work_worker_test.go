package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
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
