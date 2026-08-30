package benchmark

import (
	"math"
	"sort"
	"time"
)

type RetrievedEvidence struct {
	EvidenceID string `json:"evidence_id"`
	Rank       int    `json:"rank"`
}

type QueryMetrics struct {
	RecallAt1               float64 `json:"recall_at_1"`
	RecallAt5               float64 `json:"recall_at_5"`
	RecallAt10              float64 `json:"recall_at_10"`
	MRR                     float64 `json:"mrr"`
	NDCGAt5                 float64 `json:"ndcg_at_5"`
	GroupHitRate            float64 `json:"evidence_group_hit_rate"`
	MustNotReturnViolations int     `json:"must_not_return_violations"`
	RelevantEvidenceCount   int     `json:"relevant_evidence_count"`
}

type QueryEvaluation struct {
	QueryID        string        `json:"query_id"`
	Metrics        QueryMetrics  `json:"metrics"`
	SafetyFailures int           `json:"safety_failures"`
	Duration       time.Duration `json:"-"`
	LatencyMS      float64       `json:"latency_ms"`
}

type EvaluationMetrics struct {
	RecallAt1                   float64 `json:"recall_at_1"`
	RecallAt5                   float64 `json:"recall_at_5"`
	RecallAt10                  float64 `json:"recall_at_10"`
	MRR                         float64 `json:"mrr"`
	NDCGAt5                     float64 `json:"ndcg_at_5"`
	GroupHitRate                float64 `json:"evidence_group_hit_rate"`
	MustNotReturnViolations     int     `json:"must_not_return_violations"`
	SafetyFailures              int     `json:"safety_failures"`
	P50LatencyMS                float64 `json:"p50_latency_ms"`
	P95LatencyMS                float64 `json:"p95_latency_ms"`
	QueryCount                  int     `json:"query_count"`
	QueriesWithRelevantEvidence int     `json:"queries_with_relevant_evidence"`
	QueryCoverage               float64 `json:"query_coverage"`
}

type EvaluationReport struct {
	Queries []QueryEvaluation `json:"queries"`
	Metrics EvaluationMetrics `json:"metrics"`
}

func EvaluateQuery(query BenchmarkQuery, qrels []QREL, candidates []RetrievedEvidence, duration time.Duration) QueryEvaluation {
	ordered := append([]RetrievedEvidence(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Rank < ordered[j].Rank })
	ranks := make(map[string]int, len(ordered))
	for _, candidate := range ordered {
		if candidate.Rank <= 0 {
			continue
		}
		if existing, ok := ranks[candidate.EvidenceID]; !ok || candidate.Rank < existing {
			ranks[candidate.EvidenceID] = candidate.Rank
		}
	}
	relevant := make(map[string]int)
	for _, qrel := range qrels {
		if qrel.QueryID == query.ID && qrel.Grade > 0 {
			relevant[qrel.EvidenceID] = qrel.Grade
		}
	}
	metrics := QueryMetrics{
		RecallAt1:    recallAt(ranks, relevant, 1),
		RecallAt5:    recallAt(ranks, relevant, 5),
		RecallAt10:   recallAt(ranks, relevant, 10),
		MRR:          reciprocalRank(ranks, relevant),
		NDCGAt5:      ndcgAt(ranks, relevant, 5),
		GroupHitRate: evidenceGroupHitRate(query.EvidenceGroups, ranks),
	}
	metrics.RelevantEvidenceCount = len(relevant)
	forbidden := make(map[string]struct{}, len(query.MustNotReturnIDs))
	for _, id := range query.MustNotReturnIDs {
		forbidden[id] = struct{}{}
	}
	for id := range ranks {
		if _, found := forbidden[id]; found {
			metrics.MustNotReturnViolations++
		}
	}
	result := QueryEvaluation{QueryID: query.ID, Metrics: metrics, Duration: duration, LatencyMS: float64(duration) / float64(time.Millisecond)}
	result.SafetyFailures = metrics.MustNotReturnViolations
	return result
}

func AggregateEvaluation(items []QueryEvaluation) EvaluationReport {
	queries := append([]QueryEvaluation(nil), items...)
	sort.SliceStable(queries, func(i, j int) bool { return queries[i].QueryID < queries[j].QueryID })
	report := EvaluationReport{Queries: queries}
	if len(queries) == 0 {
		return report
	}
	latencies := make([]float64, 0, len(queries))
	for i := range queries {
		item := &queries[i]
		if item.LatencyMS == 0 && item.Duration > 0 {
			item.LatencyMS = float64(item.Duration) / float64(time.Millisecond)
		}
		latencies = append(latencies, item.LatencyMS)
		report.Metrics.RecallAt1 += item.Metrics.RecallAt1
		report.Metrics.RecallAt5 += item.Metrics.RecallAt5
		report.Metrics.RecallAt10 += item.Metrics.RecallAt10
		report.Metrics.MRR += item.Metrics.MRR
		report.Metrics.NDCGAt5 += item.Metrics.NDCGAt5
		report.Metrics.GroupHitRate += item.Metrics.GroupHitRate
		report.Metrics.MustNotReturnViolations += item.Metrics.MustNotReturnViolations
		report.Metrics.SafetyFailures += item.SafetyFailures
		report.Metrics.QueryCount++
		if item.Metrics.RelevantEvidenceCount > 0 {
			report.Metrics.QueriesWithRelevantEvidence++
		}
	}
	count := float64(len(queries))
	report.Metrics.RecallAt1 /= count
	report.Metrics.RecallAt5 /= count
	report.Metrics.RecallAt10 /= count
	report.Metrics.MRR /= count
	report.Metrics.NDCGAt5 /= count
	report.Metrics.GroupHitRate /= count
	report.Metrics.QueryCoverage = float64(report.Metrics.QueriesWithRelevantEvidence) / count
	report.Metrics.P50LatencyMS = percentile(latencies, 0.50)
	report.Metrics.P95LatencyMS = percentile(latencies, 0.95)
	return report
}

func recallAt(ranks map[string]int, relevant map[string]int, cutoff int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	hits := 0
	for id := range relevant {
		if rank := ranks[id]; rank > 0 && rank <= cutoff {
			hits++
		}
	}
	return float64(hits) / float64(len(relevant))
}

func reciprocalRank(ranks map[string]int, relevant map[string]int) float64 {
	best := 0
	for id := range relevant {
		if rank := ranks[id]; rank > 0 && (best == 0 || rank < best) {
			best = rank
		}
	}
	if best == 0 {
		return 0
	}
	return 1 / float64(best)
}

func ndcgAt(ranks map[string]int, relevant map[string]int, cutoff int) float64 {
	dcg := 0.0
	grades := make([]int, 0, len(relevant))
	for id, grade := range relevant {
		grades = append(grades, grade)
		if rank := ranks[id]; rank > 0 && rank <= cutoff {
			dcg += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(rank)+1)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	ideal := 0.0
	for index, grade := range grades {
		if index >= cutoff {
			break
		}
		ideal += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(index)+2)
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func evidenceGroupHitRate(groups []EvidenceGroup, ranks map[string]int) float64 {
	if len(groups) == 0 {
		return 0
	}
	required, hits := 0, 0
	for _, group := range groups {
		if !group.Required {
			continue
		}
		required++
		complete := len(group.EvidenceIDs) > 0
		for _, id := range group.EvidenceIDs {
			if _, found := ranks[id]; !found {
				complete = false
				break
			}
		}
		if complete {
			hits++
		}
	}
	if required == 0 {
		return 0
	}
	return float64(hits) / float64(required)
}

func percentile(values []float64, target float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(math.Ceil(float64(len(sorted))*target)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
