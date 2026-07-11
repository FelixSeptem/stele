package memory

type InsightFeedbackHealth struct {
	InsightCount                   int `json:"insight_count"`
	FeedbackCoveredCount           int `json:"feedback_covered_count"`
	NoisyInsightCount              int `json:"noisy_insight_count"`
	NeedsReviewCount               int `json:"needs_review_count"`
	FeedbackDrivenSuppressionCount int `json:"feedback_driven_suppression_count"`
}

func SummarizeInsightFeedbackHealth(insights []DerivedInsight) InsightFeedbackHealth {
	health := InsightFeedbackHealth{InsightCount: len(insights)}
	for _, insight := range insights {
		summary := insight.FeedbackSummary
		if summary.TotalActive > 0 {
			health.FeedbackCoveredCount++
		}
		if summary.Counts[InsightFeedbackTypeNoisy] > 0 {
			health.NoisyInsightCount++
		}
		if summary.NeedsReview {
			health.NeedsReviewCount++
		}
		if insight.State == DerivedInsightStateSuppressed && insight.Derivation.Metadata["feedback_policy_decision"] == "suppress" {
			health.FeedbackDrivenSuppressionCount++
		}
	}
	return health
}
