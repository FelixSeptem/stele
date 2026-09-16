package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

func TestOperationServiceIngestPropagatesMetadataAndDurableReplay(t *testing.T) {
	writer := &stubEventWriter{result: memory.IdempotentEventIngestResult{Event: memory.RawEvent{ID: "event-1"}, Replayed: true}}
	service := NewOperationService(OperationServiceOptions{Events: writer, Sequences: NewSequenceTracker()})
	binding := testRuntimeBinding()
	metadata := testOperationMetadata()

	result, err := service.IngestEvent(context.Background(), binding, metadata, memory.IngestEventInput{EventType: "conversation.message", Content: "hello"})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}
	if !result.Replayed || result.Disposition != SequenceAccepted || result.Metadata != metadata {
		t.Fatalf("result = %+v", result)
	}
	if writer.principalID != binding.PrincipalID || writer.idempotencyKey != metadata.IdempotencyKey {
		t.Fatalf("writer correlation = principal %q key %q", writer.principalID, writer.idempotencyKey)
	}
	assertOperationMetadataMap(t, writer.input.Metadata, metadata)
	if writer.input.Scope != binding.Scope {
		t.Fatalf("scope = %+v, want binding scope %+v", writer.input.Scope, binding.Scope)
	}
}

func TestOperationServiceIngestDoesNotMaskDurableConflict(t *testing.T) {
	writer := &stubEventWriter{err: memory.ErrIdempotencyConflict}
	service := NewOperationService(OperationServiceOptions{Events: writer, Sequences: NewSequenceTracker()})
	_, err := service.IngestEvent(context.Background(), testRuntimeBinding(), testOperationMetadata(), memory.IngestEventInput{EventType: "conversation.message", Content: "changed"})
	if !errors.Is(err, memory.ErrIdempotencyConflict) {
		t.Fatalf("IngestEvent() error = %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
}

func TestOperationServiceRejectsStaleSequenceBeforeWrite(t *testing.T) {
	writer := &stubEventWriter{}
	sequences := NewSequenceTracker()
	service := NewOperationService(OperationServiceOptions{Events: writer, Sequences: sequences})
	binding := testRuntimeBinding()
	metadata := testOperationMetadata()
	metadata.EventSeq = 3
	if _, err := service.IngestEvent(context.Background(), binding, metadata, memory.IngestEventInput{EventType: "message", Content: "one"}); err != nil {
		t.Fatal(err)
	}
	metadata.EventSeq = 2
	metadata.OperationID = "op-2"
	result, err := service.IngestEvent(context.Background(), binding, metadata, memory.IngestEventInput{EventType: "message", Content: "old"})
	if err != nil {
		t.Fatalf("stale IngestEvent() error = %v", err)
	}
	if result.Disposition != SequenceStale || writer.calls != 1 {
		t.Fatalf("result = %+v calls = %d", result, writer.calls)
	}
}

func TestOperationServicePropagatesOutcomeAndIntentMetadata(t *testing.T) {
	outcomes := &stubOutcomeWriter{}
	intents := &stubIntentWriter{}
	service := NewOperationService(OperationServiceOptions{Outcomes: outcomes, Intents: intents, Sequences: NewSequenceTracker()})
	binding := testRuntimeBinding()
	metadata := testOperationMetadata()

	if _, err := service.RecordSessionOutcome(context.Background(), binding, metadata, memory.RecordMemorySessionTurnOutcomeInput{TurnID: "turn-1"}); err != nil {
		t.Fatalf("RecordSessionOutcome() error = %v", err)
	}
	if outcomes.input.Scope != binding.Scope || outcomes.input.SessionID != binding.SessionID || outcomes.input.IdempotencyKey != metadata.IdempotencyKey || outcomes.input.RequestID != metadata.RequestID || outcomes.input.OperationID != metadata.OperationID {
		t.Fatalf("outcome input = %+v", outcomes.input)
	}

	metadata.EventSeq++
	_, err := service.SubmitIntent(context.Background(), binding, metadata, memory.MemoryIntentInput{Type: memory.MemoryIntentRemember, Content: "remember", Actor: "agent", Reason: "explicit"})
	if err != nil {
		t.Fatalf("SubmitIntent() error = %v", err)
	}
	if intents.input.RequestID != metadata.RequestID || intents.input.OperationID != metadata.OperationID || intents.input.IdempotencyKey != metadata.IdempotencyKey {
		t.Fatalf("intent input = %+v", intents.input)
	}
	assertOperationMetadataMap(t, intents.input.Provenance, metadata)
}

func TestOperationServiceReadCorrelationIsSideEffectFreeAndScopeSafe(t *testing.T) {
	reader := &stubReader{result: "ok"}
	lifecycle := &stubLifecycleWriter{}
	service := NewOperationService(OperationServiceOptions{Reader: reader, Lifecycle: lifecycle, Sequences: NewSequenceTracker()})
	binding := testRuntimeBinding()
	metadata := testOperationMetadata()

	result, err := service.Read(context.Background(), binding, metadata, ReadKindRetrieval, ReadRequest{Query: "preference", Limit: 5})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if result.Data != "ok" || reader.scope != binding.Scope || reader.kind != ReadKindRetrieval {
		t.Fatalf("result = %+v reader = %+v", result, reader)
	}
	if correlated, ok := OperationMetadataFromContext(reader.ctx); !ok || correlated != metadata {
		t.Fatalf("correlated metadata = %+v, %v", correlated, ok)
	}
	if lifecycle.calls != 0 {
		t.Fatalf("read caused lifecycle writes = %d", lifecycle.calls)
	}

	metadata.EventSeq++
	_, err = service.RequestLifecycle(context.Background(), binding, metadata, memory.LifecycleActionInput{MemoryID: "memory-1", Action: policy.ForgettingActionSuppress, Reason: "requested", Actor: "agent"})
	if err != nil {
		t.Fatalf("RequestLifecycle() error = %v", err)
	}
	if lifecycle.input.Scope != binding.Scope || lifecycle.input.RequestID != metadata.RequestID {
		t.Fatalf("lifecycle input = %+v", lifecycle.input)
	}
}

func TestOperationServiceDoesNotReplayDuplicateLifecycleWrite(t *testing.T) {
	lifecycle := &stubLifecycleWriter{}
	service := NewOperationService(OperationServiceOptions{Lifecycle: lifecycle, Sequences: NewSequenceTracker()})
	input := memory.LifecycleActionInput{MemoryID: "memory-1", Action: policy.ForgettingActionSuppress, Reason: "requested", Actor: "agent"}

	first, err := service.RequestLifecycle(context.Background(), testRuntimeBinding(), testOperationMetadata(), input)
	if err != nil {
		t.Fatalf("RequestLifecycle() first error = %v", err)
	}
	second, err := service.RequestLifecycle(context.Background(), testRuntimeBinding(), testOperationMetadata(), input)
	if err != nil {
		t.Fatalf("RequestLifecycle() retry error = %v", err)
	}
	if first.Disposition != SequenceAccepted || second.Disposition != SequenceDuplicate || lifecycle.calls != 1 {
		t.Fatalf("first = %+v second = %+v calls = %d", first, second, lifecycle.calls)
	}
}

func testRuntimeBinding() RuntimeBinding {
	return RuntimeBinding{PrincipalID: "principal-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, SessionID: "session-a"}
}

func testOperationMetadata() OperationMetadata {
	return OperationMetadata{RequestID: "req-1", OperationID: "op-1", IdempotencyKey: "idem-1", EventSeq: 1, SchemaVersion: SchemaVersionV1}
}

func assertOperationMetadataMap(t *testing.T, values map[string]any, want OperationMetadata) {
	t.Helper()
	for key, value := range want.Map() {
		if values[key] != value {
			t.Fatalf("metadata[%q] = %#v, want %#v", key, values[key], value)
		}
	}
}

type stubEventWriter struct {
	input                       memory.IngestEventInput
	principalID, idempotencyKey string
	result                      memory.IdempotentEventIngestResult
	err                         error
	calls                       int
}

func (s *stubEventWriter) IngestIdempotent(_ context.Context, input memory.IngestEventInput, principalID, idempotencyKey string) (memory.IdempotentEventIngestResult, error) {
	s.calls++
	s.input, s.principalID, s.idempotencyKey = input, principalID, idempotencyKey
	return s.result, s.err
}

type stubOutcomeWriter struct {
	input memory.RecordMemorySessionTurnOutcomeInput
}

func (s *stubOutcomeWriter) RecordTurnOutcome(_ context.Context, input memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error) {
	s.input = input
	return memory.MemorySessionTurn{}, nil
}

type stubIntentWriter struct{ input memory.MemoryIntentInput }

func (s *stubIntentWriter) Submit(_ context.Context, input memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	s.input = input
	return memory.MemoryIntentRecord{}, nil
}

type stubReader struct {
	ctx     context.Context
	scope   memory.Scope
	kind    ReadKind
	request ReadRequest
	result  any
}

func (s *stubReader) Read(ctx context.Context, scope memory.Scope, kind ReadKind, request ReadRequest) (any, error) {
	s.ctx, s.scope, s.kind, s.request = ctx, scope, kind, request
	return s.result, nil
}

type stubLifecycleWriter struct {
	input memory.LifecycleActionInput
	calls int
}

func (s *stubLifecycleWriter) Apply(_ context.Context, input memory.LifecycleActionInput) error {
	s.calls++
	s.input = input
	return nil
}
