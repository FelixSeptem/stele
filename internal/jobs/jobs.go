package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
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

type RepairActionStore interface {
	ClaimRepairActions(ctx context.Context, input memory.ClaimRepairActionsInput) ([]memory.RepairAction, error)
	CompleteRepairAction(ctx context.Context, input memory.CompleteRepairActionInput) error
	RecordRepairActionFailure(ctx context.Context, input memory.RecordRepairActionFailureInput) error
}

type RepairActionProcessor interface {
	ProcessRepairAction(ctx context.Context, action memory.RepairAction) error
}

type ScopeProofStepStore interface {
	ClaimScopeProofSteps(ctx context.Context, input memory.ClaimScopeProofStepsInput) ([]memory.ScopeProofStep, error)
	CompleteScopeProofStep(ctx context.Context, input memory.CompleteScopeProofStepInput) error
	RecordScopeProofStepFailure(ctx context.Context, input memory.RecordScopeProofStepFailureInput) error
	UpdateScopeProofRunStatus(ctx context.Context, input memory.UpdateScopeProofRunStatusInput) (memory.ScopeProofRun, error)
}

type ScopeProofStepExecutor interface {
	ExecuteScopeProofStep(ctx context.Context, step memory.ScopeProofStep) (memory.ScopeProofStepResult, error)
}

type ScopeProofStepWorker struct {
	Store         ScopeProofStepStore
	Executor      ScopeProofStepExecutor
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	MaxAttempts   int
	RetryBackoff  time.Duration
	Now           func() time.Time
	Observer      telemetry.Observer
	Logger        *log.Logger
}

type MemorySessionVerificationStore interface {
	ClaimMemorySessionVerifications(ctx context.Context, input memory.ClaimMemorySessionVerificationsInput) ([]memory.MemorySessionVerification, error)
	CompleteMemorySessionVerification(ctx context.Context, input memory.CompleteMemorySessionVerificationInput) error
	RecordMemorySessionVerificationFailure(ctx context.Context, input memory.RecordMemorySessionVerificationFailureInput) error
	UpdateMemorySessionRunStatus(ctx context.Context, input memory.UpdateMemorySessionRunStatusInput) (memory.MemorySessionRun, error)
	UpdateMemorySessionTurnOutcome(ctx context.Context, input memory.UpdateMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error)
}

type MemorySessionVerificationRunner interface {
	VerifyMemorySession(ctx context.Context, verification memory.MemorySessionVerification) (memory.MemorySessionVerificationResult, error)
}

type MemorySessionVerificationWorker struct {
	Store         MemorySessionVerificationStore
	Runner        MemorySessionVerificationRunner
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	MaxAttempts   int
	RetryBackoff  time.Duration
	Now           func() time.Time
	Observer      telemetry.Observer
	Logger        *log.Logger
}

type RepairActionWorker struct {
	Store         RepairActionStore
	Processor     RepairActionProcessor
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	MaxAttempts   int
	RetryBackoff  time.Duration
	Now           func() time.Time
	Observer      telemetry.Observer
}

type AssuranceAlertDeliveryStore interface {
	ClaimAlertCandidatesForDelivery(ctx context.Context, input assurance.ClaimAlertCandidatesForDeliveryInput) ([]assurance.AlertDeliveryClaim, error)
}

type AssuranceAlertDeliveryService interface {
	DeliverAlertCandidate(ctx context.Context, input assurance.AlertDeliveryInput) ([]assurance.AlertDeliveryAttempt, error)
}

type AssuranceAlertDeliveryWorker struct {
	Store         AssuranceAlertDeliveryStore
	Service       AssuranceAlertDeliveryService
	Scope         memory.Scope
	WorkerID      string
	BatchSize     int
	LeaseDuration time.Duration
	MaxAttempts   int
	RetryBackoff  time.Duration
	Config        assurance.AlertDeliveryConfig
	Now           func() time.Time
	Observer      telemetry.Observer
}

func (w ScopeProofStepWorker) RunOnce(ctx context.Context) (processed int, err error) {
	started := time.Now()
	if w.Store == nil {
		return 0, fmt.Errorf("scope proof step store is required")
	}
	if w.Executor == nil {
		return 0, fmt.Errorf("scope proof step executor is required")
	}
	now := w.nowUTC()
	defer func() {
		w.recordOperation(ctx, "scope_proof_step_worker", "scope_proof", started, now, processed, err)
	}()
	claims, err := w.Store.ClaimScopeProofSteps(ctx, memory.ClaimScopeProofStepsInput{
		Scope:         w.Scope,
		WorkerID:      w.WorkerID,
		Now:           now,
		LeaseDuration: w.leaseDuration(),
		Limit:         w.batchSize(),
	})
	if err != nil {
		return 0, err
	}
	for _, step := range claims {
		result, execErr := w.Executor.ExecuteScopeProofStep(ctx, step)
		if execErr != nil {
			if recordErr := w.recordFailure(ctx, step, execErr); recordErr != nil {
				return 0, recordErr
			}
			continue
		}
		status := result.Status
		if status == "" {
			status = memory.ScopeProofStepStatusCompleted
		}
		verdict := result.Verdict
		if verdict == "" {
			verdict = memory.ScopeProofVerdictPassed
		}
		if err := w.Store.CompleteScopeProofStep(ctx, memory.CompleteScopeProofStepInput{
			Scope:           step.Scope,
			StepID:          step.ID,
			ProofID:         step.ProofID,
			WorkerID:        step.WorkerID,
			Status:          status,
			Verdict:         verdict,
			Evidence:        result.Evidence,
			FailureCategory: result.FailureCategory,
			CompletedAt:     w.nowUTC(),
		}); err != nil {
			return 0, err
		}
		w.recordProofStep(ctx, step.Step, status, verdict, result.FailureCategory)
		w.logProofStep("step_completed", step.Step, status, verdict, result.FailureCategory)
		if step.Step == memory.ScopeProofStepCompleted {
			if err := w.reduceProofVerdict(ctx, step, result, status, verdict); err != nil {
				return 0, err
			}
		}
		processed++
	}
	return processed, nil
}

func (w ScopeProofStepWorker) reduceProofVerdict(ctx context.Context, step memory.ScopeProofStep, result memory.ScopeProofStepResult, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict) error {
	runStatus := memory.ScopeProofStatusCompleted
	if status == memory.ScopeProofStepStatusFailed || verdict == memory.ScopeProofVerdictFailed {
		runStatus = memory.ScopeProofStatusFailed
	}
	if status == memory.ScopeProofStepStatusManualReview || verdict == memory.ScopeProofVerdictManualReview {
		runStatus = memory.ScopeProofStatusManualReview
	}
	_, err := w.Store.UpdateScopeProofRunStatus(ctx, memory.UpdateScopeProofRunStatusInput{
		Scope:           step.Scope,
		ProofID:         step.ProofID,
		Status:          runStatus,
		Verdict:         verdict,
		FailureCategory: result.FailureCategory,
		Summary:         result.Evidence,
		UpdatedAt:       w.nowUTC(),
		FinishedAt:      w.nowUTC(),
	})
	if err == nil {
		w.recordProofRun(ctx, runStatus, verdict, result.FailureCategory)
		w.logProofRun("run_updated", runStatus, verdict, result.FailureCategory)
	}
	return err
}

func (w ScopeProofStepWorker) recordFailure(ctx context.Context, step memory.ScopeProofStep, cause error) error {
	failedAt := w.nowUTC()
	input := memory.RecordScopeProofStepFailureInput{
		Scope:           step.Scope,
		StepID:          step.ID,
		ProofID:         step.ProofID,
		WorkerID:        step.WorkerID,
		Status:          memory.ScopeProofStepStatusFailed,
		Verdict:         memory.ScopeProofVerdictFailed,
		FailureCategory: memory.ProofFailureCategoryWorker,
		ErrorMessage:    truncateError(cause.Error(), 512),
		FailedAt:        failedAt,
	}
	if step.Attempt >= w.maxAttempts() {
		input.Status = memory.ScopeProofStepStatusExhausted
		if _, err := w.Store.UpdateScopeProofRunStatus(ctx, memory.UpdateScopeProofRunStatusInput{
			Scope:           step.Scope,
			ProofID:         step.ProofID,
			Status:          memory.ScopeProofStatusFailed,
			Verdict:         memory.ScopeProofVerdictFailed,
			FailureCategory: memory.ProofFailureCategoryWorker,
			Summary:         map[string]any{"failed_step": string(step.Step), "error": input.ErrorMessage},
			UpdatedAt:       failedAt,
			FinishedAt:      failedAt,
		}); err != nil {
			return err
		}
		w.recordProofRun(ctx, memory.ScopeProofStatusFailed, memory.ScopeProofVerdictFailed, memory.ProofFailureCategoryWorker)
		w.logProofRun("run_updated", memory.ScopeProofStatusFailed, memory.ScopeProofVerdictFailed, memory.ProofFailureCategoryWorker)
	} else {
		input.NextAttemptAt = failedAt.Add(w.retryBackoff())
	}
	if err := w.Store.RecordScopeProofStepFailure(ctx, input); err != nil {
		return err
	}
	w.recordProofStep(ctx, step.Step, input.Status, memory.ScopeProofVerdictFailed, input.FailureCategory)
	w.logProofStep("step_failed", step.Step, input.Status, memory.ScopeProofVerdictFailed, input.FailureCategory)
	return nil
}

func (w ScopeProofStepWorker) nowUTC() time.Time {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	return now().UTC()
}

func (w ScopeProofStepWorker) batchSize() int {
	if w.BatchSize > 0 {
		return w.BatchSize
	}
	return 10
}

func (w ScopeProofStepWorker) leaseDuration() time.Duration {
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}
	return time.Minute
}

func (w ScopeProofStepWorker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}
	return 5
}

func (w ScopeProofStepWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	return w.leaseDuration()
}

func (w ScopeProofStepWorker) recordOperation(ctx context.Context, component, operation string, started time.Time, observedAt time.Time, processed int, err error) {
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
		Component:  component,
		Operation:  operation,
		Status:     status,
		Count:      processed,
		Duration:   time.Since(started),
		Error:      errorMessage,
		ObservedAt: observedAt,
	})
}

func (w ScopeProofStepWorker) recordProofStep(ctx context.Context, step memory.ScopeProofStepName, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	observer, ok := w.Observer.(interface {
		RecordScopeProofStep(ctx context.Context, event telemetry.ScopeProofStepEvent)
	})
	if !ok {
		return
	}
	observer.RecordScopeProofStep(ctx, telemetry.ScopeProofStepEvent{
		Step:            string(step),
		Status:          string(status),
		Verdict:         string(verdict),
		Component:       proofStepComponent(step),
		FailureCategory: string(category),
	})
}

func (w ScopeProofStepWorker) recordProofRun(ctx context.Context, status memory.ScopeProofStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	observer, ok := w.Observer.(interface {
		RecordScopeProofRun(ctx context.Context, event telemetry.ScopeProofRunEvent)
	})
	if !ok {
		return
	}
	observer.RecordScopeProofRun(ctx, telemetry.ScopeProofRunEvent{
		Status:          string(status),
		Verdict:         string(verdict),
		FailureCategory: string(category),
	})
}

func (w ScopeProofStepWorker) logProofStep(event string, step memory.ScopeProofStepName, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"mode=worker component=scope_proof_step_worker event=%s step=%s status=%s verdict=%s failure_category=%s",
		event,
		step,
		status,
		verdict,
		labelOrUnknownString(string(category)),
	)
}

func (w ScopeProofStepWorker) logProofRun(event string, status memory.ScopeProofStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"mode=worker component=scope_proof_step_worker event=%s status=%s verdict=%s failure_category=%s",
		event,
		status,
		verdict,
		labelOrUnknownString(string(category)),
	)
}

func (w MemorySessionVerificationWorker) RunOnce(ctx context.Context) (processed int, err error) {
	started := time.Now()
	if w.Store == nil {
		return 0, fmt.Errorf("memory session verification store is required")
	}
	if w.Runner == nil {
		return 0, fmt.Errorf("memory session verification runner is required")
	}
	now := w.nowUTC()
	defer func() {
		w.recordOperation(ctx, "memory_session_verification_worker", "memory_session_verification", started, now, processed, err)
	}()
	claims, err := w.Store.ClaimMemorySessionVerifications(ctx, memory.ClaimMemorySessionVerificationsInput{
		Scope:         w.Scope,
		WorkerID:      w.WorkerID,
		Now:           now,
		LeaseDuration: w.leaseDuration(),
		Limit:         w.batchSize(),
	})
	if err != nil {
		return 0, err
	}
	for _, verification := range claims {
		result, verifyErr := w.Runner.VerifyMemorySession(ctx, verification)
		if verifyErr != nil {
			if recordErr := w.recordFailure(ctx, verification, verifyErr); recordErr != nil {
				return 0, recordErr
			}
			continue
		}
		status := result.Status
		if status == "" {
			status = memory.ScopeProofStepStatusCompleted
		}
		verdict := result.Verdict
		if verdict == "" {
			verdict = memory.ScopeProofVerdictPassed
		}
		if err := w.Store.CompleteMemorySessionVerification(ctx, memory.CompleteMemorySessionVerificationInput{
			Scope:           verification.Scope,
			VerificationID:  verification.ID,
			SessionID:       verification.SessionID,
			TurnID:          verification.TurnID,
			WorkerID:        verification.WorkerID,
			Status:          status,
			Verdict:         verdict,
			Evidence:        result.Evidence,
			FailureCategory: result.FailureCategory,
			CompletedAt:     w.nowUTC(),
		}); err != nil {
			return 0, err
		}
		if err := w.updateVerificationTargets(ctx, verification, result, status, verdict); err != nil {
			return 0, err
		}
		w.recordSessionVerification(ctx, status, verdict, result.FailureCategory)
		w.logSessionVerification("verification_completed", status, verdict, result.FailureCategory)
		processed++
	}
	return processed, nil
}

func (w MemorySessionVerificationWorker) updateVerificationTargets(ctx context.Context, verification memory.MemorySessionVerification, result memory.MemorySessionVerificationResult, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict) error {
	sessionStatus := memory.MemorySessionStatusCompleted
	if status == memory.ScopeProofStepStatusFailed || verdict == memory.ScopeProofVerdictFailed {
		sessionStatus = memory.MemorySessionStatusFailed
	}
	if status == memory.ScopeProofStepStatusManualReview || verdict == memory.ScopeProofVerdictManualReview {
		sessionStatus = memory.MemorySessionStatusManualReview
	}
	now := w.nowUTC()
	if _, err := w.Store.UpdateMemorySessionRunStatus(ctx, memory.UpdateMemorySessionRunStatusInput{
		Scope:           verification.Scope,
		SessionID:       verification.SessionID,
		Status:          sessionStatus,
		Verdict:         verdict,
		FailureCategory: result.FailureCategory,
		UpdatedAt:       now,
		FinishedAt:      now,
	}); err != nil {
		return err
	}
	w.recordSessionRun(ctx, sessionStatus, verdict, result.FailureCategory)
	w.logSessionRun("session_updated", sessionStatus, verdict, result.FailureCategory)
	if strings.TrimSpace(verification.TurnID) == "" {
		return nil
	}
	turnStatus := memory.MemorySessionTurnStatusVerified
	if status == memory.ScopeProofStepStatusFailed || verdict == memory.ScopeProofVerdictFailed {
		turnStatus = memory.MemorySessionTurnStatusFailed
	}
	_, err := w.Store.UpdateMemorySessionTurnOutcome(ctx, memory.UpdateMemorySessionTurnOutcomeInput{
		Scope:              verification.Scope,
		SessionID:          verification.SessionID,
		TurnID:             verification.TurnID,
		Status:             turnStatus,
		ExpectedRecall:     append([]string(nil), verification.ExpectedRecall...),
		VerificationStatus: verdict,
		FailureCategory:    result.FailureCategory,
		UpdatedAt:          now,
		VerifiedAt:         now,
	})
	if err == nil {
		w.recordSessionTurn(ctx, turnStatus, verdict, result.FailureCategory)
		w.logSessionTurn("turn_updated", turnStatus, verdict, result.FailureCategory)
	}
	return err
}

func (w MemorySessionVerificationWorker) recordFailure(ctx context.Context, verification memory.MemorySessionVerification, cause error) error {
	failedAt := w.nowUTC()
	input := memory.RecordMemorySessionVerificationFailureInput{
		Scope:           verification.Scope,
		VerificationID:  verification.ID,
		SessionID:       verification.SessionID,
		TurnID:          verification.TurnID,
		WorkerID:        verification.WorkerID,
		Status:          memory.ScopeProofStepStatusFailed,
		Verdict:         memory.ScopeProofVerdictFailed,
		FailureCategory: memory.ProofFailureCategoryWorker,
		ErrorMessage:    truncateError(cause.Error(), 512),
		FailedAt:        failedAt,
	}
	if verification.Attempt >= w.maxAttempts() {
		input.Status = memory.ScopeProofStepStatusExhausted
		if _, err := w.Store.UpdateMemorySessionRunStatus(ctx, memory.UpdateMemorySessionRunStatusInput{
			Scope:           verification.Scope,
			SessionID:       verification.SessionID,
			Status:          memory.MemorySessionStatusFailed,
			Verdict:         memory.ScopeProofVerdictFailed,
			FailureCategory: memory.ProofFailureCategoryWorker,
			UpdatedAt:       failedAt,
			FinishedAt:      failedAt,
		}); err != nil {
			return err
		}
		w.recordSessionRun(ctx, memory.MemorySessionStatusFailed, memory.ScopeProofVerdictFailed, memory.ProofFailureCategoryWorker)
		w.logSessionRun("session_updated", memory.MemorySessionStatusFailed, memory.ScopeProofVerdictFailed, memory.ProofFailureCategoryWorker)
	} else {
		input.NextAttemptAt = failedAt.Add(w.retryBackoff())
		if _, err := w.Store.UpdateMemorySessionRunStatus(ctx, memory.UpdateMemorySessionRunStatusInput{
			Scope:     verification.Scope,
			SessionID: verification.SessionID,
			Status:    memory.MemorySessionStatusVerifying,
			Verdict:   memory.ScopeProofVerdictPending,
			UpdatedAt: failedAt,
		}); err != nil {
			return err
		}
		w.recordSessionRun(ctx, memory.MemorySessionStatusVerifying, memory.ScopeProofVerdictPending, "")
		w.logSessionRun("session_updated", memory.MemorySessionStatusVerifying, memory.ScopeProofVerdictPending, "")
	}
	if err := w.Store.RecordMemorySessionVerificationFailure(ctx, input); err != nil {
		return err
	}
	w.recordSessionVerification(ctx, input.Status, memory.ScopeProofVerdictFailed, input.FailureCategory)
	w.logSessionVerification("verification_failed", input.Status, memory.ScopeProofVerdictFailed, input.FailureCategory)
	return nil
}

func (w MemorySessionVerificationWorker) nowUTC() time.Time {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	return now().UTC()
}

func (w MemorySessionVerificationWorker) batchSize() int {
	if w.BatchSize > 0 {
		return w.BatchSize
	}
	return 10
}

func (w MemorySessionVerificationWorker) leaseDuration() time.Duration {
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}
	return time.Minute
}

func (w MemorySessionVerificationWorker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}
	return 5
}

func (w MemorySessionVerificationWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	return w.leaseDuration()
}

func (w MemorySessionVerificationWorker) recordOperation(ctx context.Context, component, operation string, started time.Time, observedAt time.Time, processed int, err error) {
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
		Component:  component,
		Operation:  operation,
		Status:     status,
		Count:      processed,
		Duration:   time.Since(started),
		Error:      errorMessage,
		ObservedAt: observedAt,
	})
}

func (w MemorySessionVerificationWorker) recordSessionRun(ctx context.Context, status memory.MemorySessionStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	observer, ok := w.Observer.(interface {
		RecordMemorySessionRun(ctx context.Context, event telemetry.MemorySessionRunEvent)
	})
	if !ok {
		return
	}
	observer.RecordMemorySessionRun(ctx, telemetry.MemorySessionRunEvent{
		Status:          string(status),
		Verdict:         string(verdict),
		FailureCategory: string(category),
	})
}

func (w MemorySessionVerificationWorker) recordSessionTurn(ctx context.Context, status memory.MemorySessionTurnStatus, verificationStatus memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	observer, ok := w.Observer.(interface {
		RecordMemorySessionTurn(ctx context.Context, event telemetry.MemorySessionTurnEvent)
	})
	if !ok {
		return
	}
	observer.RecordMemorySessionTurn(ctx, telemetry.MemorySessionTurnEvent{
		Status:             string(status),
		VerificationStatus: string(verificationStatus),
		FailureCategory:    string(category),
	})
}

func (w MemorySessionVerificationWorker) recordSessionVerification(ctx context.Context, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	observer, ok := w.Observer.(interface {
		RecordMemorySessionVerification(ctx context.Context, event telemetry.MemorySessionVerificationEvent)
	})
	if !ok {
		return
	}
	observer.RecordMemorySessionVerification(ctx, telemetry.MemorySessionVerificationEvent{
		Status:          string(status),
		Verdict:         string(verdict),
		FailureCategory: string(category),
	})
}

func (w MemorySessionVerificationWorker) logSessionRun(event string, status memory.MemorySessionStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"mode=worker component=memory_session_verification_worker event=%s status=%s verdict=%s failure_category=%s",
		event,
		status,
		verdict,
		labelOrUnknownString(string(category)),
	)
}

func (w MemorySessionVerificationWorker) logSessionTurn(event string, status memory.MemorySessionTurnStatus, verificationStatus memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"mode=worker component=memory_session_verification_worker event=%s status=%s verification_status=%s failure_category=%s",
		event,
		status,
		verificationStatus,
		labelOrUnknownString(string(category)),
	)
}

func (w MemorySessionVerificationWorker) logSessionVerification(event string, status memory.ScopeProofStepStatus, verdict memory.ScopeProofVerdict, category memory.ProofFailureCategory) {
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"mode=worker component=memory_session_verification_worker event=%s status=%s verdict=%s failure_category=%s",
		event,
		status,
		verdict,
		labelOrUnknownString(string(category)),
	)
}

func proofStepComponent(step memory.ScopeProofStepName) string {
	switch step {
	case memory.ScopeProofStepScopeResolved:
		return "scope"
	case memory.ScopeProofStepFixturePlanned:
		return "fixture"
	case memory.ScopeProofStepIngestion:
		return "ingestion"
	case memory.ScopeProofStepGovernanceProcessed:
		return "governance"
	case memory.ScopeProofStepRetrievalRecalled:
		return "retrieval"
	case memory.ScopeProofStepContextAssembled:
		return "context"
	case memory.ScopeProofStepReplayChecked:
		return "replay"
	case memory.ScopeProofStepQualityEvaluated:
		return "quality"
	case memory.ScopeProofStepRepairRecommended:
		return "repair"
	case memory.ScopeProofStepCompleted:
		return "verdict"
	default:
		return "unknown"
	}
}

func labelOrUnknownString(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func (w AssuranceAlertDeliveryWorker) RunOnce(ctx context.Context) (processed int, err error) {
	started := time.Now()
	if w.Store == nil {
		return 0, fmt.Errorf("assurance alert delivery store is required")
	}
	if w.Service == nil {
		return 0, fmt.Errorf("assurance alert delivery service is required")
	}
	now := w.nowUTC()
	defer func() {
		w.recordOperation(ctx, "assurance_alert_delivery_worker", "alert_delivery", started, now, processed, err)
	}()
	claims, err := w.Store.ClaimAlertCandidatesForDelivery(ctx, assurance.ClaimAlertCandidatesForDeliveryInput{
		Scope:         w.Scope,
		WorkerID:      w.workerID(),
		Now:           now,
		LeaseDuration: w.leaseDuration(),
		Limit:         w.batchSize(),
		MaxAttempts:   w.maxAttempts(),
	})
	if err != nil {
		return 0, err
	}
	for _, claim := range claims {
		if _, err := w.Service.DeliverAlertCandidate(ctx, assurance.AlertDeliveryInput{
			Scope:        claim.Candidate.Scope,
			Candidate:    claim.Candidate,
			Config:       w.Config,
			MaxAttempts:  w.maxAttempts(),
			RetryBackoff: w.retryBackoff(),
			WorkerID:     claim.WorkerID,
			Now:          now,
		}); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (w AssuranceAlertDeliveryWorker) nowUTC() time.Time {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	return now().UTC()
}

func (w AssuranceAlertDeliveryWorker) workerID() string {
	if strings.TrimSpace(w.WorkerID) != "" {
		return strings.TrimSpace(w.WorkerID)
	}
	return "stele-assurance-alert-delivery-worker"
}

func (w AssuranceAlertDeliveryWorker) batchSize() int {
	if w.BatchSize > 0 {
		return w.BatchSize
	}
	return 10
}

func (w AssuranceAlertDeliveryWorker) leaseDuration() time.Duration {
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}
	return time.Minute
}

func (w AssuranceAlertDeliveryWorker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}
	return 5
}

func (w AssuranceAlertDeliveryWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	return w.leaseDuration()
}

func (w AssuranceAlertDeliveryWorker) recordOperation(ctx context.Context, component, operation string, started time.Time, observedAt time.Time, processed int, err error) {
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
		Component:  component,
		Operation:  operation,
		Status:     status,
		Count:      processed,
		Duration:   time.Since(started),
		Error:      errorMessage,
		ObservedAt: observedAt,
	})
}

func (w RepairActionWorker) RunOnce(ctx context.Context) (processed int, err error) {
	started := time.Now()
	if w.Store == nil {
		return 0, fmt.Errorf("repair action store is required")
	}
	if w.Processor == nil {
		return 0, fmt.Errorf("repair action processor is required")
	}
	now := w.nowUTC()
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
			Component:  "repair_action_worker",
			Operation:  "quality_repair",
			Status:     status,
			Count:      processed,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: now,
		})
	}()
	claims, err := w.Store.ClaimRepairActions(ctx, memory.ClaimRepairActionsInput{
		Scope:         w.Scope,
		WorkerID:      w.WorkerID,
		Now:           now,
		LeaseDuration: w.leaseDuration(),
		Limit:         w.batchSize(),
	})
	if err != nil {
		return 0, err
	}
	for _, action := range claims {
		if err := w.Processor.ProcessRepairAction(ctx, action); err != nil {
			if recordErr := w.recordFailure(ctx, action, err); recordErr != nil {
				return 0, recordErr
			}
			continue
		}
		if err := w.Store.CompleteRepairAction(ctx, memory.CompleteRepairActionInput{
			Scope:       action.Scope,
			ActionID:    action.ID,
			WorkerID:    action.WorkerID,
			CompletedAt: w.nowUTC(),
			Status:      memory.RepairActionStatusCompleted,
		}); err != nil {
			return 0, err
		}
		processed++
	}
	return processed, nil
}

func (w RepairActionWorker) recordFailure(ctx context.Context, action memory.RepairAction, cause error) error {
	failedAt := w.nowUTC()
	input := memory.RecordRepairActionFailureInput{
		Scope:        action.Scope,
		ActionID:     action.ID,
		WorkerID:     action.WorkerID,
		ErrorMessage: truncateError(cause.Error(), 512),
		FailedAt:     failedAt,
	}
	if action.Attempt >= w.maxAttempts() {
		input.Exhausted = true
	} else {
		input.NextAttemptAt = failedAt.Add(w.retryBackoff())
	}
	return w.Store.RecordRepairActionFailure(ctx, input)
}

func (w RepairActionWorker) nowUTC() time.Time {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	return now().UTC()
}

func (w RepairActionWorker) batchSize() int {
	if w.BatchSize > 0 {
		return w.BatchSize
	}
	return 10
}

func (w RepairActionWorker) leaseDuration() time.Duration {
	if w.LeaseDuration > 0 {
		return w.LeaseDuration
	}
	return time.Minute
}

func (w RepairActionWorker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}
	return 5
}

func (w RepairActionWorker) retryBackoff() time.Duration {
	if w.RetryBackoff > 0 {
		return w.RetryBackoff
	}
	return w.leaseDuration()
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

type LoopWorker interface {
	RunOnce(ctx context.Context) (int, error)
}

type waitFunc func(ctx context.Context, d time.Duration) error

type PollingWorker struct {
	Worker       LoopWorker
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

type assuranceEvaluationService interface {
	CreateHealthEvaluation(ctx context.Context, input assurance.HealthEvaluationInput) (assurance.HealthEvaluation, error)
	GenerateAlertCandidates(ctx context.Context, input assurance.AlertCandidateGenerationInput) ([]assurance.AlertCandidate, error)
	CreateReadinessReport(ctx context.Context, input assurance.ReadinessReportInput) (assurance.ReadinessReport, error)
}

type conformanceRunService interface {
	ListConformanceProfiles(ctx context.Context, input assurance.ListConformanceProfilesInput) ([]assurance.ConformanceProfile, error)
	RunConformance(ctx context.Context, input assurance.ConformanceRunInput) (assurance.ConformanceRun, []assurance.MissingEvidenceDiagnostic, error)
}

type assuranceRetentionService interface {
	CreateRetentionRun(ctx context.Context, run assurance.RetentionRun) (assurance.RetentionRun, error)
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
	SummarizeDerivedInsightFeedback(ctx context.Context, input memory.SummarizeDerivedInsightFeedbackInput) (memory.DerivedInsightFeedbackSummary, error)
	TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error
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

type AssuranceEvaluationJob struct {
	Scope               memory.Scope
	Service             assuranceEvaluationService
	ExecutionStore      ExecutionStore
	TriggerSource       string
	Cadence             time.Duration
	AlertDeliveryPolicy string
	AlertDeduplication  time.Duration
	Now                 func() time.Time
}

func (j AssuranceEvaluationJob) Name() string {
	return "assurance_evaluation"
}

func (j AssuranceEvaluationJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Service == nil {
		return 0, fmt.Errorf("assurance evaluation service is required")
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

	processed := 0
	evaluation, err := j.Service.CreateHealthEvaluation(ctx, assurance.HealthEvaluationInput{
		Scope:      j.Scope,
		ObservedAt: current,
		RuntimeReadiness: assurance.HealthObservation{
			Status:     assurance.HealthStatusHealthy,
			Severity:   assurance.SeverityInfo,
			Reason:     assurance.ReasonRuntimeReady,
			ObservedAt: current,
			Evidence: map[string]any{
				"source": "scheduler",
			},
		},
	})
	if err != nil {
		return processed, j.fail(ctx, idempotencyKey, current, err)
	}
	processed++

	deliveryPolicy := strings.TrimSpace(j.AlertDeliveryPolicy)
	if deliveryPolicy == "" {
		deliveryPolicy = "default"
	}
	if _, err := j.Service.GenerateAlertCandidates(ctx, assurance.AlertCandidateGenerationInput{
		Scope:               j.Scope,
		Evaluation:          evaluation,
		DeliveryPolicy:      deliveryPolicy,
		DeduplicationWindow: j.AlertDeduplication,
		CreatedAt:           current,
	}); err != nil {
		return processed, j.fail(ctx, idempotencyKey, current, err)
	}
	processed++

	if _, err := j.Service.CreateReadinessReport(ctx, assurance.ReadinessReportInput{
		Scope:       j.Scope,
		GeneratedAt: current,
	}); err != nil {
		return processed, j.fail(ctx, idempotencyKey, current, err)
	}
	processed++

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return processed, err
	}

	return processed, nil
}

func (j AssuranceEvaluationJob) fail(ctx context.Context, idempotencyKey string, finishedAt time.Time, cause error) error {
	if err := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, finishedAt, cause); err != nil {
		return err
	}
	return cause
}

func (j AssuranceEvaluationJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}
	return "scheduler"
}

type ConformanceRunJob struct {
	Scope          memory.Scope
	Service        conformanceRunService
	ExecutionStore ExecutionStore
	TriggerSource  string
	Cadence        time.Duration
	Now            func() time.Time
	Limit          int
}

func (j ConformanceRunJob) Name() string {
	return "conformance_run"
}

func (j ConformanceRunJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Service == nil {
		return 0, fmt.Errorf("conformance run service is required")
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

	profiles, err := j.Service.ListConformanceProfiles(ctx, assurance.ListConformanceProfilesInput{
		Scope:  j.Scope,
		Status: assurance.ConformanceProfileStatusActive,
	})
	if err != nil {
		return 0, j.fail(ctx, idempotencyKey, current, err)
	}

	limit := j.Limit
	if limit <= 0 {
		limit = 20
	}
	processed := 0
	for _, profile := range profiles {
		if profile.Status != assurance.ConformanceProfileStatusActive {
			continue
		}
		if processed >= limit {
			break
		}
		if _, _, err := j.Service.RunConformance(ctx, assurance.ConformanceRunInput{
			Scope:     j.Scope,
			ProfileID: profile.ID,
			StartedAt: current,
		}); err != nil {
			return processed, j.fail(ctx, idempotencyKey, current, err)
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return processed, err
	}

	return processed, nil
}

func (j ConformanceRunJob) fail(ctx context.Context, idempotencyKey string, finishedAt time.Time, cause error) error {
	if err := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, finishedAt, cause); err != nil {
		return err
	}
	return cause
}

func (j ConformanceRunJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}
	return "scheduler"
}

type AssuranceRetentionJob struct {
	Scope                memory.Scope
	Service              assuranceRetentionService
	ExecutionStore       ExecutionStore
	TriggerSource        string
	Cadence              time.Duration
	HistoryRetention     time.Duration
	ConformanceRetention time.Duration
	Now                  func() time.Time
	NewRunID             func(prefix string) string
}

func (j AssuranceRetentionJob) Name() string {
	return "assurance_retention"
}

func (j AssuranceRetentionJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Service == nil {
		return 0, fmt.Errorf("assurance retention service is required")
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

	historyRetention := j.HistoryRetention
	if historyRetention <= 0 {
		historyRetention = 7 * 24 * time.Hour
	}
	conformanceRetention := j.ConformanceRetention
	if conformanceRetention <= 0 {
		conformanceRetention = historyRetention
	}

	processed := 0
	for _, spec := range []struct {
		prefix   string
		class    assurance.RetentionClass
		retained time.Duration
	}{
		{prefix: "assurance_retention_diagnostic", class: assurance.RetentionClassDiagnostic, retained: historyRetention},
		{prefix: "assurance_retention_conformance", class: assurance.RetentionClassAudit, retained: conformanceRetention},
	} {
		if _, err := j.Service.CreateRetentionRun(ctx, assurance.RetentionRun{
			ID:             j.newRunID(spec.prefix),
			Scope:          j.Scope,
			RecordCategory: spec.class,
			Cutoff:         current.Add(-spec.retained),
			DeletedCount:   0,
			Status:         assurance.HealthStatusHealthy,
			StartedAt:      current,
			FinishedAt:     current,
		}); err != nil {
			return processed, j.fail(ctx, idempotencyKey, current, err)
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return processed, err
	}

	return processed, nil
}

func (j AssuranceRetentionJob) newRunID(prefix string) string {
	if j.NewRunID != nil {
		return j.NewRunID(prefix)
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func (j AssuranceRetentionJob) fail(ctx context.Context, idempotencyKey string, finishedAt time.Time, cause error) error {
	if err := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, finishedAt, cause); err != nil {
		return err
	}
	return cause
}

func (j AssuranceRetentionJob) triggerSource() string {
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
	Observer        telemetry.Observer
}

type derivedInsightReplayExecutionService interface {
	ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error)
	ExecuteDerivedInsightReplay(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayReport, error)
}

type DerivedInsightReplayExecutionJob struct {
	Scope          memory.Scope
	Service        derivedInsightReplayExecutionService
	ExecutionStore ExecutionStore
	TriggerSource  string
	Cadence        time.Duration
	Now            func() time.Time
	Limit          int
}

func (j DerivedInsightReplayExecutionJob) Name() string {
	return "derived_insight_replay_execution"
}

func (j DerivedInsightReplayExecutionJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Service == nil {
		return 0, fmt.Errorf("derived insight replay execution service is required")
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
		limit = 20
	}

	runs, err := j.Service.ListDerivedInsightReplayRuns(ctx, memory.ListDerivedInsightReplayRunsInput{
		Scope:  j.Scope,
		Status: memory.DerivedInsightReplayStatusPending,
		Mode:   memory.DerivedInsightReplayModeApply,
		Limit:  limit,
	})
	if err != nil {
		if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
			return 0, failErr
		}
		return 0, err
	}

	processed := 0
	for _, run := range runs {
		if _, err := j.Service.ExecuteDerivedInsightReplay(ctx, run); err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return processed, failErr
			}
			return processed, err
		}
		processed++
	}

	if err := completeScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, processed); err != nil {
		return processed, err
	}

	return processed, nil
}

func (j DerivedInsightReplayExecutionJob) triggerSource() string {
	if j.TriggerSource != "" {
		return j.TriggerSource
	}

	return "scheduler"
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
		feedback, err := j.Store.SummarizeDerivedInsightFeedback(ctx, memory.SummarizeDerivedInsightFeedbackInput{
			Scope:     pattern.Scope,
			InsightID: pattern.ID,
		})
		if err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		originalPattern := pattern
		var decision insights.FeedbackPolicyDecision
		pattern, decision = insights.ApplyFeedbackPolicy(pattern, feedback)
		upsertTarget := pattern
		if decision == insights.FeedbackPolicyDecisionSuppress {
			upsertTarget = originalPattern
		}
		storedPattern, err := j.Store.UpsertDerivedInsight(ctx, upsertTarget)
		if err != nil {
			if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
				return 0, failErr
			}
			return 0, err
		}
		processed++
		if decision == insights.FeedbackPolicyDecisionSuppress {
			j.recordInsightFeedbackPolicy(ctx, storedPattern, feedback, decision, "ok")
			if err := j.Store.TransitionDerivedInsightLifecycle(ctx, memory.DerivedInsightLifecycleTransition{
				Scope:      storedPattern.Scope,
				InsightID:  storedPattern.ID,
				FromState:  originalPattern.State,
				ToState:    memory.DerivedInsightStateSuppressed,
				Actor:      "feedback_policy",
				Reason:     "suppressed by derived insight feedback policy",
				OccurredAt: current,
				Metadata: map[string]any{
					"feedback_positive_count": float64(feedback.PositiveCount),
					"feedback_negative_count": float64(feedback.NegativeCount),
					"feedback_needs_review":   feedback.NeedsReview,
				},
			}); err != nil {
				if failErr := failScheduledExecution(ctx, j.ExecutionStore, idempotencyKey, current, err); failErr != nil {
					return 0, failErr
				}
				return 0, err
			}
			storedPattern.State = memory.DerivedInsightStateSuppressed
		} else if decision != insights.FeedbackPolicyDecisionNone {
			j.recordInsightFeedbackPolicy(ctx, storedPattern, feedback, decision, "ok")
		}
		if storedPattern.State != memory.DerivedInsightStateActive {
			continue
		}

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

func (j DerivedInsightDerivationJob) recordInsightFeedbackPolicy(ctx context.Context, insight memory.DerivedInsight, summary memory.DerivedInsightFeedbackSummary, decision insights.FeedbackPolicyDecision, result string) {
	observer, ok := j.Observer.(interface {
		RecordInsightFeedback(ctx context.Context, event telemetry.InsightFeedbackEvent)
	})
	if !ok {
		return
	}
	feedbackType := "mixed"
	switch {
	case summary.Counts[memory.InsightFeedbackTypeIncorrect] > 0:
		feedbackType = string(memory.InsightFeedbackTypeIncorrect)
	case summary.Counts[memory.InsightFeedbackTypeNoisy] > 0:
		feedbackType = string(memory.InsightFeedbackTypeNoisy)
	case summary.Counts[memory.InsightFeedbackTypeStale] > 0:
		feedbackType = string(memory.InsightFeedbackTypeStale)
	case summary.Counts[memory.InsightFeedbackTypeRedundant] > 0:
		feedbackType = string(memory.InsightFeedbackTypeRedundant)
	case summary.Counts[memory.InsightFeedbackTypeNeedsReview] > 0:
		feedbackType = string(memory.InsightFeedbackTypeNeedsReview)
	case summary.Counts[memory.InsightFeedbackTypeUseful] > 0:
		feedbackType = string(memory.InsightFeedbackTypeUseful)
	}
	observer.RecordInsightFeedback(ctx, telemetry.InsightFeedbackEvent{
		Operation:    "policy",
		Result:       result,
		FeedbackType: feedbackType,
		InsightType:  string(insight.Type),
		Decision:     string(decision),
	})
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
