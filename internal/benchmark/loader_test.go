package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type recordingIngestor struct {
	inputs []memory.IngestEventInput
}

type recordingScopeCleaner struct {
	got memory.Scope
}

func (c *recordingScopeCleaner) CleanBenchmarkScope(_ context.Context, scope memory.Scope) error {
	c.got = scope
	return nil
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

func TestLongMemEvalImporterUsesDedicatedBenchmarkScope(t *testing.T) {
	ingestor := &recordingIngestor{}
	importer := NewLongMemEvalImporter(ingestor)
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{
		ID: "event-1", Scope: memoryScope(), SessionID: "session-1", SourceTurnID: "turn-1",
		Text: "fact", Class: memory.MemoryClassEpisodic, ExpectedState: memory.MemoryStateActive,
	}}}
	result, err := importer.Import(context.Background(), memory.Scope{Tenant: "tenant-a", Project: "production", Namespace: "default"}, "run-1", corpus)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Dataset != "longmemeval" || result.Run.Scope != (memory.Scope{Tenant: "tenant-a", Project: "benchmark-longmemeval", Namespace: result.Run.Scope.Namespace}) {
		t.Fatalf("LongMemEval escaped controlled benchmark scope: %#v", result.Run)
	}
	if result.Evidence["event-1"].SessionID != "session-1" || ingestor.inputs[0].Metadata["benchmark_dataset"] != "longmemeval" {
		t.Fatalf("LongMemEval provenance was not retained: %#v", result)
	}
}

func TestParseObservedAtAcceptsLongMemEvalDateFormats(t *testing.T) {
	for _, value := range []string{"2024-01-01", "2023/05/30 (Tue) 21:28"} {
		parsed, err := parseObservedAt(value)
		if err != nil {
			t.Fatalf("parseObservedAt(%q): %v", value, err)
		}
		if parsed.IsZero() {
			t.Fatalf("parseObservedAt(%q) returned zero time", value)
		}
	}
}

func TestLongMemEvalRunCleanerOnlyPassesBenchmarkScopeToDatabaseCleanup(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Name = "longmemeval"
	paths, err := cache.EnsureManifestLayout(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.Embeddings, "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := memory.Scope{Tenant: "tenant-a", Project: "production", Namespace: "default"}
	run, err := NewRunScope(base, "longmemeval", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	scopeCleaner := &recordingScopeCleaner{}
	result, err := NewLongMemEvalRunCleaner(cache, scopeCleaner).Clean(context.Background(), manifest, run, true)
	if err != nil {
		t.Fatal(err)
	}
	if scopeCleaner.got != run.Scope || scopeCleaner.got == base || !result.DatabaseScopeCleaned || !result.Artifacts.EmbeddingsRemoved {
		t.Fatalf("cleanup escaped benchmark scope or skipped artifact cleanup: scope=%#v result=%#v", scopeCleaner.got, result)
	}
}
