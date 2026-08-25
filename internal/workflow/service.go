package workflow

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type Store interface {
	CreateWorkflowTemplate(ctx context.Context, template WorkflowTemplate) (WorkflowTemplate, error)
	ReadWorkflowTemplate(ctx context.Context, input ReadTemplateInput) (WorkflowTemplate, error)
	ListWorkflowTemplates(ctx context.Context, input ListTemplatesInput) ([]WorkflowTemplate, error)
	UpdateWorkflowTemplate(ctx context.Context, input UpdateTemplateInput) (WorkflowTemplate, error)
	DisableWorkflowTemplate(ctx context.Context, input DisableTemplateInput) (WorkflowTemplate, error)
	StartWorkflowRun(ctx context.Context, run WorkflowRun) (WorkflowRun, error)
	ReadWorkflowRun(ctx context.Context, input ReadRunInput) (WorkflowRun, error)
	ListWorkflowRuns(ctx context.Context, input ListRunsInput) ([]WorkflowRun, error)
	TransitionWorkflowRun(ctx context.Context, input TransitionRunInput) (WorkflowRun, error)
	RecordWorkflowStep(ctx context.Context, record WorkflowStepRecord) (WorkflowStepRecord, error)
	ListWorkflowStepRecords(ctx context.Context, input ListStepRecordsInput) ([]WorkflowStepRecord, error)
	ListWorkflowEvidenceLinks(ctx context.Context, input ListEvidenceLinksInput) ([]EvidenceLink, error)
	RecordWorkflowGapDiagnostic(ctx context.Context, diagnostic GapDiagnostic) (GapDiagnostic, error)
	ListWorkflowGapDiagnostics(ctx context.Context, input ListDiagnosticsInput) ([]GapDiagnostic, error)
	RecordWorkflowNextAction(ctx context.Context, action NextAction) (NextAction, error)
	ResolveWorkflowNextActions(ctx context.Context, scope memory.Scope, runID string, resolvedAt time.Time) error
	ListWorkflowNextActions(ctx context.Context, input ListNextActionsInput) ([]NextAction, error)
	SupersedeWorkflowEvidenceLink(ctx context.Context, input SupersedeEvidenceLinkInput) error
	FindWorkflowRetentionEligibleHistory(ctx context.Context, input FindRetentionEligibleHistoryInput) ([]WorkflowRun, error)
	CreateWorkflowRetentionRun(ctx context.Context, run WorkflowRetentionRun) (WorkflowRetentionRun, error)
}

type EvidenceVerifier interface {
	VerifyWorkflowEvidence(ctx context.Context, input EvidenceVerificationInput) (EvidenceVerificationResult, error)
}

type ServiceOptions struct {
	Store            Store
	EvidenceVerifier EvidenceVerifier
	Now              func() time.Time
	NewID            func(prefix string) string
	Observer         telemetry.Observer
	Logger           *log.Logger
}

type Service struct {
	store            Store
	evidenceVerifier EvidenceVerifier
	now              func() time.Time
	newID            func(prefix string) string
	observer         telemetry.Observer
	logger           *log.Logger
}

type CreateTemplateInput struct {
	Scope            memory.Scope
	Steps            []TemplateStep
	IntegrationKind  IntegrationKind
	CompletionPolicy CompletionPolicy
	Actor            string
	Reason           string
	Metadata         map[string]any
}

type StartRunInput struct {
	Scope          memory.Scope
	TemplateID     string
	IdempotencyKey string
	Actor          string
	Reason         string
	Metadata       map[string]any
	ExpiresAt      time.Time
}

type RecordStepInput struct {
	Scope         memory.Scope
	RunID         string
	Kind          StepKind
	Actor         string
	Reason        string
	ObservedAt    time.Time
	Metadata      map[string]any
	EvidenceLinks []EvidenceLink
}

type MaterializeGapDiagnosticsInput struct {
	Scope memory.Scope
	RunID string
	Now   time.Time
}

type EvidenceVerificationInput struct {
	Scope    memory.Scope
	Kind     EvidenceKind
	TargetID string
}

type EvidenceVerificationResult struct {
	Exists                bool
	Scope                 memory.Scope
	Hidden                bool
	HasSubject            bool
	HasSufficientEvidence bool
	Contradictory         bool
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func(prefix string) string {
			return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), now().UTC().UnixNano())
		}
	}
	return &Service{store: options.Store, evidenceVerifier: options.EvidenceVerifier, now: now, newID: newID, observer: options.Observer, logger: options.Logger}
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (WorkflowTemplate, error) {
	if s.store == nil {
		return WorkflowTemplate{}, fmt.Errorf("workflow store is not configured")
	}
	now := s.now().UTC()
	scope := input.Scope.Normalized()
	templateID := s.newID("workflow_template")
	steps := normalizeTemplateSteps(input.Steps, templateID, scope, now, s.newID)
	template := WorkflowTemplate{
		ID:               templateID,
		Scope:            scope,
		Status:           TemplateStatusActive,
		Steps:            steps,
		IntegrationKind:  input.IntegrationKind,
		CompletionPolicy: input.CompletionPolicy,
		Actor:            strings.TrimSpace(input.Actor),
		Reason:           strings.TrimSpace(input.Reason),
		Metadata:         cloneMetadata(input.Metadata),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := template.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	created, err := s.store.CreateWorkflowTemplate(ctx, template)
	if err != nil {
		s.recordLifecycle(ctx, "template_create", "error", template.Status, "", "", "", "", "", "")
		return WorkflowTemplate{}, err
	}
	s.recordLifecycle(ctx, "template_create", "ok", created.Status, "", "", "", "", "", "")
	return created, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (WorkflowTemplate, error) {
	if s.store == nil {
		return WorkflowTemplate{}, fmt.Errorf("workflow store is not configured")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = s.now().UTC()
	}
	input.Scope = input.Scope.Normalized()
	input.Actor = strings.TrimSpace(input.Actor)
	input.Reason = strings.TrimSpace(input.Reason)
	input.Metadata = cloneMetadata(input.Metadata)
	input.Steps = normalizeTemplateSteps(input.Steps, input.TemplateID, input.Scope, input.UpdatedAt, s.newID)
	if err := input.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	updated, err := s.store.UpdateWorkflowTemplate(ctx, input)
	if err != nil {
		s.recordLifecycle(ctx, "template_update", "error", "", "", "", "", "", "", "")
		return WorkflowTemplate{}, err
	}
	s.recordLifecycle(ctx, "template_update", "ok", updated.Status, "", "", "", "", "", "")
	return updated, nil
}

func (s *Service) DisableTemplate(ctx context.Context, input DisableTemplateInput) (WorkflowTemplate, error) {
	if s.store == nil {
		return WorkflowTemplate{}, fmt.Errorf("workflow store is not configured")
	}
	if input.DisabledAt.IsZero() {
		input.DisabledAt = s.now().UTC()
	}
	input.Scope = input.Scope.Normalized()
	input.Actor = strings.TrimSpace(input.Actor)
	input.Reason = strings.TrimSpace(input.Reason)
	if err := input.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	disabled, err := s.store.DisableWorkflowTemplate(ctx, input)
	if err != nil {
		s.recordLifecycle(ctx, "template_disable", "error", "", "", "", "", "", "", "")
		return WorkflowTemplate{}, err
	}
	s.recordLifecycle(ctx, "template_disable", "ok", disabled.Status, "", "", "", "", "", "")
	return disabled, nil
}

func (s *Service) ReadTemplate(ctx context.Context, input ReadTemplateInput) (WorkflowTemplate, error) {
	if s.store == nil {
		return WorkflowTemplate{}, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return WorkflowTemplate{}, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadWorkflowTemplate(ctx, input)
}

func (s *Service) ListTemplates(ctx context.Context, input ListTemplatesInput) ([]WorkflowTemplate, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowTemplates(ctx, input)
}

func (s *Service) ReadRun(ctx context.Context, input ReadRunInput) (WorkflowRun, error) {
	if s.store == nil {
		return WorkflowRun{}, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return WorkflowRun{}, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadWorkflowRun(ctx, input)
}

func (s *Service) ListRuns(ctx context.Context, input ListRunsInput) ([]WorkflowRun, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowRuns(ctx, input)
}

func (s *Service) TransitionRun(ctx context.Context, input TransitionRunInput) (WorkflowRun, error) {
	if s.store == nil {
		return WorkflowRun{}, fmt.Errorf("workflow store is not configured")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = s.now().UTC()
	}
	if input.Transition.OccurredAt.IsZero() {
		input.Transition.OccurredAt = input.UpdatedAt
	}
	input.Transition.Scope = input.Transition.Scope.Normalized()
	input.Transition.Actor = strings.TrimSpace(input.Transition.Actor)
	input.Transition.Reason = strings.TrimSpace(input.Transition.Reason)
	if err := input.Validate(); err != nil {
		return WorkflowRun{}, err
	}
	updated, err := s.store.TransitionWorkflowRun(ctx, input)
	if err != nil {
		s.recordLifecycle(ctx, "run_transition", "error", "", input.Transition.ToStatus, "", "", "", "", "")
		return WorkflowRun{}, err
	}
	s.recordLifecycle(ctx, "run_transition", "ok", "", updated.Status, "", "", "", "", "")
	return updated, nil
}

func (s *Service) StartRun(ctx context.Context, input StartRunInput) (WorkflowRun, error) {
	if s.store == nil {
		return WorkflowRun{}, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Scope.Validate(); err != nil {
		return WorkflowRun{}, err
	}
	if err := validateID(input.TemplateID, "workflow template id"); err != nil {
		return WorkflowRun{}, err
	}
	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return WorkflowRun{}, err
	}
	if err := validateActorReason(input.Actor, input.Reason); err != nil {
		return WorkflowRun{}, err
	}
	if err := validateMetadata(input.Metadata, "metadata"); err != nil {
		return WorkflowRun{}, err
	}
	scope := input.Scope.Normalized()
	template, err := s.store.ReadWorkflowTemplate(ctx, ReadTemplateInput{Scope: scope, TemplateID: strings.TrimSpace(input.TemplateID)})
	if err != nil {
		return WorkflowRun{}, err
	}
	if template.ID == "" || template.Status != TemplateStatusActive {
		return WorkflowRun{}, fmt.Errorf("active workflow template is required")
	}
	now := s.now().UTC()
	run := WorkflowRun{
		ID:              s.newID("workflow_run"),
		TemplateID:      template.ID,
		Scope:           scope,
		Status:          RunStatusRunning,
		IntegrationKind: template.IntegrationKind,
		IdempotencyKey:  strings.TrimSpace(input.IdempotencyKey),
		Actor:           strings.TrimSpace(input.Actor),
		Reason:          strings.TrimSpace(input.Reason),
		Metadata:        cloneMetadata(input.Metadata),
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       now,
		ExpiresAt:       input.ExpiresAt.UTC(),
	}
	started, err := s.store.StartWorkflowRun(ctx, run)
	if err != nil {
		s.recordLifecycle(ctx, "run_start", "error", template.Status, "", "", "", "", "", "")
		return WorkflowRun{}, err
	}
	if started.ID == run.ID {
		if err := s.recordNextActionForStep(ctx, started.Scope, started.ID, firstIncompleteStep(template, nil, now)); err != nil {
			return WorkflowRun{}, err
		}
	}
	s.recordLifecycle(ctx, "run_start", "ok", template.Status, started.Status, "", "", "", "", "")
	return started, nil
}

func (s *Service) RecordStep(ctx context.Context, input RecordStepInput) (WorkflowStepRecord, error) {
	if s.store == nil {
		return WorkflowStepRecord{}, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Scope.Validate(); err != nil {
		return WorkflowStepRecord{}, err
	}
	if err := validateID(input.RunID, "workflow run id"); err != nil {
		return WorkflowStepRecord{}, err
	}
	if !input.Kind.Valid() {
		return WorkflowStepRecord{}, fmt.Errorf("workflow step kind %q is invalid", input.Kind)
	}
	if err := validateActorReason(input.Actor, input.Reason); err != nil {
		return WorkflowStepRecord{}, err
	}
	if err := validateMetadata(input.Metadata, "metadata"); err != nil {
		return WorkflowStepRecord{}, err
	}
	scope := input.Scope.Normalized()
	run, err := s.store.ReadWorkflowRun(ctx, ReadRunInput{Scope: scope, RunID: strings.TrimSpace(input.RunID)})
	if err != nil {
		return WorkflowStepRecord{}, err
	}
	if run.ID == "" {
		return WorkflowStepRecord{}, fmt.Errorf("workflow run is required")
	}
	template, err := s.store.ReadWorkflowTemplate(ctx, ReadTemplateInput{Scope: scope, TemplateID: run.TemplateID})
	if err != nil {
		return WorkflowStepRecord{}, err
	}
	templateStep, ok := templateStepByKind(template, input.Kind)
	if !ok {
		return WorkflowStepRecord{}, fmt.Errorf("workflow step kind %q is not in template", input.Kind)
	}
	observedAt := input.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = s.now().UTC()
	}
	links, diagnostics, err := s.normalizeAndVerifyEvidence(ctx, scope, run.ID, input.EvidenceLinks, observedAt)
	if err != nil {
		return WorkflowStepRecord{}, err
	}
	existingLinks, err := s.store.ListWorkflowEvidenceLinks(ctx, ListEvidenceLinksInput{Scope: scope, RunID: run.ID, Status: EvidenceLinkStatusActive})
	if err != nil {
		return WorkflowStepRecord{}, err
	}
	priorRecords, err := s.store.ListWorkflowStepRecords(ctx, ListStepRecordsInput{Scope: scope, RunID: run.ID})
	if err != nil {
		return WorkflowStepRecord{}, err
	}
	result := StepResultRecorded
	if stepPreviouslyRecorded(priorRecords, input.Kind) {
		result = StepResultDuplicate
		diagnostics = append(diagnostics, diagnosticForStep(scope, run.ID, input.Kind, firstEvidenceKind(templateStep), DiagnosticCategoryDuplicate, observedAt, s.newID))
	}
	if !predecessorsSatisfied(template, templateStep, existingLinks, observedAt) {
		result = StepResultOutOfOrder
		diagnostics = append(diagnostics, diagnosticForStep(scope, run.ID, input.Kind, firstEvidenceKind(templateStep), DiagnosticCategoryOutOfOrder, observedAt, s.newID))
	}
	record := WorkflowStepRecord{
		ID:            s.newID("workflow_step"),
		RunID:         run.ID,
		Scope:         scope,
		Kind:          input.Kind,
		Status:        StepStatusSatisfied,
		Result:        result,
		Actor:         strings.TrimSpace(input.Actor),
		Reason:        strings.TrimSpace(input.Reason),
		Metadata:      cloneMetadata(input.Metadata),
		ObservedAt:    observedAt,
		CreatedAt:     observedAt,
		EvidenceLinks: links,
	}
	recorded, err := s.store.RecordWorkflowStep(ctx, record)
	if err != nil {
		s.recordLifecycle(ctx, "step_record", "error", "", run.Status, input.Kind, "", "", "", "")
		return WorkflowStepRecord{}, err
	}
	for _, link := range links {
		s.recordLifecycle(ctx, "evidence_record", "ok", "", run.Status, input.Kind, link.Kind, "", "", "")
	}
	for _, diagnostic := range diagnostics {
		if _, err := s.store.RecordWorkflowGapDiagnostic(ctx, diagnostic); err != nil {
			return WorkflowStepRecord{}, err
		}
		s.recordLifecycle(ctx, "diagnostic", "ok", "", run.Status, diagnostic.StepKind, diagnostic.EvidenceKind, diagnostic.Category, "", "")
	}
	if result == StepResultRecorded {
		allLinks, err := s.store.ListWorkflowEvidenceLinks(ctx, ListEvidenceLinksInput{Scope: scope, RunID: run.ID, Status: EvidenceLinkStatusActive})
		if err != nil {
			return WorkflowStepRecord{}, err
		}
		if allRequiredStepsSatisfied(template, allLinks, observedAt) {
			if err := s.completeRun(ctx, run, observedAt); err != nil {
				return WorkflowStepRecord{}, err
			}
		}
	}
	s.recordLifecycle(ctx, "step_record", "ok", "", run.Status, recorded.Kind, "", "", "", "")
	return recorded, nil
}

func allRequiredStepsSatisfied(template WorkflowTemplate, links []EvidenceLink, now time.Time) bool {
	for _, step := range template.Steps {
		if step.Requirement == StepRequirementRequired && !StepSatisfiedByEvidence(step, links, now) {
			return false
		}
	}
	return true
}

func (s *Service) completeRun(ctx context.Context, run WorkflowRun, completedAt time.Time) error {
	if run.Status != RunStatusRunning {
		return nil
	}
	completed, err := s.TransitionRun(ctx, TransitionRunInput{Transition: WorkflowTransition{
		ID:         s.newID("workflow_transition"),
		RunID:      run.ID,
		Scope:      run.Scope,
		FromStatus: RunStatusRunning,
		ToStatus:   RunStatusCompleted,
		Actor:      "workflow_service",
		Reason:     "required evidence satisfied",
		OccurredAt: completedAt,
	}, UpdatedAt: completedAt})
	if err != nil {
		return err
	}
	if err := s.store.ResolveWorkflowNextActions(ctx, run.Scope, run.ID, completed.UpdatedAt); err != nil {
		return err
	}
	s.recordLifecycle(ctx, "next_action", "ok", "", completed.Status, "", "", "", NextActionNone, "")
	return nil
}

func (s *Service) ListStepRecords(ctx context.Context, input ListStepRecordsInput) ([]WorkflowStepRecord, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowStepRecords(ctx, input)
}

func (s *Service) ListEvidenceLinks(ctx context.Context, input ListEvidenceLinksInput) ([]EvidenceLink, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowEvidenceLinks(ctx, input)
}

func (s *Service) ListDiagnostics(ctx context.Context, input ListDiagnosticsInput) ([]GapDiagnostic, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowGapDiagnostics(ctx, input)
}

func (s *Service) ListNextActions(ctx context.Context, input ListNextActionsInput) ([]NextAction, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListWorkflowNextActions(ctx, input)
}

func (s *Service) SupersedeEvidenceLink(ctx context.Context, input SupersedeEvidenceLinkInput) error {
	if s.store == nil {
		return fmt.Errorf("workflow store is not configured")
	}
	if input.SupersededAt.IsZero() {
		input.SupersededAt = s.now().UTC()
	}
	input.Scope = input.Scope.Normalized()
	input.Actor = strings.TrimSpace(input.Actor)
	input.Reason = strings.TrimSpace(input.Reason)
	if err := input.Validate(); err != nil {
		return err
	}
	return s.store.SupersedeWorkflowEvidenceLink(ctx, input)
}

func (s *Service) MaterializeGapDiagnostics(ctx context.Context, input MaterializeGapDiagnosticsInput) ([]GapDiagnostic, error) {
	if s.store == nil {
		return nil, fmt.Errorf("workflow store is not configured")
	}
	if err := input.Scope.Validate(); err != nil {
		return nil, err
	}
	if err := validateID(input.RunID, "workflow run id"); err != nil {
		return nil, err
	}
	now := input.Now.UTC()
	if now.IsZero() {
		now = s.now().UTC()
	}
	scope := input.Scope.Normalized()
	run, err := s.store.ReadWorkflowRun(ctx, ReadRunInput{Scope: scope, RunID: strings.TrimSpace(input.RunID)})
	if err != nil {
		return nil, err
	}
	if run.ID == "" {
		return nil, fmt.Errorf("workflow run is required")
	}
	template, err := s.store.ReadWorkflowTemplate(ctx, ReadTemplateInput{Scope: scope, TemplateID: run.TemplateID})
	if err != nil {
		return nil, err
	}
	links, err := s.store.ListWorkflowEvidenceLinks(ctx, ListEvidenceLinksInput{Scope: scope, RunID: run.ID, Status: EvidenceLinkStatusActive})
	if err != nil {
		return nil, err
	}
	existingDiagnostics, err := s.store.ListWorkflowGapDiagnostics(ctx, ListDiagnosticsInput{Scope: scope, RunID: run.ID, Limit: 100})
	if err != nil {
		return nil, err
	}
	diagnostics := make([]GapDiagnostic, 0)
	for _, step := range sortedTemplateSteps(template.Steps) {
		if step.Requirement != StepRequirementRequired {
			continue
		}
		if StepSatisfiedByEvidence(step, links, now) {
			continue
		}
		category := DiagnosticCategoryMissing
		if onlyOpaqueEvidence(step, links) && step.RequiresInternal {
			category = DiagnosticCategoryOpaqueOnly
		} else if run.StartedAt.Add(step.CompletionWindow).Before(now) {
			category = DiagnosticCategoryStale
		}
		diagnostic := diagnosticForStep(scope, run.ID, step.Kind, firstEvidenceKind(step), category, now, s.newID)
		if hasOpenDiagnostic(existingDiagnostics, diagnostic) {
			continue
		}
		recorded, err := s.store.RecordWorkflowGapDiagnostic(ctx, diagnostic)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, recorded)
		s.recordLifecycle(ctx, "diagnostic", "ok", "", run.Status, recorded.StepKind, recorded.EvidenceKind, recorded.Category, "", "")
		if err := s.recordNextActionForStep(ctx, scope, run.ID, step); err != nil {
			return nil, err
		}
	}
	return diagnostics, nil
}

func hasOpenDiagnostic(existing []GapDiagnostic, candidate GapDiagnostic) bool {
	for _, diagnostic := range existing {
		if diagnostic.Status == "open" && diagnostic.ResolvedAt.IsZero() && diagnostic.StepKind == candidate.StepKind && diagnostic.EvidenceKind == candidate.EvidenceKind && diagnostic.Category == candidate.Category {
			return true
		}
	}
	return false
}

func (s *Service) normalizeAndVerifyEvidence(ctx context.Context, scope memory.Scope, runID string, input []EvidenceLink, now time.Time) ([]EvidenceLink, []GapDiagnostic, error) {
	links := make([]EvidenceLink, 0, len(input))
	diagnostics := make([]GapDiagnostic, 0)
	for _, link := range input {
		link.RunID = runID
		link.Scope = scope
		if link.ID == "" {
			link.ID = s.newID("workflow_evidence_link")
		}
		if link.Status == "" {
			link.Status = EvidenceLinkStatusActive
		}
		if link.CreatedAt.IsZero() {
			link.CreatedAt = now
		}
		if err := link.Validate(); err != nil {
			return nil, nil, err
		}
		if link.Kind != EvidenceKindOpaque && link.Source != EvidenceSourceOpaque && s.evidenceVerifier != nil {
			result, err := s.evidenceVerifier.VerifyWorkflowEvidence(ctx, EvidenceVerificationInput{
				Scope:    scope,
				Kind:     link.Kind,
				TargetID: link.TargetID,
			})
			if err != nil {
				return nil, nil, err
			}
			category := diagnosticCategoryForEvidenceVerification(scope, result)
			if category == DiagnosticCategoryOutOfScope {
				return nil, nil, fmt.Errorf("workflow evidence link is out of scope")
			}
			if category != "" {
				link.Status = EvidenceLinkStatusInvalid
				diagnostics = append(diagnostics, diagnosticForStep(scope, runID, stepKindForEvidence(link.Kind), link.Kind, category, now, s.newID))
			}
		}
		links = append(links, link)
	}
	return links, diagnostics, nil
}

func normalizeTemplateSteps(steps []TemplateStep, templateID string, scope memory.Scope, now time.Time, newID func(string) string) []TemplateStep {
	normalized := make([]TemplateStep, 0, len(steps))
	for idx, step := range steps {
		if step.ID == "" {
			step.ID = newID("workflow_template_step")
		}
		step.TemplateID = templateID
		step.Scope = scope
		if step.Position == 0 {
			step.Position = idx + 1
		}
		if step.CreatedAt.IsZero() {
			step.CreatedAt = now
		}
		step.Metadata = cloneMetadata(step.Metadata)
		normalized = append(normalized, step)
	}
	return normalized
}

func sortedTemplateSteps(steps []TemplateStep) []TemplateStep {
	sorted := append([]TemplateStep(nil), steps...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Position == sorted[j].Position {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Position < sorted[j].Position
	})
	return sorted
}

func firstIncompleteStep(template WorkflowTemplate, links []EvidenceLink, now time.Time) TemplateStep {
	for _, step := range sortedTemplateSteps(template.Steps) {
		if step.Requirement != StepRequirementRequired {
			continue
		}
		if !StepSatisfiedByEvidence(step, links, now) {
			return step
		}
	}
	return TemplateStep{}
}

func templateStepByKind(template WorkflowTemplate, kind StepKind) (TemplateStep, bool) {
	for _, step := range template.Steps {
		if step.Kind == kind {
			return step, true
		}
	}
	return TemplateStep{}, false
}

func predecessorsSatisfied(template WorkflowTemplate, current TemplateStep, links []EvidenceLink, now time.Time) bool {
	for _, step := range sortedTemplateSteps(template.Steps) {
		if step.Position >= current.Position {
			return true
		}
		if step.Requirement != StepRequirementRequired {
			continue
		}
		if !StepSatisfiedByEvidence(step, links, now) {
			return false
		}
	}
	return true
}

func stepPreviouslyRecorded(records []WorkflowStepRecord, kind StepKind) bool {
	for _, record := range records {
		if record.Kind == kind && record.Result == StepResultRecorded {
			return true
		}
	}
	return false
}

func onlyOpaqueEvidence(step TemplateStep, links []EvidenceLink) bool {
	found := false
	for _, link := range links {
		if link.Status != EvidenceLinkStatusActive {
			continue
		}
		if !evidenceAllowed(step.AllowedEvidence, link.Kind) && link.Kind != EvidenceKindOpaque {
			continue
		}
		if link.Kind != EvidenceKindOpaque && link.Source != EvidenceSourceOpaque {
			return false
		}
		found = true
	}
	return found
}

func diagnosticCategoryForEvidenceVerification(scope memory.Scope, result EvidenceVerificationResult) DiagnosticCategory {
	if !result.Exists {
		return DiagnosticCategoryInvalid
	}
	if result.Scope != (memory.Scope{}) && result.Scope.Normalized() != scope.Normalized() {
		return DiagnosticCategoryOutOfScope
	}
	if result.Hidden {
		return DiagnosticCategoryHidden
	}
	if result.Contradictory {
		return DiagnosticCategoryContradictory
	}
	if !result.HasSubject {
		return DiagnosticCategorySubjectMissing
	}
	if !result.HasSufficientEvidence {
		return DiagnosticCategoryInsufficientEvidence
	}
	return ""
}

func diagnosticForStep(scope memory.Scope, runID string, stepKind StepKind, evidenceKind EvidenceKind, category DiagnosticCategory, now time.Time, newID func(string) string) GapDiagnostic {
	return GapDiagnostic{
		ID:              newID("workflow_gap"),
		RunID:           runID,
		Scope:           scope,
		StepKind:        stepKind,
		EvidenceKind:    evidenceKind,
		Category:        category,
		ReadinessImpact: readinessImpactForDiagnostic(category),
		Status:          "open",
		CreatedAt:       now,
	}
}

func readinessImpactForDiagnostic(category DiagnosticCategory) ReadinessImpact {
	switch category {
	case DiagnosticCategoryOutOfScope, DiagnosticCategoryInvalid, DiagnosticCategoryContradictory:
		return ReadinessImpactBlocked
	case DiagnosticCategoryMissing, DiagnosticCategoryStale, DiagnosticCategoryOutOfOrder,
		DiagnosticCategoryHidden, DiagnosticCategoryOpaqueOnly, DiagnosticCategorySubjectMissing,
		DiagnosticCategoryInsufficientEvidence:
		return ReadinessImpactDegraded
	default:
		return ReadinessImpactUnknown
	}
}

func (s *Service) recordNextActionForStep(ctx context.Context, scope memory.Scope, runID string, step TemplateStep) error {
	if step.ID == "" {
		return nil
	}
	category, route := nextActionForStep(step)
	action := NextAction{
		ID:            s.newID("workflow_next_action"),
		RunID:         runID,
		Scope:         scope,
		Category:      category,
		StepKind:      step.Kind,
		EvidenceKind:  firstEvidenceKind(step),
		RouteCategory: route,
		Status:        NextActionStatusOpen,
		CreatedAt:     s.now().UTC(),
	}
	_, err := s.store.RecordWorkflowNextAction(ctx, action)
	if err == nil {
		s.recordLifecycle(ctx, "next_action", "ok", "", "", step.Kind, firstEvidenceKind(step), "", action.Category, "")
	}
	return err
}

func (s *Service) recordLifecycle(ctx context.Context, operation, result string, templateStatus TemplateStatus, runStatus RunStatus, stepKind StepKind, evidenceKind EvidenceKind, diagnostic DiagnosticCategory, nextAction NextActionCategory, cleanup string) {
	if s.logger != nil {
		s.logger.Printf("component=workflow event=lifecycle operation=%s result=%s template_status=%s run_status=%s step_kind=%s evidence_kind=%s diagnostic_category=%s next_action_category=%s cleanup_category=%s", lifecycleLabel(operation), lifecycleLabel(result), lifecycleLabel(string(templateStatus)), lifecycleLabel(string(runStatus)), lifecycleLabel(string(stepKind)), lifecycleLabel(string(evidenceKind)), lifecycleLabel(string(diagnostic)), lifecycleLabel(string(nextAction)), lifecycleLabel(cleanup))
	}
	observer, ok := s.observer.(interface {
		RecordWorkflowLifecycle(context.Context, telemetry.WorkflowLifecycleEvent)
	})
	if !ok {
		return
	}
	observer.RecordWorkflowLifecycle(ctx, telemetry.WorkflowLifecycleEvent{Operation: operation, Result: result, TemplateStatus: string(templateStatus), RunStatus: string(runStatus), StepKind: string(stepKind), EvidenceKind: string(evidenceKind), DiagnosticCategory: string(diagnostic), NextActionCategory: string(nextAction), CleanupCategory: cleanup})
}

func lifecycleLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func nextActionForStep(step TemplateStep) (NextActionCategory, RouteCategory) {
	switch step.Kind {
	case StepKindSessionStarted:
		return NextActionStartSession, RouteCategoryMemorySessions
	case StepKindContextRequested:
		return NextActionRequestContext, RouteCategoryMemorySessions
	case StepKindTurnOutcomeRecorded:
		return NextActionRecordOutcome, RouteCategoryMemorySessionOutcome
	case StepKindSessionVerificationRecorded:
		return NextActionRecordVerification, RouteCategoryMemorySessionVerification
	case StepKindUsefulnessFeedbackRecorded:
		return NextActionRecordFeedback, RouteCategoryUsefulnessFeedback
	case StepKindTaskEvaluationRecorded:
		return NextActionRecordTaskEvaluation, RouteCategoryTaskEvaluations
	case StepKindQualityChecked:
		return NextActionInspectQuality, RouteCategoryAdminQuality
	case StepKindRepairReviewed:
		return NextActionReviewRepair, RouteCategoryAdminRepair
	case StepKindRankingRolloutChecked:
		return NextActionCheckRankingRollout, RouteCategoryAdminRankingRollouts
	case StepKindConformanceChecked:
		return NextActionRunConformance, RouteCategoryAdminConformance
	case StepKindReadinessChecked:
		return NextActionReadReadiness, RouteCategoryAdminReadiness
	case StepKindRecoveryVerified:
		return NextActionVerifyRecovery, RouteCategoryAdminRecovery
	default:
		return NextActionNone, RouteCategoryNone
	}
}

func firstEvidenceKind(step TemplateStep) EvidenceKind {
	if len(step.AllowedEvidence) == 0 {
		return EvidenceKindOpaque
	}
	return step.AllowedEvidence[0]
}

func stepKindForEvidence(kind EvidenceKind) StepKind {
	switch kind {
	case EvidenceKindSession:
		return StepKindSessionStarted
	case EvidenceKindContext:
		return StepKindContextRequested
	case EvidenceKindOutcome:
		return StepKindTurnOutcomeRecorded
	case EvidenceKindVerification:
		return StepKindSessionVerificationRecorded
	case EvidenceKindUsefulnessFeedback:
		return StepKindUsefulnessFeedbackRecorded
	case EvidenceKindTaskEvaluation:
		return StepKindTaskEvaluationRecorded
	case EvidenceKindQualityFinding:
		return StepKindQualityChecked
	case EvidenceKindRepairPlan:
		return StepKindRepairReviewed
	case EvidenceKindRankingRollout:
		return StepKindRankingRolloutChecked
	case EvidenceKindConformanceRun:
		return StepKindConformanceChecked
	case EvidenceKindReadinessReport:
		return StepKindReadinessChecked
	case EvidenceKindRecoveryVerification:
		return StepKindRecoveryVerified
	default:
		return StepKindContextRequested
	}
}

func cloneMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
