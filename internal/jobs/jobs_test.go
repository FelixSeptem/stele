package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
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
