package benchmark

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildRetrievalEvaluationFixtureReusesCorpusForEachQuery(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "run-1"}
	corpus := NormalizedCorpus{
		Events: []MemoryEventRecord{
			{ID: "event-1", Scope: scope, Text: "Avery prefers congee.", Class: memory.MemoryClassProfile, ExpectedState: memory.MemoryStateActive},
			{ID: "event-2", Scope: scope, Text: "Blake works at the library.", Class: memory.MemoryClassEpisodic, ExpectedState: memory.MemoryStateActive},
		},
		Queries: []BenchmarkQuery{
			{ID: "query-1", Scope: scope, Text: "What does Avery prefer?", EvidenceGroups: []EvidenceGroup{{ID: "g1", EvidenceIDs: []string{"event-1"}, Required: true}}},
			{ID: "query-2", Scope: scope, Text: "Where does Blake work and what does Avery prefer?", EvidenceGroups: []EvidenceGroup{{ID: "g2", EvidenceIDs: []string{"event-1", "event-2"}, Required: true}}, MustNotReturnIDs: []string{"event-forbidden"}},
		},
	}
	mappings := map[string]RetrievalEvidenceMapping{
		"event-1": {MemoryID: "memory-1", Scope: scope, State: memory.MemoryStateActive},
		"event-2": {MemoryID: "memory-2", Scope: scope, State: memory.MemoryStateActive},
	}
	prepared, err := BuildRetrievalEvaluationFixture(corpus, mappings, RetrievalEvaluationMetadata{FixtureVersion: "locomo-v1", RepresentationVersion: "normalized-v1", RankingVersion: "lexical-v1", EmbeddingRevision: "lexical-only", PolicyVersion: "policy-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Fixture.Cases) != 2 || len(prepared.Fixture.Cases[0].Sources) != 2 || len(prepared.Seed.Aliases) != 4 {
		t.Fatalf("expected each query to resolve the shared corpus, got fixture=%#v seed=%#v", prepared.Fixture, prepared.Seed)
	}
	if prepared.Fixture.Cases[1].ExpectedEvidenceGroups[0][0] != "event-1" || prepared.Fixture.Cases[1].ExpectedEvidenceGroups[0][1] != "event-2" {
		t.Fatalf("unexpected grouped evidence: %#v", prepared.Fixture.Cases[1].ExpectedEvidenceGroups)
	}
}

func TestBuildRetrievalEvaluationFixtureRejectsUnknownEvidenceMapping(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "run-1"}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "event-1", Scope: scope, Text: "fact"}}, Queries: []BenchmarkQuery{{ID: "query-1", Scope: scope, Text: "question", EvidenceGroups: []EvidenceGroup{{ID: "g1", EvidenceIDs: []string{"event-1"}, Required: true}}}}}
	if _, err := BuildRetrievalEvaluationFixture(corpus, nil, RetrievalEvaluationMetadata{FixtureVersion: "locomo-v1", RepresentationVersion: "normalized-v1", RankingVersion: "lexical-v1", EmbeddingRevision: "lexical-only", PolicyVersion: "policy-v1"}); err == nil {
		t.Fatal("expected missing evidence mapping to fail")
	}
}

func TestBuildRetrievalEvaluationFixtureBoundsCandidatesToQuerySession(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "run-1"}
	corpus := NormalizedCorpus{
		Events: []MemoryEventRecord{
			{ID: "sample-a/event-1", Scope: scope, SessionID: "sample-a/session-1", Text: "relevant"},
			{ID: "sample-a/event-2", Scope: scope, SessionID: "sample-a/session-2", Text: "same sample"},
			{ID: "sample-b/event-1", Scope: scope, SessionID: "sample-b/session-1", Text: "foreign sample"},
		},
		Queries: []BenchmarkQuery{{ID: "q-a", Scope: scope, SessionID: "sample-a", Text: "question", EvidenceGroups: []EvidenceGroup{{ID: "g-a", EvidenceIDs: []string{"sample-a/event-1"}, Required: true}}}},
	}
	mappings := map[string]RetrievalEvidenceMapping{
		"sample-a/event-1": {MemoryID: "memory-a1", Scope: scope, State: memory.MemoryStateActive},
		"sample-a/event-2": {MemoryID: "memory-a2", Scope: scope, State: memory.MemoryStateActive},
		"sample-b/event-1": {MemoryID: "memory-b1", Scope: scope, State: memory.MemoryStateActive},
	}
	prepared, err := BuildRetrievalEvaluationFixture(corpus, mappings, RetrievalEvaluationMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "normalized-v1", RankingVersion: "lexical-v1", EmbeddingRevision: "lexical-only", PolicyVersion: "policy-v1"})
	if err != nil {
		t.Fatalf("BuildRetrievalEvaluationFixture() error = %v", err)
	}
	if len(prepared.Fixture.Cases) != 1 || len(prepared.Fixture.Cases[0].Sources) != 2 {
		t.Fatalf("query candidate pool should be bounded to sample session: %#v", prepared.Fixture.Cases)
	}
	for _, source := range prepared.Fixture.Cases[0].Sources {
		if source.Alias == "sample-b/event-1" {
			t.Fatalf("foreign sample leaked into query candidate pool: %#v", prepared.Fixture.Cases[0].Sources)
		}
	}
}
