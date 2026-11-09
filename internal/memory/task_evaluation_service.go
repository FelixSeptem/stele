package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type TaskEvaluationStore interface {
	CreateTaskEvaluation(ctx context.Context, evaluation TaskEvaluation) (TaskEvaluation, error)
	ReadTaskEvaluation(ctx context.Context, input ReadTaskEvaluationInput) (TaskEvaluation, error)
	ListTaskEvaluations(ctx context.Context, input ListTaskEvaluationsInput) ([]TaskEvaluation, error)
	SupersedeTaskEvaluation(ctx context.Context, input SupersedeTaskEvaluationInput) error
	SummarizeTaskEvaluations(ctx context.Context, input SummarizeTaskEvaluationsInput) (TaskEvaluationSummary, error)
}

// ProceduralVerificationEvidenceReader resolves the immutable verification
// record through its existing exact-scope access boundary. Task verdicts only
// cite the record; they cannot use it to mutate verification or memory state.
type ProceduralVerificationEvidenceReader interface {
	ReadProceduralMemoryVerification(ctx context.Context, scope Scope, verificationID string) (ProceduralMemoryVerification, error)
}

type TaskEvaluationServiceOptions struct {
	Store                          TaskEvaluationStore
	ProceduralVerificationEvidence ProceduralVerificationEvidenceReader
	Now                            func() time.Time
	NewID                          func(prefix string) string
}

type TaskEvaluationService struct {
	store                          TaskEvaluationStore
	proceduralVerificationEvidence ProceduralVerificationEvidenceReader
	now                            func() time.Time
	newID                          func(prefix string) string
}

func NewTaskEvaluationService(options TaskEvaluationServiceOptions) *TaskEvaluationService {
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
	return &TaskEvaluationService{
		store:                          options.Store,
		proceduralVerificationEvidence: options.ProceduralVerificationEvidence,
		now:                            now,
		newID:                          newID,
	}
}

func (s *TaskEvaluationService) CreateTaskEvaluation(ctx context.Context, input TaskEvaluation) (TaskEvaluation, error) {
	if err := input.Validate(); err != nil {
		return TaskEvaluation{}, err
	}
	if s.store == nil {
		return TaskEvaluation{}, fmt.Errorf("task evaluation store is not configured")
	}
	if err := s.validateProceduralVerificationEvidence(ctx, input); err != nil {
		return TaskEvaluation{}, err
	}
	now := s.now().UTC()
	if strings.TrimSpace(input.ID) == "" {
		input.ID = s.newID("task_eval")
	}
	input.Scope = input.Scope.Normalized()
	input.CreatedAt = now
	input.UpdatedAt = now
	return s.store.CreateTaskEvaluation(ctx, input)
}

func (s *TaskEvaluationService) validateProceduralVerificationEvidence(ctx context.Context, evaluation TaskEvaluation) error {
	for _, evidence := range evaluation.Evidence {
		if evidence.Kind != TaskEvidenceTargetProceduralVerification {
			continue
		}
		if s.proceduralVerificationEvidence == nil {
			return fmt.Errorf("procedural verification evidence reader is not configured")
		}
		record, err := s.proceduralVerificationEvidence.ReadProceduralMemoryVerification(ctx, evaluation.Scope.Normalized(), strings.TrimSpace(evidence.ID))
		if err != nil {
			return err
		}
		if record.Scope.Normalized() != evaluation.Scope.Normalized() || strings.TrimSpace(record.ID) != strings.TrimSpace(evidence.ID) {
			return ErrNotFound
		}
	}
	return nil
}

func (s *TaskEvaluationService) ReadTaskEvaluation(ctx context.Context, input ReadTaskEvaluationInput) (TaskEvaluation, error) {
	if err := input.Validate(); err != nil {
		return TaskEvaluation{}, err
	}
	if s.store == nil {
		return TaskEvaluation{}, fmt.Errorf("task evaluation store is not configured")
	}
	return s.store.ReadTaskEvaluation(ctx, input)
}

func (s *TaskEvaluationService) ListTaskEvaluations(ctx context.Context, input ListTaskEvaluationsInput) ([]TaskEvaluation, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("task evaluation store is not configured")
	}
	return s.store.ListTaskEvaluations(ctx, input)
}

func (s *TaskEvaluationService) SupersedeTaskEvaluation(ctx context.Context, input SupersedeTaskEvaluationInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if s.store == nil {
		return fmt.Errorf("task evaluation store is not configured")
	}
	return s.store.SupersedeTaskEvaluation(ctx, input)
}

func (s *TaskEvaluationService) SummarizeTaskEvaluations(ctx context.Context, input SummarizeTaskEvaluationsInput) (TaskEvaluationSummary, error) {
	if err := input.Validate(); err != nil {
		return TaskEvaluationSummary{}, err
	}
	if s.store == nil {
		return TaskEvaluationSummary{}, fmt.Errorf("task evaluation store is not configured")
	}
	return s.store.SummarizeTaskEvaluations(ctx, input)
}
