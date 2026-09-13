package jobs

import (
	"context"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
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
	Disposition   MaintenanceExecutionDisposition
}

type MaintenanceHistoryCursor struct {
	StartedAt time.Time
	ID        string
}

type MaintenanceHistoryPage struct {
	Records    []JobExecutionRecord
	NextCursor *MaintenanceHistoryCursor
}

type MaintenanceHistoryReader interface {
	ListMaintenanceExecutionHistory(ctx context.Context, scope memory.Scope, limit int, cursor *MaintenanceHistoryCursor) (MaintenanceHistoryPage, error)
}

type MaintenanceExecutionStore interface {
	AcquireMaintenanceLease(ctx context.Context, input MaintenanceLeaseInput) (bool, error)
	RenewMaintenanceLease(ctx context.Context, input MaintenanceLeaseInput) error
	ReclaimMaintenanceLease(ctx context.Context, input MaintenanceLeaseInput) (bool, error)
	CompleteMaintenanceExecution(ctx context.Context, input MaintenanceCompletion) error
	FailMaintenanceExecution(ctx context.Context, input MaintenanceFailure) error
}
