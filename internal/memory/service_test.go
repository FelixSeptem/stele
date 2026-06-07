package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
)

type stubIngestStore struct {
	gotInput      IngestEventInput
	gotProvenance ProvenanceRecord
	event         RawEvent
	err           error
}

func (s *stubIngestStore) IngestEvent(ctx context.Context, input IngestEventInput, provenance ProvenanceRecord) (RawEvent, error) {
	s.gotInput = input
	s.gotProvenance = provenance
	if s.err != nil {
		return RawEvent{}, s.err
	}

	return s.event, nil
}

type stubObserver struct {
	operations []telemetry.OperationEvent
	backlogs   []telemetry.BacklogEvent
}

func (s *stubObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {
	s.backlogs = append(s.backlogs, event)
}

func TestServiceIngestValidatesAndPersistsRawEventAndProvenance(t *testing.T) {
	store := &stubIngestStore{
		event: RawEvent{
			ID:        "evt_123",
			CreatedAt: time.Date(2026, 5, 29, 23, 30, 0, 0, time.UTC),
		},
	}
	service := NewService(store, func() time.Time {
		return time.Date(2026, 5, 29, 23, 31, 0, 0, time.UTC)
	})

	input := IngestEventInput{
		Scope: Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		SourceTimestamp: time.Date(2026, 5, 29, 23, 29, 0, 0, time.UTC),
		Metadata:        map[string]any{"channel": "chat"},
	}

	event, err := service.Ingest(context.Background(), input)
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if event.ID != "evt_123" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "evt_123")
	}

	if store.gotInput.EventType != input.EventType {
		t.Fatalf("store input = %+v, want event type %q", store.gotInput, input.EventType)
	}

	if store.gotProvenance.Scope != input.Scope {
		t.Fatalf("provenance scope = %+v, want %+v", store.gotProvenance.Scope, input.Scope)
	}

	if store.gotProvenance.Operation != "ingest_event" {
		t.Fatalf("provenance operation = %q, want ingest_event", store.gotProvenance.Operation)
	}
}

func TestServiceIngestRejectsInvalidInput(t *testing.T) {
	service := NewService(&stubIngestStore{}, time.Now)

	_, err := service.Ingest(context.Background(), IngestEventInput{})
	if err == nil {
		t.Fatal("Ingest() error = nil, want validation error")
	}
}

func TestServiceIngestReturnsProvenanceWriteFailure(t *testing.T) {
	store := &stubIngestStore{err: errors.New("write failed")}
	service := NewService(store, time.Now)

	input := IngestEventInput{
		Scope: Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType: "conversation.message",
		Content:   "hello",
	}

	_, err := service.Ingest(context.Background(), input)
	if err == nil {
		t.Fatal("Ingest() error = nil, want ingest write error")
	}
}

func TestServiceIngestEmitsTelemetryOperation(t *testing.T) {
	store := &stubIngestStore{event: RawEvent{ID: "evt_123"}}
	observer := &stubObserver{}
	service := NewService(store, func() time.Time {
		return time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC)
	}, observer)

	_, err := service.Ingest(context.Background(), IngestEventInput{
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		EventType: "conversation.message",
		Content:   "hello",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Operation != "ingest" || observer.operations[0].Status != "ok" {
		t.Fatalf("operation event = %+v, want ingest ok", observer.operations[0])
	}
}
