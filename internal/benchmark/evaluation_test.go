package benchmark

import (
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
