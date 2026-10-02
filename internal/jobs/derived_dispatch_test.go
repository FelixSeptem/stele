package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type dispatchQueueStub struct{ inputs []workqueue.EnqueueInput }

func (q *dispatchQueueStub) Enqueue(_ context.Context, input workqueue.EnqueueInput) (workqueue.BufferStatus, error) {
	q.inputs = append(q.inputs, input)
	return workqueue.BufferStatusDurable, nil
}
func (q *dispatchQueueStub) Flush(context.Context) (workqueue.FlushReport, error) {
	return workqueue.FlushReport{}, nil
}
func (q *dispatchQueueStub) Stats() workqueue.BufferStats       { return workqueue.BufferStats{} }
func (q *dispatchQueueStub) DrainReport() workqueue.DrainReport { return workqueue.DrainReport{} }
func (q *dispatchQueueStub) Close()                             {}

func TestProjectionWorkDispatcherIdentityIncludesRenderer(t *testing.T) {
	q := &dispatchQueueStub{}
	request := memory.ContextProjectionRebuildRequest{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: memory.ContextProjectionKindRetrieval, SchemaVersion: "schema-v1", Policy: memory.DefaultContextProjectionPolicy("policy-v1"), RendererVersion: "renderer-v1", Limit: 10}
	d := ProjectionWorkDispatcher{Queue: q, Now: func() time.Time { return time.Unix(10, 0).UTC() }}
	if _, err := d.Dispatch(context.Background(), request, "wm-1"); err != nil {
		t.Fatal(err)
	}
	if len(q.inputs) != 1 || q.inputs[0].Kind != workqueue.WorkKindProjectionRebuild || q.inputs[0].Idempotency == "" {
		t.Fatalf("input=%+v", q.inputs)
	}
}

func TestDerivedInsightReplayDispatcherRejectsDryRun(t *testing.T) {
	q := &dispatchQueueStub{}
	d := DerivedInsightReplayDispatcher{Queue: q}
	request := memory.DerivedInsightReplayRequest{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Mode: memory.DerivedInsightReplayModeDryRun, EvidenceWindowStart: time.Unix(1, 0).UTC(), EvidenceWindowEnd: time.Unix(2, 0).UTC(), EvidenceLimit: 1, Actor: "operator", Reason: "backfill", RequestedAt: time.Unix(2, 0).UTC()}
	if _, err := d.Dispatch(context.Background(), request); err == nil {
		t.Fatal("expected dry-run rejection")
	}
	if len(q.inputs) != 0 {
		t.Fatalf("inputs=%d", len(q.inputs))
	}
}
