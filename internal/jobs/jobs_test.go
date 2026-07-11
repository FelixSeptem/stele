package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/embedding"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/insights"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
)

func TestNoopWorkerStart(t *testing.T) {
	var worker NoopWorker
	if err := worker.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestNoopSchedulerStart(t *testing.T) {
	var scheduler NoopScheduler
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

type stubLoopWorker struct {
	results []int
	errs    []error
	calls   int
}

func (s *stubLoopWorker) RunOnce(ctx context.Context) (int, error) {
	idx := s.calls
	s.calls++

	var result int
	if idx < len(s.results) {
		result = s.results[idx]
	}

	var err error
	if idx < len(s.errs) {
		err = s.errs[idx]
	}

	return result, err
}

type stubMaintenanceJob struct {
	name  string
	calls int
	err   error
}

func (s *stubMaintenanceJob) Name() string {
	return s.name
}

func (s *stubMaintenanceJob) Run(ctx context.Context) (int, error) {
	s.calls++
	if s.err != nil {
		return 0, s.err
	}

	return 1, nil
}

type stubMaintenanceScopeSource struct {
	scopes   []memory.Scope
	err      error
	calls    int
	gotLimit int
}

func (s *stubMaintenanceScopeSource) ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error) {
	s.calls++
	s.gotLimit = limit
	if s.err != nil {
		return nil, s.err
	}

	return s.scopes, nil
}

type stubScopedMaintenanceJob struct {
	name      string
	scope     memory.Scope
	err       error
	processed int
	calls     int
}

func (s *stubScopedMaintenanceJob) Name() string {
	return s.name
}

func (s *stubScopedMaintenanceJob) Run(ctx context.Context) (int, error) {
	s.calls++
	if s.err != nil {
		return 0, s.err
	}
	if s.processed > 0 {
		return s.processed, nil
	}

	return 1, nil
}

type stubSummaryProcessor struct {
	calls     int
	gotScope  memory.Scope
	gotCutoff time.Time
	returnErr error
}

func (s *stubSummaryProcessor) CompactScope(ctx context.Context, scope memory.Scope, cutoff time.Time) error {
	s.calls++
	s.gotScope = scope
	s.gotCutoff = cutoff
	return s.returnErr
}

type stubExecutionStore struct {
	beginCalls    int
	completeCalls int
	failCalls     int
	beginStarted  bool
	beginErr      error
	completeErr   error
	failErr       error
	lastBegin     JobExecution
	lastComplete  JobExecutionCompletion
	lastFailure   JobExecutionFailure
}

func (s *stubExecutionStore) BeginJobExecution(ctx context.Context, execution JobExecution) (bool, error) {
	s.beginCalls++
	s.lastBegin = execution
	return s.beginStarted, s.beginErr
}

func (s *stubExecutionStore) CompleteJobExecution(ctx context.Context, completion JobExecutionCompletion) error {
	s.completeCalls++
	s.lastComplete = completion
	return s.completeErr
}

func (s *stubExecutionStore) FailJobExecution(ctx context.Context, failure JobExecutionFailure) error {
	s.failCalls++
	s.lastFailure = failure
	return s.failErr
}

type stubRetentionSource struct {
	targets []governance.RetentionTarget
	err     error
	calls   int
}

func (s *stubRetentionSource) ListRetentionTargets(ctx context.Context, scope memory.Scope, limit int) ([]governance.RetentionTarget, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.targets, nil
}

type stubRetentionEvaluator struct {
	targets []governance.RetentionTarget
	err     error
}

func (s *stubRetentionEvaluator) Evaluate(ctx context.Context, target governance.RetentionTarget) error {
	s.targets = append(s.targets, target)
	return s.err
}

type stubJobExecutionCleaner struct {
	gotCutoff time.Time
	deleted   int
	err       error
	calls     int
}

func (s *stubJobExecutionCleaner) DeleteJobExecutionsBefore(ctx context.Context, cutoff time.Time) (int, error) {
	s.calls++
	s.gotCutoff = cutoff
	return s.deleted, s.err
}

type stubEmbeddingProvider struct {
	gotInputs []embedding.ProviderRequest
	result    embedding.ProviderResult
	err       error
}

func (s *stubEmbeddingProvider) GenerateEmbedding(ctx context.Context, input embedding.ProviderRequest) (embedding.ProviderResult, error) {
	s.gotInputs = append(s.gotInputs, input)
	if s.err != nil {
		return embedding.ProviderResult{}, s.err
	}

	return s.result, nil
}

type stubEmbeddingProviderResolver struct {
	providers map[string]embedding.Provider
}

func (s *stubEmbeddingProviderResolver) ResolveProvider(name string) (embedding.Provider, error) {
	provider, ok := s.providers[name]
	if !ok {
		return nil, errors.New("provider not registered")
	}

	return provider, nil
}

type stubDerivedInsightStore struct {
	evidence        []insights.FailureEvidence
	listErr         error
	summaryErr      error
	upsertErr       error
	listCalls       int
	summaryCalls    int
	transitionCalls int
	upserted        []memory.DerivedInsight
	transitions     []memory.DerivedInsightLifecycleTransition
	gotScope        memory.Scope
	gotLimit        int
	summaries       map[string]memory.DerivedInsightFeedbackSummary
	returnStored    bool
}

func (s *stubDerivedInsightStore) ListFailureEvidence(ctx context.Context, scope memory.Scope, limit int) ([]insights.FailureEvidence, error) {
	s.listCalls++
	s.gotScope = scope
	s.gotLimit = limit
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.evidence, nil
}

func (s *stubDerivedInsightStore) UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error) {
	if s.upsertErr != nil {
		return memory.DerivedInsight{}, s.upsertErr
	}
	s.upserted = append(s.upserted, insight)
	return insight, nil
}

func (s *stubDerivedInsightStore) SummarizeDerivedInsightFeedback(ctx context.Context, input memory.SummarizeDerivedInsightFeedbackInput) (memory.DerivedInsightFeedbackSummary, error) {
	s.summaryCalls++
	if s.summaryErr != nil {
		return memory.DerivedInsightFeedbackSummary{}, s.summaryErr
	}
	return s.summaries[input.InsightID], nil
}

func (s *stubDerivedInsightStore) TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error {
	s.transitionCalls++
	s.transitions = append(s.transitions, transition)
	return nil
}

type stubEmbeddingJobsObserver struct {
	operations []telemetry.OperationEvent
	backlogs   []telemetry.BacklogEvent
}

func (s *stubEmbeddingJobsObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubEmbeddingJobsObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {
	s.backlogs = append(s.backlogs, event)
}

type embeddingFailureUpdate struct {
	record       memory.EmbeddingRebuildRecord
	failureCause string
	failedAt     time.Time
}

type stubEmbeddingLifecycleStore struct {
	candidates            []memory.EmbeddingLifecycleCandidate
	claims                []memory.EmbeddingRebuildRecord
	dispatchSequence      []string
	recordedRebuilds      []memory.EmbeddingRebuildRecord
	appendedRevisions     []memory.VectorRevision
	promotedRevisions     []memory.VectorRevision
	failedRevisions       []memory.VectorRevision
	failureUpdates        []embeddingFailureUpdate
	dispatchedCutoverWave int
	gotCandidateScope     memory.Scope
	gotCandidateLimit     int
	gotDispatchScope      memory.Scope
	gotDispatchLimit      int
	gotDispatchRequested  time.Time
	gotClaimScope         memory.Scope
	gotClaimLimit         int
	gotClaimAttemptedAt   time.Time
	dispatchErr           error
	candidateErr          error
	recordErr             error
	claimErr              error
	appendErr             error
	promoteErr            error
	failedRevisionErr     error
	failureUpdateErr      error
}

func (s *stubEmbeddingLifecycleStore) ListEmbeddingLifecycleCandidates(ctx context.Context, scope memory.Scope, limit int) ([]memory.EmbeddingLifecycleCandidate, error) {
	s.dispatchSequence = append(s.dispatchSequence, "list_candidates")
	s.gotCandidateScope = scope
	s.gotCandidateLimit = limit
	if s.candidateErr != nil {
		return nil, s.candidateErr
	}

	return s.candidates, nil
}

func (s *stubEmbeddingLifecycleStore) DispatchEmbeddingCutoverWave(ctx context.Context, scope memory.Scope, requestedAt time.Time, limit int) (int, error) {
	s.dispatchSequence = append(s.dispatchSequence, "dispatch_cutover")
	s.gotDispatchScope = scope
	s.gotDispatchLimit = limit
	s.gotDispatchRequested = requestedAt
	if s.dispatchErr != nil {
		return 0, s.dispatchErr
	}

	return s.dispatchedCutoverWave, nil
}

func (s *stubEmbeddingLifecycleStore) RecordEmbeddingRebuildRequired(ctx context.Context, record memory.EmbeddingRebuildRecord) error {
	s.recordedRebuilds = append(s.recordedRebuilds, record)
	return s.recordErr
}

func (s *stubEmbeddingLifecycleStore) ClaimEmbeddingRebuilds(ctx context.Context, scope memory.Scope, limit int, attemptedAt time.Time) ([]memory.EmbeddingRebuildRecord, error) {
	s.dispatchSequence = append(s.dispatchSequence, "claim_rebuilds")
	s.gotClaimScope = scope
	s.gotClaimLimit = limit
	s.gotClaimAttemptedAt = attemptedAt
	if s.claimErr != nil {
		return nil, s.claimErr
	}

	return s.claims, nil
}

func (s *stubEmbeddingLifecycleStore) AppendVectorRevision(ctx context.Context, revision memory.VectorRevision) error {
	s.appendedRevisions = append(s.appendedRevisions, revision)
	return s.appendErr
}

func (s *stubEmbeddingLifecycleStore) PromoteVectorRevision(ctx context.Context, revision memory.VectorRevision) error {
	s.promotedRevisions = append(s.promotedRevisions, revision)
	return s.promoteErr
}

func (s *stubEmbeddingLifecycleStore) RecordFailedVectorRevision(ctx context.Context, record memory.EmbeddingRebuildRecord, revision memory.VectorRevision) error {
	s.failedRevisions = append(s.failedRevisions, revision)
	return s.failedRevisionErr
}

func (s *stubEmbeddingLifecycleStore) RecordEmbeddingRebuildFailure(ctx context.Context, record memory.EmbeddingRebuildRecord, failureCause string, failedAt time.Time) error {
	s.failureUpdates = append(s.failureUpdates, embeddingFailureUpdate{
		record:       record,
		failureCause: failureCause,
		failedAt:     failedAt,
	})
	return s.failureUpdateErr
}

func TestPollingWorkerStartUsesIdleAndErrorBackoff(t *testing.T) {
	worker := &stubLoopWorker{
		results: []int{0, 0},
		errs:    []error{nil, errors.New("transient failure")},
	}

	waits := make([]time.Duration, 0, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loop := PollingWorker{
		Worker:       worker,
		PollInterval: 5 * time.Second,
		ErrorBackoff: 12 * time.Second,
		Wait: func(ctx context.Context, d time.Duration) error {
			waits = append(waits, d)
			if len(waits) == 2 {
				cancel()
			}
			return nil
		},
	}

	if err := loop.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if worker.calls != 2 {
		t.Fatalf("worker calls = %d, want %d", worker.calls, 2)
	}

	if len(waits) != 2 {
		t.Fatalf("len(waits) = %d, want %d", len(waits), 2)
	}

	if waits[0] != 5*time.Second {
		t.Fatalf("waits[0] = %v, want %v", waits[0], 5*time.Second)
	}

	if waits[1] != 12*time.Second {
		t.Fatalf("waits[1] = %v, want %v", waits[1], 12*time.Second)
	}
}

func TestMaintenanceSchedulerStartRunsJobsOnInterval(t *testing.T) {
	jobA := &stubMaintenanceJob{name: "job-a"}
	jobB := &stubMaintenanceJob{name: "job-b"}

	waits := make([]time.Duration, 0, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := MaintenanceScheduler{
		Jobs:         []MaintenanceJob{jobA, jobB},
		Interval:     30 * time.Second,
		ErrorBackoff: time.Minute,
		Wait: func(ctx context.Context, d time.Duration) error {
			waits = append(waits, d)
			cancel()
			return nil
		},
	}

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if jobA.calls != 1 || jobB.calls != 1 {
		t.Fatalf("job calls = (%d, %d), want both jobs to run once", jobA.calls, jobB.calls)
	}

	if len(waits) != 1 || waits[0] != 30*time.Second {
		t.Fatalf("waits = %v, want one interval wait of 30s", waits)
	}
}

func TestScopeDispatchJobRunsScopedMaintenanceAcrossEligibleScopes(t *testing.T) {
	source := &stubMaintenanceScopeSource{
		scopes: []memory.Scope{
			{Tenant: "tenant-a", Project: "project-a", Namespace: "ns-a"},
			{Tenant: "tenant-b", Project: "project-b", Namespace: "ns-b"},
		},
	}
	dispatched := make([]memory.Scope, 0, 2)
	job := ScopeDispatchJob{
		NameValue:       "summary_compaction_dispatch",
		ScopeSource:     source,
		ScopeBatchLimit: 10,
		Dispatch: func(scope memory.Scope) MaintenanceJob {
			dispatched = append(dispatched, scope)
			return &stubScopedMaintenanceJob{name: "summary_compaction", scope: scope}
		},
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}

	if source.calls != 1 || source.gotLimit != 10 {
		t.Fatalf("scope source calls/limit = (%d, %d), want (1, 10)", source.calls, source.gotLimit)
	}

	if len(dispatched) != 2 {
		t.Fatalf("len(dispatched) = %d, want 2", len(dispatched))
	}

	if dispatched[0].Namespace != "ns-a" || dispatched[1].Namespace != "ns-b" {
		t.Fatalf("dispatched scopes = %+v, want ns-a/ns-b order", dispatched)
	}
}

func TestScopeDispatchJobUsesFallbackScopeWhenDiscoveryReturnsNone(t *testing.T) {
	fallback := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	dispatched := make([]memory.Scope, 0, 1)
	job := ScopeDispatchJob{
		NameValue:       "retention_sweep_dispatch",
		ScopeSource:     &stubMaintenanceScopeSource{},
		ScopeBatchLimit: 10,
		FallbackScope:   fallback,
		Dispatch: func(scope memory.Scope) MaintenanceJob {
			dispatched = append(dispatched, scope)
			return &stubScopedMaintenanceJob{name: "retention_sweep", scope: scope}
		},
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	if len(dispatched) != 1 {
		t.Fatalf("len(dispatched) = %d, want 1", len(dispatched))
	}

	if dispatched[0] != fallback {
		t.Fatalf("dispatched[0] = %+v, want %+v", dispatched[0], fallback)
	}
}

func TestSummaryCompactionJobRunSkipsDuplicateExecutionWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	processor := &stubSummaryProcessor{}
	store := &stubExecutionStore{beginStarted: false}
	now := time.Date(2026, 6, 7, 10, 15, 0, 0, time.UTC)

	job := SummaryCompactionJob{
		Scope:          scope,
		CutoffWindow:   time.Hour,
		Cadence:        15 * time.Minute,
		Now:            func() time.Time { return now },
		Processor:      processor,
		ExecutionStore: store,
		TriggerSource:  "scheduler",
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("processed = %d, want 0 when duplicate window is skipped", processed)
	}

	if processor.calls != 0 {
		t.Fatalf("processor calls = %d, want 0 when duplicate window is skipped", processor.calls)
	}

	if store.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", store.beginCalls)
	}
}

func TestSummaryCompactionJobRunRecordsCompletionAndFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 10, 30, 0, 0, time.UTC)

	successProcessor := &stubSummaryProcessor{}
	successStore := &stubExecutionStore{beginStarted: true}
	successJob := SummaryCompactionJob{
		Scope:          scope,
		CutoffWindow:   time.Hour,
		Cadence:        30 * time.Minute,
		Now:            func() time.Time { return now },
		Processor:      successProcessor,
		ExecutionStore: successStore,
		TriggerSource:  "scheduler",
	}

	if _, err := successJob.Run(context.Background()); err != nil {
		t.Fatalf("Run() success error = %v", err)
	}

	if successStore.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", successStore.completeCalls)
	}

	failingProcessor := &stubSummaryProcessor{returnErr: errors.New("boom")}
	failingStore := &stubExecutionStore{beginStarted: true}
	failingJob := SummaryCompactionJob{
		Scope:          scope,
		CutoffWindow:   time.Hour,
		Cadence:        30 * time.Minute,
		Now:            func() time.Time { return now },
		Processor:      failingProcessor,
		ExecutionStore: failingStore,
		TriggerSource:  "scheduler",
	}

	if _, err := failingJob.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want processor failure")
	}

	if failingStore.failCalls != 1 {
		t.Fatalf("fail calls = %d, want 1", failingStore.failCalls)
	}
}

func TestRetentionSweepJobRunEvaluatesTargetsOncePerCadenceWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	source := &stubRetentionSource{
		targets: []governance.RetentionTarget{
			{
				MemoryID:       "mem_123",
				Scope:          scope,
				RetentionClass: "ephemeral",
				UpdatedAt:      now.Add(-2 * time.Hour),
			},
		},
	}
	evaluator := &stubRetentionEvaluator{}
	store := &stubExecutionStore{beginStarted: true}

	job := RetentionSweepJob{
		Scope:          scope,
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
		Source:         source,
		Evaluator:      evaluator,
		ExecutionStore: store,
		TriggerSource:  "scheduler",
		Limit:          50,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	if len(evaluator.targets) != 1 || evaluator.targets[0].MemoryID != "mem_123" {
		t.Fatalf("evaluated targets = %+v, want mem_123", evaluator.targets)
	}

	if store.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", store.completeCalls)
	}
}

func TestJobExecutionCleanupJobRunDeletesOldExecutions(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 12, 30, 0, 0, time.UTC)
	cleaner := &stubJobExecutionCleaner{deleted: 4}
	store := &stubExecutionStore{beginStarted: true}

	job := JobExecutionCleanupJob{
		Scope:           scope,
		Cadence:         30 * time.Minute,
		RetentionWindow: 24 * time.Hour,
		Now:             func() time.Time { return now },
		Cleaner:         cleaner,
		ExecutionStore:  store,
		TriggerSource:   "scheduler",
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 4 {
		t.Fatalf("processed = %d, want 4", processed)
	}

	if cleaner.calls != 1 {
		t.Fatalf("cleaner calls = %d, want 1", cleaner.calls)
	}

	if !cleaner.gotCutoff.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("cutoff = %v, want %v", cleaner.gotCutoff, now.Add(-24*time.Hour))
	}
}

func TestDerivedInsightDerivationJobRunUpsertsPatternAndLesson(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	store := &stubDerivedInsightStore{
		evidence: []insights.FailureEvidence{
			{
				Kind:       memory.DerivedInsightEvidenceKindJobExecution,
				ID:         "job_1",
				FailureKey: "provider unavailable",
				ObservedAt: now.Add(-time.Hour),
			},
			{
				Kind:       memory.DerivedInsightEvidenceKindJobExecution,
				ID:         "job_2",
				FailureKey: "provider unavailable",
				ObservedAt: now.Add(-30 * time.Minute),
			},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}

	job := DerivedInsightDerivationJob{
		Scope:           scope,
		Store:           store,
		ExecutionStore:  executions,
		TriggerSource:   "scheduler",
		Cadence:         time.Hour,
		Window:          24 * time.Hour,
		MinimumEvidence: 2,
		Limit:           50,
		Now:             func() time.Time { return now },
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed = %d, want pattern and lesson", processed)
	}
	if store.listCalls != 1 || store.gotScope != scope || store.gotLimit != 50 {
		t.Fatalf("list calls/scope/limit = %d/%+v/%d, want scoped limit 50", store.listCalls, store.gotScope, store.gotLimit)
	}
	if len(store.upserted) != 2 {
		t.Fatalf("upserted = %d, want 2", len(store.upserted))
	}
	if store.upserted[0].Type != memory.DerivedInsightTypeFailurePattern || store.upserted[1].Type != memory.DerivedInsightTypeLesson {
		t.Fatalf("upserted types = %s/%s, want failure_pattern/lesson", store.upserted[0].Type, store.upserted[1].Type)
	}
	if executions.completeCalls != 1 {
		t.Fatalf("complete calls = %d, want 1", executions.completeCalls)
	}
}

func TestDerivedInsightDerivationJobRunSuppressesPatternWithNegativeFeedback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	evidence := []insights.FailureEvidence{
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: now.Add(-time.Hour)},
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: now.Add(-30 * time.Minute)},
	}
	patterns, err := (insights.FailurePatternEvaluator{
		MinimumEvidence: 2,
		Window:          24 * time.Hour,
		Now:             func() time.Time { return now },
	}).Evaluate(scope, evidence)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	store := &stubDerivedInsightStore{
		evidence: evidence,
		summaries: map[string]memory.DerivedInsightFeedbackSummary{
			patterns[0].ID: {
				InsightID:     patterns[0].ID,
				Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNoisy: 2},
				TotalActive:   2,
				NegativeCount: 2,
			},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}

	job := DerivedInsightDerivationJob{
		Scope:           scope,
		Store:           store,
		ExecutionStore:  executions,
		Cadence:         time.Hour,
		Window:          24 * time.Hour,
		MinimumEvidence: 2,
		Limit:           50,
		Now:             func() time.Time { return now },
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want suppressed pattern without lesson", processed)
	}
	if len(store.upserted) != 1 || store.upserted[0].State != memory.DerivedInsightStateActive {
		t.Fatalf("upserted = %+v, want active pattern before audited suppression", store.upserted)
	}
	if len(store.transitions) != 1 || store.transitions[0].ToState != memory.DerivedInsightStateSuppressed || store.transitions[0].Reason == "" {
		t.Fatalf("transitions = %+v, want feedback-driven suppression audit", store.transitions)
	}
	if store.summaryCalls == 0 {
		t.Fatal("summary calls = 0, want feedback summary consumption")
	}
}

func TestDerivedInsightDerivationJobRunSkipsDuplicateExecutionWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubDerivedInsightStore{}
	executions := &stubExecutionStore{beginStarted: false}
	now := time.Date(2026, 7, 4, 14, 30, 0, 0, time.UTC)

	job := DerivedInsightDerivationJob{
		Scope:          scope,
		Store:          store,
		ExecutionStore: executions,
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}
	if store.listCalls != 0 || len(store.upserted) != 0 {
		t.Fatalf("store list/upserts = %d/%d, want no foreground derivation work", store.listCalls, len(store.upserted))
	}
	if executions.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", executions.beginCalls)
	}
}

func TestDerivedInsightDerivationJobRunFailsSafelyWithoutPartialUnsupportedActivation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubDerivedInsightStore{listErr: errors.New("scan failed")}
	executions := &stubExecutionStore{beginStarted: true}
	now := time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC)

	job := DerivedInsightDerivationJob{
		Scope:          scope,
		Store:          store,
		ExecutionStore: executions,
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
	}

	if _, err := job.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want store failure")
	}
	if executions.failCalls != 1 {
		t.Fatalf("fail calls = %d, want durable failure", executions.failCalls)
	}
	if len(store.upserted) != 0 {
		t.Fatalf("upserted = %d, want no partial activation", len(store.upserted))
	}
}

func TestDerivedInsightReplayExecutionJobRunsPendingReplayRuns(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	run := memory.DerivedInsightReplayRun{
		ID:     "replay_123",
		Scope:  scope,
		Mode:   memory.DerivedInsightReplayModeApply,
		Status: memory.DerivedInsightReplayStatusPending,
		Request: memory.DerivedInsightReplayRequest{
			Scope:               scope,
			Mode:                memory.DerivedInsightReplayModeApply,
			InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern},
			EvidenceWindowStart: now.Add(-24 * time.Hour),
			EvidenceWindowEnd:   now,
			EvidenceLimit:       100,
			Actor:               "operator-a",
			Reason:              "apply replay",
			RequestedAt:         now,
		},
		Actor:     "operator-a",
		Reason:    "apply replay",
		CreatedAt: now,
		UpdatedAt: now,
	}
	service := &stubReplayExecutionService{
		runs: []memory.DerivedInsightReplayRun{run},
		report: memory.DerivedInsightReplayReport{
			RunID:       run.ID,
			Scope:       scope,
			Counters:    memory.DerivedInsightReplayCounters{Created: 1},
			GeneratedAt: now,
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	job := DerivedInsightReplayExecutionJob{
		Scope:          scope,
		Service:        service,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
		Limit:          10,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if service.gotList.Status != memory.DerivedInsightReplayStatusPending || service.executed[0].ID != run.ID {
		t.Fatalf("service list/executed = %+v/%+v, want pending run execution", service.gotList, service.executed)
	}
	if executions.lastComplete.ProcessedCount != 1 {
		t.Fatalf("completed execution = %+v, want processed count 1", executions.lastComplete)
	}
}

func TestDerivedInsightReplayExecutionJobRecordsFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	service := &stubReplayExecutionService{listErr: errors.New("scan failed")}
	executions := &stubExecutionStore{beginStarted: true}
	job := DerivedInsightReplayExecutionJob{
		Scope:          scope,
		Service:        service,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
		Limit:          10,
	}

	if _, err := job.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want list failure")
	}
	if executions.lastFailure.ErrorMessage == "" {
		t.Fatalf("failed execution = %+v, want failure recorded", executions.lastFailure)
	}
}

func TestDerivedInsightReplayExecutionJobSkipsDuplicateExecutionWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubReplayExecutionService{}
	executions := &stubExecutionStore{beginStarted: false}
	job := DerivedInsightReplayExecutionJob{
		Scope:          scope,
		Service:        service,
		ExecutionStore: executions,
		Cadence:        time.Hour,
		Now:            func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) },
		Limit:          10,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 0 || service.gotList.Limit != 0 {
		t.Fatalf("processed/list = %d/%+v, want no replay work for duplicate window", processed, service.gotList)
	}
	if executions.beginCalls != 1 || executions.completeCalls != 0 || executions.failCalls != 0 {
		t.Fatalf("execution calls begin/complete/fail = %d/%d/%d, want begin only", executions.beginCalls, executions.completeCalls, executions.failCalls)
	}
}

func TestDerivedInsightReplayExecutionJobHonorsBatchLimit(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	runA := testReplayExecutionRun(scope, "replay_a", now)
	runB := testReplayExecutionRun(scope, "replay_b", now)
	service := &stubReplayExecutionService{
		runs: []memory.DerivedInsightReplayRun{runA, runB},
		report: memory.DerivedInsightReplayReport{
			RunID:       runA.ID,
			Scope:       scope,
			Counters:    memory.DerivedInsightReplayCounters{Created: 1},
			GeneratedAt: now,
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	job := DerivedInsightReplayExecutionJob{
		Scope:          scope,
		Service:        service,
		ExecutionStore: executions,
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
		Limit:          1,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 1 || service.gotList.Limit != 1 || len(service.executed) != 1 {
		t.Fatalf("processed/limit/executed = %d/%d/%d, want bounded single run", processed, service.gotList.Limit, len(service.executed))
	}
	if service.executed[0].ID != runA.ID {
		t.Fatalf("executed = %+v, want first bounded run", service.executed)
	}
}

func TestDerivedInsightReplayExecutionJobRecordsRunExecutionFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	service := &stubReplayExecutionService{
		runs:    []memory.DerivedInsightReplayRun{testReplayExecutionRun(scope, "replay_123", now)},
		execErr: errors.New("replay execution failed"),
	}
	executions := &stubExecutionStore{beginStarted: true}
	job := DerivedInsightReplayExecutionJob{
		Scope:          scope,
		Service:        service,
		ExecutionStore: executions,
		Cadence:        time.Hour,
		Now:            func() time.Time { return now },
		Limit:          10,
	}

	if _, err := job.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want replay execution failure")
	}
	if executions.failCalls != 1 || executions.lastFailure.ErrorMessage == "" {
		t.Fatalf("failure calls/record = %d/%+v, want durable failure", executions.failCalls, executions.lastFailure)
	}
}

type stubReplayExecutionService struct {
	gotList  memory.ListDerivedInsightReplayRunsInput
	runs     []memory.DerivedInsightReplayRun
	executed []memory.DerivedInsightReplayRun
	report   memory.DerivedInsightReplayReport
	listErr  error
	execErr  error
}

func (s *stubReplayExecutionService) ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error) {
	s.gotList = input
	if s.listErr != nil {
		return nil, s.listErr
	}
	if input.Limit > 0 && len(s.runs) > input.Limit {
		return s.runs[:input.Limit], nil
	}
	return s.runs, nil
}

func (s *stubReplayExecutionService) ExecuteDerivedInsightReplay(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayReport, error) {
	s.executed = append(s.executed, run)
	if s.execErr != nil {
		return memory.DerivedInsightReplayReport{}, s.execErr
	}
	return s.report, nil
}

func testReplayExecutionRun(scope memory.Scope, id string, now time.Time) memory.DerivedInsightReplayRun {
	return memory.DerivedInsightReplayRun{
		ID:     id,
		Scope:  scope,
		Mode:   memory.DerivedInsightReplayModeApply,
		Status: memory.DerivedInsightReplayStatusPending,
		Request: memory.DerivedInsightReplayRequest{
			Scope:               scope,
			Mode:                memory.DerivedInsightReplayModeApply,
			InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern},
			EvidenceWindowStart: now.Add(-24 * time.Hour),
			EvidenceWindowEnd:   now,
			EvidenceLimit:       100,
			Actor:               "operator-a",
			Reason:              "apply replay",
			RequestedAt:         now,
		},
		Actor:     "operator-a",
		Reason:    "apply replay",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestEmbeddingRebuildJobRunQueuesBackfillAndProviderDriftCandidates(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		candidates: []memory.EmbeddingLifecycleCandidate{
			{
				MemoryID:             "mem_missing",
				Scope:                scope,
				Class:                memory.MemoryClassProfile,
				CurrentSourceVersion: 2,
				CurrentContentHash:   "sha256:missing",
			},
			{
				MemoryID:             "mem_drift",
				Scope:                scope,
				Class:                memory.MemoryClassProfile,
				CurrentSourceVersion: 3,
				CurrentContentHash:   "sha256:drift",
				RebuildStatus:        memory.EmbeddingRebuildStatusCurrent,
				RequestedProvider:    "anthropic",
				RequestedModel:       "text-embedding-3-small",
				RequestedDimensions:  1536,
				ActiveVectorRevision: "vec_active",
				ActiveProvider:       "anthropic",
				ActiveModel:          "text-embedding-3-small",
				ActiveDimensions:     1536,
			},
			{
				MemoryID:             "mem_pending",
				Scope:                scope,
				Class:                memory.MemoryClassProfile,
				CurrentSourceVersion: 4,
				CurrentContentHash:   "sha256:pending",
				RebuildStatus:        memory.EmbeddingRebuildStatusPending,
			},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	observer := &stubEmbeddingJobsObserver{}
	job := EmbeddingRebuildJob{
		Scope: scope,
		Router: embedding.Router{
			Default: embedding.Target{
				Provider:   "openai",
				Model:      "text-embedding-3-small",
				Dimensions: 1536,
			},
		},
		Store:          store,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_unused" },
		Observer:       observer,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("processed = %d, want 0 when only discovery queues work", processed)
	}

	if len(store.recordedRebuilds) != 2 {
		t.Fatalf("len(recordedRebuilds) = %d, want 2", len(store.recordedRebuilds))
	}

	if store.recordedRebuilds[0].MemoryID != "mem_missing" || store.recordedRebuilds[1].MemoryID != "mem_drift" {
		t.Fatalf("recordedRebuilds = %+v, want mem_missing and mem_drift", store.recordedRebuilds)
	}

	if store.recordedRebuilds[0].RequestedProvider != "openai" || store.recordedRebuilds[0].RequestedDimensions != 1536 {
		t.Fatalf("recordedRebuilds[0] = %+v, want default routing target", store.recordedRebuilds[0])
	}

	if len(observer.backlogs) != 1 {
		t.Fatalf("len(observer.backlogs) = %d, want 1", len(observer.backlogs))
	}

	if observer.backlogs[0].Queue != "embedding_rebuilds" || observer.backlogs[0].Pending != 2 || observer.backlogs[0].Status != "ok" {
		t.Fatalf("backlog event = %+v, want queued embedding rebuild snapshot", observer.backlogs[0])
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Operation != "embedding_rebuild" || observer.operations[0].Count != 0 || observer.operations[0].Status != "ok" {
		t.Fatalf("operation event = %+v, want successful empty embedding execution", observer.operations[0])
	}
}

func TestEmbeddingRebuildJobRunDispatchesCutoverWaveBeforeLifecycleDiscovery(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		dispatchedCutoverWave: 2,
		candidates: []memory.EmbeddingLifecycleCandidate{
			{
				MemoryID:             "mem_missing",
				Scope:                scope,
				Class:                memory.MemoryClassProfile,
				CurrentSourceVersion: 2,
				CurrentContentHash:   "sha256:missing",
			},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	observer := &stubEmbeddingJobsObserver{}
	job := EmbeddingRebuildJob{
		Scope: scope,
		Router: embedding.Router{
			Default: embedding.Target{
				Provider:   "openai",
				Model:      "text-embedding-3-small",
				Dimensions: 1536,
			},
		},
		Store:          store,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_unused" },
		Observer:       observer,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("processed = %d, want 0 when only dispatch and discovery queue work", processed)
	}

	if store.gotDispatchScope != scope {
		t.Fatalf("gotDispatchScope = %+v, want %+v", store.gotDispatchScope, scope)
	}

	if store.gotDispatchLimit != 10 {
		t.Fatalf("gotDispatchLimit = %d, want 10", store.gotDispatchLimit)
	}

	if !store.gotDispatchRequested.Equal(now) {
		t.Fatalf("gotDispatchRequested = %v, want %v", store.gotDispatchRequested, now)
	}

	if len(store.dispatchSequence) != 3 {
		t.Fatalf("dispatchSequence = %v, want dispatch -> discovery -> claim", store.dispatchSequence)
	}

	if store.dispatchSequence[0] != "dispatch_cutover" || store.dispatchSequence[1] != "list_candidates" || store.dispatchSequence[2] != "claim_rebuilds" {
		t.Fatalf("dispatchSequence = %v, want dispatch -> discovery -> claim", store.dispatchSequence)
	}

	if len(observer.backlogs) != 1 {
		t.Fatalf("len(observer.backlogs) = %d, want 1", len(observer.backlogs))
	}

	if observer.backlogs[0].Pending != 3 {
		t.Fatalf("backlog pending = %d, want 3 including cutover dispatch and lifecycle discovery", observer.backlogs[0].Pending)
	}
}

func TestEmbeddingRebuildJobRunRecordsCutoverWaveDispatchMetrics(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 4, 10, 30, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		dispatchedCutoverWave: 3,
	}
	executions := &stubExecutionStore{beginStarted: true}
	observer := telemetry.NewMetricsObserver()
	job := EmbeddingRebuildJob{
		Scope:          scope,
		Store:          store,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_unused" },
		Observer:       observer,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_embedding_cutover_wave_dispatch_total{result="ok"} 1`,
		`stele_embedding_cutover_wave_dispatched_total{result="ok"} 3`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q:\n%s", want, metrics)
		}
	}
}

func TestEmbeddingRebuildJobRunAppendsAndPromotesGeneratedRevision(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 14, 13, 15, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		claims: []memory.EmbeddingRebuildRecord{
			{
				MemoryID:             "mem_embed",
				Scope:                scope,
				Class:                memory.MemoryClassProfile,
				Content:              "User prefers concise answers.",
				SourceVersion:        3,
				ContentHash:          "sha256:abc",
				RequestedProvider:    "openai",
				RequestedModel:       "text-embedding-3-small",
				RequestedDimensions:  3,
				Status:               memory.EmbeddingRebuildStatusRebuilding,
				RequestedAt:          now.Add(-time.Minute),
				ActiveVectorRevision: "vec_old",
			},
		},
	}
	provider := &stubEmbeddingProvider{
		result: embedding.ProviderResult{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 3,
			Embedding:  []float32{0.1, 0.2, 0.3},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	job := EmbeddingRebuildJob{
		Scope:          scope,
		Store:          store,
		Providers:      &stubEmbeddingProviderResolver{providers: map[string]embedding.Provider{"openai": provider}},
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_new" },
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	if len(provider.gotInputs) != 1 {
		t.Fatalf("len(provider.gotInputs) = %d, want 1", len(provider.gotInputs))
	}

	if provider.gotInputs[0].Target.Provider != "openai" || provider.gotInputs[0].Target.Dimensions != 3 {
		t.Fatalf("provider input = %+v, want requested target", provider.gotInputs[0].Target)
	}

	if len(store.appendedRevisions) != 1 {
		t.Fatalf("len(appendedRevisions) = %d, want 1", len(store.appendedRevisions))
	}

	if store.appendedRevisions[0].ID != "vec_new" || store.appendedRevisions[0].Status != memory.VectorRevisionStatusGenerated {
		t.Fatalf("appendedRevisions[0] = %+v, want generated vec_new", store.appendedRevisions[0])
	}

	if len(store.promotedRevisions) != 1 || store.promotedRevisions[0].ID != "vec_new" {
		t.Fatalf("promotedRevisions = %+v, want vec_new promotion", store.promotedRevisions)
	}

	if executions.completeCalls != 1 {
		t.Fatalf("completeCalls = %d, want 1", executions.completeCalls)
	}
}

func TestEmbeddingRebuildJobRunRecordsFailedAttemptWithoutStoppingScheduler(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 14, 13, 30, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		claims: []memory.EmbeddingRebuildRecord{
			{
				MemoryID:            "mem_fail",
				Scope:               scope,
				Class:               memory.MemoryClassProfile,
				Content:             "User prefers concise answers.",
				SourceVersion:       5,
				ContentHash:         "sha256:def",
				RequestedProvider:   "openai",
				RequestedModel:      "text-embedding-3-small",
				RequestedDimensions: 3,
				Status:              memory.EmbeddingRebuildStatusRebuilding,
				RequestedAt:         now.Add(-time.Minute),
			},
		},
	}
	provider := &stubEmbeddingProvider{err: errors.New("provider unavailable")}
	executions := &stubExecutionStore{beginStarted: true}
	observer := &stubEmbeddingJobsObserver{}
	job := EmbeddingRebuildJob{
		Scope:          scope,
		Store:          store,
		Providers:      &stubEmbeddingProviderResolver{providers: map[string]embedding.Provider{"openai": provider}},
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_failed" },
		Observer:       observer,
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want 1 handled failure", processed)
	}

	if len(store.failedRevisions) != 1 {
		t.Fatalf("len(failedRevisions) = %d, want 1", len(store.failedRevisions))
	}

	if store.failedRevisions[0].ID != "vec_failed" || store.failedRevisions[0].Status != memory.VectorRevisionStatusFailed {
		t.Fatalf("failedRevisions[0] = %+v, want failed vec_failed", store.failedRevisions[0])
	}

	if len(store.promotedRevisions) != 0 {
		t.Fatalf("promotedRevisions = %+v, want no promotion on failure", store.promotedRevisions)
	}

	if executions.completeCalls != 1 {
		t.Fatalf("completeCalls = %d, want 1", executions.completeCalls)
	}

	if len(observer.backlogs) != 1 {
		t.Fatalf("len(observer.backlogs) = %d, want 1", len(observer.backlogs))
	}

	if observer.backlogs[0].Queue != "embedding_rebuilds" || observer.backlogs[0].Pending != 0 || observer.backlogs[0].Status != "ok" {
		t.Fatalf("backlog event = %+v, want zero newly-queued embedding backlog snapshot", observer.backlogs[0])
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Operation != "embedding_rebuild" || observer.operations[0].Count != 1 || observer.operations[0].Status != "ok" {
		t.Fatalf("operation event = %+v, want handled failed-attempt execution", observer.operations[0])
	}
}

func TestEmbeddingRebuildJobRunEmitsBacklogErrorTelemetryWhenCandidateDiscoveryFails(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 14, 14, 0, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		candidateErr: errors.New("candidate listing unavailable"),
	}
	executions := &stubExecutionStore{beginStarted: true}
	observer := &stubEmbeddingJobsObserver{}

	job := EmbeddingRebuildJob{
		Scope:          scope,
		Store:          store,
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_unused" },
		Observer:       observer,
	}

	processed, err := job.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want candidate discovery failure")
	}

	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}

	if len(observer.backlogs) != 1 {
		t.Fatalf("len(observer.backlogs) = %d, want 1", len(observer.backlogs))
	}

	if observer.backlogs[0].Status != "error" || observer.backlogs[0].Error != "candidate listing unavailable" {
		t.Fatalf("backlog event = %+v, want error backlog telemetry", observer.backlogs[0])
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Status != "error" || observer.operations[0].Operation != "embedding_rebuild" {
		t.Fatalf("operation event = %+v, want error embedding execution telemetry", observer.operations[0])
	}
}

func TestEmbeddingRebuildJobRunTreatsStalePromotionAsHandledWithoutFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 14, 13, 45, 0, 0, time.UTC)
	store := &stubEmbeddingLifecycleStore{
		claims: []memory.EmbeddingRebuildRecord{
			{
				MemoryID:            "mem_stale",
				Scope:               scope,
				Class:               memory.MemoryClassProfile,
				Content:             "User prefers concise answers.",
				SourceVersion:       6,
				ContentHash:         "sha256:stale",
				RequestedProvider:   "openai",
				RequestedModel:      "text-embedding-3-small",
				RequestedDimensions: 3,
				Status:              memory.EmbeddingRebuildStatusRebuilding,
				RequestedAt:         now.Add(-time.Minute),
			},
		},
		promoteErr: pgx.ErrNoRows,
	}
	provider := &stubEmbeddingProvider{
		result: embedding.ProviderResult{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 3,
			Embedding:  []float32{0.1, 0.2, 0.3},
		},
	}
	executions := &stubExecutionStore{beginStarted: true}
	job := EmbeddingRebuildJob{
		Scope:          scope,
		Store:          store,
		Providers:      &stubEmbeddingProviderResolver{providers: map[string]embedding.Provider{"openai": provider}},
		ExecutionStore: executions,
		TriggerSource:  "scheduler",
		Now:            func() time.Time { return now },
		Cadence:        time.Hour,
		Limit:          10,
		NewRevisionID:  func() string { return "vec_stale" },
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want 1 handled stale promotion", processed)
	}

	if len(store.appendedRevisions) != 1 {
		t.Fatalf("len(appendedRevisions) = %d, want 1", len(store.appendedRevisions))
	}

	if len(store.failureUpdates) != 0 || len(store.failedRevisions) != 0 {
		t.Fatalf("failure state = updates:%+v failed:%+v, want no failure recording for stale promotion", store.failureUpdates, store.failedRevisions)
	}

	if executions.completeCalls != 1 {
		t.Fatalf("completeCalls = %d, want 1", executions.completeCalls)
	}
}
