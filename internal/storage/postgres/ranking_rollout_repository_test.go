package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreateActivateRollbackRankingRolloutPolicy(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 30, 0, 0, time.UTC)
	policy := memory.RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           scope,
		Status:          memory.RankingRolloutPolicyStatusDraft,
		Mode:            memory.RankingRolloutModeDryRun,
		Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch},
		SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusInsufficient,
		EvidenceMinimum: 2,
		Actor:           "operator-a",
		Reason:          "initial rollout",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO ranking_rollout_policies").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, policy.Status, policy.Mode, []string{"search"}, []string{"task_evaluations"}, policy.ThresholdStatus, policy.EvidenceMinimum, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, policy.CreatedAt, policy.UpdatedAt).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).
			AddRow(policy.ID, scope.Tenant, scope.Project, scope.Namespace, policy.Status, policy.Mode, []string{"search"}, []string{"task_evaluations"}, policy.ThresholdStatus, policy.EvidenceMinimum, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, policy.Status, policy.Actor, policy.Reason, nil, nil, nil, policy.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	created, err := repo.CreateRankingRolloutPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("CreateRankingRolloutPolicy() error = %v", err)
	}
	if created.ID != policy.ID {
		t.Fatalf("created.ID = %q, want %q", created.ID, policy.ID)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID, memory.RankingRolloutPolicyStatusActiveForScope, "operator-b", "activate after dry-run", now.Add(time.Minute), memory.RankingRolloutThresholdStatusSatisfied, memory.RankingRolloutModeActiveForScope, memory.RankingRolloutPolicyStatusDisabled, memory.RankingRolloutPolicyStatusRolledBack).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).
			AddRow(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, memory.RankingRolloutModeActiveForScope, []string{"search"}, []string{"task_evaluations"}, memory.RankingRolloutThresholdStatusSatisfied, policy.EvidenceMinimum, "operator-b", "activate after dry-run", "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now.Add(time.Minute), nil, nil, now, now.Add(time.Minute)))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, "operator-b", "activate after dry-run", now.Add(time.Minute), nil, nil, now.Add(time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	activated, err := repo.ActivateRankingRolloutPolicy(context.Background(), memory.ActivateRankingRolloutPolicyInput{
		Scope:       scope,
		PolicyID:    policy.ID,
		Actor:       "operator-b",
		Reason:      "activate after dry-run",
		ActivatedAt: now.Add(time.Minute),
		Gate: memory.RankingRolloutActivationGate{
			DryRunSucceeded:        true,
			EvidenceThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
			AttributionRecorded:    true,
		},
	})
	if err != nil {
		t.Fatalf("ActivateRankingRolloutPolicy() error = %v", err)
	}
	if activated.Status != memory.RankingRolloutPolicyStatusActiveForScope {
		t.Fatalf("activated.Status = %q, want active_for_scope", activated.Status)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies[\\s\\S]*FOR UPDATE").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).
			AddRow(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, policy.Mode, []string{"search"}, []string{"task_evaluations"}, memory.RankingRolloutThresholdStatusSatisfied, policy.EvidenceMinimum, "operator-b", "activate after dry-run", "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now.Add(time.Minute), nil, nil, now, now.Add(time.Minute)))
	mock.ExpectQuery("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(2*time.Minute)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).
			AddRow(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusRolledBack, policy.Mode, []string{"search"}, []string{"task_evaluations"}, policy.ThresholdStatus, policy.EvidenceMinimum, "operator-c", "rollback degraded ranking", nil, nil, now.Add(time.Minute), nil, now.Add(2*time.Minute), now, now.Add(2*time.Minute)))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(time.Minute), nil, now.Add(2*time.Minute), now.Add(2*time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO ranking_rollout_rollback_audit").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(2*time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	rolledBack, err := repo.RollbackRankingRolloutPolicy(context.Background(), memory.RollbackRankingRolloutPolicyInput{
		Scope:        scope,
		PolicyID:     policy.ID,
		Actor:        "operator-c",
		Reason:       "rollback degraded ranking",
		RolledBackAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("RollbackRankingRolloutPolicy() error = %v", err)
	}
	if rolledBack.Status != memory.RankingRolloutPolicyStatusRolledBack {
		t.Fatalf("rolledBack.Status = %q, want rolled_back", rolledBack.Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRecordRankingRolloutDryRunPersistsComparisonAndImpact(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	policyID := "policy_1"

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies[\\s\\S]*FOR UPDATE").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policyID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).
			AddRow(policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, []string{"search"}, []string{"task_evaluations"}, memory.RankingRolloutThresholdStatusInsufficient, 2, "operator-a", "test rollout", nil, nil, nil, nil, nil, now, now))
	mock.ExpectQuery("INSERT INTO ranking_rollout_dry_runs").
		WithArgs(pgxmock.AnyArg(), policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, memory.RankingRolloutThresholdStatusSatisfied, 2, 1, []string{"mem_1"}, []string{"subject_boosted"}, []string{"task_evaluations"}, 2, 0, now).
		WillReturnRows(pgxmock.NewRows([]string{"id", "policy_id", "tenant", "project", "namespace", "surface", "signal_source", "threshold_status", "baseline_rank", "adjusted_rank", "changed_subject_ids", "reason_codes", "signal_categories", "evidence_count", "hidden_evidence_count", "created_at"}).
			AddRow("dry_run_1", policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, memory.RankingRolloutThresholdStatusSatisfied, 2, 1, []string{"mem_1"}, []string{"subject_boosted"}, []string{"task_evaluations"}, 2, 0, now))
	mock.ExpectExec("INSERT INTO ranking_rollout_impact_entries").
		WithArgs(pgxmock.AnyArg(), "dry_run_1", policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, []string{"task_evaluations"}, "memory", "mem_1", nil, 1, true, 1, 2, 1, memory.RankingRolloutImpactReasonCodeSubjectBoosted, 2, false, now).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policyID, "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	dryRun, err := repo.RecordRankingRolloutDryRun(context.Background(), memory.RecordRankingRolloutDryRunInput{
		PolicyID:            policyID,
		Scope:               scope,
		Surface:             memory.RankingRolloutSurfaceSearch,
		SignalSource:        memory.RankingRolloutSignalSourceTaskEvaluations,
		ThresholdStatus:     memory.RankingRolloutThresholdStatusSatisfied,
		BaselineRank:        2,
		AdjustedRank:        1,
		ChangedSubjectIDs:   []string{"mem_1"},
		ReasonCodes:         []memory.RankingRolloutImpactReasonCode{memory.RankingRolloutImpactReasonCodeSubjectBoosted},
		SignalCategories:    []string{"task_evaluations"},
		EvidenceCount:       2,
		HiddenEvidenceCount: 0,
		ImpactEntries: []memory.RankingRolloutImpactEntry{{
			SignalCategories:  []string{"task_evaluations"},
			SubjectKind:       "memory",
			SubjectID:         "mem_1",
			CandidatePriority: 1,
			Included:          true,
			BudgetImpact:      1,
			BaselineRank:      2,
			AdjustedRank:      1,
			ReasonCode:        memory.RankingRolloutImpactReasonCodeSubjectBoosted,
			EvidenceCount:     2,
		}},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("RecordRankingRolloutDryRun() error = %v", err)
	}
	if dryRun.ID != "dry_run_1" || dryRun.EvidenceCount != 2 || dryRun.ChangedSubjectIDs[0] != "mem_1" {
		t.Fatalf("dryRun = %+v, want persisted comparison", dryRun)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
