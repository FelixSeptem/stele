package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

// ProceduralRevalidationDispatchRecoveryStore exposes only pending durable
// runs. It never grants the recovery job evidence or canonical-write access.
type ProceduralRevalidationDispatchRecoveryStore interface {
	ListPendingProceduralRevalidationRuns(context.Context, memory.ListPendingProceduralRevalidationRunsInput) ([]memory.ProceduralRevalidationRun, error)
}

// ProceduralRevalidationDispatchRecovery re-enqueues pending durable runs
// after a process restart or a non-durable queue delivery loss. The queue
// receives the same identity as the original request, making repeats safe.
type ProceduralRevalidationDispatchRecovery struct {
	Store      ProceduralRevalidationDispatchRecoveryStore
	Dispatcher memory.ProceduralRevalidationDispatchSink
	Scope      memory.Scope
	BatchSize  int
	Now        func() time.Time
	Observer   interface {
		RecordProceduralVerification(context.Context, telemetry.ProceduralVerificationEvent)
	}
}

func (j ProceduralRevalidationDispatchRecovery) RunOnce(ctx context.Context) (int, error) {
	if j.Store == nil || j.Dispatcher == nil {
		j.recordTelemetry(ctx, "failed", "none", "dependency")
		return 0, fmt.Errorf("procedural revalidation dispatch recovery is not configured")
	}
	if err := j.Scope.Validate(); err != nil {
		j.recordTelemetry(ctx, "denied", "none", "validation")
		return 0, err
	}
	now := time.Now().UTC()
	if j.Now != nil {
		now = j.Now().UTC()
	}
	limit := j.BatchSize
	if limit <= 0 {
		limit = 10
	}
	runs, err := j.Store.ListPendingProceduralRevalidationRuns(ctx, memory.ListPendingProceduralRevalidationRunsInput{
		Scope: j.Scope, Now: now, Limit: limit,
	})
	if err != nil {
		j.recordTelemetry(ctx, "failed", "none", "dependency")
		return 0, fmt.Errorf("list pending procedural revalidation runs: %w", err)
	}
	dispatched := 0
	for _, run := range runs {
		if run.Scope.Normalized() != j.Scope.Normalized() || run.Status != memory.ProceduralRevalidationRunStatusPending {
			j.recordTelemetry(ctx, "denied", "none", "scope_denied")
			return dispatched, fmt.Errorf("pending procedural revalidation run is outside recovery scope or state")
		}
		if err := j.Dispatcher.DispatchProceduralRevalidation(ctx, memory.ProceduralRevalidationDispatch{
			Scope: run.Scope, RunID: run.ID, Watermark: run.SourceWatermark, Identity: run.WorkIdentity,
		}); err != nil {
			j.recordTelemetry(ctx, "failed", "redelivered", "dependency")
			return dispatched, fmt.Errorf("redispatch procedural revalidation run: %w", err)
		}
		j.recordTelemetry(ctx, "accepted", "redelivered", "none")
		dispatched++
	}
	return dispatched, nil
}

func (j ProceduralRevalidationDispatchRecovery) recordTelemetry(ctx context.Context, result, recovery, failureCategory string) {
	if j.Observer == nil {
		return
	}
	j.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "recover", Result: result, Disposition: "none", Trigger: "none", Queue: "recovery", Recovery: recovery, FailureCategory: failureCategory,
	})
}
