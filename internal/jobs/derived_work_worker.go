package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type DerivedWorkStore interface {
	ClaimDerivedWork(context.Context, workqueue.ClaimInput) ([]workqueue.DerivedWorkItem, error)
	CheckpointDerivedWork(context.Context, workqueue.CheckpointInput) error
	CompleteDerivedWork(context.Context, workqueue.TerminalInput) error
	RetryDerivedWork(context.Context, workqueue.RetryInput) error
}

type DerivedWorkExecutor interface {
	ExecuteDerivedWork(context.Context, workqueue.DerivedWorkItem) (int64, string, error)
}

// DerivedWorkWorker owns the common claim and lease-safe terminal path. Kind
// specific executors only return a bounded checkpoint offset and evidence
// reference; they cannot mutate canonical memory through this interface.
type DerivedWorkWorker struct {
	Store         DerivedWorkStore
	Executor      DerivedWorkExecutor
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	RetryBackoff  time.Duration
	Kind          *workqueue.WorkKind
	Now           func() time.Time
}

func (w DerivedWorkWorker) RunOnce(ctx context.Context) (int, error) {
	if w.Store == nil || w.Executor == nil {
		return 0, fmt.Errorf("derived work store and executor are required")
	}
	if err := w.Scope.Validate(); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	limit := w.BatchSize
	if limit <= 0 {
		limit = 10
	}
	lease := w.LeaseDuration
	if lease <= 0 {
		lease = time.Minute
	}
	items, err := w.Store.ClaimDerivedWork(ctx, workqueue.ClaimInput{Scope: w.Scope, WorkerID: w.WorkerID, Now: now, LeaseDuration: lease, Limit: limit, Kind: w.Kind})
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, item := range items {
		checkpoint, evidence, execErr := w.Executor.ExecuteDerivedWork(ctx, item)
		if execErr != nil {
			retryAt := now.Add(w.RetryBackoff)
			if w.RetryBackoff <= 0 {
				retryAt = now.Add(lease)
			}
			if item.AttemptCount >= item.MaxAttempts {
				retryAt = now
			}
			if err := w.Store.RetryDerivedWork(ctx, workqueue.RetryInput{Scope: item.Scope, WorkID: item.ID, WorkerID: w.WorkerID, FailureCategory: "execution_failed", RetryAt: retryAt, FailedAt: now}); err != nil {
				return processed, err
			}
			continue
		}
		if checkpoint < 0 {
			checkpoint = 0
		}
		sequence := int64(item.AttemptCount)
		if sequence <= 0 {
			sequence = 1
		}
		if err := w.Store.CheckpointDerivedWork(ctx, workqueue.CheckpointInput{Scope: item.Scope, WorkID: item.ID, WorkerID: w.WorkerID, Sequence: sequence, ProcessedOffset: checkpoint, SourceWatermark: item.Watermark, CommittedAt: now}); err != nil {
			return processed, err
		}
		if err := w.Store.CompleteDerivedWork(ctx, workqueue.TerminalInput{Scope: item.Scope, WorkID: item.ID, WorkerID: w.WorkerID, CompletedAt: now, Disposition: "completed", CheckpointSequence: sequence, EvidenceReference: evidence}); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}
