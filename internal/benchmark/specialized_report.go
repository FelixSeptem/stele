package benchmark

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// SpecializedReport keeps targeted profile, temporal, and multi-hop metrics
// separate from generic ranking reports.
type SpecializedReport struct {
	SchemaVersion             string           `json:"schema_version"`
	Family                    BenchmarkFamily  `json:"family"`
	Subfamily                 string           `json:"subfamily"`
	Status                    Status           `json:"status"`
	QueryCount                int              `json:"query_count"`
	Recall                    float64          `json:"recall"`
	PreferenceConsistency     float64          `json:"preference_consistency"`
	CurrentPreferenceRecall   float64          `json:"current_preference_recall"`
	TemporalUpdatePrecedence  float64          `json:"temporal_update_precedence"`
	StaleFactSuppression      float64          `json:"stale_fact_suppression"`
	EvidenceGroupHit          float64          `json:"evidence_group_hit"`
	EvidenceGroupCompleteness float64          `json:"evidence_group_completeness"`
	ScopeSafetyFailures       int              `json:"scope_safety_failures"`
	UnmappedEvidenceCount     int              `json:"unmapped_evidence_count"`
	Evaluation                EvaluationReport `json:"evaluation"`
}

// EvaluateSpecializedCases evaluates repository-owned or normalized
// specialized fixtures with caller-supplied ranked evidence. It validates
// scopes and qrels, then derives targeted metrics by subfamily.
func EvaluateSpecializedCases(cases []SpecializedCase, ranked map[string][]RetrievedEvidence) (map[string]SpecializedReport, error) {
	result := map[string]SpecializedReport{}
	for _, item := range cases {
		if err := item.Query.Scope.Validate(); err != nil {
			return nil, fmt.Errorf("specialized query %s scope: %w", item.Query.ID, err)
		}
		if strings.TrimSpace(item.Subfamily) == "" {
			return nil, fmt.Errorf("specialized case %s subfamily is required", item.ID)
		}
		events := map[string]MemoryEventRecord{}
		for _, event := range item.Events {
			if err := event.Scope.Validate(); err != nil {
				return nil, fmt.Errorf("specialized event %s scope: %w", event.ID, err)
			}
			if event.Scope.Normalized() != item.Query.Scope.Normalized() {
				return nil, fmt.Errorf("specialized event %s scope mismatch", event.ID)
			}
			events[event.ID] = event
		}
		for _, qrel := range item.QRELs {
			if qrel.QueryID != item.Query.ID {
				continue
			}
			if _, ok := events[qrel.EvidenceID]; !ok {
				return nil, fmt.Errorf("specialized qrel %s references unmapped evidence", qrel.EvidenceID)
			}
		}
		evaluation := EvaluateQuery(item.Query, item.QRELs, ranked[item.Query.ID], 0)
		report := result[item.Subfamily]
		if report.QueryCount == 0 {
			report = SpecializedReport{SchemaVersion: SchemaVersion, Family: FamilySpecializedRetrieval, Subfamily: item.Subfamily, Status: StatusSuccess}
		}
		report.QueryCount++
		report.Recall += evaluation.Metrics.RecallAt10
		report.EvidenceGroupHit += evaluation.Metrics.GroupHitRate
		report.ScopeSafetyFailures += evaluation.Metrics.MustNotReturnViolations
		for _, event := range item.Events {
			if _, ok := events[event.ID]; !ok {
				report.UnmappedEvidenceCount++
			}
		}
		if item.Subfamily == "profile" || item.Subfamily == "preference" {
			for _, event := range item.Events {
				if event.ExpectedState == memory.MemoryStateActive && containsEvidence(ranked[item.Query.ID], event.ID) {
					report.CurrentPreferenceRecall++
				}
			}
			if evaluation.Metrics.MustNotReturnViolations == 0 {
				report.PreferenceConsistency++
			}
		}
		if item.Subfamily == "temporal" {
			if evaluation.Metrics.MustNotReturnViolations == 0 {
				report.StaleFactSuppression++
			}
			if evaluation.Metrics.GroupHitRate == 1 {
				report.TemporalUpdatePrecedence++
			}
		}
		if item.Subfamily == "multi-hop" && evaluation.Metrics.GroupHitRate == 1 {
			report.EvidenceGroupCompleteness++
		}
		if report.Evaluation.Queries == nil {
			report.Evaluation = EvaluationReport{}
		}
		report.Evaluation.Queries = append(report.Evaluation.Queries, evaluation)
		result[item.Subfamily] = report
	}
	for key, report := range result {
		if report.QueryCount > 0 {
			count := float64(report.QueryCount)
			report.Recall /= count
			report.EvidenceGroupHit /= count
			if key == "profile" || key == "preference" {
				report.PreferenceConsistency /= count
				report.CurrentPreferenceRecall /= count
			}
			if key == "temporal" {
				report.TemporalUpdatePrecedence /= count
				report.StaleFactSuppression /= count
			}
			if key == "multi-hop" {
				report.EvidenceGroupCompleteness /= count
			}
		}
		report.Evaluation = AggregateEvaluation(report.Evaluation.Queries)
		result[key] = report
	}
	return result, nil
}

func containsEvidence(items []RetrievedEvidence, id string) bool {
	for _, item := range items {
		if item.EvidenceID == id {
			return true
		}
	}
	return false
}

func SortedSpecializedReports(reports map[string]SpecializedReport) []SpecializedReport {
	keys := make([]string, 0, len(reports))
	for key := range reports {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]SpecializedReport, 0, len(keys))
	for _, key := range keys {
		result = append(result, reports[key])
	}
	return result
}
