package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type DurableMaintenanceJob struct {
	Job      MaintenanceJob
	Store    MaintenanceExecutionStore
	Scope    memory.Scope
	WorkerID string
	Cadence  time.Duration
	Now      func() time.Time
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
	lease := MaintenanceLeaseInput{Identity: identity, WorkerID: j.WorkerID, Now: now, LeaseUntil: now.Add(cadence), Attempt: 1}
	acquired, err := j.Store.AcquireMaintenanceLease(ctx, lease)
	if err != nil || !acquired {
		return 0, err
	}
	processed, runErr := j.Job.Run(ctx)
	if runErr != nil {
		failErr := j.Store.FailMaintenanceExecution(ctx, MaintenanceFailure{Identity: identity, WorkerID: j.WorkerID, FailedAt: now, ErrorCategory: "job_failed"})
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
