package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestShapeSearchResponseRedactsScoresScopeAndDiagnostics(t *testing.T) {
	result := retrieval.SearchResult{
		Hits: []retrieval.SearchHit{{
			Memory:    memory.CanonicalMemory{ID: "mem-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "safe"},
			Score:     retrieval.ScoreBreakdown{Overall: 0.99},
			Citations: []retrieval.Citation{{MemoryID: "mem-1", Operation: "search"}},
		}},
	}
	encoded, err := json.Marshal(shapeSearchResponse(result))
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, forbidden := range []string{"tenant-a", "project-a", "overall", "diagnostics", "0.99"} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("response contains forbidden %q: %s", forbidden, value)
		}
	}
	if !strings.Contains(value, "mem-1") || !strings.Contains(value, "safe") {
		t.Fatalf("response lost safe fields: %s", value)
	}
}

func TestParseTemporalRejectsAmbiguousOrPartialSelectors(t *testing.T) {
	if _, err := parseTemporal("2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", ""); err == nil {
		t.Fatal("parseTemporal() error = nil for ambiguous selector")
	}
	if _, err := parseTemporal("", "2026-01-01T00:00:00Z", ""); err == nil {
		t.Fatal("parseTemporal() error = nil for partial interval")
	}
}
