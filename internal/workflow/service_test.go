package workflow

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestServiceManagesTemplatesWithBoundedValidation(t *testing.T) {
	now := time.Date(2026, 7, 18, 17, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubWorkflowStore{}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	_, err := service.CreateTemplate(context.Background(), CreateTemplateInput{
		Scope:            scope,
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "prove integration workflow",
		Steps: []TemplateStep{{
			Kind:             StepKind("free_form_step"),
			Requirement:      StepRequirementRequired,
			AllowedEvidence:  []EvidenceKind{EvidenceKindSession},
			MinimumCount:     1,
			FreshnessWindow:  time.Hour,
			CompletionWindow: time.Hour,
			Position:         1,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "workflow step kind") {
		t.Fatalf("CreateTemplate() error = %v, want bounded step validation", err)
	}
	if len(store.createdTemplates) != 0 {
		t.Fatalf("created templates = %d, want no write for invalid template", len(store.createdTemplates))
	}

	created, err := service.CreateTemplate(context.Background(), CreateTemplateInput{
		Scope:            scope,
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "prove integration workflow",
		Metadata:         map[string]any{"surface": "public_api"},
		Steps: []TemplateStep{{
			Kind:             StepKindSessionStarted,
			Requirement:      StepRequirementRequired,
			AllowedEvidence:  []EvidenceKind{EvidenceKindSession},
			MinimumCount:     1,
			FreshnessWindow:  time.Hour,
			CompletionWindow: time.Hour,
			Position:         1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	if created.ID != "workflow_template_1" || created.Status != TemplateStatusActive || created.Scope != scope {
		t.Fatalf("created = %+v, want active scoped generated template", created)
	}
	if created.Steps[0].ID != "workflow_template_step_1" || created.Steps[0].TemplateID != created.ID || created.Steps[0].Scope != scope {
		t.Fatalf("created step = %+v, want generated template-scoped step", created.Steps[0])
	}

	updated, err := service.UpdateTemplate(context.Background(), UpdateTemplateInput{
		Scope:            scope,
		TemplateID:       created.ID,
		Steps:            created.Steps,
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-b",
		Reason:           "tighten workflow",
		Metadata:         map[string]any{"surface": "admin_api"},
		UpdatedAt:        now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("UpdateTemplate() error = %v", err)
	}
	if updated.Actor != "admin-b" || updated.Metadata["surface"] != "admin_api" {
		t.Fatalf("updated = %+v, want updated template metadata", updated)
	}
	disabled, err := service.DisableTemplate(context.Background(), DisableTemplateInput{
		Scope:      scope,
		TemplateID: created.ID,
		Actor:      "admin-c",
		Reason:     "retire workflow",
		DisabledAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("DisableTemplate() error = %v", err)
	}
	if disabled.Status != TemplateStatusDisabled || disabled.DisabledAt.IsZero() {
		t.Fatalf("disabled = %+v, want disabled template", disabled)
	}
}

func TestServiceStartsWorkflowRunIdempotentlyAndCreatesInitialNextAction(t *testing.T) {
	now := time.Date(2026, 7, 18, 17, 10, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	template := WorkflowTemplate{
		ID:               "workflow_template_1",
		Scope:            scope,
		Status:           TemplateStatusActive,
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "prove integration",
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps: []TemplateStep{{
			ID:               "template_step_1",
			TemplateID:       "workflow_template_1",
			Scope:            scope,
			Kind:             StepKindContextRequested,
			Requirement:      StepRequirementRequired,
			AllowedEvidence:  []EvidenceKind{EvidenceKindContext},
			MinimumCount:     1,
			RequiresInternal: true,
			FreshnessWindow:  time.Hour,
			CompletionWindow: time.Hour,
			Position:         1,
			CreatedAt:        now,
		}},
	}
	store := &stubWorkflowStore{templates: []WorkflowTemplate{template}}
	nextID := 0
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string {
			nextID++
			return fmt.Sprintf("%s_%d", prefix, nextID)
		},
	})

	first, err := service.StartRun(context.Background(), StartRunInput{
		Scope:          scope,
		TemplateID:     template.ID,
		IdempotencyKey: "session_1:turn_1",
		Actor:          "agent-a",
		Reason:         "serve user turn",
	})
	if err != nil {
		t.Fatalf("StartRun(first) error = %v", err)
	}
	second, err := service.StartRun(context.Background(), StartRunInput{
		Scope:          scope,
		TemplateID:     template.ID,
		IdempotencyKey: "session_1:turn_1",
		Actor:          "agent-a",
		Reason:         "serve user turn",
	})
	if err != nil {
		t.Fatalf("StartRun(second) error = %v", err)
	}
	if first.ID != second.ID || len(store.runs) != 1 {
		t.Fatalf("runs = %+v, first = %+v second = %+v, want idempotent run", store.runs, first, second)
	}
	if len(store.nextActions) != 1 {
		t.Fatalf("next actions = %+v, want one initial action", store.nextActions)
	}
	if store.nextActions[0].Category != NextActionRequestContext || store.nextActions[0].RouteCategory != RouteCategoryMemorySessions {
		t.Fatalf("initial next action = %+v, want context request guidance", store.nextActions[0])
	}
}

func TestServiceCompletesRunWhenAllRequiredEvidenceIsRecorded(t *testing.T) {
	now := time.Date(2026, 7, 18, 17, 15, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	template := workflowEvidenceTemplate(scope, now)
	run := WorkflowRun{ID: "workflow_run_1", TemplateID: template.ID, Scope: scope, Status: RunStatusRunning, IntegrationKind: IntegrationKindAgentTurn, IdempotencyKey: "run-1", Actor: "agent-a", Reason: "turn", CreatedAt: now, UpdatedAt: now, StartedAt: now}
	verifier := &stubEvidenceVerifier{results: map[string]EvidenceVerificationResult{
		"context:context_1":              {Exists: true, Scope: scope, HasSubject: true, HasSufficientEvidence: true},
		"usefulness_feedback:feedback_1": {Exists: true, Scope: scope, HasSubject: true, HasSufficientEvidence: true},
		"task_evaluation:task_1":         {Exists: true, Scope: scope, HasSubject: true, HasSufficientEvidence: true},
	}}
	store := &stubWorkflowStore{templates: []WorkflowTemplate{template}, runs: []WorkflowRun{run}, nextActions: []NextAction{{ID: "action_1", RunID: run.ID, Scope: scope, Category: NextActionRequestContext, StepKind: StepKindContextRequested, EvidenceKind: EvidenceKindContext, RouteCategory: RouteCategoryMemorySessions, Status: NextActionStatusOpen, CreatedAt: now}}}
	service := NewService(ServiceOptions{Store: store, EvidenceVerifier: verifier, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})
	for _, input := range []RecordStepInput{
		{Scope: scope, RunID: run.ID, Kind: StepKindContextRequested, Actor: "agent-a", Reason: "context", EvidenceLinks: []EvidenceLink{{Kind: EvidenceKindContext, Source: EvidenceSourcePublicAPI, TargetID: "context_1"}}},
		{Scope: scope, RunID: run.ID, Kind: StepKindUsefulnessFeedbackRecorded, Actor: "agent-a", Reason: "feedback", EvidenceLinks: []EvidenceLink{{Kind: EvidenceKindUsefulnessFeedback, Source: EvidenceSourcePublicAPI, TargetID: "feedback_1"}}},
		{Scope: scope, RunID: run.ID, Kind: StepKindTaskEvaluationRecorded, Actor: "agent-a", Reason: "evaluation", EvidenceLinks: []EvidenceLink{{Kind: EvidenceKindTaskEvaluation, Source: EvidenceSourcePublicAPI, TargetID: "task_1"}}},
	} {
		if _, err := service.RecordStep(context.Background(), input); err != nil {
			t.Fatalf("RecordStep(%s) error = %v", input.Kind, err)
		}
	}
	if store.runs[0].Status != RunStatusCompleted || store.runs[0].CompletedAt.IsZero() {
		t.Fatalf("run = %+v, want completed run with timestamp", store.runs[0])
	}
	for _, action := range store.nextActions {
		if action.Status == NextActionStatusOpen {
			t.Fatalf("next actions = %+v, want no open action after completion", store.nextActions)
		}
	}
}

func TestServiceRecordsStepEvidenceAndMaterializesGapsWithoutMutatingSources(t *testing.T) {
	now := time.Date(2026, 7, 18, 17, 20, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	template := workflowEvidenceTemplate(scope, now)
	run := WorkflowRun{
		ID:              "workflow_run_1",
		TemplateID:      template.ID,
		Scope:           scope,
		Status:          RunStatusRunning,
		IntegrationKind: IntegrationKindAgentTurn,
		IdempotencyKey:  "session_1:turn_1",
		Actor:           "agent-a",
		Reason:          "serve user turn",
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       now,
	}
	verifier := &stubEvidenceVerifier{
		results: map[string]EvidenceVerificationResult{
			"task_evaluation:task_eval_1": {
				Exists:                true,
				Scope:                 scope,
				HasSubject:            true,
				HasSufficientEvidence: true,
			},
			"usefulness_feedback:feedback_1": {
				Exists:     true,
				Scope:      scope,
				HasSubject: false,
			},
			"task_evaluation:foreign_task_eval": {
				Exists: true,
				Scope:  memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"},
			},
		},
	}
	store := &stubWorkflowStore{templates: []WorkflowTemplate{template}, runs: []WorkflowRun{run}}
	service := NewService(ServiceOptions{
		Store:            store,
		EvidenceVerifier: verifier,
		Now:              func() time.Time { return now },
		NewID: func(prefix string) string {
			return prefix + "_1"
		},
	})

	_, err := service.RecordStep(context.Background(), RecordStepInput{
		Scope:      scope,
		RunID:      run.ID,
		Kind:       StepKindContextRequested,
		Actor:      "agent-a",
		Reason:     "context recorded as opaque caller token",
		ObservedAt: now.Add(-time.Minute),
		EvidenceLinks: []EvidenceLink{{
			ID:          "evidence_link_opaque_context",
			Kind:        EvidenceKindOpaque,
			Source:      EvidenceSourceOpaque,
			OpaqueToken: "caller-context-token",
		}},
	})
	if err != nil {
		t.Fatalf("RecordStep(context opaque) error = %v", err)
	}

	_, err = service.RecordStep(context.Background(), RecordStepInput{
		Scope:      scope,
		RunID:      run.ID,
		Kind:       StepKindTaskEvaluationRecorded,
		Actor:      "agent-a",
		Reason:     "recorded task evaluation early",
		ObservedAt: now,
		EvidenceLinks: []EvidenceLink{{
			ID:       "evidence_link_1",
			Kind:     EvidenceKindTaskEvaluation,
			Source:   EvidenceSourcePublicAPI,
			TargetID: "task_eval_1",
		}},
	})
	if err != nil {
		t.Fatalf("RecordStep(task evaluation) error = %v", err)
	}
	if len(store.stepRecords) != 2 || store.stepRecords[1].Result != StepResultOutOfOrder {
		t.Fatalf("step records = %+v, want out-of-order recorded step", store.stepRecords)
	}
	if !store.hasDiagnostic(DiagnosticCategoryOutOfOrder) {
		t.Fatalf("diagnostics = %+v, want out-of-order diagnostic", store.gapDiagnostics)
	}
	if verifier.mutated {
		t.Fatal("evidence verifier was mutated by workflow service")
	}

	_, err = service.RecordStep(context.Background(), RecordStepInput{
		Scope:      scope,
		RunID:      run.ID,
		Kind:       StepKindUsefulnessFeedbackRecorded,
		Actor:      "agent-a",
		Reason:     "feedback missing subject",
		ObservedAt: now.Add(time.Minute),
		EvidenceLinks: []EvidenceLink{{
			ID:       "evidence_link_2",
			Kind:     EvidenceKindUsefulnessFeedback,
			Source:   EvidenceSourcePublicAPI,
			TargetID: "feedback_1",
		}},
	})
	if err != nil {
		t.Fatalf("RecordStep(feedback) error = %v", err)
	}
	if !store.hasDiagnostic(DiagnosticCategorySubjectMissing) {
		t.Fatalf("diagnostics = %+v, want subject-missing diagnostic", store.gapDiagnostics)
	}

	_, err = service.RecordStep(context.Background(), RecordStepInput{
		Scope:      scope,
		RunID:      run.ID,
		Kind:       StepKindTaskEvaluationRecorded,
		Actor:      "agent-a",
		Reason:     "foreign evidence",
		ObservedAt: now.Add(2 * time.Minute),
		EvidenceLinks: []EvidenceLink{{
			ID:       "evidence_link_3",
			Kind:     EvidenceKindTaskEvaluation,
			Source:   EvidenceSourcePublicAPI,
			TargetID: "foreign_task_eval",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "out of scope") {
		t.Fatalf("RecordStep(foreign) error = %v, want out-of-scope rejection", err)
	}

	diagnostics, err := service.MaterializeGapDiagnostics(context.Background(), MaterializeGapDiagnosticsInput{
		Scope: scope,
		RunID: run.ID,
		Now:   now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("MaterializeGapDiagnostics() error = %v", err)
	}
	if len(diagnostics) == 0 || !store.hasDiagnostic(DiagnosticCategoryOpaqueOnly) {
		t.Fatalf("diagnostics = %+v, want opaque-only gap for unsatisfied internal step", store.gapDiagnostics)
	}
	if !store.hasNextAction(NextActionRecordFeedback) {
		t.Fatalf("next actions = %+v, want feedback next action", store.nextActions)
	}
}

func TestServiceTransitionsWorkflowRunWithinScope(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubWorkflowStore{runs: []WorkflowRun{{ID: "workflow_run_1", TemplateID: "workflow_template_1", Scope: scope, Status: RunStatusRunning, IntegrationKind: IntegrationKindAgentTurn, IdempotencyKey: "run-key", Actor: "agent-a", Reason: "turn", CreatedAt: now, UpdatedAt: now, StartedAt: now}}}
	service := NewService(ServiceOptions{Store: store, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})

	updated, err := service.TransitionRun(context.Background(), TransitionRunInput{
		Transition: WorkflowTransition{ID: "workflow_transition_1", RunID: "workflow_run_1", Scope: scope, FromStatus: RunStatusRunning, ToStatus: RunStatusExpired, Actor: "stele-workflow-maintenance", Reason: "workflow stale window elapsed", OccurredAt: now},
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("TransitionRun() error = %v", err)
	}
	if updated.Status != RunStatusExpired || store.runs[0].Status != RunStatusExpired || updated.Scope != scope {
		t.Fatalf("transition updated=%+v stored=%+v, want scoped expired run", updated, store.runs[0])
	}
}

func TestServiceMaterializeGapDiagnosticsDoesNotDuplicateOpenDiagnostic(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 15, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	template := workflowEvidenceTemplate(scope, now)
	run := WorkflowRun{ID: "workflow_run_1", TemplateID: template.ID, Scope: scope, Status: RunStatusRunning, IntegrationKind: IntegrationKindAgentTurn, IdempotencyKey: "run-key", Actor: "agent-a", Reason: "turn", CreatedAt: now, UpdatedAt: now, StartedAt: now}
	store := &stubWorkflowStore{templates: []WorkflowTemplate{template}, runs: []WorkflowRun{run}}
	service := NewService(ServiceOptions{Store: store, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})

	if _, err := service.MaterializeGapDiagnostics(context.Background(), MaterializeGapDiagnosticsInput{Scope: scope, RunID: run.ID, Now: now.Add(2 * time.Hour)}); err != nil {
		t.Fatalf("first MaterializeGapDiagnostics() error = %v", err)
	}
	firstCount := len(store.gapDiagnostics)
	if firstCount == 0 {
		t.Fatal("first materialization recorded no diagnostics")
	}
	if _, err := service.MaterializeGapDiagnostics(context.Background(), MaterializeGapDiagnosticsInput{Scope: scope, RunID: run.ID, Now: now.Add(3 * time.Hour)}); err != nil {
		t.Fatalf("second MaterializeGapDiagnostics() error = %v", err)
	}
	if len(store.gapDiagnostics) != firstCount {
		t.Fatalf("diagnostics after retry = %+v, want no duplicate open diagnostics", store.gapDiagnostics)
	}
}

func TestServiceExportsBoundedWorkflowLifecycleTelemetryAndLogs(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 30, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-secret", Project: "project-secret", Namespace: "namespace-secret"}
	store := &stubWorkflowStore{templates: []WorkflowTemplate{workflowEvidenceTemplate(scope, now)}}
	observer := telemetry.NewMetricsObserver()
	var logs bytes.Buffer
	service := NewService(ServiceOptions{Store: store, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_secret" }, Observer: observer, Logger: log.New(&logs, "", 0)})
	_, err := service.StartRun(context.Background(), StartRunInput{Scope: scope, TemplateID: "workflow_template_1", IdempotencyKey: "run-secret", Actor: "agent-secret", Reason: "secret reason", Metadata: map[string]any{"prompt": "hidden prompt"}})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, `stele_workflow_lifecycle_total`) || !strings.Contains(metrics, `operation="run_start"`) {
		t.Fatalf("workflow metrics = %s, want bounded run lifecycle", metrics)
	}
	for _, forbidden := range []string{"tenant-secret", "project-secret", "namespace-secret", "workflow_template_1", "run-secret", "agent-secret", "secret reason", "hidden prompt"} {
		if strings.Contains(metrics, forbidden) || strings.Contains(logs.String(), forbidden) {
			t.Fatalf("workflow telemetry/logs leak %q\nmetrics=%s\nlogs=%s", forbidden, metrics, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "component=workflow event=lifecycle operation=next_action") || !strings.Contains(logs.String(), "component=workflow event=lifecycle operation=run_start") {
		t.Fatalf("workflow logs = %s, want bounded lifecycle entries", logs.String())
	}
}

func workflowEvidenceTemplate(scope memory.Scope, now time.Time) WorkflowTemplate {
	return WorkflowTemplate{
		ID:               "workflow_template_1",
		Scope:            scope,
		Status:           TemplateStatusActive,
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "prove integration",
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps: []TemplateStep{
			{
				ID:               "template_step_context",
				TemplateID:       "workflow_template_1",
				Scope:            scope,
				Kind:             StepKindContextRequested,
				Requirement:      StepRequirementRequired,
				AllowedEvidence:  []EvidenceKind{EvidenceKindContext},
				MinimumCount:     1,
				RequiresInternal: true,
				FreshnessWindow:  time.Hour,
				CompletionWindow: time.Hour,
				Position:         1,
				CreatedAt:        now,
			},
			{
				ID:               "template_step_feedback",
				TemplateID:       "workflow_template_1",
				Scope:            scope,
				Kind:             StepKindUsefulnessFeedbackRecorded,
				Requirement:      StepRequirementRequired,
				AllowedEvidence:  []EvidenceKind{EvidenceKindUsefulnessFeedback},
				MinimumCount:     1,
				RequiresInternal: true,
				FreshnessWindow:  time.Hour,
				CompletionWindow: time.Hour,
				Position:         2,
				CreatedAt:        now,
			},
			{
				ID:               "template_step_task",
				TemplateID:       "workflow_template_1",
				Scope:            scope,
				Kind:             StepKindTaskEvaluationRecorded,
				Requirement:      StepRequirementRequired,
				AllowedEvidence:  []EvidenceKind{EvidenceKindTaskEvaluation},
				MinimumCount:     1,
				RequiresInternal: true,
				FreshnessWindow:  time.Hour,
				CompletionWindow: time.Hour,
				Position:         3,
				CreatedAt:        now,
			},
		},
	}
}

type stubEvidenceVerifier struct {
	results map[string]EvidenceVerificationResult
	mutated bool
}

func (v *stubEvidenceVerifier) VerifyWorkflowEvidence(ctx context.Context, input EvidenceVerificationInput) (EvidenceVerificationResult, error) {
	result, ok := v.results[string(input.Kind)+":"+input.TargetID]
	if !ok {
		return EvidenceVerificationResult{Exists: false, Scope: input.Scope}, nil
	}
	return result, nil
}

type stubWorkflowStore struct {
	templates        []WorkflowTemplate
	createdTemplates []WorkflowTemplate
	runs             []WorkflowRun
	stepRecords      []WorkflowStepRecord
	gapDiagnostics   []GapDiagnostic
	nextActions      []NextAction
}

func (s *stubWorkflowStore) CreateWorkflowTemplate(ctx context.Context, template WorkflowTemplate) (WorkflowTemplate, error) {
	s.createdTemplates = append(s.createdTemplates, template)
	s.templates = append(s.templates, template)
	return template, nil
}

func (s *stubWorkflowStore) ReadWorkflowTemplate(ctx context.Context, input ReadTemplateInput) (WorkflowTemplate, error) {
	for _, template := range s.templates {
		if template.Scope == input.Scope && template.ID == input.TemplateID {
			return template, nil
		}
	}
	return WorkflowTemplate{}, nil
}

func (s *stubWorkflowStore) ListWorkflowTemplates(ctx context.Context, input ListTemplatesInput) ([]WorkflowTemplate, error) {
	templates := make([]WorkflowTemplate, 0, len(s.templates))
	for _, template := range s.templates {
		if template.Scope != input.Scope {
			continue
		}
		if input.Status != "" && template.Status != input.Status {
			continue
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (s *stubWorkflowStore) UpdateWorkflowTemplate(ctx context.Context, input UpdateTemplateInput) (WorkflowTemplate, error) {
	for idx := range s.templates {
		if s.templates[idx].Scope == input.Scope && s.templates[idx].ID == input.TemplateID {
			s.templates[idx].Steps = append([]TemplateStep(nil), input.Steps...)
			s.templates[idx].IntegrationKind = input.IntegrationKind
			s.templates[idx].CompletionPolicy = input.CompletionPolicy
			s.templates[idx].Actor = input.Actor
			s.templates[idx].Reason = input.Reason
			s.templates[idx].Metadata = cloneMapForTest(input.Metadata)
			s.templates[idx].UpdatedAt = input.UpdatedAt
			return s.templates[idx], nil
		}
	}
	return WorkflowTemplate{}, nil
}

func (s *stubWorkflowStore) DisableWorkflowTemplate(ctx context.Context, input DisableTemplateInput) (WorkflowTemplate, error) {
	for idx := range s.templates {
		if s.templates[idx].Scope == input.Scope && s.templates[idx].ID == input.TemplateID {
			s.templates[idx].Status = TemplateStatusDisabled
			s.templates[idx].Actor = input.Actor
			s.templates[idx].Reason = input.Reason
			s.templates[idx].UpdatedAt = input.DisabledAt
			s.templates[idx].DisabledAt = input.DisabledAt
			return s.templates[idx], nil
		}
	}
	return WorkflowTemplate{}, nil
}

func (s *stubWorkflowStore) StartWorkflowRun(ctx context.Context, run WorkflowRun) (WorkflowRun, error) {
	for _, existing := range s.runs {
		if existing.Scope == run.Scope && existing.TemplateID == run.TemplateID && existing.IdempotencyKey == run.IdempotencyKey {
			return existing, nil
		}
	}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *stubWorkflowStore) ReadWorkflowRun(ctx context.Context, input ReadRunInput) (WorkflowRun, error) {
	for _, run := range s.runs {
		if run.Scope == input.Scope && run.ID == input.RunID {
			return run, nil
		}
	}
	return WorkflowRun{}, nil
}

func (s *stubWorkflowStore) ListWorkflowRuns(ctx context.Context, input ListRunsInput) ([]WorkflowRun, error) {
	runs := make([]WorkflowRun, 0, len(s.runs))
	for _, run := range s.runs {
		if run.Scope != input.Scope {
			continue
		}
		if input.TemplateID != "" && run.TemplateID != input.TemplateID {
			continue
		}
		if input.Status != "" && run.Status != input.Status {
			continue
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (s *stubWorkflowStore) TransitionWorkflowRun(ctx context.Context, input TransitionRunInput) (WorkflowRun, error) {
	for idx := range s.runs {
		if s.runs[idx].Scope == input.Transition.Scope && s.runs[idx].ID == input.Transition.RunID {
			s.runs[idx].Status = input.Transition.ToStatus
			s.runs[idx].UpdatedAt = input.UpdatedAt
			if input.Transition.ToStatus == RunStatusCompleted {
				s.runs[idx].CompletedAt = input.UpdatedAt
			}
			return s.runs[idx], nil
		}
	}
	return WorkflowRun{}, nil
}

func (s *stubWorkflowStore) RecordWorkflowStep(ctx context.Context, record WorkflowStepRecord) (WorkflowStepRecord, error) {
	s.stepRecords = append(s.stepRecords, record)
	return record, nil
}

func (s *stubWorkflowStore) ListWorkflowStepRecords(ctx context.Context, input ListStepRecordsInput) ([]WorkflowStepRecord, error) {
	records := make([]WorkflowStepRecord, 0, len(s.stepRecords))
	for _, record := range s.stepRecords {
		if record.Scope == input.Scope && record.RunID == input.RunID {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s *stubWorkflowStore) ListWorkflowEvidenceLinks(ctx context.Context, input ListEvidenceLinksInput) ([]EvidenceLink, error) {
	links := make([]EvidenceLink, 0)
	for _, record := range s.stepRecords {
		if record.Scope != input.Scope || record.RunID != input.RunID {
			continue
		}
		for _, link := range record.EvidenceLinks {
			if input.Status != "" && link.Status != input.Status {
				continue
			}
			links = append(links, link)
		}
	}
	return links, nil
}

func (s *stubWorkflowStore) RecordWorkflowGapDiagnostic(ctx context.Context, diagnostic GapDiagnostic) (GapDiagnostic, error) {
	s.gapDiagnostics = append(s.gapDiagnostics, diagnostic)
	return diagnostic, nil
}

func (s *stubWorkflowStore) ListWorkflowGapDiagnostics(ctx context.Context, input ListDiagnosticsInput) ([]GapDiagnostic, error) {
	diagnostics := make([]GapDiagnostic, 0, len(s.gapDiagnostics))
	for _, diagnostic := range s.gapDiagnostics {
		if diagnostic.Scope != input.Scope || diagnostic.RunID != input.RunID {
			continue
		}
		if input.Category != "" && diagnostic.Category != input.Category {
			continue
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics, nil
}

func (s *stubWorkflowStore) RecordWorkflowNextAction(ctx context.Context, action NextAction) (NextAction, error) {
	s.nextActions = append(s.nextActions, action)
	return action, nil
}

func (s *stubWorkflowStore) ResolveWorkflowNextActions(ctx context.Context, scope memory.Scope, runID string, resolvedAt time.Time) error {
	for index := range s.nextActions {
		if s.nextActions[index].Scope == scope && s.nextActions[index].RunID == runID && s.nextActions[index].Status == NextActionStatusOpen {
			s.nextActions[index].Status = NextActionStatusSatisfied
			s.nextActions[index].ResolvedAt = resolvedAt
		}
	}
	return nil
}

func (s *stubWorkflowStore) ListWorkflowNextActions(ctx context.Context, input ListNextActionsInput) ([]NextAction, error) {
	actions := make([]NextAction, 0, len(s.nextActions))
	for _, action := range s.nextActions {
		if action.Scope != input.Scope || action.RunID != input.RunID {
			continue
		}
		if input.Status != "" && action.Status != input.Status {
			continue
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func (s *stubWorkflowStore) SupersedeWorkflowEvidenceLink(ctx context.Context, input SupersedeEvidenceLinkInput) error {
	for recordIndex := range s.stepRecords {
		for linkIndex := range s.stepRecords[recordIndex].EvidenceLinks {
			link := &s.stepRecords[recordIndex].EvidenceLinks[linkIndex]
			if link.Scope == input.Scope && link.ID == input.LinkID {
				link.Status = EvidenceLinkStatusSuperseded
				link.SupersededAt = input.SupersededAt
				return nil
			}
		}
	}
	return nil
}

func (s *stubWorkflowStore) FindWorkflowRetentionEligibleHistory(ctx context.Context, input FindRetentionEligibleHistoryInput) ([]WorkflowRun, error) {
	runs := make([]WorkflowRun, 0)
	for _, run := range s.runs {
		if run.Scope != input.Scope {
			continue
		}
		if run.UpdatedAt.After(input.Cutoff) {
			continue
		}
		if run.Status == RunStatusCompleted || run.Status == RunStatusAbandoned || run.Status == RunStatusExpired {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *stubWorkflowStore) CreateWorkflowRetentionRun(ctx context.Context, run WorkflowRetentionRun) (WorkflowRetentionRun, error) {
	return run, nil
}

func (s *stubWorkflowStore) hasDiagnostic(category DiagnosticCategory) bool {
	for _, diagnostic := range s.gapDiagnostics {
		if diagnostic.Category == category {
			return true
		}
	}
	return false
}

func (s *stubWorkflowStore) hasNextAction(category NextActionCategory) bool {
	for _, action := range s.nextActions {
		if action.Category == category {
			return true
		}
	}
	return false
}

func cloneMapForTest(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
