package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type NoopWorker struct{}

func (NoopWorker) Start() error {
	return nil
}

type GovernanceWorker struct {
	Claimer       governance.RawEventClaimer
	Processor     governance.RawEventProcessor
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	Now           func() time.Time
	Observer      telemetry.Observer
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

		if err := w.Processor.ProcessClaimedRawEvent(ctx, claim); err != nil {
			return 0, err
		}
	}

	processed = len(claims)
	return processed, nil
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

func isContextDone(ctx context.Context, err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded || ctx.Err() != nil
}
