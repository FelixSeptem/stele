package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// ProceduralRevalidationDispatcher deliberately accepts a durable run ID only.
// The worker must re-read it in scope; raw evidence and corrected content never
// travel through the derived work queue.
type ProceduralRevalidationDispatcher struct {
	Queue       workqueue.QueueAdapter
	Policy      memory.ProceduralVerificationPolicy
	Now         func() time.Time
	MaxAttempts int
	Retention   time.Duration
}

func (d ProceduralRevalidationDispatcher) Dispatch(ctx context.Context, scope memory.Scope, runID, watermark, identity string) (workqueue.BufferStatus, error) {
	if d.Queue == nil || d.Policy == nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("procedural revalidation queue and policy are required")
	}
	if err := scope.Validate(); err != nil {
		return workqueue.BufferStatusDropped, err
	}
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(watermark) == "" || strings.TrimSpace(identity) == "" {
		return workqueue.BufferStatusDropped, fmt.Errorf("procedural revalidation run identity is required")
	}
	decision, err := d.Policy.EvaluateProceduralVerification(ctx, scope)
	if err != nil {
		return workqueue.BufferStatusDropped, fmt.Errorf("evaluate procedural verification policy: %w", err)
	}
	if !decision.Enabled {
		return workqueue.BufferStatusDropped, fmt.Errorf("procedural verification policy is disabled")
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
		retention = 30 * 24 * time.Hour
	}
	return d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: scope, Kind: workqueue.WorkKindProceduralRevalidation, Watermark: watermark, Idempotency: identity, Reference: runID},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
}
