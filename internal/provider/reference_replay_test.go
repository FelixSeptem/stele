package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type referenceReplayStore struct {
	events   map[string]memory.RawEvent
	admitted int
}

func (s *referenceReplayStore) IngestIdempotent(_ context.Context, input memory.IngestEventInput, _, key string) (memory.IdempotentEventIngestResult, error) {
	if event, ok := s.events[key]; ok {
		return memory.IdempotentEventIngestResult{Event: event, Replayed: true}, nil
	}
	if s.events == nil {
		s.events = make(map[string]memory.RawEvent)
	}
	s.admitted++
	event := memory.RawEvent{ID: fmt.Sprintf("0192a1a2-01f1-4000-8000-%012d", s.admitted), Content: input.Content}
	s.events[key] = event
	return memory.IdempotentEventIngestResult{Event: event}, nil
}

func TestReferenceReplaySurvivesLaterSequenceAndPreservesMetadata(t *testing.T) {
	store := &referenceReplayStore{}
	adapter := NewAdapter(AdapterDependencies{IdempotentIngestor: store})
	binding := RuntimeBinding{BindingID: "binding", PrincipalID: "principal", SessionID: "session", Scope: memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}}
	request := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "request", OperationID: "operation", IdempotencyKey: "key-a", EventSeq: 1, SchemaVersion: "provider-v1"}, Input: ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "reference", MemoryPath: "tasks/run", EventType: "provider.reference.run", Value: "recorded evidence"}}
	first, err := adapter.WriteReference(context.Background(), binding, request)
	if err != nil {
		t.Fatal(err)
	}
	later := request
	later.Metadata.IdempotencyKey, later.Metadata.EventSeq = "key-b", 2
	if _, err := adapter.WriteReference(context.Background(), binding, later); err != nil {
		t.Fatal(err)
	}
	replay, err := adapter.WriteReference(context.Background(), binding, request)
	if err != nil {
		t.Fatalf("equivalent durable retry was blocked by a later sequence: %v", err)
	}
	if !replay.Replayed || replay.EventID != first.EventID || replay.Metadata.EventSeq != 1 || store.admitted != 2 {
		t.Fatalf("replay lost original identity or metadata: %+v; admissions=%d", replay, store.admitted)
	}
	// A fresh adapter must reach the same durable replay identity after restart.
	restarted := NewAdapter(AdapterDependencies{IdempotentIngestor: store})
	afterRestart, err := restarted.WriteReference(context.Background(), binding, request)
	if err != nil || !afterRestart.Replayed || afterRestart.EventID != first.EventID || store.admitted != 2 {
		t.Fatalf("restart replay changed identity: %+v %v; admissions=%d", afterRestart, err, store.admitted)
	}
}

func TestReferenceSameSequenceWithDifferentKeysAdmitsIndependentWrites(t *testing.T) {
	store := &referenceReplayStore{}
	adapter := NewAdapter(AdapterDependencies{IdempotentIngestor: store})
	binding := RuntimeBinding{BindingID: "binding", PrincipalID: "principal", SessionID: "session", Scope: memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}}
	request := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "request", OperationID: "operation", IdempotencyKey: "key-a", EventSeq: 3, SchemaVersion: "provider-v1"}, Input: ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "reference", MemoryPath: "tasks/run", EventType: "provider.reference.run", Value: "recorded evidence"}}
	first, err := adapter.WriteReference(context.Background(), binding, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Metadata.IdempotencyKey = "key-b"
	second, err := adapter.WriteReference(context.Background(), binding, request)
	if err != nil || second.Replayed || second.EventID == first.EventID || second.Metadata.EventSeq != 3 || store.admitted != 2 {
		t.Fatalf("sequence metadata blocked an independent durable key: %+v %v; admissions=%d", second, err, store.admitted)
	}
}

// Exercise the real adapter projection and the shared durable fingerprint,
// including attribution fields that an HTTP retry must not silently replace.
func TestReferenceReplayFingerprintBindsSemanticInputAndAttribution(t *testing.T) {
	binding := RuntimeBinding{BindingID: "binding", PrincipalID: "principal", SessionID: "session", Scope: memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}}
	request := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "request", OperationID: "operation", IdempotencyKey: "key", SchemaVersion: "provider-v1"}, Input: ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "reference", MemoryPath: "tasks/run", EventType: "provider.reference.run", Value: "recorded evidence"}}
	fingerprint := func(b RuntimeBinding, r ReferenceWriteRequest) string {
		t.Helper()
		fixture := &referenceIngestFixture{}
		adapter := NewAdapter(AdapterDependencies{IdempotentIngestor: fixture})
		if _, err := adapter.WriteReference(context.Background(), b, r); err != nil {
			t.Fatal(err)
		}
		got, err := memory.EventRequestFingerprint(fixture.input)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	baseline := fingerprint(binding, request)
	equivalent := request
	equivalent.Input.MemoryPath = "/tasks/run/"
	equivalent.Metadata.RequestID = "retry-request"
	if fingerprint(binding, equivalent) != baseline {
		t.Fatal("equivalent path and fresh request identity changed durable semantics")
	}
	for _, tc := range []struct {
		name   string
		change func(*RuntimeBinding, *ReferenceWriteRequest)
	}{
		{"binding", func(b *RuntimeBinding, _ *ReferenceWriteRequest) { b.BindingID = "other-binding" }},
		{"principal", func(b *RuntimeBinding, _ *ReferenceWriteRequest) { b.PrincipalID = "other-principal" }},
		{"operation attribution", func(_ *RuntimeBinding, r *ReferenceWriteRequest) { r.Metadata.OperationID = "other-operation" }},
		{"reference identity", func(_ *RuntimeBinding, r *ReferenceWriteRequest) { r.Input.ReferenceID = "other-reference" }},
		{"event kind", func(_ *RuntimeBinding, r *ReferenceWriteRequest) { r.Input.EventType = "provider.reference.task" }},
		{"exact path", func(_ *RuntimeBinding, r *ReferenceWriteRequest) { r.Input.MemoryPath += "/child" }},
		{"value", func(_ *RuntimeBinding, r *ReferenceWriteRequest) { r.Input.Value += " changed" }},
		{"source labels", func(_ *RuntimeBinding, r *ReferenceWriteRequest) {
			r.Input.SourceMetadata = &memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, r := binding, request
			tc.change(&b, &r)
			if fingerprint(b, r) == baseline {
				t.Fatal("material replay change retained original durable fingerprint")
			}
		})
	}
}
