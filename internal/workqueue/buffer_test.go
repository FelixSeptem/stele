package workqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type bufferSink struct {
	items []DerivedWorkItem
	err   error
}

func (s *bufferSink) EnqueueDerivedWork(_ context.Context, input EnqueueInput) (DerivedWorkItem, error) {
	if s.err != nil {
		return DerivedWorkItem{}, s.err
	}
	s.items = append(s.items, DerivedWorkItem{DerivedWorkIdentity: DerivedWorkIdentity{WorkKey: input.WorkKey()}})
	return s.items[len(s.items)-1], nil
}

func TestMemoryBufferBoundsAndFlushAccounting(t *testing.T) {
	sink := &bufferSink{}
	cfg := QueueConfig{Mode: QueueModeMemoryBuffer, Capacity: 1, BatchSize: 1, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}
	buffer, err := NewMemoryBuffer(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	input := EnqueueInput{DerivedWorkInput: DerivedWorkInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: WorkKindReflection, Watermark: "wm", Idempotency: "key", Reference: "ref"}, MaxAttempts: 3, Now: time.Now().UTC(), DetailExpiresAt: time.Now().UTC().Add(time.Hour)}
	if got, err := buffer.Enqueue(context.Background(), input); err != nil || got != BufferStatusBuffered {
		t.Fatalf("first enqueue = %q, %v", got, err)
	}
	if got, err := buffer.Enqueue(context.Background(), input); !errors.Is(err, ErrBufferFull) || got != BufferStatusDropped {
		t.Fatalf("overflow enqueue = %q, %v", got, err)
	}
	if report, err := buffer.Flush(context.Background()); err != nil || report.Durable != 1 {
		t.Fatalf("flush = %+v, %v", report, err)
	}
	stats := buffer.Stats()
	if stats.DroppedOverflow != 1 || stats.Buffered != 0 || stats.Durable != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestMemoryBufferFlushFailureIsNonDurable(t *testing.T) {
	sink := &bufferSink{err: errors.New("database unavailable")}
	cfg := QueueConfig{Mode: QueueModeMemoryBuffer, Capacity: 2, BatchSize: 2, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}
	buffer, err := NewMemoryBuffer(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	input := EnqueueInput{DerivedWorkInput: DerivedWorkInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: WorkKindCompaction, Watermark: "wm", Idempotency: "key", Reference: "ref"}, MaxAttempts: 3, Now: time.Now().UTC(), DetailExpiresAt: time.Now().UTC().Add(time.Hour)}
	if _, err := buffer.Enqueue(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	report, err := buffer.Flush(context.Background())
	if err == nil || report.Durable != 0 || report.DroppedFlushFailure != 1 {
		t.Fatalf("flush = %+v, %v", report, err)
	}
}

func TestMemoryBufferCloseCountsOnlyUnflushedWorkAsProcessLoss(t *testing.T) {
	sink := &bufferSink{}
	cfg := QueueConfig{Mode: QueueModeMemoryBuffer, Capacity: 2, BatchSize: 1, FlushInterval: time.Second, LeaseDuration: time.Minute, MaxAttempts: 3, Retention: time.Hour}
	buffer, err := NewMemoryBuffer(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	input := EnqueueInput{DerivedWorkInput: DerivedWorkInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: WorkKindReflection, Watermark: "wm", Idempotency: "key", Reference: "ref"}, MaxAttempts: 3, Now: time.Now().UTC(), DetailExpiresAt: time.Now().UTC().Add(time.Hour)}
	if _, err := buffer.Enqueue(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Enqueue(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	buffer.Close()
	stats := buffer.Stats()
	if stats.Durable != 1 || stats.DroppedProcessLoss != 1 || stats.Buffered != 0 {
		t.Fatalf("stats = %+v", stats)
	}
}
