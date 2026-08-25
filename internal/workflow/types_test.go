package workflow

import (
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestWorkflowTemplateRejectsUnboundedStepAndEvidenceKinds(t *testing.T) {
	now := time.Date(2026, 7, 18, 15, 0, 0, 0, time.UTC)
	template := WorkflowTemplate{
		ID:     "workflow_template_1",
		Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status: TemplateStatusActive,
		Steps: []TemplateStep{
			{
				ID:               "template_step_1",
				TemplateID:       "workflow_template_1",
				Scope:            memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Kind:             StepKind("free_form_step"),
				Requirement:      StepRequirementRequired,
				AllowedEvidence:  []EvidenceKind{EvidenceKindSession},
				MinimumCount:     1,
				FreshnessWindow:  time.Hour,
				CompletionWindow: time.Hour,
				Position:         1,
				CreatedAt:        now,
			},
		},
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "operator-a",
		Reason:           "prove integration workflow",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := template.Validate(); err == nil || !strings.Contains(err.Error(), "workflow step kind") {
		t.Fatalf("Validate() error = %v, want invalid step kind", err)
	}

	template.Steps[0].Kind = StepKindSessionStarted
	template.Steps[0].AllowedEvidence = []EvidenceKind{EvidenceKind("free_form_evidence")}
	if err := template.Validate(); err == nil || !strings.Contains(err.Error(), "workflow evidence kind") {
		t.Fatalf("Validate() error = %v, want invalid evidence kind", err)
	}
}

func TestWorkflowRunRequiresScopeTemplateAndIdempotency(t *testing.T) {
	now := time.Date(2026, 7, 18, 15, 10, 0, 0, time.UTC)
	run := WorkflowRun{
		ID:              "workflow_run_1",
		TemplateID:      "workflow_template_1",
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status:          RunStatusRunning,
		IntegrationKind: IntegrationKindAgentTurn,
		IdempotencyKey:  "turn-123",
		Actor:           "agent-a",
		Reason:          "serve user turn",
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       now,
	}
	if err := run.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	run.IdempotencyKey = ""
	if err := run.Validate(); err == nil || !strings.Contains(err.Error(), "idempotency key") {
		t.Fatalf("Validate() error = %v, want idempotency key validation", err)
	}
}

func TestStepRecordRejectsOutOfScopeEvidenceLink(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 15, 20, 0, 0, time.UTC)
	step := WorkflowStepRecord{
		ID:         "workflow_step_1",
		RunID:      "workflow_run_1",
		Scope:      scope,
		Kind:       StepKindContextRequested,
		Status:     StepStatusSatisfied,
		Result:     StepResultRecorded,
		Actor:      "agent-a",
		Reason:     "context assembled",
		ObservedAt: now,
		CreatedAt:  now,
		EvidenceLinks: []EvidenceLink{
			{
				ID:           "evidence_link_1",
				RunID:        "workflow_run_1",
				StepRecordID: "workflow_step_1",
				Scope:        memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"},
				Kind:         EvidenceKindContext,
				Status:       EvidenceLinkStatusActive,
				Source:       EvidenceSourcePublicAPI,
				TargetID:     "turn_1",
				CreatedAt:    now,
			},
		},
	}

	if err := step.Validate(); err == nil || !strings.Contains(err.Error(), "evidence link scope") {
		t.Fatalf("Validate() error = %v, want evidence link scope mismatch", err)
	}
}

func TestOpaqueEvidenceCannotSatisfyInternalRequirement(t *testing.T) {
	step := TemplateStep{
		ID:               "template_step_1",
		TemplateID:       "workflow_template_1",
		Scope:            memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Kind:             StepKindTaskEvaluationRecorded,
		Requirement:      StepRequirementRequired,
		AllowedEvidence:  []EvidenceKind{EvidenceKindTaskEvaluation},
		MinimumCount:     1,
		RequiresInternal: true,
		FreshnessWindow:  time.Hour,
		CompletionWindow: time.Hour,
		Position:         1,
		CreatedAt:        time.Date(2026, 7, 18, 15, 30, 0, 0, time.UTC),
	}
	links := []EvidenceLink{{
		ID:          "evidence_link_1",
		RunID:       "workflow_run_1",
		Scope:       step.Scope,
		Kind:        EvidenceKindOpaque,
		Status:      EvidenceLinkStatusActive,
		Source:      EvidenceSourceOpaque,
		OpaqueToken: "caller-token",
		CreatedAt:   step.CreatedAt,
	}}

	if StepSatisfiedByEvidence(step, links, step.CreatedAt.Add(time.Minute)) {
		t.Fatal("StepSatisfiedByEvidence() = true, want opaque evidence rejected for internal requirement")
	}
}

func TestWorkflowDiagnosticAndNextActionValidateBoundedCategories(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 15, 40, 0, 0, time.UTC)
	diagnostic := GapDiagnostic{
		ID:              "workflow_gap_1",
		RunID:           "workflow_run_1",
		Scope:           scope,
		StepKind:        StepKindUsefulnessFeedbackRecorded,
		EvidenceKind:    EvidenceKindUsefulnessFeedback,
		Category:        DiagnosticCategorySubjectMissing,
		ReadinessImpact: ReadinessImpactDegraded,
		CreatedAt:       now,
	}
	if err := diagnostic.Validate(); err != nil {
		t.Fatalf("GapDiagnostic.Validate() error = %v", err)
	}

	action := NextAction{
		ID:            "workflow_next_action_1",
		RunID:         "workflow_run_1",
		Scope:         scope,
		Category:      NextActionRecordFeedback,
		StepKind:      StepKindUsefulnessFeedbackRecorded,
		EvidenceKind:  EvidenceKindUsefulnessFeedback,
		RouteCategory: RouteCategoryUsefulnessFeedback,
		Status:        NextActionStatusOpen,
		CreatedAt:     now,
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("NextAction.Validate() error = %v", err)
	}

	action.Category = NextActionCategory("free_form_action")
	if err := action.Validate(); err == nil || !strings.Contains(err.Error(), "next action category") {
		t.Fatalf("NextAction.Validate() error = %v, want invalid next action category", err)
	}
}

func TestWorkflowRepositoryInputsValidateScopeAndBoundedFilters(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 16, 0, 0, 0, time.UTC)

	if err := (UpdateTemplateInput{
		Scope:            scope,
		TemplateID:       "workflow_template_1",
		Steps:            []TemplateStep{validTemplateStep(scope, "workflow_template_1", now)},
		IntegrationKind:  IntegrationKindAgentTurn,
		CompletionPolicy: CompletionPolicyStrict,
		Actor:            "admin-a",
		Reason:           "update workflow contract",
		UpdatedAt:        now,
	}).Validate(); err != nil {
		t.Fatalf("UpdateTemplateInput.Validate() error = %v", err)
	}

	input := ListRunsInput{Scope: scope, Status: RunStatus("free_form_status"), Limit: 10}
	if err := input.Validate(); err == nil || !strings.Contains(err.Error(), "workflow run status") {
		t.Fatalf("ListRunsInput.Validate() error = %v, want invalid status", err)
	}

	if err := (FindRetentionEligibleHistoryInput{
		Scope:          scope,
		RecordCategory: RetentionClassDiagnostic,
		Cutoff:         now.Add(-24 * time.Hour),
		Limit:          100,
	}).Validate(); err != nil {
		t.Fatalf("FindRetentionEligibleHistoryInput.Validate() error = %v", err)
	}
}

func validTemplateStep(scope memory.Scope, templateID string, now time.Time) TemplateStep {
	return TemplateStep{
		ID:               "template_step_1",
		TemplateID:       templateID,
		Scope:            scope,
		Kind:             StepKindSessionStarted,
		Requirement:      StepRequirementRequired,
		AllowedEvidence:  []EvidenceKind{EvidenceKindSession},
		MinimumCount:     1,
		FreshnessWindow:  time.Hour,
		CompletionWindow: time.Hour,
		Position:         1,
		CreatedAt:        now,
	}
}
