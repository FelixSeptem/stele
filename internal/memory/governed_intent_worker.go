package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MemoryIntentOutcome struct {
	Status             MemoryIntentStatus
	DiagnosticCategory MemoryIntentDiagnosticCategory
	CandidateReference string
	OutcomeReference   string
}

func (o MemoryIntentOutcome) Validate() error {
	if !o.Status.Valid() || !o.DiagnosticCategory.Valid() {
		return fmt.Errorf("memory intent outcome state or diagnostic category is invalid")
	}
	if len(o.CandidateReference) > MemoryIntentMaxEvidenceRef || len(o.OutcomeReference) > MemoryIntentMaxEvidenceRef {
		return fmt.Errorf("memory intent outcome references are too large")
	}
	return nil
}

// MemoryIntentGovernanceRouter adapts one intent to the existing candidate,
// lifecycle, insight-review, or feedback policy. Implementations own all
// canonical writes and must keep them append-only.
type MemoryIntentGovernanceRouter interface {
	ProcessRememberOrUpdate(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error)
	ProcessForget(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error)
	ProcessContradiction(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error)
	ProcessFeedback(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error)
}

type MemoryIntentWorker struct {
	Router    MemoryIntentGovernanceRouter
	Validator MemoryIntentTargetValidator
	Now       func() time.Time
}

type MemoryIntentOutcomeRecorder interface {
	AppendMemoryIntentTransition(context.Context, MemoryIntentTransition) error
}

func (w MemoryIntentWorker) ProcessAndRecord(ctx context.Context, record MemoryIntentRecord, recorder MemoryIntentOutcomeRecorder) (MemoryIntentOutcome, error) {
	return w.processAndRecord(ctx, record, recorder, 2, record.Status)
}

// ProcessAndRecordRetry resumes a queue retry after a failed transition. A
// failed outcome remains auditable, but it must not make the durable work item
// look successfully replayed; the next attempt appends the next sequence.
func (w MemoryIntentWorker) ProcessAndRecordRetry(ctx context.Context, record MemoryIntentRecord, recorder MemoryIntentOutcomeRecorder, sequence int64, from MemoryIntentStatus) (MemoryIntentOutcome, error) {
	if sequence < 2 {
		sequence = 2
	}
	return w.processAndRecord(ctx, record, recorder, sequence, from)
}

func (w MemoryIntentWorker) processAndRecord(ctx context.Context, record MemoryIntentRecord, recorder MemoryIntentOutcomeRecorder, sequence int64, from MemoryIntentStatus) (MemoryIntentOutcome, error) {
	out, err := w.Process(ctx, record)
	if recorder == nil {
		return out, err
	}
	if record.ID == "" {
		return MemoryIntentOutcome{}, fmt.Errorf("intent id is required")
	}
	category := out.DiagnosticCategory
	if !category.Valid() {
		category = MemoryIntentDiagnosticFailed
	}
	status := out.Status
	if !status.Valid() {
		status = MemoryIntentStatusFailed
	}
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	transitionErr := recorder.AppendMemoryIntentTransition(ctx, MemoryIntentTransition{IntentID: record.ID, Scope: record.Scope, Sequence: sequence, From: from, To: status, Actor: record.Actor, Reason: record.Reason, DiagnosticCategory: category, OutcomeReference: out.OutcomeReference, WorkReference: out.CandidateReference, OccurredAt: now})
	if transitionErr != nil {
		return out, fmt.Errorf("record intent outcome: %w", transitionErr)
	}
	return out, err
}

func (w MemoryIntentWorker) Process(ctx context.Context, record MemoryIntentRecord) (MemoryIntentOutcome, error) {
	if err := record.Scope.Validate(); err != nil {
		return MemoryIntentOutcome{}, err
	}
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Actor) == "" || strings.TrimSpace(record.Reason) == "" {
		return MemoryIntentOutcome{}, fmt.Errorf("intent identity and attribution are required")
	}
	if record.Status != MemoryIntentStatusAccepted && record.Status != MemoryIntentStatusPending {
		return MemoryIntentOutcome{Status: MemoryIntentStatusReplayed, DiagnosticCategory: MemoryIntentDiagnosticReplayed, OutcomeReference: record.ID}, nil
	}
	if w.Router == nil {
		return MemoryIntentOutcome{}, fmt.Errorf("memory intent governance router is not configured")
	}
	if w.Validator != nil && record.Type != MemoryIntentRemember {
		input := MemoryIntentInput{Scope: record.Scope, MemoryPath: record.MemoryPath, Type: record.Type, TargetMemoryID: record.TargetMemoryID, TargetVersion: record.TargetVersion, Content: record.Content, Actor: record.Actor, Reason: record.Reason, Provenance: record.Provenance, RequestID: record.RequestID, OperationID: record.OperationID, IdempotencyKey: record.IdempotencyKey, TargetInsightID: record.TargetInsightID, Evidence: record.Evidence}
		if err := w.Validator.ValidateMemoryIntentTarget(ctx, input); err != nil {
			return MemoryIntentOutcome{Status: MemoryIntentStatusRejected, DiagnosticCategory: MemoryIntentDiagnosticTargetStale, OutcomeReference: record.ID}, nil
		}
	}
	var (
		out MemoryIntentOutcome
		err error
	)
	switch record.Type {
	case MemoryIntentRemember, MemoryIntentUpdate:
		out, err = w.Router.ProcessRememberOrUpdate(ctx, record)
	case MemoryIntentForget:
		out, err = w.Router.ProcessForget(ctx, record)
	case MemoryIntentContradiction:
		out, err = w.Router.ProcessContradiction(ctx, record)
		if err == nil && out.Status == MemoryIntentStatusActive {
			return MemoryIntentOutcome{Status: MemoryIntentStatusSuppressed, DiagnosticCategory: MemoryIntentDiagnosticSuppressed, OutcomeReference: record.ID}, nil
		}
	case MemoryIntentFeedback:
		out, err = w.Router.ProcessFeedback(ctx, record)
		if err == nil && out.Status == MemoryIntentStatusActive {
			return MemoryIntentOutcome{Status: MemoryIntentStatusCandidate, DiagnosticCategory: MemoryIntentDiagnosticEvidenceIncomplete, OutcomeReference: record.ID}, nil
		}
	default:
		return MemoryIntentOutcome{Status: MemoryIntentStatusRejected, DiagnosticCategory: MemoryIntentDiagnosticRejected, OutcomeReference: record.ID}, nil
	}
	if err != nil {
		return MemoryIntentOutcome{Status: MemoryIntentStatusFailed, DiagnosticCategory: MemoryIntentDiagnosticFailed, OutcomeReference: record.ID}, err
	}
	if err := out.Validate(); err != nil {
		return MemoryIntentOutcome{}, err
	}
	return out, nil
}
