package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
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
	Observer    interface {
		RecordProceduralVerification(context.Context, telemetry.ProceduralVerificationEvent)
	}
}

// DispatchProceduralRevalidation adapts the memory-domain request contract to
// the derived-work queue without widening its payload. The status remains an
// internal queue concern; admission gets a durable success/failure signal.
func (d ProceduralRevalidationDispatcher) DispatchProceduralRevalidation(ctx context.Context, request memory.ProceduralRevalidationDispatch) error {
	if err := request.Validate(); err != nil {
		return err
	}
	_, err := d.Dispatch(ctx, request.Scope, request.RunID, request.Watermark, request.Identity)
	return err
}

func (d ProceduralRevalidationDispatcher) Dispatch(ctx context.Context, scope memory.Scope, runID, watermark, identity string) (workqueue.BufferStatus, error) {
	if d.Queue == nil || d.Policy == nil {
		d.recordTelemetry(ctx, workqueue.BufferStatusDropped, "failed", "dependency")
		return workqueue.BufferStatusDropped, fmt.Errorf("procedural revalidation queue and policy are required")
	}
	if err := scope.Validate(); err != nil {
		d.recordTelemetry(ctx, workqueue.BufferStatusDropped, "denied", "validation")
		return workqueue.BufferStatusDropped, err
	}
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(watermark) == "" || strings.TrimSpace(identity) == "" {
		d.recordTelemetry(ctx, workqueue.BufferStatusDropped, "denied", "validation")
		return workqueue.BufferStatusDropped, fmt.Errorf("procedural revalidation run identity is required")
	}
	decision, err := d.Policy.EvaluateProceduralVerification(ctx, scope)
	if err != nil {
		d.recordTelemetry(ctx, workqueue.BufferStatusDropped, "failed", "dependency")
		return workqueue.BufferStatusDropped, fmt.Errorf("evaluate procedural verification policy: %w", err)
	}
	if !decision.Enabled {
		d.recordTelemetry(ctx, workqueue.BufferStatusDropped, "disabled", "policy_disabled")
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
	status, enqueueErr := d.Queue.Enqueue(ctx, workqueue.EnqueueInput{
		DerivedWorkInput: workqueue.DerivedWorkInput{Scope: scope, Kind: workqueue.WorkKindProceduralRevalidation, Watermark: watermark, Idempotency: identity, Reference: runID},
		MaxAttempts:      maxAttempts, Now: now, DetailExpiresAt: now.Add(retention),
	})
	result, failure := "accepted", "none"
	if enqueueErr != nil {
		result, failure = "failed", "dependency"
	}
	d.recordTelemetry(ctx, status, result, failure)
	return status, enqueueErr
}

func (d ProceduralRevalidationDispatcher) recordTelemetry(ctx context.Context, status workqueue.BufferStatus, result, failureCategory string) {
	if d.Observer == nil {
		return
	}
	queue := string(status)
	if queue != "buffered" && queue != "durable" {
		queue = "none"
	}
	d.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "dispatch", Result: result, Disposition: "none", Trigger: "none", Queue: queue, Recovery: "none", FailureCategory: failureCategory,
	})
}
