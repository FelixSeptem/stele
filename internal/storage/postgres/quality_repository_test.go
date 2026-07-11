package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesAndReadsQualityEvaluationRunWithScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	run := memory.QualityEvaluationRun{
		ID:        "eval_1",
		Scope:     scope,
		Status:    memory.QualityEvaluationStatusPending,
		Checks:    []memory.QualityEvaluationCheck{memory.QualityEvaluationCheckRetrieval},
		Actor:     "operator-a",
		Reason:    "baseline",
		CreatedAt: now,
		UpdatedAt: now,
	}

	mock.ExpectQuery("INSERT INTO quality_evaluation_runs").
		WithArgs(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, []string{"retrieval"}, run.Actor, run.Reason, run.CreatedAt, run.UpdatedAt, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "checks", "actor", "reason", "created_at", "updated_at", "started_at", "finished_at"}).
			AddRow(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, []string{"retrieval"}, run.Actor, run.Reason, now, now, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM quality_evaluation_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "checks", "actor", "reason", "created_at", "updated_at", "started_at", "finished_at"}).
			AddRow(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, []string{"retrieval"}, run.Actor, run.Reason, now, now, nil, nil))

	repo := NewRepository(mock)
	created, err := repo.CreateQualityEvaluationRun(context.Background(), run)
	if err != nil {
		t.Fatalf("CreateQualityEvaluationRun() error = %v", err)
	}
	if created.ID != run.ID {
		t.Fatalf("created id = %q, want %q", created.ID, run.ID)
	}

	read, err := repo.ReadQualityEvaluationRun(context.Background(), memory.ReadQualityEvaluationRunInput{
		Scope:           scope,
		EvaluationRunID: run.ID,
	})
	if err != nil {
		t.Fatalf("ReadQualityEvaluationRun() error = %v", err)
	}
	if read.Scope != scope {
		t.Fatalf("read scope = %+v, want %+v", read.Scope, scope)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreatesRepairPlanAndClaimsAction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 12, 30, 0, 0, time.UTC)
	plan := memory.RepairPlan{
		ID:                 "plan_1",
		Scope:              scope,
		EvaluationRunID:    "eval_1",
		BaselineRunID:      "eval_1",
		Status:             memory.RepairPlanStatusDraft,
		VerificationStatus: memory.RepairVerificationStatusPending,
		Actor:              "operator-a",
		Reason:             "repair",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	action := memory.RepairAction{
		ID:              "action_1",
		PlanID:          plan.ID,
		EvaluationRunID: plan.EvaluationRunID,
		FindingID:       "finding_1",
		Scope:           scope,
		Category:        memory.RepairActionCategoryEmbeddingRetry,
		Status:          memory.RepairActionStatusPending,
		ReasonCode:      memory.QualityFindingSemanticProjectionDegraded,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	mock.ExpectQuery("INSERT INTO repair_plans").
		WithArgs(plan.ID, scope.Tenant, scope.Project, scope.Namespace, plan.EvaluationRunID, plan.BaselineRunID, nil, plan.Status, string(plan.VerificationStatus), plan.DryRun, plan.Actor, plan.Reason, plan.CreatedAt, plan.UpdatedAt, nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "evaluation_run_id", "baseline_run_id", "verification_run_id", "status", "verification_status", "dry_run", "actor", "reason", "created_at", "updated_at", "approved_at", "completed_at"}).
			AddRow(plan.ID, scope.Tenant, scope.Project, scope.Namespace, plan.EvaluationRunID, plan.BaselineRunID, nil, plan.Status, plan.VerificationStatus, false, plan.Actor, plan.Reason, now, now, nil, nil))
	mock.ExpectQuery("INSERT INTO repair_actions").
		WithArgs(action.ID, action.PlanID, action.EvaluationRunID, action.FindingID, scope.Tenant, scope.Project, scope.Namespace, action.Category, action.Status, nil, nil, string(action.ReasonCode), 0, nil, nil, nil, nil, action.CreatedAt, action.UpdatedAt, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "plan_id", "evaluation_run_id", "finding_id", "tenant", "project", "namespace", "category", "status", "target_kind", "target_id", "reason_code", "attempt", "worker_id", "lease_until", "last_error", "next_attempt_at", "created_at", "updated_at", "completed_at"}).
			AddRow(action.ID, action.PlanID, action.EvaluationRunID, action.FindingID, scope.Tenant, scope.Project, scope.Namespace, action.Category, action.Status, nil, nil, action.ReasonCode, 0, nil, nil, nil, nil, now, now, nil))
	mock.ExpectQuery("WITH claimed AS").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "worker-a", now, now.Add(time.Minute), 2, memory.RepairActionStatusRunning, memory.RepairActionStatusPending, memory.RepairActionStatusFailed, memory.RepairPlanStatusApproved).
		WillReturnRows(pgxmock.NewRows([]string{"id", "plan_id", "evaluation_run_id", "finding_id", "tenant", "project", "namespace", "category", "status", "target_kind", "target_id", "reason_code", "attempt", "worker_id", "lease_until", "last_error", "next_attempt_at", "created_at", "updated_at", "completed_at"}).
			AddRow(action.ID, action.PlanID, action.EvaluationRunID, action.FindingID, scope.Tenant, scope.Project, scope.Namespace, action.Category, memory.RepairActionStatusRunning, nil, nil, action.ReasonCode, 1, "worker-a", now.Add(time.Minute), nil, nil, now, now, nil))

	repo := NewRepository(mock)
	if _, err := repo.CreateRepairPlan(context.Background(), plan); err != nil {
		t.Fatalf("CreateRepairPlan() error = %v", err)
	}
	if _, err := repo.CreateRepairAction(context.Background(), action); err != nil {
		t.Fatalf("CreateRepairAction() error = %v", err)
	}
	claimed, err := repo.ClaimRepairActions(context.Background(), memory.ClaimRepairActionsInput{
		Scope:         scope,
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: time.Minute,
		Limit:         2,
	})
	if err != nil {
		t.Fatalf("ClaimRepairActions() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].Status != memory.RepairActionStatusRunning || claimed[0].WorkerID != "worker-a" {
		t.Fatalf("claimed = %+v, want running action for worker-a", claimed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
