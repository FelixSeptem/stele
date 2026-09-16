package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

var (
	ErrCompatibility     = errors.New("provider schema compatibility failure")
	ErrOperationConflict = errors.New("provider operation conflict")
	ErrLifecycleDenied   = errors.New("provider lifecycle denied")
	ErrStaleProjection   = errors.New("provider projection is stale")
)

type EventWriter interface {
	IngestIdempotent(context.Context, memory.IngestEventInput, string, string) (memory.IdempotentEventIngestResult, error)
}

type OutcomeWriter interface {
	RecordTurnOutcome(context.Context, memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error)
}

type IntentWriter interface {
	Submit(context.Context, memory.MemoryIntentInput) (memory.MemoryIntentRecord, error)
}

type LifecycleWriter interface {
	Apply(context.Context, memory.LifecycleActionInput) error
}

type ReadKind string

const (
	ReadKindRetrieval ReadKind = "retrieval"
	ReadKindContext   ReadKind = "context"
	ReadKindStatus    ReadKind = "status"
	ReadKindReport    ReadKind = "report"
)

func (k ReadKind) valid() bool {
	switch k {
	case ReadKindRetrieval, ReadKindContext, ReadKindStatus, ReadKindReport:
		return true
	default:
		return false
	}
}

type ReadRequest struct {
	Query string
	Limit int
}

type Reader interface {
	Read(context.Context, memory.Scope, ReadKind, ReadRequest) (any, error)
}

type OperationServiceOptions struct {
	Events    EventWriter
	Outcomes  OutcomeWriter
	Intents   IntentWriter
	Lifecycle LifecycleWriter
	Reader    Reader
	Sequences *SequenceTracker
}

type OperationService struct {
	events    EventWriter
	outcomes  OutcomeWriter
	intents   IntentWriter
	lifecycle LifecycleWriter
	reader    Reader
	sequences *SequenceTracker
}

type OperationResult struct {
	Metadata    OperationMetadata   `json:"metadata"`
	Disposition SequenceDisposition `json:"disposition"`
	Replayed    bool                `json:"replayed,omitempty"`
	Data        any                 `json:"data,omitempty"`
}

func NewOperationService(options OperationServiceOptions) *OperationService {
	sequences := options.Sequences
	if sequences == nil {
		sequences = NewSequenceTracker()
	}
	return &OperationService{
		events: options.Events, outcomes: options.Outcomes, intents: options.Intents,
		lifecycle: options.Lifecycle, reader: options.Reader, sequences: sequences,
	}
}

// Execute is the single bounded provider dispatch surface. It accepts only the
// documented operation names and delegates writes and reads to existing Stele
// services; direct canonical-memory mutation is deliberately not represented.
func (s *OperationService) Execute(ctx context.Context, binding RuntimeBinding, operation string, metadata OperationMetadata, raw json.RawMessage) (OperationResult, error) {
	if strings.TrimSpace(metadata.SchemaVersion) != SchemaVersionV1 {
		return OperationResult{}, ErrCompatibility
	}
	switch strings.TrimSpace(operation) {
	case "event.ingest":
		var input struct {
			EventType       string         `json:"event_type"`
			Content         string         `json:"content"`
			Metadata        map[string]any `json:"metadata,omitempty"`
			SourceTimestamp string         `json:"source_timestamp,omitempty"`
		}
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		event := memory.IngestEventInput{EventType: input.EventType, Content: input.Content, Metadata: input.Metadata}
		if input.SourceTimestamp != "" {
			parsed, err := time.Parse(time.RFC3339, input.SourceTimestamp)
			if err != nil {
				return OperationResult{}, fmt.Errorf("invalid source timestamp")
			}
			event.SourceTimestamp = parsed
		}
		return s.IngestEvent(ctx, binding, metadata, event)
	case "memory.intent":
		var input struct {
			Type                   memory.MemoryIntentType `json:"type"`
			TargetMemoryID         string                  `json:"target_memory_id,omitempty"`
			TargetVersion          int64                   `json:"target_version,omitempty"`
			Content, Actor, Reason string
		}
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		return s.SubmitIntent(ctx, binding, metadata, memory.MemoryIntentInput{Type: input.Type, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion, Content: input.Content, Actor: input.Actor, Reason: input.Reason})
	case "memory.forget":
		var input struct {
			MemoryID, Reason, Actor string
			Action                  policy.ForgettingAction
		}
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		return s.RequestLifecycle(ctx, binding, metadata, memory.LifecycleActionInput{MemoryID: input.MemoryID, Action: input.Action, Reason: input.Reason, Actor: input.Actor})
	case "memory.retrieve":
		var input ReadRequest
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		return s.Read(ctx, binding, metadata, ReadKindRetrieval, input)
	case "context.assemble":
		var input ReadRequest
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		return s.Read(ctx, binding, metadata, ReadKindContext, input)
	case "status.read":
		var input ReadRequest
		if err := DecodeJSON(raw, &input); err != nil {
			return OperationResult{}, err
		}
		return s.Read(ctx, binding, metadata, ReadKindStatus, input)
	default:
		return OperationResult{}, fmt.Errorf("unsupported provider operation")
	}
}

func (s *OperationService) prepare(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata) (context.Context, OperationMetadata, SequenceDisposition, error) {
	if err := binding.Scope.Validate(); err != nil {
		return ctx, OperationMetadata{}, "", err
	}
	if strings.TrimSpace(binding.SessionID) == "" {
		return ctx, OperationMetadata{}, "", fmt.Errorf("runtime session is required")
	}
	normalized, err := metadata.Normalize()
	if err != nil {
		return ctx, OperationMetadata{}, "", err
	}
	disposition := s.sequences.CheckAndRecord(binding.SessionID, normalized)
	return withOperationMetadata(ctx, normalized), normalized, disposition, nil
}

func (s *OperationService) IngestEvent(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata, input memory.IngestEventInput) (OperationResult, error) {
	ctx, metadata, disposition, err := s.prepare(ctx, binding, metadata)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Metadata: metadata, Disposition: disposition}
	if disposition == SequenceStale {
		return result, nil
	}
	if s.events == nil {
		return OperationResult{}, fmt.Errorf("event writer is not configured")
	}
	input.Scope = binding.Scope.Normalized()
	input.Metadata = mergeOperationMetadata(input.Metadata, metadata)
	written, err := s.events.IngestIdempotent(ctx, input, binding.PrincipalID, metadata.IdempotencyKey)
	if err != nil {
		return OperationResult{}, err
	}
	result.Replayed = written.Replayed || disposition == SequenceDuplicate
	result.Data = written.Event
	return result, nil
}

func (s *OperationService) RecordSessionOutcome(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata, input memory.RecordMemorySessionTurnOutcomeInput) (OperationResult, error) {
	ctx, metadata, disposition, err := s.prepare(ctx, binding, metadata)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Metadata: metadata, Disposition: disposition}
	if disposition == SequenceStale {
		return result, nil
	}
	if s.outcomes == nil {
		return OperationResult{}, fmt.Errorf("outcome writer is not configured")
	}
	input.Scope = binding.Scope.Normalized()
	input.SessionID = binding.SessionID
	input.RequestID = metadata.RequestID
	input.OperationID = metadata.OperationID
	input.IdempotencyKey = metadata.IdempotencyKey
	turn, err := s.outcomes.RecordTurnOutcome(ctx, input)
	if err != nil {
		return OperationResult{}, err
	}
	result.Replayed = disposition == SequenceDuplicate
	result.Data = turn
	return result, nil
}

func (s *OperationService) SubmitIntent(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata, input memory.MemoryIntentInput) (OperationResult, error) {
	ctx, metadata, disposition, err := s.prepare(ctx, binding, metadata)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Metadata: metadata, Disposition: disposition}
	if disposition == SequenceStale {
		return result, nil
	}
	if s.intents == nil {
		return OperationResult{}, fmt.Errorf("intent writer is not configured")
	}
	input.Scope = binding.Scope.Normalized()
	input.RequestID = metadata.RequestID
	input.OperationID = metadata.OperationID
	input.IdempotencyKey = metadata.IdempotencyKey
	input.Provenance = mergeOperationMetadata(input.Provenance, metadata)
	record, err := s.intents.Submit(ctx, input)
	if err != nil {
		return OperationResult{}, err
	}
	result.Replayed = disposition == SequenceDuplicate
	result.Data = record
	return result, nil
}

func (s *OperationService) RequestLifecycle(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata, input memory.LifecycleActionInput) (OperationResult, error) {
	ctx, metadata, disposition, err := s.prepare(ctx, binding, metadata)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{Metadata: metadata, Disposition: disposition}
	if disposition != SequenceAccepted {
		result.Replayed = disposition == SequenceDuplicate
		return result, nil
	}
	if s.lifecycle == nil {
		return OperationResult{}, fmt.Errorf("lifecycle writer is not configured")
	}
	input.Scope = binding.Scope.Normalized()
	input.RequestID = metadata.RequestID
	if err := s.lifecycle.Apply(ctx, input); err != nil {
		return OperationResult{}, err
	}
	return result, nil
}

func (s *OperationService) Read(ctx context.Context, binding RuntimeBinding, metadata OperationMetadata, kind ReadKind, request ReadRequest) (OperationResult, error) {
	if !kind.valid() {
		return OperationResult{}, fmt.Errorf("read kind %q is invalid", kind)
	}
	ctx, metadata, disposition, err := s.prepare(ctx, binding, metadata)
	if err != nil {
		return OperationResult{}, err
	}
	if request.Limit < 0 {
		return OperationResult{}, fmt.Errorf("read limit must be non-negative")
	}
	if s.reader == nil {
		return OperationResult{}, fmt.Errorf("reader is not configured")
	}
	data, err := s.reader.Read(ctx, binding.Scope.Normalized(), kind, request)
	if err != nil {
		return OperationResult{}, err
	}
	return OperationResult{Metadata: metadata, Disposition: disposition, Data: data}, nil
}

func mergeOperationMetadata(values map[string]any, metadata OperationMetadata) map[string]any {
	result := make(map[string]any, len(values)+5)
	for key, value := range values {
		result[key] = value
	}
	for key, value := range metadata.Map() {
		result[key] = value
	}
	return result
}
