package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type recordingIngestor struct {
	inputs []memory.IngestEventInput
}

type idempotentRecordingIngestor struct {
	recordingIngestor
	idempotencyKeys []string
}

func (r *idempotentRecordingIngestor) IngestIdempotent(ctx context.Context, input memory.IngestEventInput, _ string, key string) (memory.IdempotentEventIngestResult, error) {
	r.idempotencyKeys = append(r.idempotencyKeys, key)
	event, err := r.Ingest(ctx, input)
	return memory.IdempotentEventIngestResult{Event: event}, err
}

func (r *recordingIngestor) Ingest(_ context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	r.inputs = append(r.inputs, input)
	return memory.RawEvent{ID: "raw-" + input.Metadata["benchmark_event_id"].(string), Scope: input.Scope}, nil
}

func TestCorpusLoaderUsesStableIdempotencyKeyWhenSupported(t *testing.T) {
	ingestor := &idempotentRecordingIngestor{}
	loader := NewCorpusLoader(ingestor)
	run, err := NewRunScope(memoryScope(), "locomo", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "event-1", Scope: memoryScope(), Text: "fact"}}}
	if _, err := loader.Load(context.Background(), run, corpus); err != nil {
		t.Fatal(err)
	}
	if len(ingestor.idempotencyKeys) != 1 || ingestor.idempotencyKeys[0] != "benchmark/locomo/run-1/event-1" {
		t.Fatalf("unexpected idempotency keys: %#v", ingestor.idempotencyKeys)
	}
}

func TestNewRunScopeDerivesDistinctBenchmarkNamespace(t *testing.T) {
	base := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "production"}
	first, err := NewRunScope(base, "locomo", "run-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRunScope(base, "locomo", "run-b")
	if err != nil {
		t.Fatal(err)
	}
	if first.Scope == base || first.Scope == second.Scope || first.Scope.Tenant != base.Tenant {
		t.Fatalf("expected distinct benchmark scopes, got %#v and %#v", first, second)
	}
}

func TestCorpusLoaderUsesRunScopeAndTracksEvidenceProvenance(t *testing.T) {
	ingestor := &recordingIngestor{}
	loader := NewCorpusLoader(ingestor)
	run, err := NewRunScope(memoryScope(), "locomo", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "event-1", Scope: memoryScope(), SessionID: "session-1", SourceTurnID: "turn-1", Text: "fact", Class: memory.MemoryClassEpisodic, ExpectedState: memory.MemoryStateActive, ObservedAt: time.Now().UTC().Format(time.RFC3339)}}}
	mapping, err := loader.Load(context.Background(), run, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(ingestor.inputs) != 1 || ingestor.inputs[0].Scope != run.Scope || mapping["event-1"].RawEventID != "raw-event-1" || mapping["event-1"].SourceTurnID != "turn-1" {
		t.Fatalf("unexpected import mapping: inputs=%#v mapping=%#v", ingestor.inputs, mapping)
	}
}
