package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/embedding"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/insights"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
)

type NoopWorker struct{}

func (NoopWorker) Start() error {
	return nil
}

type GovernanceWorker struct {
	Claimer            governance.RawEventClaimer
	Processor          governance.RawEventProcessor
	FailureRecorder    governance.RawEventFailureRecorder
	LeaseRenewer       governance.RawEventLeaseRenewer
	WorkerID           string
	BatchSize          int
	LeaseDuration      time.Duration
	LeaseRenewInterval time.Duration
	MaxAttempts        int
	RetryBackoff       time.Duration
	Now                func() time.Time
	Observer           telemetry.Observer
}

func (w GovernanceWorker) RunOnce(ctx context.Context) (processed int, err error) {
	started := time.Now()
	if w.Claimer == nil {
		return 0, fmt.Errorf("governance raw event claimer is required")
	}

	if w.Processor == nil {
		return 0, fmt.Errorf("governance raw event processor is required")
	}

	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	observedAt := now().UTC()
	defer func() {
		if w.Observer == nil {
			return
		}

		status := "ok"
		errorMessage := ""
		if err != nil {
			status = "error"
			processed = 0
			errorMessage = err.Error()
		}

		w.Observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "worker",
			Component:  "governance_worker",
			Operation:  "governance",
			Status:     status,
			Count:      processed,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: observedAt,
		})
	}()

	input := governance.ClaimPendingRawEventsInput{
		WorkerID:      w.WorkerID,
		BatchSize:     w.BatchSize,
		LeaseDuration: w.LeaseDuration,
		Now:           observedAt,
	}
	if err := input.Validate(); err != nil {
		return 0, err
	}

	claims, err := w.Claimer.ClaimPendingRawEvents(ctx, input)
	if err != nil {
		return 0, err
	}

	for _, claim := range claims {
		if err := claim.Validate(); err != nil {
			return 0, err
		}

		ok, claimErr := w.processClaim(ctx, claim)
		if claimErr != nil {
			return 0, claimErr
		}
		if ok {
			processed++
		}
	}

	return processed, nil
}

func (w GovernanceWorker) processClaim(ctx context.Context, claim governance.ClaimedRawEvent) (bool, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewalErrCh := w.startLeaseRenewal(runCtx, cancel, claim)
	err := w.Processor.ProcessClaimedRawEvent(runCtx, claim)
	cancel()

	if renewalErr := waitRenewalErr(renewalErrCh); renewalErr != nil {
		return false, renewalErr
	}

	if err == nil {
		return true, nil
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return false, err
	}

	if w.FailureRecorder == nil {
		return false, err
	}

	if recordErr := w.recordClaimFailure(ctx, claim, err); recordErr != nil {
		return false, recordErr
	}

	return false, nil
}

func (w GovernanceWorker) startLeaseRenewal(ctx context.Context, cancel context.CancelFunc, claim governance.ClaimedRawEvent) <-chan error {
	result := make(chan error, 1)
	if w.LeaseRenewer == nil || w.LeaseRenewInterval <= 0 {
		close(result)
		return result
	}

	go func() {
		defer close(result)

		ticker := time.NewTicker(w.LeaseRenewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewedAt := w.nowUTC()
				input := governance.RenewClaimedRawEventLeaseInput{
					RawEventID: claim.Event.ID,
					WorkerID:   claim.WorkerID,
					RenewedAt:  renewedAt,
					LeaseUntil: renewedAt.Add(w.LeaseDuration),
				}
				if err := w.LeaseRenewer.RenewClaimedRawEventLease(ctx, input); err != nil {
					cancel()
					result <- err
					return
				}
			}
		}
	}()

	return result
}

func waitRenewalErr(result <-chan error) error {
	if result == nil {
		return nil
	}

	err, ok := <-result
	if !ok {
		return nil
	}

	return err
}

func (w GovernanceWorker) recordClaimFailure(ctx context.Context, claim governance.ClaimedRawEvent, cause error) error {
	failedAt := w.nowUTC()
	input := governance.RecordClaimedRawEventFailureInput{
		RawEventID:   claim.Event.ID,
		WorkerID:     claim.WorkerID,
		FailedAt:     failedAt,
		ErrorMessage: truncateError(cause.Error(), 512),
	}

	if claim.Attempt >= w.maxAttempts() {
		input.ExhaustedAt = failedAt
	} else {
		input.NextAttemptAt = failedAt.Add(w.retryBackoff())
	}

	return w.FailureRecorder.RecordClaimedRawEventFailure(ctx, input)
}

func (w GovernanceWorker) nowUTC() time.Time {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}

	return now().UTC()
}

func (w GovernanceWorker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}

	return 5
}

func (w GovernanceWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}

	return 30 * time.Second
}

func truncateError(message string, limit int) string {
	message = strings.TrimSpace(message)
	if limit <= 0 || len(message) <= limit {
		return message
	}

	return message[:limit]
}

type NoopScheduler struct{}

func (NoopScheduler) Start() error {
	return nil
}

type GovernanceStatus struct {
	PendingRawEvents       int64     `json:"pending_raw_events"`
	LeasedRawEvents        int64     `json:"leased_raw_events"`
	ProcessedRawEvents     int64     `json:"processed_raw_events"`
	OldestPendingCreatedAt time.Time `json:"oldest_pending_created_at,omitempty"`
	ObservedAt             time.Time `json:"observed_at,omitempty"`
}

type JobExecutionStatus string

const (
	JobExecutionStatusRunning   JobExecutionStatus = "running"
	JobExecutionStatusCompleted JobExecutionStatus = "completed"
	JobExecutionStatusFailed    JobExecutionStatus = "failed"
)

type JobExecution struct {
	JobName        string
	Scope          memory.Scope
	TriggerSource  string
	IdempotencyKey string
	StartedAt      time.Time
}

type JobExecutionCompletion struct {
	IdempotencyKey string
	FinishedAt     time.Time
	ProcessedCount int
}

type JobExecutionFailure struct {
	IdempotencyKey string
	FinishedAt     time.Time
	ErrorMessage   string
}

type JobExecutionRecord struct {
	JobName        string             `json:"job_name"`
	Scope          memory.Scope       `json:"scope"`
	TriggerSource  string             `json:"trigger_source"`
	IdempotencyKey string             `json:"idempotency_key"`
	Status         JobExecutionStatus `json:"status"`
	Attempt        int                `json:"attempt"`
	ProcessedCount int                `json:"processed_count"`
	ErrorMessage   string             `json:"error_message,omitempty"`
	StartedAt      time.Time          `json:"started_at"`
	FinishedAt     time.Time          `json:"finished_at,omitempty"`
}

type ExecutionStore interface {
	BeginJobExecution(ctx context.Context, execution JobExecution) (bool, error)
	CompleteJobExecution(ctx context.Context, completion JobExecutionCompletion) error
	FailJobExecution(ctx context.Context, failure JobExecutionFailure) error
}

type loopWorker interface {
	RunOnce(ctx context.Context) (int, error)
}

type waitFunc func(ctx context.Context, d time.Duration) error

type PollingWorker struct {
	Worker       loopWorker
	PollInterval time.Duration
	ErrorBackoff time.Duration
	Wait         waitFunc
	Logger       *log.Logger
}

func (w PollingWorker) Start(ctx context.Context) error {
	if w.Worker == nil {
		return fmt.Errorf("polling worker target is required")
	}

	wait := w.Wait
	if wait == nil {
		wait = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}

	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}

	idleDelay := w.PollInterval
	if idleDelay <= 0 {
		idleDelay = 5 * time.Second
	}

	errorDelay := w.ErrorBackoff
	if errorDelay <= 0 {
		errorDelay = idleDelay
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		processed, err := w.Worker.RunOnce(ctx)
		if err != nil {
			if isContextDone(ctx, err) {
				return nil
			}

			logger.Printf("mode=worker component=polling_worker event=run_once_failed err=%v", err)
			if err := wait(ctx, errorDelay); err != nil {
				if isContextDone(ctx, err) {
					return nil
				}
				return err
			}
			continue
		}

		if processed > 0 {
			logger.Printf("mode=worker component=polling_worker event=run_once processed=%d", processed)
			continue
		}

		if err := wait(ctx, idleDelay); err != nil {
			if isContextDone(ctx, err) {
				return nil
			}
			return err
		}
	}
}

type MaintenanceJob interface {
	Name() string
	Run(ctx context.Context) (int, error)
}

type MaintenanceScopeSource interface {
	ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error)
}

type ScopeDispatchJob struct {
	NameValue       string
	ScopeSource     MaintenanceScopeSource
	ScopeBatchLimit int
	FallbackScope   memory.Scope
	Dispatch        func(scope memory.Scope) MaintenanceJob
}

func (j ScopeDispatchJob) Name() string {
	if strings.TrimSpace(j.NameValue) != "" {
		return j.NameValue
	}

	return "scope_dispatch"
}

func (j ScopeDispatchJob) Run(ctx context.Context) (int, error) {
	if j.ScopeSource == nil {
		return 0, fmt.Errorf("maintenance scope source is required")
	}
	if j.Dispatch == nil {
		return 0, fmt.Errorf("scope dispatch function is required")
	}

	scopes, err := j.ScopeSource.ListMaintenanceScopes(ctx, scopeBatchLimitOrDefault(j.ScopeBatchLimit))
	if err != nil {
		return 0, err
	}
	if len(scopes) == 0 {
		fallback := j.FallbackScope.Normalized()
		if err := fallback.Validate(); err == nil {
			scopes = []memory.Scope{fallback}
		}
	}

	total := 0
	for _, scope := range scopes {
		normalized := scope.Normalized()
		if err := normalized.Validate(); err != nil {
			return total, err
		}

		job := j.Dispatch(normalized)
		if job == nil {
			continue
		}

		processed, err := job.Run(ctx)
		if err != nil {
			return total, err
		}
		total += processed
	}

	return total, nil
}

type MaintenanceScheduler struct {
	Jobs         []MaintenanceJob
	Interval     time.Duration
	ErrorBackoff time.Duration
	Wait         waitFunc
	Logger       *log.Logger
}

func (s MaintenanceScheduler) Start(ctx context.Context) error {
	wait := s.Wait
	if wait == nil {
		wait = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}

	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}

	interval := s.Interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	errorDelay := s.ErrorBackoff
	if errorDelay <= 0 {
		errorDelay = interval
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var runErr error
		for _, job := range s.Jobs {
			if job == nil {
				continue
			}

			processed, err := job.Run(ctx)
			if err != nil {
				if isContextDone(ctx, err) {
					return nil
				}
				logger.Printf("mode=scheduler component=maintenance_scheduler job=%s event=run_failed err=%v", job.Name(), err)
				runErr = err
				break
			}

			logger.Printf("mode=scheduler component=maintenance_scheduler job=%s event=run_completed processed=%d", job.Name(), processed)
		}

		delay := interval
		if runErr != nil {
			delay = errorDelay
		}

		if err := wait(ctx, delay); err != nil {
			if isContextDone(ctx, err) {
				return nil
			}
			return err
		}
	}
}

type summaryCompactor interface {
	CompactScope(ctx context.Context, scope memory.Scope, cutoff time.Time) error
}

type retentionTargetSource interface {
	ListRetentionTargets(ctx context.Context, scope memory.Scope, limit int) ([]governance.RetentionTarget, error)
}

type retentionEvaluator interface {
	Evaluate(ctx context.Context, target governance.RetentionTarget) error
}

type jobExecutionCleaner interface {
	DeleteJobExecutionsBefore(ctx context.Context, cutoff time.Time) (int, error)
}

type embeddingProviderResolver interface {
	ResolveProvider(name string) (embedding.Provider, error)
}

type embeddingLifecycleStore interface {
	DispatchEmbeddingCutoverWave(ctx context.Context, scope memory.Scope, requestedAt time.Time, limit int) (int, error)
	ListEmbeddingLifecycleCandidates(ctx context.Context, scope memory.Scope, limit int) ([]memory.EmbeddingLifecycleCandidate, error)
	RecordEmbeddingRebuildRequired(ctx context.Context, record memory.EmbeddingRebuildRecord) error
	ClaimEmbeddingRebuilds(ctx context.Context, scope memory.Scope, limit int, attemptedAt time.Time) ([]memory.EmbeddingRebuildRecord, error)
	AppendVectorRevision(ctx context.Context, revision memory.VectorRevision) error
	PromoteVectorRevision(ctx context.Context, revision memory.VectorRevision) error
	RecordFailedVectorRevision(ctx context.Context, record memory.EmbeddingRebuildRecord, revision memory.VectorRevision) error
	RecordEmbeddingRebuildFailure(ctx context.Context, record memory.EmbeddingRebuildRecord, failureCause string, failedAt time.Time) error
}

type derivedInsightStore interface {
	ListFailureEvidence(ctx context.Context, scope memory.Scope, limit int) ([]insights.FailureEvidence, error)
	UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error)
}

type SummaryCompactionJob struct {
	Scope          memory.Scope
	CutoffWindow   time.Duration
	Cadence        time.Duration
	Now            func() time.Time
	Processor      summaryCompactor
	ExecutionStore ExecutionStore
	TriggerSource  string
}

func (j SummaryCompactionJob) Name() string {
	return "summary_compaction"
}

func (j SummaryCompactionJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Processor == nil {
		return 0, fmt.Errorf("summary compaction processor is required")
	}

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	current := now().UTC()

	cutoffWindow := j.CutoffWindow
	if cutoffWindow <= 0 {
		cutoffWindow = time.Hour
	}

	runWindow := scheduledRunWindow(current, j.Cadence)

	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, j.triggerSource(), runWindow)
	if err != nil {
		return 0, err
	}
	if !started {
		return 0, nil
	}

	if err := j.Processor.CompactScope(ctx, j.Scope, runWindow.Add(-cutoffWindow)); err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, 1); err != nil {
		return 0, err
	}

	return 1, nil
}

func (j SummaryCompactionJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
}

type RetentionSweepJob struct {
	Scope          memory.Scope
	Cadence        time.Duration
	Now            func() time.Time
	Source         retentionTargetSource
	Evaluator      retentionEvaluator
	ExecutionStore ExecutionStore
	TriggerSource  string
	Limit          int
}

func (j RetentionSweepJob) Name() string {
	return "retention_sweep"
}

func (j RetentionSweepJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Source == nil {
		return 0, fmt.Errorf("retention target source is required")
	}
	if j.Evaluator == nil {
		return 0, fmt.Errorf("retention evaluator is required")
	}

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	current := now().UTC()
	runWindow := scheduledRunWindow(current, j.Cadence)

	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, j.triggerSource(), runWindow)
	if err != nil {
		return 0, err
	}
	if !started {
		return 0, nil
	}

	limit := j.Limit
	if limit <= 0 {
		limit = 100
	}

	targets, err := j.Source.ListRetentionTargets(ctx, j.Scope, limit)
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	processed := 0
	for _, target := range targets {
		if err := target.Validate(); err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}

		if err := j.Evaluator.Evaluate(ctx, target); err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return 0, err
	}

	return processed, nil
}

func (j RetentionSweepJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
}

type JobExecutionCleanupJob struct {
	Scope           memory.Scope
	Cadence         time.Duration
	RetentionWindow time.Duration
	Now             func() time.Time
	Cleaner         jobExecutionCleaner
	ExecutionStore  ExecutionStore
	TriggerSource   string
}

func (j JobExecutionCleanupJob) Name() string {
	return "job_execution_cleanup"
}

func (j JobExecutionCleanupJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Cleaner == nil {
		return 0, fmt.Errorf("job execution cleaner is required")
	}

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	current := now().UTC()
	runWindow := scheduledRunWindow(current, j.Cadence)

	retentionWindow := j.RetentionWindow
	if retentionWindow <= 0 {
		retentionWindow = 7 * 24 * time.Hour
	}

	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, j.triggerSource(), runWindow)
	if err != nil {
		return 0, err
	}
	if !started {
		return 0, nil
	}

	deleted, err := j.Cleaner.DeleteJobExecutionsBefore(ctx, current.Add(-retentionWindow))
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, deleted); err != nil {
		return 0, err
	}

	return deleted, nil
}

func (j JobExecutionCleanupJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
}

type DerivedInsightDerivationJob struct {
	Scope           memory.Scope
	Store           derivedInsightStore
	ExecutionStore  ExecutionStore
	TriggerSource   string
	Cadence         time.Duration
	Window          time.Duration
	MinimumEvidence int
	Limit           int
	Now             func() time.Time
}

func (j DerivedInsightDerivationJob) Name() string {
	return "derived_insight_derivation"
}

func (j DerivedInsightDerivationJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Store == nil {
		return 0, fmt.Errorf("derived insight store is required")
	}

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	current := now().UTC()
	runWindow := scheduledRunWindow(current, j.Cadence)

	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, j.triggerSource(), runWindow)
	if err != nil {
		return 0, err
	}
	if !started {
		return 0, nil
	}

	limit := j.Limit
	if limit <= 0 {
		limit = 100
	}

	evidence, err := j.Store.ListFailureEvidence(ctx, j.Scope, limit)
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	evaluator := insights.FailurePatternEvaluator{
		MinimumEvidence: j.MinimumEvidence,
		Window:          j.Window,
		Now:             func() time.Time { return current },
	}
	patterns, err := evaluator.Evaluate(j.Scope, evidence)
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	processed := 0
	for _, pattern := range patterns {
		storedPattern, err := j.Store.UpsertDerivedInsight(ctx, pattern)
		if err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		processed++

		lesson, err := insights.ProjectLesson(storedPattern)
		if err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		if _, err := j.Store.UpsertDerivedInsight(ctx, lesson); err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return 0, err
	}

	return processed, nil
}

func (j DerivedInsightDerivationJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
}

type EmbeddingRebuildJob struct {
	Scope          memory.Scope
	Router         embedding.Router
	Providers      embeddingProviderResolver
	Store          embeddingLifecycleStore
	ExecutionStore ExecutionStore
	Observer       telemetry.Observer
	TriggerSource  string
	Cadence        time.Duration
	Now            func() time.Time
	Limit          int
	NewRevisionID  func() string
}

func (j EmbeddingRebuildJob) Name() string {
	return "embedding_rebuild"
}

func (j EmbeddingRebuildJob) Run(ctx context.Context) (processed int, err error) {
	startedAt := time.Now()
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Store == nil {
		return 0, fmt.Errorf("embedding lifecycle store is required")
	}
	if j.NewRevisionID == nil {
		return 0, fmt.Errorf("embedding revision id generator is required")
	}

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	current := now().UTC()
	defer func() {
		if j.Observer == nil {
			return
		}

		status := "ok"
		errorMessage := ""
		if err != nil {
			status = "error"
			processed = 0
			errorMessage = err.Error()
		}

		j.Observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "scheduler",
			Component:  "embedding_rebuild_job",
			Operation:  "embedding_rebuild",
			Status:     status,
			Count:      processed,
			Duration:   time.Since(startedAt),
			Error:      errorMessage,
			ObservedAt: current,
		})
	}()
	runWindow := scheduledRunWindow(current, j.Cadence)

	started, idempotencyKey, err := beginScheduledExecution(ctx, j.ExecutionStore, j.Name(), j.Scope, j.triggerSource(), runWindow)
	if err != nil {
		return 0, err
	}
	if !started {
		return 0, nil
	}

	limit := j.Limit
	if limit <= 0 {
		limit = 100
	}

	dispatched, dispatchErr := j.Store.DispatchEmbeddingCutoverWave(ctx, j.Scope, current, limit)
	if dispatchErr != nil {
		j.recordCutoverWaveDispatch(ctx, "error", 0)
		j.recordBacklog(ctx, current, 0, dispatchErr)
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, dispatchErr); failErr != nil {
			return 0, failErr
		}
		return 0, dispatchErr
	}
	j.recordCutoverWaveDispatch(ctx, "ok", dispatched)

	queued, queueErr := j.queueLifecycleCandidates(ctx, current, limit)
	if queueErr != nil {
		j.recordBacklog(ctx, current, 0, queueErr)
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, queueErr); failErr != nil {
			return 0, failErr
		}
		return 0, queueErr
	}
	j.recordBacklog(ctx, current, int64(dispatched+queued), nil)

	claims, err := j.Store.ClaimEmbeddingRebuilds(ctx, j.Scope, limit, current)
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	processed = 0
	for _, claim := range claims {
		if err := j.processClaim(ctx, claim, current); err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return 0, err
	}

	return processed, nil
}

func (j EmbeddingRebuildJob) recordBacklog(ctx context.Context, observedAt time.Time, pending int64, recordErr error) {
	if j.Observer == nil {
		return
	}

	event := telemetry.BacklogEvent{
		Mode:       "scheduler",
		Component:  "embedding_rebuild_job",
		Queue:      "embedding_rebuilds",
		Pending:    pending,
		ObservedAt: observedAt,
	}
	if recordErr != nil {
		event.Status = "error"
		event.Error = recordErr.Error()
	} else {
		event.Status = "ok"
	}

	j.Observer.RecordBacklog(ctx, event)
}

func (j EmbeddingRebuildJob) recordCutoverWaveDispatch(ctx context.Context, result string, dispatched int) {
	observer, ok := j.Observer.(interface {
		RecordCutoverWaveDispatch(ctx context.Context, event telemetry.CutoverWaveDispatchEvent)
	})
	if !ok {
		return
	}
	observer.RecordCutoverWaveDispatch(ctx, telemetry.CutoverWaveDispatchEvent{
		Result:     result,
		Dispatched: dispatched,
	})
}

func (j EmbeddingRebuildJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
}

func (j EmbeddingRebuildJob) queueLifecycleCandidates(ctx context.Context, requestedAt time.Time, limit int) (int, error) {
	candidates, err := j.Store.ListEmbeddingLifecycleCandidates(ctx, j.Scope, limit)
	if err != nil {
		return 0, err
	}

	queued := 0
	for _, candidate := range candidates {
		target, ok, err := j.resolveTarget(candidate)
		if err != nil {
			return queued, err
		}
		if !ok {
			continue
		}

		record := memory.EmbeddingRebuildRecord{
			MemoryID:            candidate.MemoryID,
			Scope:               candidate.Scope,
			Class:               candidate.Class,
			SourceVersion:       candidate.CurrentSourceVersion,
			ContentHash:         candidate.CurrentContentHash,
			RequestedProvider:   target.Provider,
			RequestedModel:      target.Model,
			RequestedDimensions: target.Dimensions,
			Status:              memory.EmbeddingRebuildStatusPending,
			RequestedAt:         requestedAt,
		}

		if candidate.ActiveVectorRevision != "" {
			record.ActiveVectorRevision = candidate.ActiveVectorRevision
		}

		if err := j.Store.RecordEmbeddingRebuildRequired(ctx, record); err != nil {
			return queued, err
		}
		queued++
	}

	return queued, nil
}

func (j EmbeddingRebuildJob) resolveTarget(candidate memory.EmbeddingLifecycleCandidate) (embedding.Target, bool, error) {
	if j.Router.Default == (embedding.Target{}) && len(j.Router.ByClass) == 0 {
		return embedding.Target{}, false, nil
	}

	target, err := j.Router.ResolveTarget(string(candidate.Class))
	if err != nil {
		return embedding.Target{}, false, err
	}

	switch {
	case candidate.RebuildStatus == memory.EmbeddingRebuildStatusPending:
		return embedding.Target{}, false, nil
	case candidate.RebuildStatus == memory.EmbeddingRebuildStatusCurrent:
		if !embedding.DetermineDrift(target, candidate.ActiveProvider, candidate.ActiveModel, candidate.ActiveDimensions) {
			return embedding.Target{}, false, nil
		}
	}

	return target, true, nil
}

func (j EmbeddingRebuildJob) processClaim(ctx context.Context, claim memory.EmbeddingRebuildRecord, processedAt time.Time) error {
	if j.Providers == nil {
		return fmt.Errorf("embedding provider resolver is required")
	}

	provider, err := j.Providers.ResolveProvider(claim.RequestedProvider)
	if err != nil {
		return err
	}

	result, err := provider.GenerateEmbedding(ctx, embedding.ProviderRequest{
		Text: claim.Content,
		Target: embedding.Target{
			Provider:   claim.RequestedProvider,
			Model:      claim.RequestedModel,
			Dimensions: claim.RequestedDimensions,
		},
	})
	if err != nil {
		return j.recordFailure(ctx, claim, processedAt, err)
	}

	revision := memory.VectorRevision{
		ID:                 j.NewRevisionID(),
		MemoryID:           claim.MemoryID,
		Scope:              claim.Scope,
		SourceVersion:      claim.SourceVersion,
		ContentHash:        claim.ContentHash,
		Provider:           result.Provider,
		Model:              result.Model,
		Dimensions:         result.Dimensions,
		Embedding:          result.Embedding,
		Status:             memory.VectorRevisionStatusGenerated,
		GeneratedAt:        processedAt,
		LastRebuildRequest: claim.RequestedAt,
	}

	if err := j.Store.AppendVectorRevision(ctx, revision); err != nil {
		return err
	}

	revision.Status = memory.VectorRevisionStatusActive
	revision.ActivatedAt = processedAt
	if err := j.Store.PromoteVectorRevision(ctx, revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	return nil
}

func (j EmbeddingRebuildJob) recordFailure(ctx context.Context, claim memory.EmbeddingRebuildRecord, failedAt time.Time, cause error) error {
	revision := memory.VectorRevision{
		ID:                 j.NewRevisionID(),
		MemoryID:           claim.MemoryID,
		Scope:              claim.Scope,
		SourceVersion:      claim.SourceVersion,
		ContentHash:        claim.ContentHash,
		Provider:           claim.RequestedProvider,
		Model:              claim.RequestedModel,
		Dimensions:         claim.RequestedDimensions,
		Embedding:          []float32{},
		Status:             memory.VectorRevisionStatusFailed,
		FailureReason:      cause.Error(),
		GeneratedAt:        failedAt,
		LastRebuildRequest: claim.RequestedAt,
	}

	if err := j.Store.RecordFailedVectorRevision(ctx, claim, revision); err != nil {
		return err
	}

	return j.Store.RecordEmbeddingRebuildFailure(ctx, claim, cause.Error(), failedAt)
}

func scheduledRunWindow(current time.Time, cadence time.Duration) time.Time {
	if cadence > 0 {
		return current.Truncate(cadence)
	}

	return current
}

func beginScheduledExecution(ctx context.Context, store ExecutionStore, jobName string, scope memory.Scope, triggerSource string, runWindow time.Time) (bool, string, error) {
	if store == nil {
		return true, "", nil
	}

	idempotencyKey := fmt.Sprintf("%s:%s:%s:%s:%s", jobName, scope.Tenant, scope.Project, scope.Namespace, runWindow.Format(time.RFC3339))
	started, err := store.BeginJobExecution(ctx, JobExecution{
		JobName:        jobName,
		Scope:          scope,
		TriggerSource:  triggerSource,
		IdempotencyKey: idempotencyKey,
		StartedAt:      runWindow,
	})
	if err != nil {
		return false, "", err
	}

	return started, idempotencyKey, nil
}

func completeScheduledExecution(ctx context.Context, store ExecutionStore, idempotencyKey string, finishedAt time.Time, processed int) error {
	if store == nil {
		return nil
	}

	return store.CompleteJobExecution(ctx, JobExecutionCompletion{
		IdempotencyKey: idempotencyKey,
		FinishedAt:     finishedAt,
		ProcessedCount: processed,
	})
}

func failScheduledExecution(ctx context.Context, store ExecutionStore, idempotencyKey string, finishedAt time.Time, cause error) error {
	if store == nil {
		return nil
	}

	return store.FailJobExecution(ctx, JobExecutionFailure{
		IdempotencyKey: idempotencyKey,
		FinishedAt:     finishedAt,
		ErrorMessage:   cause.Error(),
	})
}

func scopeBatchLimitOrDefault(value int) int {
	if value > 0 {
		return value
	}

	return 100
}

func isContextDone(ctx context.Context, err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded || ctx.Err() != nil
}
