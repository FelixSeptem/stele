package provider

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestOperationMetadataNormalizeValidateAndSequence(t *testing.T) {
	m := OperationMetadata{RequestID: " req-1 ", OperationID: " op-1 ", IdempotencyKey: " idem-1 ", EventSeq: 1, SchemaVersion: " schema-v1 "}
	if err := m.NormalizeValidate(); err != nil {
		t.Fatal(err)
	}
	if m.RequestID != "req-1" || m.SchemaVersion != "schema-v1" {
		t.Fatalf("normalized=%+v", m)
	}
	bad := OperationMetadata{RequestID: "bad space", OperationID: "op", IdempotencyKey: "i", SchemaVersion: "s"}
	if err := bad.NormalizeValidate(); err == nil {
		t.Fatal("invalid token accepted")
	}
	tracker := NewSequenceTracker()
	if got := tracker.Accept("session-1", 2); got != SequenceAccepted {
		t.Fatalf("first=%s", got)
	}
	if got := tracker.Accept("session-1", 2); got != SequenceDuplicate {
		t.Fatalf("duplicate=%s", got)
	}
	if got := tracker.Accept("session-1", 1); got != SequenceStale {
		t.Fatalf("stale=%s", got)
	}
}

func TestProviderAdapterIngestUsesIdempotencyAndIntentGovernance(t *testing.T) {
	var got memory.IngestEventInput
	ing := &adapterIngestor{event: memory.RawEvent{ID: "evt-1", Admission: &memory.AdmissionPressureReport{}}}
	intent := &adapterIntentService{}
	a := NewAdapter(AdapterDependencies{Ingestor: ing, Intent: intent})
	b := RuntimeBinding{BindingID: "b", PrincipalID: "p", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "a", SessionID: "s", ConversationID: "c", ProviderInstanceID: "pi"}
	meta := OperationMetadata{RequestID: "req", OperationID: "op", IdempotencyKey: "idem", SchemaVersion: "schema-v1"}
	r, err := a.Ingest(context.Background(), b, meta, memory.IngestEventInput{EventType: "message", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	got = ing.got
	if got.Scope != b.Scope || r.EventID != "evt-1" {
		t.Fatalf("input/result=%+v/%+v", got, r)
	}
	if _, err := a.SubmitIntent(context.Background(), b, meta, memory.MemoryIntentInput{Type: memory.MemoryIntentRemember, Actor: "agent", Reason: "test", Content: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestShapeCitationsOmitsScoresAndHiddenItems(t *testing.T) {
	s := retrieval.SearchResult{Hits: []retrieval.SearchHit{{Memory: memory.CanonicalMemory{ID: "m1", State: memory.MemoryStateActive}, Citations: []retrieval.Citation{{MemoryID: "m1", RawEventID: "e1", Operation: "ingest"}}}, {Memory: memory.CanonicalMemory{ID: "m2", State: memory.MemoryStateSuppressed}}}}
	cs := ShapeSearchCitations(s)
	if len(cs) != 1 || cs[0].Reference != "m1" || cs[0].SourceKind != "memory" {
		t.Fatalf("citations=%+v", cs)
	}
}

type adapterIngestor struct {
	got   memory.IngestEventInput
	event memory.RawEvent
}

func (s *adapterIngestor) Ingest(_ context.Context, in memory.IngestEventInput) (memory.RawEvent, error) {
	s.got = in
	return s.event, nil
}

type adapterIntentService struct{ got memory.MemoryIntentInput }

func (s *adapterIntentService) Submit(_ context.Context, in memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	s.got = in
	return memory.MemoryIntentRecord{ID: "intent-1", Scope: in.Scope, RequestID: in.RequestID, OperationID: in.OperationID}, nil
}
