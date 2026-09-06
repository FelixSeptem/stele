package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestProfileAndGenericFixtureReportsDistinctQueryTypes(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "profile-generic-fixture-v1.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var corpus NormalizedCorpus
	if err := json.Unmarshal(encoded, &corpus); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := corpus.Validate(); err != nil {
		t.Fatalf("fixture validation: %v", err)
	}
	prepared, err := BuildRetrievalEvaluationFixture(corpus, map[string]RetrievalEvidenceMapping{
		"event-profile":    {MemoryID: "memory-profile", Scope: profileGenericScope(), State: memory.MemoryStateActive},
		"event-preference": {MemoryID: "memory-preference", Scope: profileGenericScope(), State: memory.MemoryStateActive},
		"event-generic":    {MemoryID: "memory-generic", Scope: profileGenericScope(), State: memory.MemoryStateActive},
	}, RetrievalEvaluationMetadata{FixtureVersion: "profile-generic-v1", RepresentationVersion: "canonical-v1", RankingVersion: "fixture-v1", EmbeddingRevision: "deterministic-v1", PolicyVersion: "quality-policy-v1"})
	if err != nil {
		t.Fatalf("prepare fixture: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range prepared.Fixture.Cases {
		seen[item.Category] = true
	}
	for _, queryType := range []string{"profile", "preference", "generic_retrieval"} {
		if !seen[queryType] {
			t.Fatalf("fixture missing distinct query type %q: %#v", queryType, seen)
		}
	}
}

func profileGenericScope() memory.Scope {
	return memory.Scope{Tenant: "bench", Project: "profile-generic", Namespace: "fixture"}
}
