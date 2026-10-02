package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// ProjectionWorkDispatcher submits rebuild requests to the unified derived
// queue. The payload is intentionally limited to the source watermark and
// renderer/policy identity; source content is loaded by the worker.
type ProjectionWorkDispatcher struct {
	Queue       workqueue.QueueAdapter
	Now         func() time.Time
	MaxAttempts int
	Retention   time.Duration
}

func (d ProjectionWorkDispatcher) Dispatch(ctx context.Context, request memory.ContextProjectionRebuildRequest, sourceWatermark string) (workqueue.BufferStatus, error) {
	if d.Queue == nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("projection work queue is required")
	}
	if err := request.Scope.Validate(); err != nil {
		return workqueue.BufferStatusDropped, err
	}
	if !request.Kind.Valid() {
		return workqueue.BufferStatusDropped, fmt.Errorf("projection kind %q is invalid", request.Kind)
	}
	if strings.TrimSpace(sourceWatermark) == "" {
		return workqueue.BufferStatusDropped, fmt.Errorf("projection source watermark is required")
	}
	if strings.TrimSpace(request.Policy.Version) == "" || strings.TrimSpace(request.RendererVersion) == "" {
		return workqueue.BufferStatusDropped, fmt.Errorf("projection policy and renderer versions are required")
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
	// Policy and renderer are part of the deterministic identity so a rebuild
	// under a new renderer cannot resolve to an older result.
	idempotency := fmt.Sprintf("%s:%s:%s", request.Kind, request.Policy.Version, request.RendererVersion)
	reference := fmt.Sprintf("%s:%s:%s", request.SchemaVersion, request.Policy.Version, request.RendererVersion)
	return d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: request.Scope.Normalized(), Kind: workqueue.WorkKindProjectionRebuild, Watermark: sourceWatermark, Idempotency: idempotency, Reference: reference},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
}
