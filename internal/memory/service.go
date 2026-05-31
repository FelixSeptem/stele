package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type IngestStore interface {
	IngestEvent(ctx context.Context, input IngestEventInput, provenance ProvenanceRecord) (RawEvent, error)
}

type Service struct {
	store IngestStore
	now   func() time.Time
}

func NewService(store IngestStore, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		store: store,
		now:   now,
	}
}

func (s *Service) Ingest(ctx context.Context, input IngestEventInput) (RawEvent, error) {
	if err := input.Validate(); err != nil {
		return RawEvent{}, err
	}

	if s.store == nil {
		return RawEvent{}, fmt.Errorf("ingest store is not configured")
	}

	provenance := ProvenanceRecord{
		ID:         uuid.NewString(),
		Scope:      input.Scope,
		Operation:  "ingest_event",
		CreatedAt:  s.now().UTC(),
		RequestID:  "",
		Actor:      "",
		MemoryID:   "",
		RawEventID: "",
	}

	event, err := s.store.IngestEvent(ctx, input, provenance)
	if err != nil {
		return RawEvent{}, err
	}

	return event, nil
}
