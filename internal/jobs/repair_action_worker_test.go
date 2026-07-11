package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRepairActionWorkerCompletesClaimedAction(t *testing.T) {
	now := time.Date(2026, 7, 11, 13, 0, 0, 0, time.UTC)
	store := &stubRepairActionStore{
		claims: []memory.RepairAction{{
			ID:       "action_1",
			Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Category: memory.RepairActionCategoryEmbeddingRetry,
			Status:   memory.RepairActionStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}},
	}
	processor := &stubRepairActionProcessor{}
	worker := RepairActionWorker{
		Store:         store,
		Processor:     processor,
		Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		WorkerID:      "worker-a",
		BatchSize:     2,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 || store.completed.ActionID != "action_1" || processor.gotAction.ID != "action_1" {
		t.Fatalf("processed=%d completed=%+v processor=%+v, want action completed", processed, store.completed, processor.gotAction)
	}
}

func TestRepairActionWorkerRecordsRetryableFailure(t *testing.T) {
	now := time.Date(2026, 7, 11, 13, 0, 0, 0, time.UTC)
	store := &stubRepairActionStore{
		claims: []memory.RepairAction{{
			ID:       "action_1",
			Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Category: memory.RepairActionCategoryEmbeddingRetry,
			Status:   memory.RepairActionStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}},
	}
	worker := RepairActionWorker{
		Store:         store,
		Processor:     &stubRepairActionProcessor{err: errors.New("provider unavailable")},
		Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		WorkerID:      "worker-a",
		BatchSize:     2,
		LeaseDuration: time.Minute,
		MaxAttempts:   3,
		RetryBackoff:  5 * time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 0 || store.failed.ActionID != "action_1" || store.failed.NextAttemptAt != now.Add(5*time.Minute) {
		t.Fatalf("processed=%d failed=%+v, want retryable failure", processed, store.failed)
	}
}

func TestGovernedRepairActionProcessorRoutesEmbeddingGovernanceAndReplay(t *testing.T) {
	now := time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	embedding := &stubEmbeddingRecoveryApplier{}
	governanceRecovery := &stubGovernanceRecoveryApplier{}
	replay := &stubReplayApplier{}
	processor := GovernedRepairActionProcessor{
		Embedding:           embedding,
		Governance:          governanceRecovery,
		Replay:              replay,
		Now:                 func() time.Time { return now },
		ReplayWindow:        2 * time.Hour,
		ReplayEvidenceLimit: 25,
	}

	if err := processor.ProcessRepairAction(context.Background(), memory.RepairAction{
		ID:         "action_embedding",
		PlanID:     "plan_1",
		Scope:      scope,
		Category:   memory.RepairActionCategoryEmbeddingRetry,
		TargetID:   "mem_1",
		ReasonCode: memory.QualityFindingSemanticProjectionDegraded,
	}); err != nil {
		t.Fatalf("embedding ProcessRepairAction() error = %v", err)
	}
	if embedding.got.MemoryID != "mem_1" || embedding.got.Action != memory.EmbeddingRecoveryActionRetry {
		t.Fatalf("embedding input = %+v, want retry mem_1", embedding.got)
	}

	if err := processor.ProcessRepairAction(context.Background(), memory.RepairAction{
		ID:       "action_governance",
		PlanID:   "plan_1",
		Scope:    scope,
		Category: memory.RepairActionCategoryGovernanceRequeue,
		TargetID: "evt_1",
	}); err != nil {
		t.Fatalf("governance ProcessRepairAction() error = %v", err)
	}
	if governanceRecovery.got.RawEventID != "evt_1" || governanceRecovery.got.Action != governance.GovernanceRecoveryActionRequeue {
		t.Fatalf("governance input = %+v, want requeue evt_1", governanceRecovery.got)
	}

	if err := processor.ProcessRepairAction(context.Background(), memory.RepairAction{
		ID:       "action_replay",
		PlanID:   "plan_1",
		Scope:    scope,
		Category: memory.RepairActionCategoryInsightReplay,
	}); err != nil {
		t.Fatalf("replay ProcessRepairAction() error = %v", err)
	}
	if replay.got.IdempotencyKey != "repair_action:action_replay" || replay.got.EvidenceLimit != 25 || !replay.got.EvidenceWindowStart.Equal(now.Add(-2*time.Hour)) {
		t.Fatalf("replay input = %+v, want bounded idempotent replay request", replay.got)
	}
}

type stubRepairActionStore struct {
	claims    []memory.RepairAction
	completed memory.CompleteRepairActionInput
	failed    memory.RecordRepairActionFailureInput
}

func (s *stubRepairActionStore) ClaimRepairActions(ctx context.Context, input memory.ClaimRepairActionsInput) ([]memory.RepairAction, error) {
	return s.claims, nil
}

func (s *stubRepairActionStore) CompleteRepairAction(ctx context.Context, input memory.CompleteRepairActionInput) error {
	s.completed = input
	return nil
}

func (s *stubRepairActionStore) RecordRepairActionFailure(ctx context.Context, input memory.RecordRepairActionFailureInput) error {
	s.failed = input
	return nil
}

type stubRepairActionProcessor struct {
	gotAction memory.RepairAction
	err       error
}

type stubEmbeddingRecoveryApplier struct {
	got memory.ApplyEmbeddingRecoveryInput
}

func (s *stubEmbeddingRecoveryApplier) ApplyEmbeddingRecovery(ctx context.Context, input memory.ApplyEmbeddingRecoveryInput) (memory.EmbeddingRecoveryOutcome, error) {
	s.got = input
	return memory.EmbeddingRecoveryOutcome{}, nil
}

type stubGovernanceRecoveryApplier struct {
	got governance.ApplyGovernanceRecoveryInput
}

func (s *stubGovernanceRecoveryApplier) ApplyGovernanceRecovery(ctx context.Context, input governance.ApplyGovernanceRecoveryInput) (governance.GovernanceRecoveryOutcome, error) {
	s.got = input
	return governance.GovernanceRecoveryOutcome{}, nil
}

type stubReplayApplier struct {
	got memory.DerivedInsightReplayRequest
}

func (s *stubReplayApplier) ApplyDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayRun, error) {
	s.got = input
	return memory.DerivedInsightReplayRun{}, nil
}

func (s *stubRepairActionProcessor) ProcessRepairAction(ctx context.Context, action memory.RepairAction) error {
	s.gotAction = action
	return s.err
}
