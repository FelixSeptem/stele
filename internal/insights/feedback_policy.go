package insights

import "github.com/FelixSeptem/stele/internal/memory"

type FeedbackPolicyDecision string

const (
	FeedbackPolicyDecisionNone        FeedbackPolicyDecision = "none"
	FeedbackPolicyDecisionPreserve    FeedbackPolicyDecision = "preserve"
	FeedbackPolicyDecisionNeedsReview FeedbackPolicyDecision = "needs_review"
	FeedbackPolicyDecisionSuppress    FeedbackPolicyDecision = "suppress"
)

func ApplyFeedbackPolicy(insight memory.DerivedInsight, summary memory.DerivedInsightFeedbackSummary) (memory.DerivedInsight, FeedbackPolicyDecision) {
	decision := FeedbackPolicyDecisionNone

	switch {
	case summary.NegativeCount >= 2 || summary.Counts[memory.InsightFeedbackTypeIncorrect] > 0:
		insight.State = memory.DerivedInsightStateSuppressed
		decision = FeedbackPolicyDecisionSuppress
	case summary.NeedsReview:
		insight.State = memory.DerivedInsightStateCandidate
		decision = FeedbackPolicyDecisionNeedsReview
	case summary.PositiveCount > 0:
		decision = FeedbackPolicyDecisionPreserve
	}

	if insight.Derivation.Metadata == nil {
		insight.Derivation.Metadata = map[string]any{}
	}
	if decision != FeedbackPolicyDecisionNone {
		insight.Derivation.Metadata["feedback_policy_decision"] = string(decision)
		insight.Derivation.Metadata["feedback_positive_count"] = float64(summary.PositiveCount)
		insight.Derivation.Metadata["feedback_negative_count"] = float64(summary.NegativeCount)
		insight.Derivation.Metadata["feedback_needs_review"] = summary.NeedsReview
	}

	return insight, decision
}
