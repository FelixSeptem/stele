package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/google/uuid"
)

type IngestStore interface {
	IngestEvent(ctx context.Context, input IngestEventInput, provenance ProvenanceRecord) (RawEvent, error)
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
			return RawEvent{}, fmt.Errorf("%w: %s", ErrAdmissionRejected, admission.Findings[0].Code)
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
