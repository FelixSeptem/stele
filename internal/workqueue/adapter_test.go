package workqueue

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestNewAdapterSelectsStartupMode(t *testing.T) {
	sink := &bufferSink{}
	cfg := QueueConfig{Mode: QueueModePostgresDurable, Capacity: 1, BatchSize: 1, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 1, Retention: time.Hour}
	adapter, err := NewAdapter(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	input := EnqueueInput{DerivedWorkInput: DerivedWorkInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: WorkKindReflection, Watermark: "wm", Idempotency: "key", Reference: "ref"}, MaxAttempts: 1, Now: time.Now().UTC(), DetailExpiresAt: time.Now().UTC().Add(time.Hour)}
	status, err := adapter.Enqueue(context.Background(), input)
	if err != nil || status != BufferStatusDurable {
		t.Fatalf("durable enqueue = %q, %v", status, err)
	}
	if adapter.Stats().Durable != 1 || adapter.Stats().Buffered != 0 {
		t.Fatalf("durable stats = %+v", adapter.Stats())
	}
}
