package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ScopeProofStore interface {
	CreateScopeProofRun(ctx context.Context, run ScopeProofRun) (ScopeProofRun, error)
	CreateScopeProofStep(ctx context.Context, step ScopeProofStep) (ScopeProofStep, error)
	ListScopeProofRuns(ctx context.Context, input ListScopeProofRunsInput) ([]ScopeProofRun, error)
	ReadScopeProofRun(ctx context.Context, input ReadScopeProofRunInput) (ScopeProofRun, error)
}

type ScopeProofServiceOptions struct {
	Store ScopeProofStore
	Now   func() time.Time
	NewID func(prefix string) string
}

type ScopeProofService struct {
	store ScopeProofStore
	now   func() time.Time
	newID func(prefix string) string
}

func NewScopeProofService(options ScopeProofServiceOptions) *ScopeProofService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func(prefix string) string {
			return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), now().UnixNano())
		}
	}
	return &ScopeProofService{store: options.Store, now: now, newID: newID}
}

func (s *ScopeProofService) CreateProofRun(ctx context.Context, input CreateScopeProofRunInput) (ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return ScopeProofRun{}, err
	}
	if s.store == nil {
		return ScopeProofRun{}, fmt.Errorf("scope proof store is not configured")
	}
	now := s.now().UTC()
	fixtureMode := input.FixtureMode
	if fixtureMode == "" {
		fixtureMode = ScopeProofFixtureModeSmoke
	}
	run, err := s.store.CreateScopeProofRun(ctx, ScopeProofRun{
		ID:          s.newID("proof"),
		Scope:       input.Scope.Normalized(),
		Status:      ScopeProofStatusPending,
		Verdict:     ScopeProofVerdictPending,
		Checks:      append([]ScopeProofCheck(nil), input.Checks...),
		FixtureMode: fixtureMode,
		Actor:       strings.TrimSpace(input.Actor),
		Reason:      strings.TrimSpace(input.Reason),
		Summary:     map[string]any{},
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return ScopeProofRun{}, err
	}
	return s.createProofSteps(ctx, run)
}

func (s *ScopeProofService) ListProofRuns(ctx context.Context, input ListScopeProofRunsInput) ([]ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("scope proof store is not configured")
	}
	return s.store.ListScopeProofRuns(ctx, input)
}

func (s *ScopeProofService) ReadProofRun(ctx context.Context, input ReadScopeProofRunInput) (ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return ScopeProofRun{}, err
	}
	if s.store == nil {
		return ScopeProofRun{}, fmt.Errorf("scope proof store is not configured")
	}
	return s.store.ReadScopeProofRun(ctx, input)
}

type ScopeProofReport struct {
	Run         ScopeProofRun      `json:"run"`
	Evidence    LoopReportEvidence `json:"evidence,omitempty"`
	NextActions []string           `json:"next_actions,omitempty"`
}

type LoopReportEvidence struct {
	QualityEvaluationIDs []string               `json:"quality_evaluation_ids,omitempty"`
	ReplayRunIDs         []string               `json:"replay_run_ids,omitempty"`
	RepairPlanIDs        []string               `json:"repair_plan_ids,omitempty"`
	FailureCategories    []ProofFailureCategory `json:"failure_categories,omitempty"`
}

func (s *ScopeProofService) ReadProofReport(ctx context.Context, input ReadScopeProofRunInput) (ScopeProofReport, error) {
	run, err := s.ReadProofRun(ctx, input)
	if err != nil {
		return ScopeProofReport{}, err
	}
	evidence := loopReportEvidenceFromProof(run)
	return ScopeProofReport{
		Run:         run,
		Evidence:    evidence,
		NextActions: scopeProofNextActions(run, evidence),
	}, nil
}

type RerunScopeProofRunInput struct {
	Scope   Scope
	ProofID string
	Actor   string
	Reason  string
}

func (i RerunScopeProofRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.ProofID) == "" {
		return fmt.Errorf("proof id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	return nil
}

func (s *ScopeProofService) RerunProofRun(ctx context.Context, input RerunScopeProofRunInput) (ScopeProofRun, error) {
	if err := input.Validate(); err != nil {
		return ScopeProofRun{}, err
	}
	if s.store == nil {
		return ScopeProofRun{}, fmt.Errorf("scope proof store is not configured")
	}
	previous, err := s.store.ReadScopeProofRun(ctx, ReadScopeProofRunInput{Scope: input.Scope, ProofID: input.ProofID})
	if err != nil {
		return ScopeProofRun{}, err
	}
	now := s.now().UTC()
	run, err := s.store.CreateScopeProofRun(ctx, ScopeProofRun{
		ID:          s.newID("proof"),
		Scope:       input.Scope.Normalized(),
		Status:      ScopeProofStatusPending,
		Verdict:     ScopeProofVerdictPending,
		Checks:      append([]ScopeProofCheck(nil), previous.Checks...),
		FixtureMode: previous.FixtureMode,
		Actor:       strings.TrimSpace(input.Actor),
		Reason:      strings.TrimSpace(input.Reason),
		RerunOf:     previous.ID,
		Summary:     map[string]any{},
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return ScopeProofRun{}, err
	}
	return s.createProofSteps(ctx, run)
}

func scopeProofNextActions(run ScopeProofRun, evidence LoopReportEvidence) []string {
	if run.Verdict == ScopeProofVerdictPassed {
		return []string{"no_action_required"}
	}
	actions := make([]string, 0)
	if run.FailureCategory != "" {
		actions = append(actions, nextActionsForFailureCategory(run.FailureCategory)...)
	}
	for _, category := range evidence.FailureCategories {
		actions = append(actions, nextActionsForFailureCategory(category)...)
	}
	if len(evidence.QualityEvaluationIDs) > 0 {
		actions = append(actions, "open_quality_evaluation")
	}
	if len(evidence.RepairPlanIDs) > 0 {
		actions = append(actions, "open_repair_plan")
	}
	if len(evidence.ReplayRunIDs) > 0 {
		actions = append(actions, "inspect_derived_insight_replay")
	}
	actions = uniqueStrings(actions)
	if len(actions) > 0 {
		return actions
	}
	return []string{"inspect_proof_steps"}
}

func loopReportEvidenceFromProof(run ScopeProofRun) LoopReportEvidence {
	var evidence LoopReportEvidence
	if run.FailureCategory != "" {
		evidence.FailureCategories = append(evidence.FailureCategories, run.FailureCategory)
	}
	for _, step := range run.Steps {
		if step.FailureCategory != "" {
			evidence.FailureCategories = append(evidence.FailureCategories, step.FailureCategory)
		}
		if id := strings.TrimSpace(stringEvidenceValue(step.Evidence, "evaluation_run_id")); id != "" {
			evidence.QualityEvaluationIDs = append(evidence.QualityEvaluationIDs, id)
		}
		if id := strings.TrimSpace(stringEvidenceValue(step.Evidence, "replay_run_id")); id != "" {
			evidence.ReplayRunIDs = append(evidence.ReplayRunIDs, id)
		}
		if id := strings.TrimSpace(stringEvidenceValue(step.Evidence, "repair_plan_id")); id != "" {
			evidence.RepairPlanIDs = append(evidence.RepairPlanIDs, id)
		}
	}
	evidence.QualityEvaluationIDs = uniqueStrings(evidence.QualityEvaluationIDs)
	evidence.ReplayRunIDs = uniqueStrings(evidence.ReplayRunIDs)
	evidence.RepairPlanIDs = uniqueStrings(evidence.RepairPlanIDs)
	evidence.FailureCategories = uniqueFailureCategories(evidence.FailureCategories)
	return evidence
}

func stringEvidenceValue(evidence map[string]any, key string) string {
	if evidence == nil {
		return ""
	}
	value, _ := evidence[key].(string)
	return value
}

func nextActionsForFailureCategory(category ProofFailureCategory) []string {
	switch category {
	case ProofFailureCategoryIngestion:
		return []string{"inspect_event_admission", "inspect_governance_status"}
	case ProofFailureCategoryGovernance:
		return []string{"inspect_governance_status", "inspect_worker_jobs"}
	case ProofFailureCategoryRetrieval:
		return []string{"inspect_retrieval_quality", "open_quality_evaluation", "open_repair_plan"}
	case ProofFailureCategoryContext:
		return []string{"inspect_context_diagnostics", "open_quality_evaluation", "open_repair_plan"}
	case ProofFailureCategoryReplay:
		return []string{"inspect_derived_insight_replay"}
	case ProofFailureCategoryQuality:
		return []string{"open_quality_evaluation"}
	case ProofFailureCategoryRepair:
		return []string{"open_repair_plan"}
	case ProofFailureCategoryWorker:
		return []string{"inspect_worker_jobs"}
	case ProofFailureCategoryScope:
		return []string{"inspect_scope_configuration"}
	default:
		return []string{"inspect_proof_steps"}
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func uniqueFailureCategories(values []ProofFailureCategory) []ProofFailureCategory {
	seen := map[ProofFailureCategory]struct{}{}
	out := make([]ProofFailureCategory, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *ScopeProofService) createProofSteps(ctx context.Context, run ScopeProofRun) (ScopeProofRun, error) {
	steps := make([]ScopeProofStep, 0, len(defaultScopeProofStepPlan()))
	for _, stepName := range defaultScopeProofStepPlan() {
		step, err := s.store.CreateScopeProofStep(ctx, ScopeProofStep{
			ID:        s.newID("proof_step"),
			ProofID:   run.ID,
			Scope:     run.Scope,
			Step:      stepName,
			Status:    ScopeProofStepStatusPending,
			Evidence:  map[string]any{},
			CreatedAt: run.CreatedAt,
			UpdatedAt: run.UpdatedAt,
		})
		if err != nil {
			return ScopeProofRun{}, err
		}
		steps = append(steps, step)
	}
	run.Steps = steps
	return run, nil
}

func defaultScopeProofStepPlan() []ScopeProofStepName {
	return []ScopeProofStepName{
		ScopeProofStepScopeResolved,
		ScopeProofStepFixturePlanned,
		ScopeProofStepIngestion,
		ScopeProofStepGovernanceProcessed,
		ScopeProofStepRetrievalRecalled,
		ScopeProofStepContextAssembled,
		ScopeProofStepReplayChecked,
		ScopeProofStepQualityEvaluated,
		ScopeProofStepRepairRecommended,
		ScopeProofStepCompleted,
	}
}
