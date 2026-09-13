package jobs

import (
	"context"
	"time"
)

type MaintenanceLeaseInput struct {
	Identity   MaintenanceIdentity
	WorkerID   string
	Now        time.Time
	LeaseUntil time.Time
	Attempt    int
	Checkpoint string
	Watermark  string
}

type MaintenanceCompletion struct {
	Identity       MaintenanceIdentity
	WorkerID       string
	FinishedAt     time.Time
	ProcessedCount int
	Disposition    MaintenanceExecutionDisposition
	ErrorCategory  string
}

type MaintenanceFailure struct {
	Identity      MaintenanceIdentity
	WorkerID      string
	FailedAt      time.Time
	NextAttemptAt time.Time
	ErrorCategory string
	Checkpoint    string
	Watermark     string
}

type MaintenanceExecutionStore interface {
	AcquireMaintenanceLease(ctx context.Context, input MaintenanceLeaseInput) (bool, error)
	RenewMaintenanceLease(ctx context.Context, input MaintenanceLeaseInput) error
	CompleteMaintenanceExecution(ctx context.Context, input MaintenanceCompletion) error
	FailMaintenanceExecution(ctx context.Context, input MaintenanceFailure) error
}
