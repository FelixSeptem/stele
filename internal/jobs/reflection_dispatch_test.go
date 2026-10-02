package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestReflectionWorkDispatcherUsesUnifiedQueueForTriggers(t *testing.T) {
	sink := &dispatchSink{}
	queue, err := workqueue.NewAdapter(workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 4, BatchSize: 4, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}, sink)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := ReflectionWorkDispatcher{Queue: queue, Now: func() time.Time { return time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC) }}
	status, err := dispatcher.Dispatch(context.Background(), memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, memory.ReflectionTriggerSessionCompletion, "wm-1", "session-1", "trigger-1")
	if err != nil || status != workqueue.BufferStatusBuffered {
		t.Fatalf("dispatch = %q, %v", status, err)
	}
	if _, err := queue.Flush(context.Background()); err != nil || len(sink.inputs) != 1 || sink.inputs[0].Kind != workqueue.WorkKindReflection {
		t.Fatalf("flush sink = %+v, %v", sink.inputs, err)
	}
}

type dispatchSink struct{ inputs []workqueue.DerivedWorkInput }

func (s *dispatchSink) EnqueueDerivedWork(_ context.Context, input workqueue.EnqueueInput) (workqueue.DerivedWorkItem, error) {
	s.inputs = append(s.inputs, input.DerivedWorkInput)
	return workqueue.DerivedWorkItem{}, nil
}
