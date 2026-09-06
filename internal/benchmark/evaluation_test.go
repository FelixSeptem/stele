package benchmark

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvaluateQueryCalculatesGradedMetricsAndCompleteEvidenceGroups(t *testing.T) {
	query := BenchmarkQuery{ID: "q1", EvidenceGroups: []EvidenceGroup{{ID: "g1", EvidenceIDs: []string{"e1", "e2"}, Required: true}}}
	qrels := []QREL{{QueryID: "q1", EvidenceID: "e1", Grade: 3, GroupID: "g1"}, {QueryID: "q1", EvidenceID: "e2", Grade: 1, GroupID: "g1"}}
	result := EvaluateQuery(query, qrels, []RetrievedEvidence{{EvidenceID: "e1", Rank: 1}, {EvidenceID: "e2", Rank: 3}}, 5*time.Millisecond)
	if result.Metrics.RecallAt1 != 0.5 || result.Metrics.RecallAt5 != 1 || result.Metrics.MRR != 1 || result.Metrics.NDCGAt5 <= 0 || result.Metrics.GroupHitRate != 1 {
		t.Fatalf("unexpected metrics: %#v", result.Metrics)
	}
	if result.SafetyFailures != 0 {
		t.Fatalf("unexpected safety failures: %#v", result)
	}
}

func TestEvaluateQueryDistinguishesPartialEvidenceGroupFromCompleteGroup(t *testing.T) {
	query := BenchmarkQuery{ID: "q1", EvidenceGroups: []EvidenceGroup{{ID: "g1", EvidenceIDs: []string{"e1", "e2"}, Required: true}}}
	qrels := []QREL{{QueryID: "q1", EvidenceID: "e1", Grade: 3, GroupID: "g1"}, {QueryID: "q1", EvidenceID: "e2", Grade: 1, GroupID: "g1"}}
	partial := EvaluateQuery(query, qrels, []RetrievedEvidence{{EvidenceID: "e1", Rank: 1}}, time.Millisecond)
	complete := EvaluateQuery(query, qrels, []RetrievedEvidence{{EvidenceID: "e1", Rank: 1}, {EvidenceID: "e2", Rank: 2}}, time.Millisecond)
	if partial.Metrics.RecallAt5 != 0.5 || partial.Metrics.GroupHitRate != 0 {
		t.Fatalf("partial evidence should retain graded recall but not complete group hit: %#v", partial.Metrics)
	}
	if complete.Metrics.RecallAt5 != 1 || complete.Metrics.GroupHitRate != 1 {
		t.Fatalf("complete evidence should hit the whole group: %#v", complete.Metrics)
	}
}

func TestEvaluateQueryUsesGradedQRELsForNDCGOrdering(t *testing.T) {
	query := BenchmarkQuery{ID: "q1"}
	qrels := []QREL{{QueryID: "q1", EvidenceID: "high", Grade: 3}, {QueryID: "q1", EvidenceID: "low", Grade: 1}}
	highFirst := EvaluateQuery(query, qrels, []RetrievedEvidence{{EvidenceID: "high", Rank: 1}, {EvidenceID: "low", Rank: 2}}, time.Millisecond)
	lowFirst := EvaluateQuery(query, qrels, []RetrievedEvidence{{EvidenceID: "low", Rank: 1}, {EvidenceID: "high", Rank: 2}}, time.Millisecond)
	if highFirst.Metrics.NDCGAt5 <= lowFirst.Metrics.NDCGAt5 {
		t.Fatalf("higher graded evidence at an earlier rank should improve nDCG: high-first=%v low-first=%v", highFirst.Metrics.NDCGAt5, lowFirst.Metrics.NDCGAt5)
	}
}

func TestEvaluateQueryRejectsMustNotReturnEvidence(t *testing.T) {
	query := BenchmarkQuery{ID: "q1", MustNotReturnIDs: []string{"hidden"}}
	result := EvaluateQuery(query, nil, []RetrievedEvidence{{EvidenceID: "hidden", Rank: 1}}, time.Millisecond)
	if result.SafetyFailures != 1 || result.Metrics.MustNotReturnViolations != 1 {
		t.Fatalf("expected forbidden return to fail safety, got %#v", result)
	}
}

func TestAggregateEvaluationUsesDeterministicOrderingAndPercentiles(t *testing.T) {
	report := AggregateEvaluation([]QueryEvaluation{{QueryID: "q2", Duration: 20 * time.Millisecond, Metrics: QueryMetrics{RecallAt5: 1}}, {QueryID: "q1", Duration: 10 * time.Millisecond, Metrics: QueryMetrics{RecallAt5: 0.5}}})
	if report.Queries[0].QueryID != "q1" || report.Metrics.RecallAt5 != 0.75 || report.Metrics.P50LatencyMS != 10 || report.Metrics.P95LatencyMS != 20 {
		t.Fatalf("unexpected aggregate report: %#v", report)
	}
}

func TestAggregateEvaluationReportsQueryCoverage(t *testing.T) {
	report := AggregateEvaluation([]QueryEvaluation{{QueryID: "q1", Metrics: QueryMetrics{RecallAt5: 1, RelevantEvidenceCount: 1}}, {QueryID: "q2", Metrics: QueryMetrics{}}})
	if report.Metrics.QueryCount != 2 || report.Metrics.QueriesWithRelevantEvidence != 1 || report.Metrics.QueryCoverage != 0.5 {
		t.Fatalf("unexpected query coverage: %#v", report.Metrics)
	}
}

func TestAggregateEvaluationByQueryTypeKeepsProfileAndGenericRetrievalSeparate(t *testing.T) {
	profile := EvaluateQuery(BenchmarkQuery{ID: "profile-1", QueryType: "profile"}, []QREL{{QueryID: "profile-1", EvidenceID: "profile-event", Grade: 2}}, []RetrievedEvidence{{EvidenceID: "profile-event", Rank: 1}}, time.Millisecond)
	generic := EvaluateQuery(BenchmarkQuery{ID: "generic-1", QueryType: "generic_retrieval"}, []QREL{{QueryID: "generic-1", EvidenceID: "generic-event", Grade: 2}}, []RetrievedEvidence{{EvidenceID: "generic-event", Rank: 1}}, 2*time.Millisecond)
	byType := AggregateEvaluationByQueryType([]QueryEvaluation{generic, profile})
	if len(byType) != 2 {
		t.Fatalf("query types were merged: %#v", byType)
	}
	if byType["profile"].Metrics.QueryCount != 1 || byType["generic_retrieval"].Metrics.QueryCount != 1 {
		t.Fatalf("query type counts = %#v", byType)
	}
	if byType["profile"].Queries[0].QueryID != "profile-1" || byType["generic_retrieval"].Queries[0].QueryID != "generic-1" {
		t.Fatalf("query type rows crossed families: %#v", byType)
	}
}

func TestAggregateEvaluationOrdersQueriesDeterministically(t *testing.T) {
	first := AggregateEvaluation([]QueryEvaluation{{QueryID: "q2"}, {QueryID: "q1"}})
	second := AggregateEvaluation([]QueryEvaluation{{QueryID: "q1"}, {QueryID: "q2"}})
	if len(first.Queries) != 2 || first.Queries[0].QueryID != "q1" || first.Queries[1].QueryID != "q2" {
		t.Fatalf("queries were not sorted by stable id: %#v", first.Queries)
	}
	firstEncoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first report: %v", err)
	}
	secondEncoded, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second report: %v", err)
	}
	if string(firstEncoded) != string(secondEncoded) {
		t.Fatalf("equivalent evaluations must serialize identically:\n%s\n%s", firstEncoded, secondEncoded)
	}
}
