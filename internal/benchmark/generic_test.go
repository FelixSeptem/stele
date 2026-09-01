package benchmark

import (
	"github.com/FelixSeptem/stele/internal/memory"
	"testing"
)

func TestNormalizeGenericRetrievalAndRejectForeignQREL(t *testing.T) {
	scope := memory.Scope{Tenant: "bench", Project: "generic", Namespace: "run"}
	source := []byte(`{"documents":[{"id":"d1","text":"PostgreSQL"}],"queries":[{"id":"q1","text":"database"}],"qrels":[{"query_id":"q1","evidence_id":"d1","grade":2}]}`)
	corpus, err := NormalizeGenericRetrieval(scope, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Events) != 1 || corpus.Queries[0].QueryType != "generic_retrieval" {
		t.Fatalf("corpus=%#v", corpus)
	}
}

func TestCompareStrategyReportsRejectsFamilyMismatch(t *testing.T) {
	base := StrategyReport{Family: FamilyMemory, Dataset: "d", Version: "v", NormalizedChecksum: "n", QRELChecksum: "q", Profile: StrategyProfileLexical}
	candidate := base
	candidate.Family = FamilyGenericRetrieval
	if _, err := CompareStrategyReports(base, candidate, nil); err == nil {
		t.Fatal("expected family mismatch")
	}
}
