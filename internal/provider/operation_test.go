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
	idem := &adapterIdempotentIngestor{adapterIngestor: ing}
	intent := &adapterIntentService{}
	a := NewAdapter(AdapterDependencies{Ingestor: ing, IdempotentIngestor: idem, Intent: intent})
	b := RuntimeBinding{BindingID: "b", PrincipalID: "p", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "a", SessionID: "s", ConversationID: "c", ProviderInstanceID: "pi"}
	meta := OperationMetadata{RequestID: "req", OperationID: "op", SchemaVersion: "schema-v1"}
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

func TestProviderAdapterEnforcesConfiguredLimits(t *testing.T) {
	searcher := &adapterSearcher{}
	assembler := &adapterAssembler{}
	a := NewAdapter(AdapterDependencies{
		Searcher: searcher, Assembler: assembler,
		Limits: ProviderLimits{MaxEventBytes: 128, MaxIntentBytes: 128, MaxRetrievalResults: 2, MaxContextBytes: 32, MaxCitations: 1, MaxMetadataBytes: 64},
	})
	b := RuntimeBinding{BindingID: "b", PrincipalID: "p", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "a", SessionID: "s", ProviderInstanceID: "pi"}
	meta := OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "schema-v1"}
	if _, _, err := a.Search(context.Background(), b, meta, retrieval.SearchInput{Query: "q", TopK: 99}); err != nil {
		t.Fatal(err)
	}
	if searcher.got.TopK != 2 {
		t.Fatalf("top_k=%d want 2", searcher.got.TopK)
	}
	if _, _, err := a.AssembleContext(context.Background(), b, meta, retrieval.AssembleContextInput{Query: "q", Budget: 99, CharacterBudget: 99}); err != nil {
		t.Fatal(err)
	}
	if assembler.got.Budget > 32 || assembler.got.CharacterBudget > 32 {
		t.Fatalf("context limits not enforced: %+v", assembler.got)
	}
}

func TestShapeSearchCitationsWithLimit(t *testing.T) {
	s := retrieval.SearchResult{Hits: []retrieval.SearchHit{
		{Memory: memory.CanonicalMemory{ID: "m1", State: memory.MemoryStateActive}, Citations: []retrieval.Citation{{MemoryID: "m1"}}},
		{Memory: memory.CanonicalMemory{ID: "m2", State: memory.MemoryStateActive}, Citations: []retrieval.Citation{{MemoryID: "m2"}}},
	}}
	if got := ShapeSearchCitationsWithLimit(s, 1); len(got) != 1 {
		t.Fatalf("citations=%+v", got)
	}
}

type adapterIngestor struct {
	got   memory.IngestEventInput
	event memory.RawEvent
}

type adapterIdempotentIngestor struct{ *adapterIngestor }

func (s *adapterIdempotentIngestor) IngestIdempotent(ctx context.Context, in memory.IngestEventInput, principalID, key string) (memory.IdempotentEventIngestResult, error) {
	e, err := s.Ingest(ctx, in)
	return memory.IdempotentEventIngestResult{Event: e}, err
}

func (s *adapterIngestor) Ingest(_ context.Context, in memory.IngestEventInput) (memory.RawEvent, error) {
	s.got = in
	return s.event, nil
}

type adapterIntentService struct{ got memory.MemoryIntentInput }

type adapterSearcher struct{ got retrieval.SearchInput }

func (s *adapterSearcher) Search(_ context.Context, in retrieval.SearchInput) (retrieval.SearchResult, error) {
	s.got = in
	return retrieval.SearchResult{}, nil
}

type adapterAssembler struct {
	got retrieval.AssembleContextInput
}

func (s *adapterAssembler) AssembleContext(_ context.Context, in retrieval.AssembleContextInput) (retrieval.AssembledContext, error) {
	s.got = in
	return retrieval.AssembledContext{}, nil
}

func (s *adapterIntentService) Submit(_ context.Context, in memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	s.got = in
	return memory.MemoryIntentRecord{ID: "intent-1", Scope: in.Scope, RequestID: in.RequestID, OperationID: in.OperationID}, nil
}
