package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ReflectionRunExecutor interface {
	ExecuteReflection(context.Context, memory.ReflectionRun, int64) (int64, []string, []string, error)
}

type ReflectionRunWorker struct {
	Store         memory.ReflectionRunStore
	Executor      ReflectionRunExecutor
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	RetryBackoff  time.Duration
	Now           func() time.Time
}

func (w ReflectionRunWorker) RunOnce(ctx context.Context) (processed int, err error) {
	if w.Store == nil {
		return 0, fmt.Errorf("reflection run store is required")
	}
	if w.Executor == nil {
		return 0, fmt.Errorf("reflection run executor is required")
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
	claims, err := w.Store.ClaimReflectionRuns(ctx, memory.ClaimReflectionRunsInput{Scope: w.Scope, WorkerID: w.WorkerID, Now: now, LeaseDuration: lease, Limit: limit})
	if err != nil {
		return 0, err
	}
	for _, run := range claims {
		offset, candidates, evidence, execErr := w.Executor.ExecuteReflection(ctx, run, run.ProcessedOffset)
		if execErr != nil {
			category := memory.ReflectionFailureTransient
			if strings.Contains(strings.ToLower(execErr.Error()), "scope") {
				category = memory.ReflectionFailureScope
			}
			retryAt := time.Time{}
			if run.MaxAttempts <= 0 || run.Attempt < run.MaxAttempts {
				retryAt = now.Add(w.retryBackoff())
			}
			if err := w.Store.FailReflectionRun(ctx, memory.FailReflectionRunInput{Scope: run.Scope, RunID: run.ID, WorkerID: w.WorkerID, Category: category, Message: truncateError(execErr.Error(), 512), RetryAt: retryAt, FailedAt: now}); err != nil {
				return processed, err
			}
			continue
		}
		if err := w.Store.CheckpointReflectionRun(ctx, memory.CheckpointReflectionRunInput{Scope: run.Scope, RunID: run.ID, WorkerID: w.WorkerID, ProcessedOffset: offset, OutputCandidateIDs: candidates, EvidenceReferences: evidence, UpdatedAt: now}); err != nil {
			return processed, err
		}
		if err := w.Store.CompleteReflectionRun(ctx, memory.CompleteReflectionRunInput{Scope: run.Scope, RunID: run.ID, WorkerID: w.WorkerID, ProcessedOffset: offset, OutputCandidateIDs: candidates, EvidenceReferences: evidence, CompletedAt: now}); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (w ReflectionRunWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}
	return time.Minute
}
