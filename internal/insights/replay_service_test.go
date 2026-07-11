package insights

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

var errReplayStoreInjected = errors.New("injected replay store failure")

func TestReplayServicePlanDryRunReportsCreateDecisionsWithoutMutating(t *testing.T) {
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	report, err := service.PlanDerivedInsightReplay(context.Background(), replayRequest(memory.DerivedInsightReplayModeDryRun))
	if err != nil {
		t.Fatalf("PlanDerivedInsightReplay() error = %v", err)
	}

	if report.Counters.EvidenceEvaluated != 2 || report.Counters.Created != 2 {
		t.Fatalf("counters = %+v, want 2 evidence and pattern+lesson creates", report.Counters)
	}
	if len(report.Decisions) != 2 {
		t.Fatalf("decisions = %+v, want pattern and lesson decisions", report.Decisions)
	}
	if len(store.upserted) != 0 || len(store.transitions) != 0 {
		t.Fatalf("dry-run mutated store: upserted=%d transitions=%d", len(store.upserted), len(store.transitions))
	}
}

func TestReplayServicePlanRecordsReplayMetrics(t *testing.T) {
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
	}
	observer := telemetry.NewMetricsObserver()
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
		Observer:        observer,
	}

	if _, err := service.PlanDerivedInsightReplay(context.Background(), replayRequest(memory.DerivedInsightReplayModeDryRun)); err != nil {
		t.Fatalf("PlanDerivedInsightReplay() error = %v", err)
	}

	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, `stele_derived_insight_replay_total{decision="create",insight_type="failure_pattern",mode="dry_run",reason="repeated_evidence",result="completed"} 1`) {
		t.Fatalf("metrics missing failure pattern replay create:\n%s", metrics)
	}
	if !strings.Contains(metrics, `stele_derived_insight_replay_total{decision="create",insight_type="lesson",mode="dry_run",reason="repeated_evidence",result="completed"} 1`) {
		t.Fatalf("metrics missing lesson replay create:\n%s", metrics)
	}
}

func TestReplayServiceApplyCreatesPendingRunIdempotently(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	existing := memory.DerivedInsightReplayRun{
		ID:        "replay_existing",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{runByKey: existing}
	service := ReplayService{
		Store:    store,
		NewRunID: func() string { return "replay_new" },
		Now:      func() time.Time { return request.RequestedAt },
	}

	run, err := service.ApplyDerivedInsightReplay(context.Background(), request)
	if err != nil {
		t.Fatalf("ApplyDerivedInsightReplay() error = %v", err)
	}
	if run.ID != existing.ID {
		t.Fatalf("run.ID = %q, want existing idempotent run", run.ID)
	}
	if store.createdRun.ID != "" {
		t.Fatalf("createdRun = %+v, want no new run for idempotency hit", store.createdRun)
	}
}

func TestReplayServiceExecuteAppliesInsightsAndPersistsReport(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	run := memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	report, err := service.ExecuteDerivedInsightReplay(context.Background(), run)
	if err != nil {
		t.Fatalf("ExecuteDerivedInsightReplay() error = %v", err)
	}
	if report.Counters.Created != 2 || len(store.upserted) != 2 {
		t.Fatalf("created=%d upserted=%d, want pattern and lesson", report.Counters.Created, len(store.upserted))
	}
	if store.report.RunID != run.ID || store.statuses[len(store.statuses)-1].Status != memory.DerivedInsightReplayStatusCompleted {
		t.Fatalf("report/status = %+v/%+v, want completed report", store.report, store.statuses)
	}
}

func TestReplayServicePlanReportsInsufficientEvidenceSkip(t *testing.T) {
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
		},
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	report, err := service.PlanDerivedInsightReplay(context.Background(), replayRequest(memory.DerivedInsightReplayModeDryRun))
	if err != nil {
		t.Fatalf("PlanDerivedInsightReplay() error = %v", err)
	}
	if report.Counters.Skipped != 1 || len(report.Decisions) != 1 || report.Decisions[0].Reason != memory.DerivedInsightReplayReasonInsufficientEvidence {
		t.Fatalf("report = %+v, want insufficient evidence skip", report)
	}
}

func TestReplayServiceExecuteRecordsFeedbackDrivenSuppression(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	run := memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
		summaryByInsight: map[string]memory.DerivedInsightFeedbackSummary{
			"any": {
				Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNoisy: 2},
				TotalActive:   2,
				NegativeCount: 2,
			},
		},
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	report, err := service.ExecuteDerivedInsightReplay(context.Background(), run)
	if err != nil {
		t.Fatalf("ExecuteDerivedInsightReplay() error = %v", err)
	}
	if report.Counters.Suppressed != 1 || len(store.transitions) != 1 {
		t.Fatalf("suppressed=%d transitions=%d, want feedback suppression", report.Counters.Suppressed, len(store.transitions))
	}
	if len(report.Decisions) != 1 || report.Decisions[0].Decision != memory.DerivedInsightReplayDecisionSuppress || report.Decisions[0].Reason != memory.DerivedInsightReplayReasonFeedbackPolicy {
		t.Fatalf("decisions = %+v, want feedback suppression decision", report.Decisions)
	}
	if len(store.upserted) != 1 || store.upserted[0].Type != memory.DerivedInsightTypeFailurePattern {
		t.Fatalf("upserted = %+v, want only source pattern upserted", store.upserted)
	}
}

func TestReplayServiceExecuteMarksContinuationRequiredAtEvidenceLimit(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	request.EvidenceLimit = 2
	run := memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_3", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)},
		},
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	report, err := service.ExecuteDerivedInsightReplay(context.Background(), run)
	if err != nil {
		t.Fatalf("ExecuteDerivedInsightReplay() error = %v", err)
	}
	if report.Counters.EvidenceEvaluated != request.EvidenceLimit {
		t.Fatalf("evidence evaluated = %d, want limit %d", report.Counters.EvidenceEvaluated, request.EvidenceLimit)
	}
	if store.statuses[len(store.statuses)-1].Status != memory.DerivedInsightReplayStatusContinuationRequired {
		t.Fatalf("final status = %s, want continuation_required", store.statuses[len(store.statuses)-1].Status)
	}
}

func TestReplayServiceExecutePersistsPartialFailureReport(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	run := memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
		upsertErr: errReplayStoreInjected,
	}
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
	}

	if _, err := service.ExecuteDerivedInsightReplay(context.Background(), run); err == nil {
		t.Fatal("ExecuteDerivedInsightReplay() error = nil, want upsert failure")
	}
	if store.report.RunID != run.ID || store.report.Failure == "" || store.report.Counters.Failed != 1 {
		t.Fatalf("partial report = %+v, want failed report persisted", store.report)
	}
	if store.statuses[len(store.statuses)-1].Status != memory.DerivedInsightReplayStatusFailed {
		t.Fatalf("final status = %s, want failed", store.statuses[len(store.statuses)-1].Status)
	}
}

func TestReplayServiceExecuteRecordsFailureReplayMetrics(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeApply)
	run := memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
	store := &stubReplayStore{
		evidence: []FailureEvidence{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
		upsertErr: errReplayStoreInjected,
	}
	observer := telemetry.NewMetricsObserver()
	service := ReplayService{
		Store:           store,
		MinimumEvidence: 2,
		Now:             func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
		Observer:        observer,
	}

	if _, err := service.ExecuteDerivedInsightReplay(context.Background(), run); err == nil {
		t.Fatal("ExecuteDerivedInsightReplay() error = nil, want upsert failure")
	}

	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, `stele_derived_insight_replay_total{decision="failed",insight_type="unknown",mode="apply",reason="execution_failed",result="failed"} 1`) {
		t.Fatalf("metrics missing failed apply replay:\n%s", metrics)
	}
}

func TestReplayServicePlanRejectsUnsupportedReservedType(t *testing.T) {
	request := replayRequest(memory.DerivedInsightReplayModeDryRun)
	request.InsightTypes = []memory.DerivedInsightType{memory.DerivedInsightTypeHypothesis}
	service := ReplayService{Store: &stubReplayStore{}}

	if _, err := service.PlanDerivedInsightReplay(context.Background(), request); err == nil {
		t.Fatal("PlanDerivedInsightReplay() error = nil, want unsupported type error")
	}
}

type stubReplayStore struct {
	evidence         []FailureEvidence
	upserted         []memory.DerivedInsight
	transitions      []memory.DerivedInsightLifecycleTransition
	summaries        map[string]memory.DerivedInsightFeedbackSummary
	summaryByInsight map[string]memory.DerivedInsightFeedbackSummary
	upsertErr        error
	runByKey         memory.DerivedInsightReplayRun
	createdRun       memory.DerivedInsightReplayRun
	report           memory.DerivedInsightReplayReport
	statuses         []memory.UpdateDerivedInsightReplayRunStatusInput
}

func (s *stubReplayStore) ListFailureEvidence(ctx context.Context, scope memory.Scope, limit int) ([]FailureEvidence, error) {
	if len(s.evidence) > limit {
		return s.evidence[:limit], nil
	}
	return s.evidence, nil
}

func (s *stubReplayStore) UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error) {
	if s.upsertErr != nil {
		return memory.DerivedInsight{}, s.upsertErr
	}
	s.upserted = append(s.upserted, insight)
	return insight, nil
}

func (s *stubReplayStore) SummarizeDerivedInsightFeedback(ctx context.Context, input memory.SummarizeDerivedInsightFeedbackInput) (memory.DerivedInsightFeedbackSummary, error) {
	if s.summaryByInsight != nil {
		summary := s.summaryByInsight[input.InsightID]
		if summary.Counts == nil {
			summary = s.summaryByInsight["any"]
		}
		summary.InsightID = input.InsightID
		return summary, nil
	}
	if s.summaries == nil {
		return memory.DerivedInsightFeedbackSummary{InsightID: input.InsightID}, nil
	}
	return s.summaries[input.InsightID], nil
}

func (s *stubReplayStore) TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error {
	s.transitions = append(s.transitions, transition)
	return nil
}

func (s *stubReplayStore) FindDerivedInsightReplayRunByIdempotencyKey(ctx context.Context, input memory.FindDerivedInsightReplayRunByIdempotencyKeyInput) (memory.DerivedInsightReplayRun, error) {
	return s.runByKey, nil
}

func (s *stubReplayStore) CreateDerivedInsightReplayRun(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayRun, error) {
	s.createdRun = run
	return run, nil
}

func (s *stubReplayStore) ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error) {
	if s.runByKey.ID == "" {
		return nil, nil
	}
	return []memory.DerivedInsightReplayRun{s.runByKey}, nil
}

func (s *stubReplayStore) ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error) {
	return s.runByKey, nil
}

func (s *stubReplayStore) UpdateDerivedInsightReplayRunStatus(ctx context.Context, input memory.UpdateDerivedInsightReplayRunStatusInput) error {
	s.statuses = append(s.statuses, input)
	return nil
}

func (s *stubReplayStore) StoreDerivedInsightReplayReport(ctx context.Context, report memory.DerivedInsightReplayReport) error {
	s.report = report
	return nil
}

func replayRequest(mode memory.DerivedInsightReplayMode) memory.DerivedInsightReplayRequest {
	return memory.DerivedInsightReplayRequest{
		Scope:               memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Mode:                mode,
		InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern, memory.DerivedInsightTypeLesson},
		EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		EvidenceLimit:       100,
		Actor:               "operator-a",
		Reason:              "replay insight window",
		IdempotencyKey:      "replay:key",
		RequestedAt:         time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}
}
