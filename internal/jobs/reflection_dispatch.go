package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// ReflectionWorkDispatcher is the compatibility boundary for existing
// reflection trigger callers. It records a derived work item and leaves
// execution to the worker claim path.
type ReflectionWorkDispatcher struct {
	Queue       workqueue.QueueAdapter
	Now         func() time.Time
	MaxAttempts int
	Retention   time.Duration
}

func (d ReflectionWorkDispatcher) Dispatch(ctx context.Context, scope memory.Scope, trigger memory.ReflectionTrigger, watermark, reference, idempotencyKey string) (workqueue.BufferStatus, error) {
	if d.Queue == nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("reflection work queue is required")
	}
	if !trigger.Valid() {
		return workqueue.BufferStatusDropped, fmt.Errorf("reflection trigger %q is invalid", trigger)
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	if strings.TrimSpace(reference) == "" {
		reference = string(trigger)
	}
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	retention := d.Retention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: scope, Kind: workqueue.WorkKindReflection, Watermark: watermark, Idempotency: idempotencyKey, Reference: reference},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
}
