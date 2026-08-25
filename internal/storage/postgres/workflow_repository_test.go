package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workflow"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesReadsUpdatesDisablesAndListsWorkflowTemplates(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 16, 10, 0, 0, time.UTC)
	template := workflow.WorkflowTemplate{
		ID:               "workflow_template_1",
		Scope:            scope,
		Status:           workflow.TemplateStatusActive,
		IntegrationKind:  workflow.IntegrationKindAgentTurn,
		CompletionPolicy: workflow.CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "prove integration workflow",
		Steps:            []workflow.TemplateStep{workflowTemplateStep(scope, "workflow_template_1", now)},
		Metadata:         map[string]any{"route_hint": "memory_sessions"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO integration_workflow_templates").
		WithArgs(template.ID, scope.Tenant, scope.Project, scope.Namespace, template.Status, template.IntegrationKind, template.CompletionPolicy, template.Actor, template.Reason, []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, template.UpdatedAt, nil).
		WillReturnRows(workflowTemplateRows().AddRow(template.ID, scope.Tenant, scope.Project, scope.Namespace, template.Status, template.IntegrationKind, template.CompletionPolicy, template.Actor, template.Reason, []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, template.UpdatedAt, nil))
	mock.ExpectQuery("INSERT INTO integration_workflow_template_steps").
		WithArgs("template_step_1", template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.StepKindSessionStarted, workflow.StepRequirementRequired, []string{"session"}, 1, false, int64(time.Hour), int64(time.Hour), 1, []byte(`{}`), now).
		WillReturnRows(workflowTemplateStepRows().AddRow("template_step_1", template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.StepKindSessionStarted, workflow.StepRequirementRequired, []string{"session"}, 1, false, int64(time.Hour), int64(time.Hour), 1, []byte(`{}`), now))
	mock.ExpectCommit()

	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_templates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, template.ID).
		WillReturnRows(workflowTemplateRows().AddRow(template.ID, scope.Tenant, scope.Project, scope.Namespace, template.Status, template.IntegrationKind, template.CompletionPolicy, template.Actor, template.Reason, []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, template.UpdatedAt, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_template_steps").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, template.ID).
		WillReturnRows(workflowTemplateStepRows().AddRow("template_step_1", template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.StepKindSessionStarted, workflow.StepRequirementRequired, []string{"session"}, 1, false, int64(time.Hour), int64(time.Hour), 1, []byte(`{}`), now))

	updatedAt := now.Add(time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE integration_workflow_templates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, template.ID, workflow.IntegrationKindAgentTurn, workflow.CompletionPolicyStrict, "admin-b", "tighten workflow", []byte(`{"route_hint":"memory_sessions"}`), updatedAt).
		WillReturnRows(workflowTemplateRows().AddRow(template.ID, scope.Tenant, scope.Project, scope.Namespace, template.Status, template.IntegrationKind, template.CompletionPolicy, "admin-b", "tighten workflow", []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, updatedAt, nil))
	mock.ExpectExec("DELETE FROM integration_workflow_template_steps").
		WithArgs(template.ID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectQuery("INSERT INTO integration_workflow_template_steps").
		WithArgs("template_step_1", template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.StepKindSessionStarted, workflow.StepRequirementRequired, []string{"session"}, 1, false, int64(time.Hour), int64(time.Hour), 1, []byte(`{}`), now).
		WillReturnRows(workflowTemplateStepRows().AddRow("template_step_1", template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.StepKindSessionStarted, workflow.StepRequirementRequired, []string{"session"}, 1, false, int64(time.Hour), int64(time.Hour), 1, []byte(`{}`), now))
	mock.ExpectCommit()

	disabledAt := now.Add(2 * time.Minute)
	mock.ExpectQuery("UPDATE integration_workflow_templates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, template.ID, workflow.TemplateStatusDisabled, "admin-c", "retire workflow", disabledAt).
		WillReturnRows(workflowTemplateRows().AddRow(template.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.TemplateStatusDisabled, template.IntegrationKind, template.CompletionPolicy, "admin-c", "retire workflow", []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, disabledAt, disabledAt))

	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_templates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(workflow.TemplateStatusActive), 10).
		WillReturnRows(workflowTemplateRows().AddRow(template.ID, scope.Tenant, scope.Project, scope.Namespace, template.Status, template.IntegrationKind, template.CompletionPolicy, template.Actor, template.Reason, []byte(`{"route_hint":"memory_sessions"}`), template.CreatedAt, template.UpdatedAt, nil))

	repo := NewRepository(mock)
	created, err := repo.CreateWorkflowTemplate(context.Background(), template)
	if err != nil {
		t.Fatalf("CreateWorkflowTemplate() error = %v", err)
	}
	if created.Scope != scope || len(created.Steps) != 1 || created.Metadata["route_hint"] != "memory_sessions" {
		t.Fatalf("created = %+v, want scoped template with one step and metadata", created)
	}
	read, err := repo.ReadWorkflowTemplate(context.Background(), workflow.ReadTemplateInput{Scope: scope, TemplateID: template.ID})
	if err != nil {
		t.Fatalf("ReadWorkflowTemplate() error = %v", err)
	}
	if read.Scope != scope || len(read.Steps) != 1 {
		t.Fatalf("read = %+v, want scoped template with steps", read)
	}
	updated, err := repo.UpdateWorkflowTemplate(context.Background(), workflow.UpdateTemplateInput{
		Scope:            scope,
		TemplateID:       template.ID,
		Steps:            template.Steps,
		IntegrationKind:  workflow.IntegrationKindAgentTurn,
		CompletionPolicy: workflow.CompletionPolicyStrict,
		Actor:            "admin-b",
		Reason:           "tighten workflow",
		Metadata:         template.Metadata,
		UpdatedAt:        updatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateWorkflowTemplate() error = %v", err)
	}
	if updated.Actor != "admin-b" || len(updated.Steps) != 1 {
		t.Fatalf("updated = %+v, want actor update and replacement steps", updated)
	}
	disabled, err := repo.DisableWorkflowTemplate(context.Background(), workflow.DisableTemplateInput{Scope: scope, TemplateID: template.ID, Actor: "admin-c", Reason: "retire workflow", DisabledAt: disabledAt})
	if err != nil {
		t.Fatalf("DisableWorkflowTemplate() error = %v", err)
	}
	if disabled.Status != workflow.TemplateStatusDisabled || !disabled.DisabledAt.Equal(disabledAt) {
		t.Fatalf("disabled = %+v, want disabled template", disabled)
	}
	listed, err := repo.ListWorkflowTemplates(context.Background(), workflow.ListTemplatesInput{Scope: scope, Status: workflow.TemplateStatusActive, Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkflowTemplates() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Scope != scope {
		t.Fatalf("listed = %+v, want scoped templates", listed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReadsWorkflowHealthByScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 16, 30, 0, 0, time.UTC)
	mock.ExpectQuery("FROM integration_workflow_runs wr").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, now).
		WillReturnRows(pgxmock.NewRows([]string{"completed_runs", "incomplete_runs", "stale_runs", "blocking_diagnostics", "latest_observed_at"}).
			AddRow(int64(2), int64(1), int64(1), int64(3), now.Add(-time.Minute)))

	repo := NewRepository(mock)
	snapshot, err := repo.ReadWorkflowHealth(context.Background(), scope, now)
	if err != nil {
		t.Fatalf("ReadWorkflowHealth() error = %v", err)
	}
	if snapshot.Scope != scope || snapshot.CompletedRuns != 2 || snapshot.IncompleteRuns != 1 || snapshot.StaleRuns != 1 || snapshot.BlockingDiagnostics != 3 {
		t.Fatalf("workflow health = %+v, want scoped aggregate counts", snapshot)
	}
	if snapshot.Status != assurance.HealthStatusUnhealthy || snapshot.Reason != assurance.ReasonWorkflowGap {
		t.Fatalf("workflow health = %+v, want unhealthy bounded workflow gap", snapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryVerifiesWorkflowEvidenceWithoutLeakingForeignScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	foreign := memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}
	mock.ExpectQuery("FROM usefulness_feedback f").WithArgs("feedback_foreign").WillReturnRows(
		pgxmock.NewRows([]string{"tenant", "project", "namespace", "hidden", "has_subject", "has_sufficient_evidence", "contradictory"}).
			AddRow(foreign.Tenant, foreign.Project, foreign.Namespace, false, true, true, false),
	)
	mock.ExpectQuery("FROM task_evaluations e").WithArgs("task_hidden").WillReturnRows(
		pgxmock.NewRows([]string{"tenant", "project", "namespace", "hidden", "has_subject", "has_sufficient_evidence", "contradictory"}).
			AddRow(scope.Tenant, scope.Project, scope.Namespace, true, true, false, false),
	)
	mock.ExpectQuery("FROM memory_session_runs").WithArgs("missing_session").WillReturnRows(
		pgxmock.NewRows([]string{"tenant", "project", "namespace", "hidden", "has_subject", "has_sufficient_evidence", "contradictory"}),
	)

	repo := NewRepository(mock)
	foreignResult, err := repo.VerifyWorkflowEvidence(context.Background(), workflow.EvidenceVerificationInput{Scope: scope, Kind: workflow.EvidenceKindUsefulnessFeedback, TargetID: "feedback_foreign"})
	if err != nil || !foreignResult.Exists || foreignResult.Scope != foreign {
		t.Fatalf("foreign verification = %+v, %v; want existing foreign record", foreignResult, err)
	}
	hiddenResult, err := repo.VerifyWorkflowEvidence(context.Background(), workflow.EvidenceVerificationInput{Scope: scope, Kind: workflow.EvidenceKindTaskEvaluation, TargetID: "task_hidden"})
	if err != nil || !hiddenResult.Exists || !hiddenResult.Hidden || hiddenResult.HasSufficientEvidence {
		t.Fatalf("hidden verification = %+v, %v; want hidden insufficient record", hiddenResult, err)
	}
	missingResult, err := repo.VerifyWorkflowEvidence(context.Background(), workflow.EvidenceVerificationInput{Scope: scope, Kind: workflow.EvidenceKindSession, TargetID: "missing_session"})
	if err != nil || missingResult.Exists {
		t.Fatalf("missing verification = %+v, %v; want absent record", missingResult, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCleansWorkflowHistoryByScopeDeletingTerminalRuns(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("DELETE FROM integration_workflow_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, cutoff).
		WillReturnResult(pgxmock.NewResult("DELETE", 2))

	repo := NewRepository(mock)
	deleted, err := repo.DeleteWorkflowHistoryBefore(context.Background(), scope, cutoff)
	if err != nil {
		t.Fatalf("DeleteWorkflowHistoryBefore() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want terminal workflow runs with cascading history", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryStartsWorkflowRunIdempotentlyAndTransitionsAppendOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 16, 20, 0, 0, time.UTC)
	run := workflow.WorkflowRun{
		ID:              "workflow_run_1",
		TemplateID:      "workflow_template_1",
		Scope:           scope,
		Status:          workflow.RunStatusRunning,
		IntegrationKind: workflow.IntegrationKindAgentTurn,
		IdempotencyKey:  "session_1:turn_1",
		Actor:           "agent-a",
		Reason:          "serve user turn",
		Metadata:        highCardinalityMetadata(32),
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       now,
	}

	mock.ExpectQuery("INSERT INTO integration_workflow_runs").
		WithArgs(run.ID, run.TemplateID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.IntegrationKind, run.IdempotencyKey, run.Actor, run.Reason, pgxmock.AnyArg(), run.CreatedAt, run.UpdatedAt, run.StartedAt, nil, nil).
		WillReturnRows(workflowRunRows().AddRow(run.ID, run.TemplateID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.IntegrationKind, run.IdempotencyKey, run.Actor, run.Reason, mustJSON(highCardinalityMetadata(32)), run.CreatedAt, run.UpdatedAt, run.StartedAt, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ID).
		WillReturnRows(workflowRunRows().AddRow(run.ID, run.TemplateID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.IntegrationKind, run.IdempotencyKey, run.Actor, run.Reason, mustJSON(highCardinalityMetadata(32)), run.CreatedAt, run.UpdatedAt, run.StartedAt, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.TemplateID, string(workflow.RunStatusRunning), 10).
		WillReturnRows(workflowRunRows().AddRow(run.ID, run.TemplateID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.IntegrationKind, run.IdempotencyKey, run.Actor, run.Reason, mustJSON(highCardinalityMetadata(32)), run.CreatedAt, run.UpdatedAt, run.StartedAt, nil, nil))

	transition := workflow.WorkflowTransition{
		ID:         "workflow_transition_1",
		RunID:      run.ID,
		Scope:      scope,
		FromStatus: workflow.RunStatusRunning,
		ToStatus:   workflow.RunStatusCompleted,
		Actor:      "agent-a",
		Reason:     "workflow complete",
		OccurredAt: now.Add(time.Minute),
	}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO integration_workflow_transitions").
		WithArgs(transition.ID, transition.RunID, scope.Tenant, scope.Project, scope.Namespace, transition.FromStatus, transition.ToStatus, transition.Actor, transition.Reason, transition.OccurredAt).
		WillReturnRows(workflowTransitionRows().AddRow(transition.ID, transition.RunID, scope.Tenant, scope.Project, scope.Namespace, transition.FromStatus, transition.ToStatus, transition.Actor, transition.Reason, transition.OccurredAt))
	mock.ExpectQuery("UPDATE integration_workflow_runs").
		WithArgs(transition.ToStatus, transition.OccurredAt, transition.OccurredAt, transition.RunID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(workflowRunRows().AddRow(run.ID, run.TemplateID, scope.Tenant, scope.Project, scope.Namespace, workflow.RunStatusCompleted, run.IntegrationKind, run.IdempotencyKey, run.Actor, run.Reason, mustJSON(highCardinalityMetadata(32)), run.CreatedAt, transition.OccurredAt, run.StartedAt, transition.OccurredAt, nil))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	started, err := repo.StartWorkflowRun(context.Background(), run)
	if err != nil {
		t.Fatalf("StartWorkflowRun() error = %v", err)
	}
	if started.Scope != scope || started.IdempotencyKey != run.IdempotencyKey || len(started.Metadata) != 32 {
		t.Fatalf("started = %+v, want scoped idempotent run with metadata preserved", started)
	}
	read, err := repo.ReadWorkflowRun(context.Background(), workflow.ReadRunInput{Scope: scope, RunID: run.ID})
	if err != nil {
		t.Fatalf("ReadWorkflowRun() error = %v", err)
	}
	if read.ID != run.ID || read.Scope != scope {
		t.Fatalf("read = %+v, want scoped run", read)
	}
	listed, err := repo.ListWorkflowRuns(context.Background(), workflow.ListRunsInput{Scope: scope, TemplateID: run.TemplateID, Status: workflow.RunStatusRunning, Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkflowRuns() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Scope != scope {
		t.Fatalf("listed = %+v, want scoped runs", listed)
	}
	updated, err := repo.TransitionWorkflowRun(context.Background(), workflow.TransitionRunInput{Transition: transition, UpdatedAt: transition.OccurredAt})
	if err != nil {
		t.Fatalf("TransitionWorkflowRun() error = %v", err)
	}
	if updated.Status != workflow.RunStatusCompleted || updated.CompletedAt.IsZero() {
		t.Fatalf("updated = %+v, want completed workflow run", updated)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRecordsWorkflowStepsEvidenceDiagnosticsNextActionsAndRetention(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 16, 30, 0, 0, time.UTC)
	step := workflow.WorkflowStepRecord{
		ID:         "workflow_step_1",
		RunID:      "workflow_run_1",
		Scope:      scope,
		Kind:       workflow.StepKindTaskEvaluationRecorded,
		Status:     workflow.StepStatusSatisfied,
		Result:     workflow.StepResultRecorded,
		Actor:      "agent-a",
		Reason:     "task evaluation recorded",
		ObservedAt: now,
		CreatedAt:  now,
		EvidenceLinks: []workflow.EvidenceLink{{
			ID:           "workflow_evidence_link_1",
			RunID:        "workflow_run_1",
			StepRecordID: "workflow_step_1",
			Scope:        scope,
			Kind:         workflow.EvidenceKindOpaque,
			Status:       workflow.EvidenceLinkStatusActive,
			Source:       workflow.EvidenceSourceOpaque,
			OpaqueToken:  "caller-token",
			Metadata:     map[string]any{"opaque_subject": "task-123"},
			CreatedAt:    now,
		}},
	}
	diagnostic := workflow.GapDiagnostic{
		ID:              "workflow_gap_1",
		RunID:           "workflow_run_1",
		StepRecordID:    "workflow_step_1",
		EvidenceLinkID:  "workflow_evidence_link_1",
		Scope:           scope,
		StepKind:        workflow.StepKindTaskEvaluationRecorded,
		EvidenceKind:    workflow.EvidenceKindTaskEvaluation,
		Category:        workflow.DiagnosticCategoryOpaqueOnly,
		ReadinessImpact: workflow.ReadinessImpactDegraded,
		Status:          "open",
		CreatedAt:       now,
	}
	action := workflow.NextAction{
		ID:            "workflow_next_action_1",
		RunID:         "workflow_run_1",
		Scope:         scope,
		Category:      workflow.NextActionRecordTaskEvaluation,
		StepKind:      workflow.StepKindTaskEvaluationRecorded,
		EvidenceKind:  workflow.EvidenceKindTaskEvaluation,
		RouteCategory: workflow.RouteCategoryTaskEvaluations,
		Status:        workflow.NextActionStatusOpen,
		CreatedAt:     now,
	}
	retention := workflow.WorkflowRetentionRun{
		ID:             "workflow_retention_1",
		Scope:          scope,
		RecordCategory: workflow.RetentionClassDiagnostic,
		Cutoff:         now.Add(-24 * time.Hour),
		DeletedCount:   4,
		StartedAt:      now,
		FinishedAt:     now.Add(time.Minute),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO integration_workflow_step_records").
		WithArgs(step.ID, step.RunID, scope.Tenant, scope.Project, scope.Namespace, step.Kind, step.Status, step.Result, step.Actor, step.Reason, []byte(`{}`), step.ObservedAt, step.CreatedAt).
		WillReturnRows(workflowStepRecordRows().AddRow(step.ID, step.RunID, scope.Tenant, scope.Project, scope.Namespace, step.Kind, step.Status, step.Result, step.Actor, step.Reason, []byte(`{}`), step.ObservedAt, step.CreatedAt))
	mock.ExpectQuery("INSERT INTO integration_workflow_evidence_links").
		WithArgs("workflow_evidence_link_1", step.RunID, step.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.EvidenceKindOpaque, workflow.EvidenceLinkStatusActive, workflow.EvidenceSourceOpaque, nil, "caller-token", []byte(`{"opaque_subject":"task-123"}`), now, nil).
		WillReturnRows(workflowEvidenceLinkRows().AddRow("workflow_evidence_link_1", step.RunID, step.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.EvidenceKindOpaque, workflow.EvidenceLinkStatusActive, workflow.EvidenceSourceOpaque, nil, "caller-token", []byte(`{"opaque_subject":"task-123"}`), now, nil))
	mock.ExpectCommit()

	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_step_records").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, step.RunID).
		WillReturnRows(workflowStepRecordRows().AddRow(step.ID, step.RunID, scope.Tenant, scope.Project, scope.Namespace, step.Kind, step.Status, step.Result, step.Actor, step.Reason, []byte(`{}`), step.ObservedAt, step.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_evidence_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, step.RunID, string(workflow.EvidenceLinkStatusActive)).
		WillReturnRows(workflowEvidenceLinkRows().AddRow("workflow_evidence_link_1", step.RunID, step.ID, scope.Tenant, scope.Project, scope.Namespace, workflow.EvidenceKindOpaque, workflow.EvidenceLinkStatusActive, workflow.EvidenceSourceOpaque, nil, "caller-token", []byte(`{"opaque_subject":"task-123"}`), now, nil))
	mock.ExpectQuery("INSERT INTO integration_workflow_gap_diagnostics").
		WithArgs(diagnostic.ID, diagnostic.RunID, diagnostic.StepRecordID, diagnostic.EvidenceLinkID, scope.Tenant, scope.Project, scope.Namespace, diagnostic.StepKind, diagnostic.EvidenceKind, diagnostic.Category, diagnostic.ReadinessImpact, diagnostic.Status, []byte(`{}`), diagnostic.CreatedAt, nil).
		WillReturnRows(workflowGapDiagnosticRows().AddRow(diagnostic.ID, diagnostic.RunID, diagnostic.StepRecordID, diagnostic.EvidenceLinkID, scope.Tenant, scope.Project, scope.Namespace, diagnostic.StepKind, diagnostic.EvidenceKind, diagnostic.Category, diagnostic.ReadinessImpact, diagnostic.Status, []byte(`{}`), diagnostic.CreatedAt, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_gap_diagnostics").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, diagnostic.RunID, string(workflow.DiagnosticCategoryOpaqueOnly), 10).
		WillReturnRows(workflowGapDiagnosticRows().AddRow(diagnostic.ID, diagnostic.RunID, diagnostic.StepRecordID, diagnostic.EvidenceLinkID, scope.Tenant, scope.Project, scope.Namespace, diagnostic.StepKind, diagnostic.EvidenceKind, diagnostic.Category, diagnostic.ReadinessImpact, diagnostic.Status, []byte(`{}`), diagnostic.CreatedAt, nil))
	mock.ExpectQuery("INSERT INTO integration_workflow_next_actions").
		WithArgs(action.ID, action.RunID, scope.Tenant, scope.Project, scope.Namespace, action.Category, action.StepKind, action.EvidenceKind, action.RouteCategory, action.Status, []byte(`{}`), action.CreatedAt, nil).
		WillReturnRows(workflowNextActionRows().AddRow(action.ID, action.RunID, scope.Tenant, scope.Project, scope.Namespace, action.Category, action.StepKind, action.EvidenceKind, action.RouteCategory, action.Status, []byte(`{}`), action.CreatedAt, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_next_actions").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, action.RunID, string(workflow.NextActionStatusOpen), 10).
		WillReturnRows(workflowNextActionRows().AddRow(action.ID, action.RunID, scope.Tenant, scope.Project, scope.Namespace, action.Category, action.StepKind, action.EvidenceKind, action.RouteCategory, action.Status, []byte(`{}`), action.CreatedAt, nil))
	mock.ExpectExec("UPDATE integration_workflow_evidence_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "workflow_evidence_link_1", workflow.EvidenceLinkStatusSuperseded, "admin-a", "bad evidence", now.Add(time.Minute)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM integration_workflow_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, retention.Cutoff, 100).
		WillReturnRows(workflowRunRows().AddRow("workflow_run_old", "workflow_template_1", scope.Tenant, scope.Project, scope.Namespace, workflow.RunStatusCompleted, workflow.IntegrationKindAgentTurn, "old-turn", "agent-a", "old turn", []byte(`{}`), now.Add(-48*time.Hour), now.Add(-47*time.Hour), now.Add(-48*time.Hour), now.Add(-47*time.Hour), nil))
	mock.ExpectQuery("INSERT INTO integration_workflow_retention_runs").
		WithArgs(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.StartedAt, retention.FinishedAt).
		WillReturnRows(workflowRetentionRunRows().AddRow(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.StartedAt, retention.FinishedAt))

	repo := NewRepository(mock)
	recorded, err := repo.RecordWorkflowStep(context.Background(), step)
	if err != nil {
		t.Fatalf("RecordWorkflowStep() error = %v", err)
	}
	if recorded.Scope != scope || len(recorded.EvidenceLinks) != 1 || recorded.EvidenceLinks[0].OpaqueToken != "caller-token" {
		t.Fatalf("recorded = %+v, want scoped step preserving opaque evidence", recorded)
	}
	steps, err := repo.ListWorkflowStepRecords(context.Background(), workflow.ListStepRecordsInput{Scope: scope, RunID: step.RunID})
	if err != nil {
		t.Fatalf("ListWorkflowStepRecords() error = %v", err)
	}
	if len(steps) != 1 || steps[0].Scope != scope {
		t.Fatalf("steps = %+v, want scoped step history", steps)
	}
	links, err := repo.ListWorkflowEvidenceLinks(context.Background(), workflow.ListEvidenceLinksInput{Scope: scope, RunID: step.RunID, Status: workflow.EvidenceLinkStatusActive})
	if err != nil {
		t.Fatalf("ListWorkflowEvidenceLinks() error = %v", err)
	}
	if len(links) != 1 || links[0].OpaqueToken != "caller-token" {
		t.Fatalf("links = %+v, want opaque token preserved", links)
	}
	if _, err := repo.RecordWorkflowGapDiagnostic(context.Background(), diagnostic); err != nil {
		t.Fatalf("RecordWorkflowGapDiagnostic() error = %v", err)
	}
	diagnostics, err := repo.ListWorkflowGapDiagnostics(context.Background(), workflow.ListDiagnosticsInput{Scope: scope, RunID: diagnostic.RunID, Category: workflow.DiagnosticCategoryOpaqueOnly, Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkflowGapDiagnostics() error = %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Category != workflow.DiagnosticCategoryOpaqueOnly {
		t.Fatalf("diagnostics = %+v, want opaque-only diagnostic", diagnostics)
	}
	if _, err := repo.RecordWorkflowNextAction(context.Background(), action); err != nil {
		t.Fatalf("RecordWorkflowNextAction() error = %v", err)
	}
	actions, err := repo.ListWorkflowNextActions(context.Background(), workflow.ListNextActionsInput{Scope: scope, RunID: action.RunID, Status: workflow.NextActionStatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkflowNextActions() error = %v", err)
	}
	if len(actions) != 1 || actions[0].RouteCategory != workflow.RouteCategoryTaskEvaluations {
		t.Fatalf("actions = %+v, want task evaluation next action", actions)
	}
	if err := repo.SupersedeWorkflowEvidenceLink(context.Background(), workflow.SupersedeEvidenceLinkInput{Scope: scope, LinkID: "workflow_evidence_link_1", Actor: "admin-a", Reason: "bad evidence", SupersededAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("SupersedeWorkflowEvidenceLink() error = %v", err)
	}
	eligible, err := repo.FindWorkflowRetentionEligibleHistory(context.Background(), workflow.FindRetentionEligibleHistoryInput{Scope: scope, RecordCategory: workflow.RetentionClassDiagnostic, Cutoff: retention.Cutoff, Limit: 100})
	if err != nil {
		t.Fatalf("FindWorkflowRetentionEligibleHistory() error = %v", err)
	}
	if len(eligible) != 1 || eligible[0].ID != "workflow_run_old" {
		t.Fatalf("eligible = %+v, want old completed run", eligible)
	}
	if _, err := repo.CreateWorkflowRetentionRun(context.Background(), retention); err != nil {
		t.Fatalf("CreateWorkflowRetentionRun() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func workflowTemplateStep(scope memory.Scope, templateID string, now time.Time) workflow.TemplateStep {
	return workflow.TemplateStep{
		ID:               "template_step_1",
		TemplateID:       templateID,
		Scope:            scope,
		Kind:             workflow.StepKindSessionStarted,
		Requirement:      workflow.StepRequirementRequired,
		AllowedEvidence:  []workflow.EvidenceKind{workflow.EvidenceKindSession},
		MinimumCount:     1,
		FreshnessWindow:  time.Hour,
		CompletionWindow: time.Hour,
		Position:         1,
		CreatedAt:        now,
	}
}

func workflowTemplateRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "integration_kind", "completion_policy", "actor", "reason", "metadata", "created_at", "updated_at", "disabled_at"})
}

func workflowTemplateStepRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "template_id", "tenant", "project", "namespace", "kind", "requirement", "allowed_evidence", "minimum_count", "requires_internal", "freshness_window_ns", "completion_window_ns", "position", "metadata", "created_at"})
}

func workflowRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "template_id", "tenant", "project", "namespace", "status", "integration_kind", "idempotency_key", "actor", "reason", "metadata", "created_at", "updated_at", "started_at", "completed_at", "expires_at"})
}

func workflowStepRecordRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "run_id", "tenant", "project", "namespace", "kind", "status", "result", "actor", "reason", "metadata", "observed_at", "created_at"})
}

func workflowEvidenceLinkRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "run_id", "step_record_id", "tenant", "project", "namespace", "kind", "status", "source", "target_id", "opaque_token", "metadata", "created_at", "superseded_at"})
}

func workflowGapDiagnosticRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "run_id", "step_record_id", "evidence_link_id", "tenant", "project", "namespace", "step_kind", "evidence_kind", "category", "readiness_impact", "status", "metadata", "created_at", "resolved_at"})
}

func workflowNextActionRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "run_id", "tenant", "project", "namespace", "category", "step_kind", "evidence_kind", "route_category", "status", "metadata", "created_at", "resolved_at"})
}

func workflowTransitionRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "run_id", "tenant", "project", "namespace", "from_status", "to_status", "actor", "reason", "occurred_at"})
}

func workflowRetentionRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "record_category", "cutoff", "deleted_count", "started_at", "finished_at"})
}

func highCardinalityMetadata(entries int) map[string]any {
	metadata := make(map[string]any, entries)
	for i := 0; i < entries; i++ {
		metadata[fmt.Sprintf("key_%02d", i)] = fmt.Sprintf("value_%02d", i)
	}
	return metadata
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
