package retrieval

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestFilterMemoryPathHitsUsesExactAndSegmentBoundaryPrefixSelectors(t *testing.T) {
	hits := []SearchHit{
		{Memory: memory.CanonicalMemory{ID: "exact", MemoryPath: "agents/research"}},
		{Memory: memory.CanonicalMemory{ID: "child", MemoryPath: "agents/research/preferences"}},
		{Memory: memory.CanonicalMemory{ID: "sibling", MemoryPath: "agents/researcher"}},
	}
	exact, err := memory.NewMemoryPathSelector("agents/research", "")
	if err != nil {
		t.Fatal(err)
	}
	got := filterMemoryPathHits(hits, exact)
	if len(got) != 1 || got[0].Memory.ID != "exact" {
		t.Fatalf("exact filter returned %#v", got)
	}
	prefix, err := memory.NewMemoryPathSelector("", "agents/research")
	if err != nil {
		t.Fatal(err)
	}
	got = filterMemoryPathHits(hits, prefix)
	if len(got) != 2 || got[0].Memory.ID != "exact" || got[1].Memory.ID != "child" {
		t.Fatalf("prefix filter returned %#v", got)
	}
}

func TestSearchInputValidateRejectsConflictingPathSelectors(t *testing.T) {
	err := (SearchInput{
		Scope:      memory.Scope{Tenant: "t", Project: "p", Namespace: "n"},
		Query:      "q",
		Path:       "agents/research",
		PathPrefix: "agents",
	}).Validate()
	if err == nil {
		t.Fatal("expected conflicting path selectors to be rejected")
	}
}
