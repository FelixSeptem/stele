package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestCompactionDispatcherUsesDeterministicFollowUpIdentity(t *testing.T) {
	sink := &compactionDispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := CompactionWorkDispatcher{Queue: queue, Now: func() time.Time { return time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC) }}
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	for i := 0; i < 2; i++ {
		if _, err := dispatcher.DispatchFollowUpReflection(context.Background(), scope, "wm-1", "evidence-1", "summary-v1"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := queue.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.inputs) != 2 || sink.inputs[0].WorkKey() != sink.inputs[1].WorkKey() {
		t.Fatalf("follow-up identities = %+v", sink.inputs)
	}
}

type compactionDispatchSink struct{ inputs []workqueue.EnqueueInput }

func (s *compactionDispatchSink) EnqueueDerivedWork(_ context.Context, input workqueue.EnqueueInput) (workqueue.DerivedWorkItem, error) {
	s.inputs = append(s.inputs, input)
	return workqueue.DerivedWorkItem{}, nil
}
