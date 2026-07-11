package memory

import (
	"context"
	"testing"
	"time"
)

func TestScopeProofServiceCreatesListsReadsReportsAndReruns(t *testing.T) {
	now := time.Date(2026, 7, 11, 19, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubScopeProofStore{}
	service := NewScopeProofService(ScopeProofServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string {
			if prefix == "proof" && len(store.createdRuns) == 0 {
				return "proof_1"
			}
			if prefix == "proof" {
				return "proof_2"
			}
			return prefix + "_1"
		},
	})

	created, err := service.CreateProofRun(context.Background(), CreateScopeProofRunInput{
		Scope:       scope,
		Checks:      []ScopeProofCheck{ScopeProofCheckIngestion, ScopeProofCheckContext},
		FixtureMode: ScopeProofFixtureModeSmoke,
		Actor:       "operator-a",
		Reason:      "prove onboarding",
	})
	if err != nil {
		t.Fatalf("CreateProofRun() error = %v", err)
	}
	if created.ID != "proof_1" || created.Status != ScopeProofStatusPending || created.Verdict != ScopeProofVerdictPending {
		t.Fatalf("created = %+v, want pending proof_1", created)
	}
	if len(created.Steps) == 0 || created.Steps[0].ProofID != "proof_1" {
		t.Fatalf("created steps = %+v, want proof steps linked to proof_1", created.Steps)
	}

	listed, err := service.ListProofRuns(context.Background(), ListScopeProofRunsInput{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("ListProofRuns() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "proof_1" {
		t.Fatalf("listed = %+v, want proof_1", listed)
	}

	report, err := service.ReadProofReport(context.Background(), ReadScopeProofRunInput{Scope: scope, ProofID: "proof_1"})
	if err != nil {
		t.Fatalf("ReadProofReport() error = %v", err)
	}
	if report.Run.ID != "proof_1" || report.NextActions[0] == "" {
		t.Fatalf("report = %+v, want run and next actions", report)
	}

	rerun, err := service.RerunProofRun(context.Background(), RerunScopeProofRunInput{
		Scope:   scope,
		ProofID: "proof_1",
		Actor:   "operator-b",
		Reason:  "verify remediation",
	})
	if err != nil {
		t.Fatalf("RerunProofRun() error = %v", err)
	}
	if rerun.ID != "proof_2" || rerun.RerunOf != "proof_1" || rerun.Actor != "operator-b" {
		t.Fatalf("rerun = %+v, want proof_2 linked to proof_1", rerun)
	}
}

func TestScopeProofReportExtractsQualityRepairEvidenceAndNextActions(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubScopeProofStore{
		createdRuns: []ScopeProofRun{{
			ID:              "proof_1",
			Scope:           scope,
			Status:          ScopeProofStatusFailed,
			Verdict:         ScopeProofVerdictFailed,
			FailureCategory: ProofFailureCategoryContext,
			Steps: []ScopeProofStep{{
				ID:      "step_quality",
				ProofID: "proof_1",
				Scope:   scope,
				Step:    ScopeProofStepQualityEvaluated,
				Evidence: map[string]any{
					"evaluation_run_id": "eval_1",
				},
			}, {
				ID:      "step_replay",
				ProofID: "proof_1",
				Scope:   scope,
				Step:    ScopeProofStepReplayChecked,
				Evidence: map[string]any{
					"replay_run_id": "replay_1",
				},
			}, {
				ID:              "step_repair",
				ProofID:         "proof_1",
				Scope:           scope,
				Step:            ScopeProofStepRepairRecommended,
				FailureCategory: ProofFailureCategoryContext,
				Evidence: map[string]any{
					"repair_plan_id": "repair_1",
				},
			}},
		}},
	}
	service := NewScopeProofService(ScopeProofServiceOptions{Store: store})

	report, err := service.ReadProofReport(context.Background(), ReadScopeProofRunInput{Scope: scope, ProofID: "proof_1"})
	if err != nil {
		t.Fatalf("ReadProofReport() error = %v", err)
	}
	if report.Evidence.QualityEvaluationIDs[0] != "eval_1" || report.Evidence.ReplayRunIDs[0] != "replay_1" || report.Evidence.RepairPlanIDs[0] != "repair_1" {
		t.Fatalf("report evidence = %+v, want eval/replay/repair ids", report.Evidence)
	}
	if !containsString(report.NextActions, "inspect_context_diagnostics") || !containsString(report.NextActions, "open_quality_evaluation") || !containsString(report.NextActions, "open_repair_plan") {
		t.Fatalf("next actions = %+v, want context quality repair diagnostics", report.NextActions)
	}
}

type stubScopeProofStore struct {
	createdRuns  []ScopeProofRun
	createdSteps []ScopeProofStep
}

func (s *stubScopeProofStore) CreateScopeProofRun(ctx context.Context, run ScopeProofRun) (ScopeProofRun, error) {
	s.createdRuns = append(s.createdRuns, run)
	return run, nil
}

func (s *stubScopeProofStore) CreateScopeProofStep(ctx context.Context, step ScopeProofStep) (ScopeProofStep, error) {
	s.createdSteps = append(s.createdSteps, step)
	return step, nil
}

func (s *stubScopeProofStore) ListScopeProofRuns(ctx context.Context, input ListScopeProofRunsInput) ([]ScopeProofRun, error) {
	return append([]ScopeProofRun(nil), s.createdRuns...), nil
}

func (s *stubScopeProofStore) ReadScopeProofRun(ctx context.Context, input ReadScopeProofRunInput) (ScopeProofRun, error) {
	for _, run := range s.createdRuns {
		if run.ID == input.ProofID {
			return run, nil
		}
	}
	return ScopeProofRun{}, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
