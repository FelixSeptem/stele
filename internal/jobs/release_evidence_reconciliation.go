package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type ReleaseEvidenceReconciliationStore interface {
	ListReleaseEvidenceReconciliationInputs(context.Context, memory.Scope, int) ([]retrieval.ReleaseEvidenceReconciliationInput, error)
	ApplyReleaseEvidenceReconciliation(context.Context, memory.Scope, string, retrieval.ReleaseEvidenceReconciliationResult) error
}

type ReleaseEvidenceReconciliationCheckpointStore interface {
	CheckpointReleaseEvidenceReconciliation(context.Context, memory.Scope, string, string, int) error
}

type ReleaseEvidenceReconciliationJob struct {
	Scope          memory.Scope
	Store          ReleaseEvidenceReconciliationStore
	ExecutionStore ExecutionStore
	TriggerSource  string
	Cadence        time.Duration
	Limit          int
	Now            func() time.Time
}

func (j ReleaseEvidenceReconciliationJob) Name() string { return "release_evidence_reconciliation" }

func (j ReleaseEvidenceReconciliationJob) Run(ctx context.Context) (processed int, err error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Store == nil {
		return 0, fmt.Errorf("release evidence reconciliation store is required")
	}
	now := time.Now().UTC()
	if j.Now != nil {
		now = j.Now().UTC()
	}
	window := scheduledRunWindow(now, j.Cadence)
	trigger := j.TriggerSource
	if trigger == "" {
		trigger = "scheduler"
	}
	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, trigger, window)
	if err != nil || !started {
		return 0, err
	}
	limit := j.Limit
	if limit <= 0 {
		limit = 100
	}
	inputs, err := j.Store.ListReleaseEvidenceReconciliationInputs(ctx, j.Scope, limit)
	if err != nil {
		_ = failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, now, err)
		return 0, err
	}
	for _, input := range inputs {
		if input.Now.IsZero() {
			input.Now = now
		}
		result := retrieval.ReconcileReleaseEvidence(input)
		handoff := result.HandoffIdentity
		if handoff == "" {
			handoff = "missing"
		}
		replayKey := fmt.Sprintf("%s:%s:%s:%s", idempotencyKey, handoff, input.ExpectedSourceWatermarkHash, input.ExpectedPolicyVersion)
		if err := j.Store.ApplyReleaseEvidenceReconciliation(ctx, j.Scope, replayKey, result); err != nil {
			_ = failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, now, err)
			return processed, err
		}
		processed++
		if checkpointStore, ok := j.Store.(ReleaseEvidenceReconciliationCheckpointStore); ok {
			if err := checkpointStore.CheckpointReleaseEvidenceReconciliation(ctx, j.Scope, replayKey, handoff, processed); err != nil {
				_ = failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, now, err)
				return processed, err
			}
		}
	}
	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, now, processed); err != nil {
		return processed, err
	}
	return processed, nil
}
