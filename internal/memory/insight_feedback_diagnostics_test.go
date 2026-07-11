package memory

import "testing"

func TestSummarizeInsightFeedbackHealthAggregatesLowCardinalityCounts(t *testing.T) {
	health := SummarizeInsightFeedbackHealth([]DerivedInsight{
		{
			ID:    "insight_noisy",
			State: DerivedInsightStateActive,
			FeedbackSummary: DerivedInsightFeedbackSummary{
				Counts:        map[InsightFeedbackType]int{InsightFeedbackTypeNoisy: 1},
				NegativeCount: 1,
				TotalActive:   1,
			},
		},
		{
			ID:    "insight_review",
			State: DerivedInsightStateActive,
			FeedbackSummary: DerivedInsightFeedbackSummary{
				Counts:      map[InsightFeedbackType]int{InsightFeedbackTypeNeedsReview: 1},
				NeedsReview: true,
				TotalActive: 1,
			},
		},
		{
			ID:    "insight_suppressed",
			State: DerivedInsightStateSuppressed,
			Derivation: DerivedInsightDerivation{
				Metadata: map[string]any{"feedback_policy_decision": "suppress"},
			},
		},
	})

	if health.InsightCount != 3 || health.FeedbackCoveredCount != 2 || health.NoisyInsightCount != 1 || health.NeedsReviewCount != 1 || health.FeedbackDrivenSuppressionCount != 1 {
		t.Fatalf("health = %+v, want aggregate insight feedback counts", health)
	}
}
