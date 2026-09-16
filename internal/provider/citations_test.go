package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestShapeSearchResultRedactsScoresQueriesAndHiddenItems(t *testing.T) {
	result, err := ShapeSearchResult(retrieval.SearchResult{Hits: []retrieval.SearchHit{
		{Memory: memory.CanonicalMemory{ID: "memory-1", State: memory.MemoryStateActive, Content: "visible"}, Score: retrieval.ScoreBreakdown{Overall: .99}, Citations: []retrieval.Citation{{MemoryID: "memory-1", RawEventID: "event-1"}}},
		{Memory: memory.CanonicalMemory{ID: "hidden-id", State: memory.MemoryStateSuppressed, Content: "hidden"}},
	}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(result)
	text := string(body)
	for _, forbidden := range []string{"0.99", "hidden-id", "hidden\""} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"source_kind":"raw_event"`) || !strings.Contains(text, `"reference":"event-1"`) {
		t.Fatalf("citation missing: %s", text)
	}
}

func TestShapeContextFailsClosedWhenCitationBudgetExceeded(t *testing.T) {
	_, err := ShapeContext(retrieval.AssembledContext{Citations: []retrieval.Citation{{MemoryID: "one"}, {MemoryID: "two"}}}, 1)
	if err == nil {
		t.Fatal("expected citation budget failure")
	}
}
