package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/google/uuid"
)

type IngestStore interface {
	IngestEvent(ctx context.Context, input IngestEventInput, provenance ProvenanceRecord) (RawEvent, error)
}

type IdempotentIngestStore interface {
	IngestEventIdempotent(ctx context.Context, input IdempotentEventIngestInput, provenance ProvenanceRecord, admission AdmissionPressureReport) (IdempotentEventIngestResult, error)
}

type Service struct {
	store    IngestStore
	now      func() time.Time
	observer telemetry.Observer
}

func NewService(store IngestStore, now func() time.Time, observers ...telemetry.Observer) *Service {
	if now == nil {
		now = time.Now
	}

	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}

	return &Service{
		store:    store,
		now:      now,
		observer: observer,
	}
}

func (s *Service) Ingest(ctx context.Context, input IngestEventInput) (event RawEvent, err error) {
	started := time.Now()
	defer func() {
		if s.observer == nil {
			return
		}

		status := "ok"
		count := 1
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		s.observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "api",
			Component:  "memory_service",
			Operation:  "ingest",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: s.now().UTC(),
		})
	}()

	if err := input.Validate(); err != nil {
		return RawEvent{}, err
	}

	if s.store == nil {
		return RawEvent{}, fmt.Errorf("ingest store is not configured")
	}

	observedAt := s.now().UTC()
	admission := DefaultAdmissionPressureReport(input.Scope, AdmissionPressureOperationIngest, observedAt)
	if reader, ok := s.store.(AdmissionPressureSnapshotReader); ok {
		snapshot, err := reader.ReadAdmissionPressureSnapshot(ctx, input.Scope, AdmissionPressureOperationIngest, observedAt)
		if err != nil {
			return RawEvent{}, err
		}
		admission = AdmissionPressureEvaluator{}.Evaluate(AdmissionPressureInput{
			Scope:      input.Scope,
			Operation:  AdmissionPressureOperationIngest,
			Snapshot:   snapshot,
			ObservedAt: observedAt,
		})
		if admission.Decision == AdmissionPressureDecisionReject {
			return RawEvent{}, fmt.Errorf("%w: %s", ErrAdmissionRejected, admissionRejectionCategory(admission))
		}
	}

	provenance := ProvenanceRecord{
		ID:         uuid.NewString(),
		Scope:      input.Scope,
		Operation:  "ingest_event",
		CreatedAt:  observedAt,
		RequestID:  "",
		Actor:      "",
		MemoryID:   "",
		RawEventID: "",
	}

	event, err = s.store.IngestEvent(ctx, input, provenance)
	if err != nil {
		return RawEvent{}, err
	}

	event.Admission = &admission
	return event, nil
}

func (s *Service) IngestIdempotent(ctx context.Context, input IngestEventInput, principalID, idempotencyKey string) (IdempotentEventIngestResult, error) {
	started := time.Now()
	var ingestErr error
	defer func() {
		if s == nil || s.observer == nil {
			return
		}
		status := "completed"
		count := 1
		if ingestErr != nil {
			count = 0
			switch {
			case errors.Is(ingestErr, ErrIdempotencyConflict):
				status = "conflict"
			case errors.Is(ingestErr, ErrIdempotencyInProgress):
				status = "in_progress"
			case errors.Is(ingestErr, ErrAdmissionRejected):
				status = "admission_rejected"
			default:
				status = "error"
			}
		}
		s.observer.RecordOperation(ctx, telemetry.OperationEvent{Mode: "api", Component: "memory_service", Operation: "ingest_idempotent", Status: status, Count: count, Duration: time.Since(started), ObservedAt: s.now().UTC()})
	}()
	setErr := func(err error) error { ingestErr = err; return err }
	if err := input.Validate(); err != nil {
		return IdempotentEventIngestResult{}, setErr(err)
	}
	if s == nil || s.store == nil {
		return IdempotentEventIngestResult{}, setErr(fmt.Errorf("ingest store is not configured"))
	}
	store, ok := s.store.(IdempotentIngestStore)
	if !ok {
		return IdempotentEventIngestResult{}, setErr(fmt.Errorf("idempotent ingest store is not configured"))
	}
	fingerprint, err := EventRequestFingerprint(input)
	if err != nil {
		return IdempotentEventIngestResult{}, setErr(err)
	}
	claim := IdempotentEventIngestInput{PrincipalID: principalID, IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint}
	if err := claim.Validate(); err != nil {
		return IdempotentEventIngestResult{}, setErr(err)
	}
	observedAt := s.now().UTC()
	admission := DefaultAdmissionPressureReport(input.Scope, AdmissionPressureOperationIngest, observedAt)
	if reader, ok := s.store.(AdmissionPressureSnapshotReader); ok {
		snapshot, err := reader.ReadAdmissionPressureSnapshot(ctx, input.Scope, AdmissionPressureOperationIngest, observedAt)
		if err != nil {
			return IdempotentEventIngestResult{}, setErr(err)
		}
		admission = AdmissionPressureEvaluator{}.Evaluate(AdmissionPressureInput{Scope: input.Scope, Operation: AdmissionPressureOperationIngest, Snapshot: snapshot, ObservedAt: observedAt})
		if admission.Decision == AdmissionPressureDecisionReject {
			return IdempotentEventIngestResult{}, setErr(fmt.Errorf("%w: %s", ErrAdmissionRejected, admissionRejectionCategory(admission)))
		}
	}
	result, err := store.IngestEventIdempotent(ctx, claim, ProvenanceRecord{ID: uuid.NewString(), Scope: input.Scope, Operation: "ingest_event", CreatedAt: observedAt}, admission)
	if err != nil {
		return IdempotentEventIngestResult{}, setErr(err)
	}
	if !result.Replayed {
		result.Event.Admission = &admission
	}
	return result, nil
}

func admissionRejectionCategory(admission AdmissionPressureReport) string {
	if len(admission.Findings) == 0 {
		return "admission_rejected"
	}
	return string(admission.Findings[0].Code)
}
