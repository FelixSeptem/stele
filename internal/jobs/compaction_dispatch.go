package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type CompactionWorkDispatcher struct {
	Queue       workqueue.QueueAdapter
	Now         func() time.Time
	MaxAttempts int
	Retention   time.Duration
}

func (d CompactionWorkDispatcher) enqueue(ctx context.Context, kind workqueue.WorkKind, scope memory.Scope, watermark, reference, idempotency string) (workqueue.BufferStatus, error) {
	if d.Queue == nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("compaction work queue is required")
	}
	now := time.Now().UTC()
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
	return d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: scope, Kind: kind, Watermark: watermark, Idempotency: idempotency, Reference: reference},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
}

func (d CompactionWorkDispatcher) DispatchCompaction(ctx context.Context, scope memory.Scope, watermark, requestID string) (workqueue.BufferStatus, error) {
	if strings.TrimSpace(requestID) == "" {
		return workqueue.BufferStatusDropped, fmt.Errorf("compaction request id is required")
	}
	return d.enqueue(ctx, workqueue.WorkKindCompaction, scope, watermark, requestID, requestID)
}

func (d CompactionWorkDispatcher) DispatchFollowUpReflection(ctx context.Context, scope memory.Scope, watermark, evidenceID, summaryVersion string) (workqueue.BufferStatus, error) {
	if strings.TrimSpace(evidenceID) == "" || strings.TrimSpace(summaryVersion) == "" {
		return workqueue.BufferStatusDropped, fmt.Errorf("compaction evidence and summary version are required")
	}
	idempotency := "compaction-follow-up:" + evidenceID + ":" + summaryVersion
	return d.enqueue(ctx, workqueue.WorkKindReflection, scope, watermark, evidenceID, idempotency)
}
