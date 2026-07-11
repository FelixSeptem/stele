package insights

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
)

type ReplayStore interface {
	ListFailureEvidence(ctx context.Context, scope memory.Scope, limit int) ([]FailureEvidence, error)
	UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error)
	SummarizeDerivedInsightFeedback(ctx context.Context, input memory.SummarizeDerivedInsightFeedbackInput) (memory.DerivedInsightFeedbackSummary, error)
	TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error
	CreateDerivedInsightReplayRun(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayRun, error)
	FindDerivedInsightReplayRunByIdempotencyKey(ctx context.Context, input memory.FindDerivedInsightReplayRunByIdempotencyKeyInput) (memory.DerivedInsightReplayRun, error)
	ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error)
	ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error)
	UpdateDerivedInsightReplayRunStatus(ctx context.Context, input memory.UpdateDerivedInsightReplayRunStatusInput) error
	StoreDerivedInsightReplayReport(ctx context.Context, report memory.DerivedInsightReplayReport) error
}

type ReplayService struct {
	Store           ReplayStore
	MinimumEvidence int
	Now             func() time.Time
	NewRunID        func() string
	Observer        derivedInsightReplayObserver
}

type derivedInsightReplayObserver interface {
	RecordDerivedInsightReplay(ctx context.Context, event telemetry.DerivedInsightReplayEvent)
}

func (s ReplayService) PlanDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayReport, error) {
	result := "completed"
	defer func() {
		if result != "completed" {
			s.recordReplayMetric(ctx, input.Mode, result, memory.DerivedInsightReplayReport{})
		}
	}()
	if err := input.Validate(); err != nil {
		result = "failed"
		return memory.DerivedInsightReplayReport{}, err
	}
	if s.Store == nil {
		result = "failed"
		return memory.DerivedInsightReplayReport{}, fmt.Errorf("derived insight replay store is required")
	}

	current := s.now()
	evidence, err := s.Store.ListFailureEvidence(ctx, input.Scope, input.EvidenceLimit)
	if err != nil {
		result = "failed"
		return memory.DerivedInsightReplayReport{}, err
	}
	evidence = filterReplayEvidenceWindow(evidence, input.EvidenceWindowStart, input.EvidenceWindowEnd)

	evaluator := FailurePatternEvaluator{
		MinimumEvidence: s.MinimumEvidence,
		Window:          input.EvidenceWindowEnd.Sub(input.EvidenceWindowStart),
		Now:             func() time.Time { return current },
	}
	patterns, err := evaluator.Evaluate(input.Scope, evidence)
	if err != nil {
		result = "failed"
		return memory.DerivedInsightReplayReport{}, err
	}

	report := memory.DerivedInsightReplayReport{
		RunID: "dry_run",
		Scope: input.Scope,
		Counters: memory.DerivedInsightReplayCounters{
			EvidenceEvaluated: len(evidence),
		},
		GeneratedAt: current,
	}

	requested := requestedReplayInsightTypes(input.InsightTypes)
	if requested[memory.DerivedInsightTypeFailurePattern] {
		for _, pattern := range patterns {
			report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
				InsightID:     pattern.ID,
				InsightType:   pattern.Type,
				Fingerprint:   pattern.Derivation.Fingerprint,
				Decision:      memory.DerivedInsightReplayDecisionCreate,
				Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: len(pattern.Evidence),
			})
			report.Counters.Created++
		}
	}
	if requested[memory.DerivedInsightTypeLesson] {
		for _, pattern := range patterns {
			lesson, err := ProjectLesson(pattern)
			if err != nil {
				result = "failed"
				return memory.DerivedInsightReplayReport{}, err
			}
			report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
				InsightID:     lesson.ID,
				InsightType:   lesson.Type,
				Fingerprint:   lesson.Derivation.Fingerprint,
				Decision:      memory.DerivedInsightReplayDecisionCreate,
				Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: len(lesson.Evidence),
			})
			report.Counters.Created++
		}
	}
	if len(evidence) > 0 && len(report.Decisions) == 0 {
		report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
			InsightType:   memory.DerivedInsightTypeFailurePattern,
			Fingerprint:   "failure_pattern:insufficient_evidence",
			Decision:      memory.DerivedInsightReplayDecisionSkip,
			Reason:        memory.DerivedInsightReplayReasonInsufficientEvidence,
			EvidenceCount: len(evidence),
		})
		report.Counters.Skipped++
	}

	if err := report.Validate(); err != nil {
		result = "failed"
		return memory.DerivedInsightReplayReport{}, err
	}
	s.recordReplayMetric(ctx, input.Mode, result, report)
	return report, nil
}

func (s ReplayService) ApplyDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayRun, error) {
	if input.Mode != memory.DerivedInsightReplayModeApply {
		input.Mode = memory.DerivedInsightReplayModeApply
	}
	if err := input.Validate(); err != nil {
		return memory.DerivedInsightReplayRun{}, err
	}
	if s.Store == nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("derived insight replay store is required")
	}
	if strings.TrimSpace(input.IdempotencyKey) != "" {
		existing, err := s.Store.FindDerivedInsightReplayRunByIdempotencyKey(ctx, memory.FindDerivedInsightReplayRunByIdempotencyKeyInput{
			Scope:          input.Scope,
			IdempotencyKey: input.IdempotencyKey,
		})
		if err == nil && strings.TrimSpace(existing.ID) != "" {
			return existing, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return memory.DerivedInsightReplayRun{}, err
		}
	}

	now := s.now()
	input.RequestedAt = now
	run := memory.DerivedInsightReplayRun{
		ID:        s.newRunID(),
		Scope:     input.Scope,
		Mode:      input.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   input,
		Actor:     input.Actor,
		Reason:    input.Reason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.Store.CreateDerivedInsightReplayRun(ctx, run)
}

func (s ReplayService) ExecuteDerivedInsightReplay(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayReport, error) {
	if err := run.Validate(); err != nil {
		s.recordReplayMetric(ctx, run.Mode, "failed", memory.DerivedInsightReplayReport{})
		return memory.DerivedInsightReplayReport{}, err
	}
	if s.Store == nil {
		s.recordReplayMetric(ctx, run.Mode, "failed", memory.DerivedInsightReplayReport{})
		return memory.DerivedInsightReplayReport{}, fmt.Errorf("derived insight replay store is required")
	}

	current := s.now()
	if err := s.Store.UpdateDerivedInsightReplayRunStatus(ctx, memory.UpdateDerivedInsightReplayRunStatusInput{
		Scope:     run.Scope,
		RunID:     run.ID,
		Status:    memory.DerivedInsightReplayStatusRunning,
		UpdatedAt: current,
		StartedAt: current,
	}); err != nil {
		s.recordReplayMetric(ctx, run.Mode, "failed", memory.DerivedInsightReplayReport{})
		return memory.DerivedInsightReplayReport{}, err
	}

	report, err := s.planForRun(ctx, run, true)
	if err != nil {
		if strings.TrimSpace(report.RunID) != "" && !report.GeneratedAt.IsZero() {
			report.Failure = err.Error()
			if report.Counters.Failed == 0 {
				report.Counters.Failed = 1
			}
			_ = s.Store.StoreDerivedInsightReplayReport(ctx, report)
		}
		_ = s.Store.UpdateDerivedInsightReplayRunStatus(ctx, memory.UpdateDerivedInsightReplayRunStatusInput{
			Scope:      run.Scope,
			RunID:      run.ID,
			Status:     memory.DerivedInsightReplayStatusFailed,
			Failure:    err.Error(),
			UpdatedAt:  current,
			FinishedAt: current,
		})
		s.recordReplayMetric(ctx, run.Mode, "failed", report)
		return memory.DerivedInsightReplayReport{}, err
	}
	report.RunID = run.ID
	if err := s.Store.StoreDerivedInsightReplayReport(ctx, report); err != nil {
		s.recordReplayMetric(ctx, run.Mode, "failed", report)
		return memory.DerivedInsightReplayReport{}, err
	}
	finalStatus := memory.DerivedInsightReplayStatusCompleted
	if run.Request.EvidenceLimit > 0 && report.Counters.EvidenceEvaluated >= run.Request.EvidenceLimit {
		finalStatus = memory.DerivedInsightReplayStatusContinuationRequired
	}
	if err := s.Store.UpdateDerivedInsightReplayRunStatus(ctx, memory.UpdateDerivedInsightReplayRunStatusInput{
		Scope:      run.Scope,
		RunID:      run.ID,
		Status:     finalStatus,
		UpdatedAt:  report.GeneratedAt,
		FinishedAt: report.GeneratedAt,
	}); err != nil {
		s.recordReplayMetric(ctx, run.Mode, "failed", report)
		return memory.DerivedInsightReplayReport{}, err
	}
	s.recordReplayMetric(ctx, run.Mode, string(finalStatus), report)
	return report, nil
}

func (s ReplayService) ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("derived insight replay store is required")
	}
	return s.Store.ListDerivedInsightReplayRuns(ctx, input)
}

func (s ReplayService) ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error) {
	if s.Store == nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("derived insight replay store is required")
	}
	return s.Store.ReadDerivedInsightReplayRun(ctx, input)
}

func (s ReplayService) ReadDerivedInsightReplayReport(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayReport, error) {
	run, err := s.ReadDerivedInsightReplayRun(ctx, input)
	if err != nil {
		return memory.DerivedInsightReplayReport{}, err
	}
	if run.Report == nil {
		return memory.DerivedInsightReplayReport{}, pgx.ErrNoRows
	}
	return *run.Report, nil
}

func (s ReplayService) planForRun(ctx context.Context, run memory.DerivedInsightReplayRun, apply bool) (memory.DerivedInsightReplayReport, error) {
	report, err := s.PlanDerivedInsightReplay(ctx, run.Request)
	if err != nil {
		return memory.DerivedInsightReplayReport{}, err
	}
	report.RunID = run.ID

	if !apply {
		return report, nil
	}

	return s.executeReplayDecisions(ctx, run)
}

func (s ReplayService) executeReplayDecisions(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayReport, error) {
	input := run.Request
	current := s.now()
	report := memory.DerivedInsightReplayReport{
		RunID:       run.ID,
		Scope:       run.Scope,
		GeneratedAt: current,
	}

	evidence, err := s.Store.ListFailureEvidence(ctx, input.Scope, input.EvidenceLimit)
	if err != nil {
		report.Failure = err.Error()
		report.Counters.Failed++
		return report, err
	}
	evidence = filterReplayEvidenceWindow(evidence, input.EvidenceWindowStart, input.EvidenceWindowEnd)
	report.Counters.EvidenceEvaluated = len(evidence)

	evaluator := FailurePatternEvaluator{
		MinimumEvidence: s.MinimumEvidence,
		Window:          input.EvidenceWindowEnd.Sub(input.EvidenceWindowStart),
		Now:             func() time.Time { return current },
	}
	patterns, err := evaluator.Evaluate(input.Scope, evidence)
	if err != nil {
		report.Failure = err.Error()
		report.Counters.Failed++
		return report, err
	}
	requested := requestedReplayInsightTypes(input.InsightTypes)
	if len(evidence) > 0 && len(patterns) == 0 {
		report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
			InsightType:   memory.DerivedInsightTypeFailurePattern,
			Fingerprint:   "failure_pattern:insufficient_evidence",
			Decision:      memory.DerivedInsightReplayDecisionSkip,
			Reason:        memory.DerivedInsightReplayReasonInsufficientEvidence,
			EvidenceCount: len(evidence),
		})
		report.Counters.Skipped++
		return report, report.Validate()
	}

	for _, pattern := range patterns {
		if requested[memory.DerivedInsightTypeFailurePattern] {
			feedback, err := s.Store.SummarizeDerivedInsightFeedback(ctx, memory.SummarizeDerivedInsightFeedbackInput{
				Scope:     pattern.Scope,
				InsightID: pattern.ID,
			})
			if err != nil {
				report.Failure = err.Error()
				report.Counters.Failed++
				return report, err
			}
			original := pattern
			var decision FeedbackPolicyDecision
			pattern, decision = ApplyFeedbackPolicy(pattern, feedback)
			upsertTarget := pattern
			if decision == FeedbackPolicyDecisionSuppress {
				upsertTarget = original
			}
			stored, err := s.Store.UpsertDerivedInsight(ctx, upsertTarget)
			if err != nil {
				report.Failure = err.Error()
				report.Counters.Failed++
				return report, err
			}
			if decision == FeedbackPolicyDecisionSuppress {
				if err := s.Store.TransitionDerivedInsightLifecycle(ctx, memory.DerivedInsightLifecycleTransition{
					Scope:      stored.Scope,
					InsightID:  stored.ID,
					FromState:  original.State,
					ToState:    memory.DerivedInsightStateSuppressed,
					Actor:      "replay_feedback_policy",
					Reason:     "suppressed by derived insight replay feedback policy",
					OccurredAt: current,
				}); err != nil {
					report.Failure = err.Error()
					report.Counters.Failed++
					return report, err
				}
				report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
					InsightID:     stored.ID,
					InsightType:   stored.Type,
					Fingerprint:   stored.Derivation.Fingerprint,
					Decision:      memory.DerivedInsightReplayDecisionSuppress,
					Reason:        memory.DerivedInsightReplayReasonFeedbackPolicy,
					EvidenceCount: len(stored.Evidence),
				})
				report.Counters.Suppressed++
				continue
			}
			report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
				InsightID:     stored.ID,
				InsightType:   stored.Type,
				Fingerprint:   stored.Derivation.Fingerprint,
				Decision:      memory.DerivedInsightReplayDecisionCreate,
				Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: len(stored.Evidence),
			})
			report.Counters.Created++
			pattern = stored
		}
		if requested[memory.DerivedInsightTypeLesson] && pattern.State == memory.DerivedInsightStateActive {
			lesson, err := ProjectLesson(pattern)
			if err != nil {
				report.Failure = err.Error()
				report.Counters.Failed++
				return report, err
			}
			storedLesson, err := s.Store.UpsertDerivedInsight(ctx, lesson)
			if err != nil {
				report.Failure = err.Error()
				report.Counters.Failed++
				return report, err
			}
			report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
				InsightID:     storedLesson.ID,
				InsightType:   storedLesson.Type,
				Fingerprint:   storedLesson.Derivation.Fingerprint,
				Decision:      memory.DerivedInsightReplayDecisionCreate,
				Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: len(storedLesson.Evidence),
			})
			report.Counters.Created++
		}
	}
	return report, report.Validate()
}

func (s ReplayService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s ReplayService) newRunID() string {
	if s.NewRunID != nil {
		return s.NewRunID()
	}
	return fmt.Sprintf("replay_%d", s.now().UnixNano())
}

func (s ReplayService) recordReplayMetric(ctx context.Context, mode memory.DerivedInsightReplayMode, result string, report memory.DerivedInsightReplayReport) {
	if s.Observer == nil {
		return
	}
	if result == "" {
		result = "unknown"
	}
	if len(report.Decisions) == 0 {
		decision := "none"
		reason := "none"
		if report.Counters.Failed > 0 || result == "failed" {
			decision = "failed"
			reason = string(memory.DerivedInsightReplayReasonExecutionFailed)
		}
		s.Observer.RecordDerivedInsightReplay(ctx, telemetry.DerivedInsightReplayEvent{
			Mode:        string(mode),
			Result:      result,
			InsightType: "unknown",
			Decision:    decision,
			Reason:      reason,
			Count:       maxReplayMetricCount(report.Counters.Failed),
		})
		return
	}

	for _, decision := range report.Decisions {
		s.Observer.RecordDerivedInsightReplay(ctx, telemetry.DerivedInsightReplayEvent{
			Mode:        string(mode),
			Result:      result,
			InsightType: string(decision.InsightType),
			Decision:    string(decision.Decision),
			Reason:      string(decision.Reason),
			Count:       1,
		})
	}
}

func maxReplayMetricCount(count int) int {
	if count > 0 {
		return count
	}
	return 1
}

func filterReplayEvidenceWindow(evidence []FailureEvidence, start, end time.Time) []FailureEvidence {
	items := make([]FailureEvidence, 0, len(evidence))
	for _, item := range evidence {
		observedAt := item.ObservedAt.UTC()
		if observedAt.IsZero() {
			continue
		}
		if observedAt.Before(start.UTC()) || observedAt.After(end.UTC()) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func requestedReplayInsightTypes(types []memory.DerivedInsightType) map[memory.DerivedInsightType]bool {
	if len(types) == 0 {
		return map[memory.DerivedInsightType]bool{
			memory.DerivedInsightTypeFailurePattern: true,
			memory.DerivedInsightTypeLesson:         true,
		}
	}
	requested := make(map[memory.DerivedInsightType]bool, len(types))
	for _, insightType := range types {
		requested[insightType] = true
	}
	return requested
}
