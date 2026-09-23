package memory

import (
	"context"
	"fmt"
	"strings"
)

// ContextCalibrationFeedbackSource and ContextCalibrationTaskSource expose
// bounded, exact-scope durable evidence. They intentionally do not expose
// free-form feedback text to a request-time caller.
type ContextCalibrationFeedbackSource interface {
	ListUsefulnessFeedback(context.Context, ListUsefulnessFeedbackInput) ([]UsefulnessFeedback, error)
}

type ContextCalibrationTaskSource interface {
	ListTaskEvaluations(context.Context, ListTaskEvaluationsInput) ([]TaskEvaluation, error)
}

type ContextCalibrationSummaryWriter interface {
	UpsertContextCalibrationSummary(context.Context, ContextCalibrationSummary) (ContextCalibrationSummary, error)
}

// ContextCalibrationSummaryRebuildService converts durable source evidence to
// bounded, derived signals. The derived summary contains aggregates only; it
// never persists feedback reason text, actors, source IDs, or subject IDs.
type ContextCalibrationSummaryRebuildService struct {
	Feedback ContextCalibrationFeedbackSource
	Tasks    ContextCalibrationTaskSource
	Store    ContextCalibrationSummaryWriter
	Limit    int
}

func (s ContextCalibrationSummaryRebuildService) BuildContextCalibrationSummary(ctx context.Context, input ContextCalibrationBuildInput) (ContextCalibrationSummary, error) {
	if err := input.Scope.Validate(); err != nil {
		return ContextCalibrationSummary{}, err
	}
	if s.Feedback == nil || s.Tasks == nil || s.Store == nil {
		return ContextCalibrationSummary{}, fmt.Errorf("context calibration rebuild dependencies are required")
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		return ContextCalibrationSummary{}, fmt.Errorf("context calibration rebuild limit exceeds hard bound")
	}
	scope := input.Scope.Normalized()
	feedback, err := s.Feedback.ListUsefulnessFeedback(ctx, ListUsefulnessFeedbackInput{Scope: scope, Limit: limit})
	if err != nil {
		return ContextCalibrationSummary{}, fmt.Errorf("list context calibration feedback: %w", err)
	}
	tasks, err := s.Tasks.ListTaskEvaluations(ctx, ListTaskEvaluationsInput{Scope: scope, Limit: limit})
	if err != nil {
		return ContextCalibrationSummary{}, fmt.Errorf("list context calibration task evaluations: %w", err)
	}
	signals := make([]ContextCalibrationSignal, 0, minContextCalibrationSignalCapacity(limit, len(feedback)+len(tasks)))
	for _, item := range feedback {
		if item.Scope.Normalized() != scope || !item.SupersededAt.IsZero() {
			continue
		}
		weight, include := contextCalibrationFeedbackWeight(item.Type)
		if !include {
			continue
		}
		for _, subject := range item.Subjects {
			if subject.Kind != UsefulnessFeedbackSubjectMemory || strings.TrimSpace(subject.ID) == "" {
				continue
			}
			signals = append(signals, ContextCalibrationSignal{Scope: scope, SubjectKey: subject.ID, Weight: weight, Confidence: 1, CreatedAt: item.CreatedAt})
			if len(signals) == limit {
				break
			}
		}
		if len(signals) == limit {
			break
		}
	}
	if len(signals) < limit {
		for _, item := range tasks {
			if item.Scope.Normalized() != scope || item.CorrectionState == TaskEvaluationCorrectionStateSuperseded || !item.SupersededAt.IsZero() {
				continue
			}
			weight, include := contextCalibrationTaskWeight(item.Verdict)
			if !include {
				continue
			}
			for _, evidence := range item.Evidence {
				if evidence.Kind != TaskEvidenceTargetMemory || strings.TrimSpace(evidence.ID) == "" {
					continue
				}
				signals = append(signals, ContextCalibrationSignal{Scope: scope, SubjectKey: evidence.ID, Weight: weight, Confidence: 1, CreatedAt: item.CreatedAt})
				if len(signals) == limit {
					break
				}
			}
			if len(signals) == limit {
				break
			}
		}
	}
	input.Scope = scope
	input.Signals = signals
	summary, err := BuildContextCalibrationSummary(input)
	if err != nil {
		return ContextCalibrationSummary{}, err
	}
	return s.Store.UpsertContextCalibrationSummary(ctx, summary)
}

func contextCalibrationFeedbackWeight(kind UsefulnessFeedbackType) (float64, bool) {
	switch kind {
	case UsefulnessFeedbackTypeUseful:
		return 1, true
	case UsefulnessFeedbackTypeIrrelevant, UsefulnessFeedbackTypeNoisy, UsefulnessFeedbackTypeStale, UsefulnessFeedbackTypeMissingExpected, UsefulnessFeedbackTypeUnsafeOrHidden:
		return -1, true
	default:
		return 0, false
	}
}

func contextCalibrationTaskWeight(verdict TaskEvaluationVerdict) (float64, bool) {
	switch verdict {
	case TaskEvaluationVerdictSucceeded:
		return 1, true
	case TaskEvaluationVerdictFailed, TaskEvaluationVerdictPartial:
		return -1, true
	default:
		return 0, false
	}
}

func minContextCalibrationSignalCapacity(limit, available int) int {
	if available < limit {
		return available
	}
	return limit
}
