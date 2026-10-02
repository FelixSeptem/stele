package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type recordingDurableSink struct {
	keys map[string]workqueue.DerivedWorkItem
}

func (s *recordingDurableSink) EnqueueDerivedWork(_ context.Context, in workqueue.EnqueueInput) (workqueue.DerivedWorkItem, error) {
	if s.keys == nil {
		s.keys = map[string]workqueue.DerivedWorkItem{}
	}
	key := in.WorkKey()
	if existing, ok := s.keys[key]; ok {
		return existing, nil
	}
	item := workqueue.DerivedWorkItem{DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: in.Scope, WorkKey: key}, Kind: in.Kind, Watermark: in.Watermark, State: workqueue.WorkStateQueued}
	s.keys[key] = item
	return item, nil
}

func TestDerivedDispatchChainIsIdempotentAcrossQueueModes(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "default"}
	for _, mode := range []workqueue.QueueMode{workqueue.QueueModePostgresDurable, workqueue.QueueModeMemoryBuffer} {
		sink := &recordingDurableSink{}
		cfg := workqueue.QueueConfig{Mode: mode, Capacity: 16, BatchSize: 16, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}
		queue, err := workqueue.NewAdapter(cfg, sink)
		if err != nil {
			t.Fatal(err)
		}
		reflection := ReflectionWorkDispatcher{Queue: queue}
		compaction := CompactionWorkDispatcher{Queue: queue}
		projection := ProjectionWorkDispatcher{Queue: queue}
		if _, err := reflection.Dispatch(context.Background(), scope, memory.ReflectionTriggerSessionCompletion, "wm-1", "session-1", "session-1"); err != nil {
			t.Fatal(err)
		}
		if _, err := compaction.DispatchCompaction(context.Background(), scope, "wm-1", "compact-1"); err != nil {
			t.Fatal(err)
		}
		if _, err := compaction.DispatchFollowUpReflection(context.Background(), scope, "wm-2", "evidence-1", "summary-1"); err != nil {
			t.Fatal(err)
		}
		request := memory.ContextProjectionRebuildRequest{Scope: scope, Kind: memory.ContextProjectionKindAlwaysVisible, SchemaVersion: "schema-v1", Policy: memory.DefaultContextProjectionPolicy("policy-v1"), RendererVersion: "renderer-v1", Limit: 10}
		if _, err := projection.Dispatch(context.Background(), request, "wm-2"); err != nil {
			t.Fatal(err)
		}
		if _, err := reflection.Dispatch(context.Background(), scope, memory.ReflectionTriggerSessionCompletion, "wm-1", "session-1", "session-1"); err != nil {
			t.Fatal(err)
		}
		if mode == workqueue.QueueModeMemoryBuffer {
			if _, err := queue.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if len(sink.keys) != 4 {
			t.Fatalf("mode=%s durable work=%d, want 4", mode, len(sink.keys))
		}
		queue.Close()
	}
}
