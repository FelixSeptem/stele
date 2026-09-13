package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type DurableMaintenanceJob struct {
	Job                MaintenanceJob
	Store              MaintenanceExecutionStore
	Scope              memory.Scope
	WorkerID           string
	Cadence            time.Duration
	LeaseDuration      time.Duration
	LeaseRenewInterval time.Duration
	RetryBackoff       time.Duration
	MaxAttempts        int
	Observer           telemetry.Observer
	Now                func() time.Time
}

func (j DurableMaintenanceJob) Name() string {
	if j.Job == nil {
		return "durable_maintenance"
	}
	return j.Job.Name()
}

func (j DurableMaintenanceJob) Run(ctx context.Context) (int, error) {
	if j.Job == nil {
		return 0, fmt.Errorf("maintenance job is required")
	}
	if j.Store == nil {
		return j.Job.Run(ctx)
	}
	now := time.Now().UTC()
	if j.Now != nil {
		now = j.Now().UTC()
	}
	cadence := j.Cadence
	if cadence <= 0 {
		cadence = time.Minute
	}
	identity, err := NewMaintenanceIdentity(j.Name(), j.Scope, now.Truncate(cadence), cadence)
	if err != nil {
		return 0, err
	}
	leaseDuration := j.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = cadence
	}
	lease := MaintenanceLeaseInput{Identity: identity, WorkerID: j.WorkerID, Now: now, LeaseUntil: now.Add(leaseDuration), Attempt: 1}
	acquired, err := j.Store.AcquireMaintenanceLease(ctx, lease)
	if err != nil || !acquired {
		j.recordTelemetry(ctx, "duplicate", "none")
		return 0, err
	}
	state := MaintenanceExecutionState{Identity: identity, WorkerID: j.WorkerID, Attempt: 1, LeaseUntil: lease.LeaseUntil}
	if reader, ok := j.Store.(interface {
		ReadOwnedMaintenanceExecution(context.Context, MaintenanceIdentity, string) (MaintenanceExecutionState, error)
	}); ok {
		if persisted, readErr := reader.ReadOwnedMaintenanceExecution(ctx, identity, j.WorkerID); readErr == nil {
			state = persisted
			if state.Attempt < 1 {
				state.Attempt = 1
			}
		}
	}
	lease.Attempt = state.Attempt
	lease.Checkpoint = state.Checkpoint
	lease.Watermark = state.SourceWatermark
	j.recordTelemetry(ctx, "success", "acquired")
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	renewErrors := make(chan error, 1)
	renewDone := make(chan struct{})
	renewInterval := j.LeaseRenewInterval
	if renewInterval <= 0 {
		renewInterval = leaseDuration / 2
	}
	if renewInterval <= 0 {
		renewInterval = time.Second
	}
	go func() {
		defer close(renewDone)
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case tick := <-ticker.C:
				renew := lease
				renew.Now = tick.UTC()
				renew.LeaseUntil = renew.Now.Add(leaseDuration)
				if err := j.Store.RenewMaintenanceLease(runCtx, renew); err != nil {
					select {
					case renewErrors <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	processed, runErr := j.Job.Run(runCtx)
	cancel()
	<-renewDone
	select {
	case renewErr := <-renewErrors:
		if runErr == nil || runErr == context.Canceled {
			runErr = renewErr
		}
	default:
	}
	if runErr != nil {
		backoff := j.RetryBackoff
		if backoff <= 0 {
			backoff = leaseDuration
		}
		disposition := MaintenanceDispositionFailed
		nextAttempt := now.Add(backoff)
		if j.MaxAttempts > 0 && state.Attempt >= j.MaxAttempts {
			disposition = MaintenanceDispositionExhausted
			nextAttempt = time.Time{}
		}
		failErr := j.Store.FailMaintenanceExecution(ctx, MaintenanceFailure{Identity: identity, WorkerID: j.WorkerID, FailedAt: now, NextAttemptAt: nextAttempt, ErrorCategory: "job_failed", Checkpoint: state.Checkpoint, Watermark: state.SourceWatermark, Disposition: disposition})
		if failErr != nil {
			return processed, failErr
		}
		return processed, runErr
	}
	if err := j.Store.CompleteMaintenanceExecution(ctx, MaintenanceCompletion{Identity: identity, WorkerID: j.WorkerID, FinishedAt: now, ProcessedCount: processed, Disposition: MaintenanceDispositionCompleted}); err != nil {
		return processed, err
	}
	return processed, nil
}

func (j DurableMaintenanceJob) recordTelemetry(ctx context.Context, outcome, leaseOutcome string) {
	if observer, ok := j.Observer.(interface {
		RecordMaintenance(context.Context, telemetry.MaintenanceEvent)
	}); ok {
		observer.RecordMaintenance(ctx, telemetry.MaintenanceEvent{JobClass: j.Name(), Outcome: outcome, LeaseOutcome: leaseOutcome, Freshness: "unknown", SLO: "unknown", LatencyBucket: "unknown", CandidateBucket: "unknown"})
	}
}
