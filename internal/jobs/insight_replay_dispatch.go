package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// DerivedInsightReplayDispatcher turns an operator replay apply request into
// one bounded queue item. Planning remains synchronous and never mutates
// active insights; workers perform the governed lifecycle transitions later.
type DerivedInsightReplayDispatcher struct {
	Queue       workqueue.QueueAdapter
	Now         func() time.Time
	MaxAttempts int
	Retention   time.Duration
}

func (d DerivedInsightReplayDispatcher) Dispatch(ctx context.Context, request memory.DerivedInsightReplayRequest) (workqueue.BufferStatus, error) {
	if d.Queue == nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("insight replay work queue is required")
	}
	if err := request.Validate(); err != nil {
		return workqueue.BufferStatusDropped, err
	}
	if request.Mode != memory.DerivedInsightReplayModeApply {
		return workqueue.BufferStatusDropped, fmt.Errorf("only replay apply requests can be queued")
	}
	now := request.RequestedAt.UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	retention := d.Retention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	watermark := strings.TrimSpace(request.ActivationSourceWatermark)
	if watermark == "" {
		watermark = fmt.Sprintf("%d:%d:%d", request.EvidenceWindowStart.UnixNano(), request.EvidenceWindowEnd.UnixNano(), request.EvidenceLimit)
	}
	idempotency := strings.TrimSpace(request.IdempotencyKey)
	if idempotency == "" {
		idempotency = fmt.Sprintf("replay:%d:%d:%d", request.EvidenceWindowStart.UnixNano(), request.EvidenceWindowEnd.UnixNano(), request.EvidenceLimit)
	}
	reference := strings.TrimSpace(request.Actor) + ":" + strings.TrimSpace(request.Reason)
	if len([]byte(reference)) > workqueue.MaxReferenceBytes {
		reference = reference[:workqueue.MaxReferenceBytes]
	}
	return d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: request.Scope.Normalized(), Kind: workqueue.WorkKindInsightMaintenance, Watermark: watermark, Idempotency: idempotency, Reference: reference},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
}
