package retrieval

import (
	"context"
	"testing"
)

func TestValidateRerankResponseRejectsUnknownAndDuplicateCandidates(t *testing.T) {
	known := []RerankCandidate{{ID: "m1", Text: "one"}, {ID: "m2", Text: "two"}}
	if err := ValidateRerankResponse(known, []RerankScore{{ID: "m1", Score: 0.5}, {ID: "m3", Score: 0.4}}); err == nil {
		t.Fatal("expected unknown candidate rejection")
	}
	if err := ValidateRerankResponse(known, []RerankScore{{ID: "m1", Score: 0.5}, {ID: "m1", Score: 0.4}}); err == nil {
		t.Fatal("expected duplicate candidate rejection")
	}
}

func TestStaticRerankerReturnsDeterministicScores(t *testing.T) {
	r := StaticReranker{Scores: map[string]float64{"m2": 0.9, "m1": 0.1}}
	got, err := r.Rerank(context.Background(), RerankRequest{Query: "q", Candidates: []RerankCandidate{{ID: "m1", Text: "one"}, {ID: "m2", Text: "two"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "m2" || got[1].ID != "m1" {
		t.Fatalf("scores=%+v, want deterministic descending order", got)
	}
}
