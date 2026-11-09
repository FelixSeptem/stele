package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// ProceduralRevalidationStore is deliberately narrower than a governed-intent
// processor. A revalidator may append an audit result and settle its own run;
// it has no capability for candidate creation or canonical mutation.
type ProceduralRevalidationStore interface {
	ClaimProceduralRevalidationRun(context.Context, memory.ClaimProceduralRevalidationRunInput) (memory.ProceduralRevalidationRun, error)
	ReadProceduralMemoryVerification(context.Context, memory.Scope, string) (memory.ProceduralMemoryVerification, error)
	AppendProceduralMemoryVerification(context.Context, memory.ProceduralMemoryVerification) (memory.ProceduralMemoryVerification, error)
	CompleteProceduralRevalidationRun(context.Context, memory.CompleteProceduralRevalidationRunInput) error
	ExhaustProceduralRevalidationRun(context.Context, memory.ExhaustProceduralRevalidationRunInput) error
}

// ProceduralRevalidationWorker is a DerivedWorkExecutor. It can only record
// source drift or lack of a new administrator conclusion; it never infers
// support/refutation, generates correction text, or submits a governed intent.
type ProceduralRevalidationWorker struct {
	Store         ProceduralRevalidationStore
	Targets       memory.ProceduralVerificationTargetResolver
	Evidence      memory.ProceduralVerificationEvidenceResolver
	Policy        memory.ProceduralVerificationPolicy
	WorkerID      string
	LeaseDuration time.Duration
	Now           func() time.Time
	NewID         func() string
	Observer      interface {
		RecordProceduralVerification(context.Context, telemetry.ProceduralVerificationEvent)
	}
}

func (w ProceduralRevalidationWorker) ExecuteDerivedWork(ctx context.Context, item workqueue.DerivedWorkItem) (int64, string, error) {
	if item.Kind != workqueue.WorkKindProceduralRevalidation {
		return 0, "", fmt.Errorf("procedural revalidation worker cannot execute %q", item.Kind)
	}
	if w.Store == nil || w.Targets == nil || w.Evidence == nil || w.Policy == nil || strings.TrimSpace(w.WorkerID) == "" {
		return 0, "", fmt.Errorf("procedural revalidation worker is not configured")
	}
	if err := item.Scope.Validate(); err != nil || strings.TrimSpace(item.Reference) == "" {
		return 0, "", fmt.Errorf("procedural revalidation work item is invalid")
	}
	decision, err := w.Policy.EvaluateProceduralVerification(ctx, item.Scope)
	if err != nil {
		return 0, "", fmt.Errorf("evaluate procedural revalidation policy: %w", err)
	}
	if !decision.Enabled {
		return 0, "", fmt.Errorf("procedural revalidation policy is disabled")
	}
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	lease := w.LeaseDuration
	if lease <= 0 {
		lease = time.Minute
	}
	run, err := w.Store.ClaimProceduralRevalidationRun(ctx, memory.ClaimProceduralRevalidationRunInput{Scope: item.Scope, RunID: item.Reference, WorkerID: w.WorkerID, Now: now, LeaseDuration: lease})
	if err != nil {
		return 0, "", fmt.Errorf("claim procedural revalidation run: %w", err)
	}
	if run.Scope.Normalized() != item.Scope.Normalized() || run.ID != item.Reference || run.Status != memory.ProceduralRevalidationRunStatusRunning || run.LeaseOwner != w.WorkerID {
		return 0, "", fmt.Errorf("procedural revalidation run is unavailable")
	}
	verification, err := w.Store.ReadProceduralMemoryVerification(ctx, item.Scope, run.VerificationID)
	if err != nil {
		return 0, "", fmt.Errorf("read procedural verification: %w", err)
	}
	if verification.Scope.Normalized() != item.Scope.Normalized() || verification.MemoryID != run.MemoryID || verification.TargetVersion != run.TargetVersion {
		return 0, "", fmt.Errorf("procedural verification run target mismatch")
	}

	// A target that can no longer be viewed as current/active is terminal work,
	// not semantic proof. Preserve its audit chain by settling to the original
	// verification without appending a result whose DB target guard would fail.
	if err := w.Targets.ValidateProceduralVerificationTarget(ctx, verification); err != nil {
		if completeErr := w.Store.CompleteProceduralRevalidationRun(ctx, memory.CompleteProceduralRevalidationRunInput{Scope: item.Scope, RunID: run.ID, WorkerID: run.LeaseOwner, Status: memory.ProceduralRevalidationRunStatusFailed, TerminalVerificationID: verification.ID, CompletedAt: now}); completeErr != nil {
			w.recordRevalidationTelemetry(ctx, run, "failed", "dependency")
			return 0, "", fmt.Errorf("complete unavailable procedural revalidation run: %w", completeErr)
		}
		w.recordRevalidationTelemetry(ctx, run, "failed", "target_ineligible")
		return 1, verification.ID, nil
	}
	disposition := memory.ProceduralVerificationDispositionInconclusive
	for _, evidence := range verification.Evidence {
		if evidence.Scope == (memory.Scope{}) {
			evidence.Scope = verification.Scope.Normalized()
		}
		if err := w.Evidence.ValidateProceduralVerificationEvidence(ctx, item.Scope, evidence); err != nil {
			disposition = memory.ProceduralVerificationDispositionNeedsRevalidation
			break
		}
	}
	id := ""
	if w.NewID != nil {
		id = w.NewID()
	}
	if strings.TrimSpace(id) == "" {
		return 0, "", fmt.Errorf("procedural revalidation verification id is required")
	}
	result := verification
	result.ID = id
	result.Disposition = disposition
	result.CorrectedContent = ""
	result.Actor = "procedural-revalidation-worker"
	if disposition == memory.ProceduralVerificationDispositionNeedsRevalidation {
		result.Reason = "recorded verification evidence is no longer eligible or current"
	} else {
		result.Reason = "recorded verification evidence remains eligible without a new administrator conclusion"
	}
	result.CreatedAt = now
	stored, err := w.Store.AppendProceduralMemoryVerification(ctx, result)
	if err != nil {
		return 0, "", fmt.Errorf("append procedural revalidation result: %w", err)
	}
	if err := w.Store.CompleteProceduralRevalidationRun(ctx, memory.CompleteProceduralRevalidationRunInput{Scope: item.Scope, RunID: run.ID, WorkerID: run.LeaseOwner, Status: memory.ProceduralRevalidationRunStatusCompleted, TerminalVerificationID: stored.ID, CompletedAt: now}); err != nil {
		return 0, "", fmt.Errorf("complete procedural revalidation run: %w", err)
	}
	w.recordRevalidationTelemetry(ctx, run, string(disposition), "none")
	return 1, stored.ID, nil
}

func (w ProceduralRevalidationWorker) recordRevalidationTelemetry(ctx context.Context, run memory.ProceduralRevalidationRun, result, failureCategory string) {
	if w.Observer == nil {
		return
	}
	disposition := "none"
	if result == string(memory.ProceduralVerificationDispositionInconclusive) || result == string(memory.ProceduralVerificationDispositionNeedsRevalidation) {
		disposition = result
	}
	w.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "revalidate", Result: result, Disposition: disposition, Trigger: string(run.Trigger), Queue: "durable", Recovery: "none", FailureCategory: failureCategory,
	})
}

// HandleDerivedWorkExhaustion implements DerivedWorkExhaustionHandler. The
// shared queue worker has already made the delivery terminal; this method
// settles the matching child run only if this worker still owns its durable
// lease. A lease lost to restart/recovery is benign because its owner or the
// recovery path is responsible for the run.
func (w ProceduralRevalidationWorker) HandleDerivedWorkExhaustion(ctx context.Context, item workqueue.DerivedWorkItem) error {
	if w.Store == nil || strings.TrimSpace(w.WorkerID) == "" {
		return fmt.Errorf("procedural revalidation worker is not configured")
	}
	if item.Kind != workqueue.WorkKindProceduralRevalidation {
		return fmt.Errorf("procedural revalidation worker cannot exhaust %q", item.Kind)
	}
	if err := item.Scope.Validate(); err != nil || strings.TrimSpace(item.Reference) == "" {
		return fmt.Errorf("procedural revalidation work item is invalid")
	}
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	err := w.Store.ExhaustProceduralRevalidationRun(ctx, memory.ExhaustProceduralRevalidationRunInput{
		Scope: item.Scope, RunID: item.Reference, WorkerID: w.WorkerID, ExhaustedAt: now,
	})
	if errors.Is(err, memory.ErrProceduralRevalidationLeaseLost) {
		w.recordExhaustionTelemetry(ctx, "completed", "lease_lost")
		return nil
	}
	if err != nil {
		w.recordExhaustionTelemetry(ctx, "failed", "dependency")
		return fmt.Errorf("exhaust procedural revalidation run: %w", err)
	}
	w.recordExhaustionTelemetry(ctx, "exhausted", "none")
	return nil
}

func (w ProceduralRevalidationWorker) recordExhaustionTelemetry(ctx context.Context, result, failureCategory string) {
	if w.Observer == nil {
		return
	}
	w.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "exhaust", Result: result, Disposition: "none", Trigger: "none", Queue: "durable", Recovery: "exhausted", FailureCategory: failureCategory,
	})
}
