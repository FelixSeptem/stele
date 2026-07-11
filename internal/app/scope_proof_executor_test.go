package app

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
)

func TestServiceScopeProofStepExecutorChecksGovernanceStatus(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 55, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	governance := &stubProofGovernanceStatusReader{
		status: jobs.GovernanceStatus{
			PendingRawEvents:   2,
			LeasedRawEvents:    1,
			ProcessedRawEvents: 10,
			ObservedAt:         now,
		},
	}
	executor := serviceScopeProofStepExecutor{Governance: governance}

	result, err := executor.ExecuteScopeProofStep(context.Background(), memory.ScopeProofStep{
		ID:      "step_governance",
		ProofID: "proof_1",
		Scope:   scope,
		Step:    memory.ScopeProofStepGovernanceProcessed,
	})
	if err != nil {
		t.Fatalf("ExecuteScopeProofStep() error = %v", err)
	}
	if !governance.called {
		t.Fatal("governance status reader was not called")
	}
	if result.Verdict != memory.ScopeProofVerdictPassedDegraded {
		t.Fatalf("verdict = %q, want passed_degraded while backlog remains", result.Verdict)
	}
	if result.Evidence["pending_raw_events"] != int64(2) || result.Evidence["leased_raw_events"] != int64(1) {
		t.Fatalf("evidence = %+v, want governance backlog counts", result.Evidence)
	}
}

func TestServiceScopeProofStepExecutorUsesReplayQualityAndRepairServices(t *testing.T) {
	now := time.Date(2026, 7, 11, 22, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	replay := &stubProofReplayPlanner{
		report: memory.DerivedInsightReplayReport{
			RunID:       "dry_run",
			Scope:       scope,
			Counters:    memory.DerivedInsightReplayCounters{EvidenceEvaluated: 3, Created: 1},
			GeneratedAt: now,
		},
	}
	quality := &stubProofQualityService{
		evaluation: memory.QualityEvaluationRun{
			ID:     "eval_1",
			Scope:  scope,
			Status: memory.QualityEvaluationStatusCompleted,
		},
		repairPlan: memory.RepairPlan{
			ID:              "repair_plan_1",
			Scope:           scope,
			EvaluationRunID: "eval_1",
			Status:          memory.RepairPlanStatusDraft,
			DryRun:          true,
		},
	}
	executor := serviceScopeProofStepExecutor{
		Replay:  replay,
		Quality: quality,
		Now:     func() time.Time { return now },
	}

	replayResult, err := executor.ExecuteScopeProofStep(context.Background(), memory.ScopeProofStep{
		ID:      "step_replay",
		ProofID: "proof_1",
		Scope:   scope,
		Step:    memory.ScopeProofStepReplayChecked,
	})
	if err != nil {
		t.Fatalf("replay ExecuteScopeProofStep() error = %v", err)
	}
	if replay.got.Scope != scope || replay.got.Mode != memory.DerivedInsightReplayModeDryRun || replay.got.IdempotencyKey != "scope-proof:proof_1:replay" {
		t.Fatalf("replay input = %+v, want scoped dry-run idempotent replay", replay.got)
	}
	if replayResult.Evidence["replay_run_id"] != "dry_run" {
		t.Fatalf("replay evidence = %+v, want replay_run_id dry_run", replayResult.Evidence)
	}

	qualityResult, err := executor.ExecuteScopeProofStep(context.Background(), memory.ScopeProofStep{
		ID:      "step_quality",
		ProofID: "proof_1",
		Scope:   scope,
		Step:    memory.ScopeProofStepQualityEvaluated,
	})
	if err != nil {
		t.Fatalf("quality ExecuteScopeProofStep() error = %v", err)
	}
	if quality.gotEvaluation.Scope != scope || quality.gotEvaluation.ContextBudget != 1200 {
		t.Fatalf("quality input = %+v, want scoped context budget", quality.gotEvaluation)
	}
	if qualityResult.Evidence["evaluation_run_id"] != "eval_1" {
		t.Fatalf("quality evidence = %+v, want eval_1", qualityResult.Evidence)
	}

	repairResult, err := executor.ExecuteScopeProofStep(context.Background(), memory.ScopeProofStep{
		ID:       "step_repair",
		ProofID:  "proof_1",
		Scope:    scope,
		Step:     memory.ScopeProofStepRepairRecommended,
		Evidence: map[string]any{"evaluation_run_id": "eval_1"},
	})
	if err != nil {
		t.Fatalf("repair ExecuteScopeProofStep() error = %v", err)
	}
	if quality.gotRepair.EvaluationRunID != "eval_1" || !quality.gotRepair.DryRun {
		t.Fatalf("repair input = %+v, want dry-run eval_1", quality.gotRepair)
	}
	if repairResult.Evidence["repair_plan_id"] != "repair_plan_1" {
		t.Fatalf("repair evidence = %+v, want repair_plan_1", repairResult.Evidence)
	}
}

func TestServiceScopeProofStepExecutorCreatesEvaluationForRepairWhenMissing(t *testing.T) {
	now := time.Date(2026, 7, 11, 22, 5, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	quality := &stubProofQualityService{
		evaluation: memory.QualityEvaluationRun{
			ID:     "eval_generated",
			Scope:  scope,
			Status: memory.QualityEvaluationStatusCompleted,
		},
		repairPlan: memory.RepairPlan{
			ID:              "repair_plan_generated",
			Scope:           scope,
			EvaluationRunID: "eval_generated",
			Status:          memory.RepairPlanStatusDraft,
			DryRun:          true,
		},
	}
	executor := serviceScopeProofStepExecutor{
		Quality: quality,
		Now:     func() time.Time { return now },
	}

	result, err := executor.ExecuteScopeProofStep(context.Background(), memory.ScopeProofStep{
		ID:      "step_repair",
		ProofID: "proof_1",
		Scope:   scope,
		Step:    memory.ScopeProofStepRepairRecommended,
	})
	if err != nil {
		t.Fatalf("ExecuteScopeProofStep() error = %v", err)
	}
	if quality.gotEvaluation.Scope != scope || quality.gotRepair.EvaluationRunID != "eval_generated" {
		t.Fatalf("quality=%+v repair=%+v, want generated evaluation used for repair", quality.gotEvaluation, quality.gotRepair)
	}
	if result.Evidence["evaluation_run_id"] != "eval_generated" || result.Evidence["repair_plan_id"] != "repair_plan_generated" {
		t.Fatalf("evidence = %+v, want generated evaluation and repair plan", result.Evidence)
	}
}

type stubProofReplayPlanner struct {
	got    memory.DerivedInsightReplayRequest
	report memory.DerivedInsightReplayReport
	err    error
}

func (s *stubProofReplayPlanner) PlanDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayReport, error) {
	s.got = input
	return s.report, s.err
}

type stubProofGovernanceStatusReader struct {
	called bool
	status jobs.GovernanceStatus
	err    error
}

func (s *stubProofGovernanceStatusReader) ReadGovernanceStatus(ctx context.Context) (jobs.GovernanceStatus, error) {
	s.called = true
	return s.status, s.err
}

type stubProofQualityService struct {
	gotEvaluation memory.CreateQualityEvaluationInput
	gotRepair     memory.CreateRepairPlanInput
	evaluation    memory.QualityEvaluationRun
	repairPlan    memory.RepairPlan
	err           error
}

func (s *stubProofQualityService) CreateEvaluation(ctx context.Context, input memory.CreateQualityEvaluationInput) (memory.QualityEvaluationRun, error) {
	s.gotEvaluation = input
	return s.evaluation, s.err
}

func (s *stubProofQualityService) CreateRepairPlan(ctx context.Context, input memory.CreateRepairPlanInput) (memory.RepairPlan, error) {
	s.gotRepair = input
	return s.repairPlan, s.err
}
