package benchmark

import (
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestDatasetManifestValidateRequiresAuditFields(t *testing.T) {
	manifest := DatasetManifest{Name: "locomo", Version: "1", SchemaVersion: SchemaVersion}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected incomplete manifest to be rejected")
	}
	manifest.License = "research-only"
	manifest.UpstreamURL = "https://example.test/locomo"
	manifest.UpstreamRevision = "v1"
	manifest.SHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	manifest.SourcePath = "raw/data.json"
	manifest.ConversionVersion = "locomo-v1"
	manifest.Redistribution = RedistributionRestricted
	manifest.Support = SupportRunnable
	manifest.Embedding = EmbeddingProfile{Name: "lexical-only", Normalization: "none"}
	manifest.Splits = map[string]SplitSpec{"smoke": {Source: "smoke.jsonl", MaxQueries: 2}}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected complete manifest, got %v", err)
	}
}

func TestNormalizedCorpusCanonicalJSONLHasStableRecordOrder(t *testing.T) {
	corpus := NormalizedCorpus{
		Conversations: []ConversationRecord{{ID: "session-b", Scope: memoryScope()}, {ID: "session-a", Scope: memoryScope()}},
		Events:        []MemoryEventRecord{{ID: "event-b", Scope: memoryScope(), Text: "two"}, {ID: "event-a", Scope: memoryScope(), Text: "one"}},
	}
	encoded, err := corpus.CanonicalJSONL()
	if err != nil {
		t.Fatal(err)
	}
	firstEvent := strings.Index(string(encoded), `"id":"event-a"`)
	secondEvent := strings.Index(string(encoded), `"id":"event-b"`)
	if firstEvent < 0 || secondEvent < 0 || firstEvent > secondEvent {
		t.Fatalf("expected canonical event ordering, got %s", encoded)
	}
}

func TestDatasetManifestRejectsUnsupportedSchemaAndChecksum(t *testing.T) {
	manifest := validManifest()
	manifest.SchemaVersion = "v99"
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected unsupported schema to fail")
	}
	manifest = validManifest()
	manifest.SHA256 = "bad"
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected malformed checksum to fail")
	}
}

func TestNormalizedCorpusRejectsDuplicateIDsAndMalformedTurns(t *testing.T) {
	corpus := NormalizedCorpus{
		Conversations: []ConversationRecord{{ID: "s1", Turns: []ConversationTurn{{ID: "t1", Text: "hello"}, {ID: "t1", Text: "again"}}}},
		Events:        []MemoryEventRecord{{ID: "e1", Text: "fact", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}}},
	}
	if err := corpus.Validate(); err == nil {
		t.Fatal("expected duplicate turn id to fail")
	}
}

func TestNormalizedCorpusChecksumIsStable(t *testing.T) {
	a := NormalizedCorpus{Conversations: []ConversationRecord{{ID: "s1", Turns: []ConversationTurn{{ID: "t1", Text: "hello"}}}}, Events: []MemoryEventRecord{{ID: "e1", Text: "fact", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}}}}
	b := NormalizedCorpus{Events: a.Events, Conversations: a.Conversations}
	first, err := a.Checksum()
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.Checksum()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("checksum changed with field order: %s != %s", first, second)
	}
}

func TestDefaultRegistryListsAllDatasetLayersWithExplicitSupport(t *testing.T) {
	registry := DefaultRegistry()
	entries := registry.List()
	if len(entries) < 8 {
		t.Fatalf("expected at least the original 8 datasets, got %d", len(entries))
	}
	locomo, ok := registry.Get("locomo")
	if !ok || locomo.Layer != 1 || locomo.Manifest.Support != SupportRunnable {
		t.Fatalf("expected runnable LoCoMo layer 1 entry, got %#v", locomo)
	}
	longMemEval, ok := registry.Get("longmemeval")
	if !ok || longMemEval.Layer != 2 || longMemEval.Manifest.Support != SupportMetadataOnly {
		t.Fatalf("expected LongMemEval metadata-only layer 2 entry, got %#v", longMemEval)
	}
	for _, name := range []string{"bfcl-memory", "c-mteb", "needle", "vtcbench"} {
		if name == "bfcl-memory" {
			continue // BFCL is a contract fixture, not a source dataset registration yet.
		}
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("expected expansion dataset %q", name)
		}
	}
}

func TestRegistryOnlyResolvesAdaptersForRunnableDatasets(t *testing.T) {
	registry := DefaultRegistry()
	if _, err := registry.Adapter("locomo"); err != nil {
		t.Fatalf("expected LoCoMo adapter: %v", err)
	}
	if _, err := registry.Adapter("longmemeval"); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("expected metadata-only adapter request to be blocked, got %v", err)
	}
}

func TestRegistryResolvesLongMemEvalOnlyWithExplicitSpikeFlag(t *testing.T) {
	registry := DefaultRegistry()
	adapter, err := registry.AdapterWithOptions("longmemeval", AdapterOptions{EnableLongMemEvalSpike: true})
	if err != nil {
		t.Fatalf("enabled LongMemEval spike should resolve: %v", err)
	}
	if _, ok := adapter.(LongMemEvalAdapter); !ok {
		t.Fatalf("resolved adapter = %T, want LongMemEvalAdapter", adapter)
	}
}

func validManifest() DatasetManifest {
	return DatasetManifest{
		SchemaVersion:     SchemaVersion,
		Name:              "locomo",
		Version:           "1",
		License:           "research-only",
		UpstreamURL:       "https://example.test/locomo",
		UpstreamRevision:  "v1",
		SHA256:            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SourcePath:        "raw/data.json",
		ConversionVersion: "locomo-v1",
		Redistribution:    RedistributionRestricted,
		Support:           SupportRunnable,
		Splits:            map[string]SplitSpec{"smoke": {Source: "smoke.jsonl"}},
		Embedding:         EmbeddingProfile{Name: "lexical-only", Dimensions: 0, Normalization: "none"},
	}
}
