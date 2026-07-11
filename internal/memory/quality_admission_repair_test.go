package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEvaluateAdmissionPressureReturnsDegradedQueueAndReject(t *testing.T) {
	observedAt := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	evaluator := AdmissionPressureEvaluator{
		MaxPending: 10,
		MaxLeased:  5,
	}

	normal := evaluator.Evaluate(AdmissionPressureInput{
		Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Operation:  AdmissionPressureOperationIngest,
		Snapshot:   AdmissionPressureSnapshot{IntentWritable: true, PendingGovernance: 2, LeasedGovernance: 1},
		ObservedAt: observedAt,
	})
	if normal.Decision != AdmissionPressureDecisionAccept {
		t.Fatalf("normal decision = %q, want accept", normal.Decision)
	}

	degraded := evaluator.Evaluate(AdmissionPressureInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Operation: AdmissionPressureOperationIngest,
		Snapshot: AdmissionPressureSnapshot{
			IntentWritable:             true,
			PendingGovernance:          2,
			SemanticProjectionDegraded: true,
		},
		ObservedAt: observedAt,
	})
	if degraded.Decision != AdmissionPressureDecisionAcceptDegraded || !hasQualityFindingCode(degraded.Findings, QualityFindingSemanticProjectionDegraded) {
		t.Fatalf("degraded report = %+v, want accept_degraded with semantic projection finding", degraded)
	}

	queued := evaluator.Evaluate(AdmissionPressureInput{
		Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Operation:  AdmissionPressureOperationRepair,
		Snapshot:   AdmissionPressureSnapshot{IntentWritable: true, PendingGovernance: 11, LeasedGovernance: 2},
		ObservedAt: observedAt,
	})
	if queued.Decision != AdmissionPressureDecisionQueue || !hasQualityFindingCode(queued.Findings, QualityFindingGovernanceBacklogHigh) {
		t.Fatalf("queued report = %+v, want queue with governance backlog finding", queued)
	}

	rejected := evaluator.Evaluate(AdmissionPressureInput{
		Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Operation:  AdmissionPressureOperationRepair,
		Snapshot:   AdmissionPressureSnapshot{IntentWritable: false},
		ObservedAt: observedAt,
	})
	if rejected.Decision != AdmissionPressureDecisionReject || !hasQualityFindingCode(rejected.Findings, QualityFindingIntentNotWritable) {
		t.Fatalf("rejected report = %+v, want reject with intent_not_writable", rejected)
	}
}

func TestQualityServiceCreatesEvaluationAndRepairPlan(t *testing.T) {
	now := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	store := &stubQualityStore{
		createdEvaluation: QualityEvaluationRun{
			ID:        "eval_1",
			Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status:    QualityEvaluationStatusCompleted,
			Checks:    []QualityEvaluationCheck{QualityEvaluationCheckRetrieval, QualityEvaluationCheckAdmissionPressure},
			CreatedAt: now,
			UpdatedAt: now,
		},
		findings: []QualityEvaluationFinding{
			{
				ID:                      "finding_1",
				EvaluationRunID:         "eval_1",
				Scope:                   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Code:                    QualityFindingSemanticProjectionDegraded,
				Severity:                QualityFindingSeverityWarning,
				Component:               QualityFindingComponentEmbedding,
				SuggestedActionCategory: RepairActionCategoryEmbeddingRetry,
				Evidence:                map[string]any{"memory_id": "mem_1"},
				CreatedAt:               now,
			},
			{
				ID:                      "finding_2",
				EvaluationRunID:         "eval_1",
				Scope:                   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Code:                    QualityFindingGovernanceBacklogHigh,
				Severity:                QualityFindingSeverityWarning,
				Component:               QualityFindingComponentGovernance,
				SuggestedActionCategory: RepairActionCategoryGovernanceRequeue,
				Evidence:                map[string]any{"raw_event_id": "evt_1"},
				CreatedAt:               now,
			},
			{
				ID:              "finding_3",
				EvaluationRunID: "eval_1",
				Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Code:            QualityFindingUnsupportedAutomaticRepair,
				Severity:        QualityFindingSeverityWarning,
				Component:       QualityFindingComponentRetrieval,
				CreatedAt:       now,
			},
		},
	}
	service := NewQualityService(QualityServiceOptions{
		Store:        store,
		Now:          func() time.Time { return now },
		NewID:        func(prefix string) string { return prefix + "_1" },
		MaxPlanItems: 10,
	})

	run, err := service.CreateEvaluation(context.Background(), CreateQualityEvaluationInput{
		Scope:  Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Checks: []QualityEvaluationCheck{QualityEvaluationCheckRetrieval, QualityEvaluationCheckAdmissionPressure},
		Actor:  "operator-a",
	})
	if err != nil {
		t.Fatalf("CreateEvaluation() error = %v", err)
	}
	if run.ID != "eval_1" {
		t.Fatalf("evaluation id = %q, want eval_1", run.ID)
	}

	plan, err := service.CreateRepairPlan(context.Background(), CreateRepairPlanInput{
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		EvaluationRunID: "eval_1",
		Actor:           "operator-a",
		Reason:          "repair degraded projection",
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("CreateRepairPlan() error = %v", err)
	}
	if plan.ID == "" || len(plan.Actions) != 3 {
		t.Fatalf("plan = %+v, want id and three actions", plan)
	}
	if plan.Actions[0].Category != RepairActionCategoryEmbeddingRetry || plan.Actions[0].TargetKind != "memory" || plan.Actions[0].TargetID != "mem_1" {
		t.Fatalf("first action = %+v, want embedding retry targeting mem_1", plan.Actions[0])
	}
	if plan.Actions[1].Category != RepairActionCategoryGovernanceRequeue || plan.Actions[1].TargetKind != "raw_event" || plan.Actions[1].TargetID != "evt_1" {
		t.Fatalf("second action = %+v, want governance requeue targeting evt_1", plan.Actions[1])
	}
	if plan.Actions[2].Category != RepairActionCategoryManualReview {
		t.Fatalf("third action category = %q, want manual_review", plan.Actions[2].Category)
	}
}

func TestQualityServiceRejectsCanonicalRewriteRepair(t *testing.T) {
	now := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)
	service := NewQualityService(QualityServiceOptions{
		Store: &stubQualityStore{findings: []QualityEvaluationFinding{
			{
				ID:                      "finding_1",
				EvaluationRunID:         "eval_1",
				Scope:                   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Code:                    QualityFindingCanonicalRewriteRequired,
				Severity:                QualityFindingSeverityBlocker,
				Component:               QualityFindingComponentLifecycle,
				SuggestedActionCategory: RepairActionCategoryCanonicalRewrite,
				CreatedAt:               now,
			},
		}},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	_, err := service.CreateRepairPlan(context.Background(), CreateRepairPlanInput{
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		EvaluationRunID: "eval_1",
		Actor:           "operator-a",
		Reason:          "rewrite",
	})
	if !errors.Is(err, ErrRepairActionRejected) {
		t.Fatalf("CreateRepairPlan() error = %v, want ErrRepairActionRejected", err)
	}
}

func hasQualityFindingCode(findings []QualityFinding, code QualityFindingCode) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

type stubQualityStore struct {
	createdEvaluation QualityEvaluationRun
	findings          []QualityEvaluationFinding
	plans             []RepairPlan
	actions           []RepairAction
}

func (s *stubQualityStore) CreateQualityEvaluationRun(ctx context.Context, run QualityEvaluationRun) (QualityEvaluationRun, error) {
	if s.createdEvaluation.ID != "" {
		return s.createdEvaluation, nil
	}
	return run, nil
}

func (s *stubQualityStore) CreateQualityEvaluationFinding(ctx context.Context, finding QualityEvaluationFinding) (QualityEvaluationFinding, error) {
	s.findings = append(s.findings, finding)
	return finding, nil
}

func (s *stubQualityStore) ReadQualityEvaluationRun(ctx context.Context, input ReadQualityEvaluationRunInput) (QualityEvaluationRun, error) {
	return QualityEvaluationRun{
		ID:     input.EvaluationRunID,
		Scope:  input.Scope,
		Status: QualityEvaluationStatusCompleted,
	}, nil
}

func (s *stubQualityStore) ListQualityEvaluationFindings(ctx context.Context, input ListQualityEvaluationFindingsInput) ([]QualityEvaluationFinding, error) {
	return s.findings, nil
}

func (s *stubQualityStore) CreateRepairPlan(ctx context.Context, plan RepairPlan) (RepairPlan, error) {
	s.plans = append(s.plans, plan)
	return plan, nil
}

func (s *stubQualityStore) CreateRepairAction(ctx context.Context, action RepairAction) (RepairAction, error) {
	s.actions = append(s.actions, action)
	return action, nil
}

func (s *stubQualityStore) ReadRepairPlan(ctx context.Context, input ReadRepairPlanInput) (RepairPlan, error) {
	return RepairPlan{ID: input.RepairPlanID, Scope: input.Scope, Status: RepairPlanStatusDraft}, nil
}

func (s *stubQualityStore) ApproveRepairPlan(ctx context.Context, input ApproveRepairPlanInput) (RepairPlan, error) {
	return RepairPlan{ID: input.RepairPlanID, Scope: input.Scope, Status: RepairPlanStatusApproved, Actor: input.Actor, Reason: input.Reason, ApprovedAt: input.ApprovedAt}, nil
}

func (s *stubQualityStore) UpdateRepairPlanVerification(ctx context.Context, input UpdateRepairPlanVerificationInput) (RepairPlan, error) {
	return RepairPlan{ID: input.RepairPlanID, Scope: input.Scope, VerificationRunID: input.VerificationRunID, VerificationStatus: input.VerificationStatus}, nil
}

func (s *stubQualityStore) ReadQualityDiagnostics(ctx context.Context, input ReadQualityDiagnosticsInput) (QualityDiagnostics, error) {
	return QualityDiagnostics{Scope: input.Scope, EvaluationStatus: map[string]int64{"completed": 1}}, nil
}
