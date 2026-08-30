package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestLoCoMoAdapterNormalizesTurnsEventsQueriesAndGroupedQrels(t *testing.T) {
	adapter := NewLoCoMoAdapter()
	corpus, err := adapter.Normalize(LoCoMoDataset{Samples: []LoCoMoSample{{
		ID:       "sample-1",
		Sessions: []LoCoMoSession{{ID: "session-1", Turns: []LoCoMoTurn{{ID: "turn-1", Speaker: "Alice", Text: "My favorite fruit is pear.", Timestamp: "2024-01-01T10:00:00Z"}, {ID: "turn-2", Speaker: "Bob", Text: "I work at the museum."}}}},
		Questions: []LoCoMoQuestion{
			{ID: "q-single", Text: "What fruit does Alice prefer?", EvidenceTurnIDs: []string{"turn-1"}, Category: "preference"},
			{ID: "q-multi", Text: "Where does Bob work and what fruit does Alice prefer?", EvidenceTurnIDs: []string{"turn-1", "turn-2"}, Category: "multi-hop"},
		},
	}}}, memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"})
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Conversations) != 1 || len(corpus.Conversations[0].Turns) != 2 || corpus.Conversations[0].Turns[0].Source != "sample-1/session-1/turn-1" {
		t.Fatalf("unexpected conversations: %#v", corpus.Conversations)
	}
	if len(corpus.Events) != 2 || corpus.Events[0].Class != memory.MemoryClassProfile || corpus.Events[0].ExpectedState != memory.MemoryStateActive {
		t.Fatalf("unexpected events: %#v", corpus.Events)
	}
	if len(corpus.Queries) != 2 || len(corpus.QRELs) != 3 {
		t.Fatalf("unexpected query/qrel result: %#v %#v", corpus.Queries, corpus.QRELs)
	}
	if len(corpus.Queries[1].EvidenceGroups) != 1 || len(corpus.Queries[1].EvidenceGroups[0].EvidenceIDs) != 2 {
		t.Fatalf("expected multi-hop evidence group, got %#v", corpus.Queries[1].EvidenceGroups)
	}
}

func TestLoCoMoSmokeFixtureIsRepositoryOwnedAndNormalizes(t *testing.T) {
	path := filepath.Join("testdata", "locomo-smoke-fixture-v1.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	dataset, err := LoadLoCoMoDataset(file)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLoCoMoAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Queries) != 2 || len(corpus.QRELs) != 3 {
		t.Fatalf("unexpected normalized fixture: %#v", corpus)
	}
}

func TestLoCoMoSmokeManifestMatchesFixtureChecksum(t *testing.T) {
	manifestFile, err := os.Open(filepath.Join("testdata", "locomo-smoke-manifest-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer manifestFile.Close()
	manifest, err := LoadDatasetManifest(manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("testdata", manifest.SourcePath))
	if err != nil {
		t.Fatal(err)
	}
	if checksumCanonicalText(data) != manifest.SHA256 {
		t.Fatalf("manifest checksum %s does not match fixture", manifest.SHA256)
	}
}

func TestLoCoMoAdapterRejectsUnmappedEvidence(t *testing.T) {
	_, err := NewLoCoMoAdapter().Normalize(LoCoMoDataset{Samples: []LoCoMoSample{{
		ID:        "sample-1",
		Sessions:  []LoCoMoSession{{ID: "session-1", Turns: []LoCoMoTurn{{ID: "turn-1", Text: "known"}}}},
		Questions: []LoCoMoQuestion{{ID: "q-1", Text: "question", EvidenceTurnIDs: []string{"turn-missing"}}},
	}}}, memoryScope())
	if err == nil {
		t.Fatal("expected unmapped LoCoMo evidence to be rejected")
	}
}

func TestLoCoMoAdapterPreservesTemporalUpdateQuestionType(t *testing.T) {
	corpus, err := NewLoCoMoAdapter().Normalize(LoCoMoDataset{Samples: []LoCoMoSample{{
		ID:        "sample-update",
		Sessions:  []LoCoMoSession{{ID: "first", Turns: []LoCoMoTurn{{ID: "turn-old", Text: "I work at the museum.", Timestamp: "2024-01-01T10:00:00Z"}}}, {ID: "second", Turns: []LoCoMoTurn{{ID: "turn-new", Text: "I work at the library now.", Timestamp: "2024-02-01T10:00:00Z"}}}},
		Questions: []LoCoMoQuestion{{ID: "q-update", Text: "Where does this person work now?", Category: "temporal-update", EvidenceTurnIDs: []string{"turn-new"}}},
	}}}, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Queries[0].QueryType != "temporal-update" || corpus.Events[1].SourceTurnID != "turn-new" || corpus.Events[1].ObservedAt != "2024-02-01T10:00:00Z" {
		t.Fatalf("expected temporal update provenance, got %#v", corpus)
	}
}

func TestLoCoMoAdapterRejectsDuplicateTurnID(t *testing.T) {
	_, err := NewLoCoMoAdapter().Normalize(LoCoMoDataset{Samples: []LoCoMoSample{{
		ID:       "sample-duplicate",
		Sessions: []LoCoMoSession{{ID: "one", Turns: []LoCoMoTurn{{ID: "turn-1", Text: "one"}}}, {ID: "two", Turns: []LoCoMoTurn{{ID: "turn-1", Text: "two"}}}},
	}}}, memoryScope())
	if err == nil {
		t.Fatal("expected duplicate turn id to fail")
	}
}
