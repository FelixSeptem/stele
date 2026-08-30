package retrieval

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/telemetry"
)

const (
	evaluationDecisionSafetyFailure     = "safety_failure"
	evaluationDecisionProtectedRecall   = "protected_recall_regression"
	evaluationDecisionProtectedMultiHop = "protected_multihop_regression"
	evaluationDecisionLatencyExceeded   = "latency_budget_exceeded"
)

// EvaluateReleasePolicy turns a compatible baseline/candidate pair into a bounded
// rollout decision. Safety failures are evaluated first and always override quality
// gains.
func EvaluateReleasePolicy(policy EvaluationReleasePolicy, baseline, candidate EvaluationReport) (EvaluationReleaseDecision, error) {
	if err := policy.Validate(); err != nil {
		return EvaluationReleaseDecision{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if candidate.Metadata.PolicyVersion != policy.Version {
		return EvaluationReleaseDecision{}, fmt.Errorf("candidate report policy version does not match release policy")
	}
	comparison, err := CompareEvaluationReports(baseline, candidate, policy.ProtectedCategories)
	if err != nil {
		return EvaluationReleaseDecision{}, err
	}
	decision := EvaluationReleaseDecision{PolicyVersion: policy.Version, Eligible: true}
	if len(candidate.SafetyFailures) > 0 {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionSafetyFailure)
		return decision, nil
	}
	for _, cutoff := range policy.ProtectedCutoffs {
		baselineValue := evaluationRecallAt(baseline.Metrics, cutoff)
		candidateValue := evaluationRecallAt(candidate.Metrics, cutoff)
		if candidateValue < baselineValue-policy.MaxRecallRegression {
			decision.Eligible = false
			decision.HardFailures = append(decision.HardFailures, evaluationDecisionProtectedRecall)
			break
		}
	}
	if candidate.Metrics.MultiHopEvidenceCoverage < baseline.Metrics.MultiHopEvidenceCoverage-policy.MaxMultiHopCoverageRegression {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionProtectedMultiHop)
	}
	if candidate.Metrics.P95LatencyMS > float64(policy.MaxP95LatencyMS) {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionLatencyExceeded)
	}
	if len(comparison.ProtectedRegressions) > 0 {
		decision.Advisories = append(decision.Advisories, "protected_category_regression_observed")
	}
	return decision, nil
}

func evaluationRecallAt(metrics EvaluationMetricReport, cutoff int) float64 {
	switch cutoff {
	case 1:
		return metrics.RecallAt1
	case 5:
		return metrics.RecallAt5
	case 10:
		return metrics.RecallAt10
	default:
		return metrics.RecallAt10
	}
}

// RecordEvaluationReleaseDecision emits only the policy version, bounded decision,
// and first stable failure category. It is a no-op for observers that do not opt into
// evaluation telemetry.
func RecordEvaluationReleaseDecision(ctx context.Context, observer telemetry.Observer, decision EvaluationReleaseDecision) {
	if observer == nil {
		return
	}
	recorder, ok := observer.(interface {
		RecordRetrievalEvaluation(context.Context, telemetry.RetrievalEvaluationEvent)
	})
	if !ok {
		return
	}
	status := "accepted"
	if !decision.Eligible {
		status = "rejected"
	}
	failureCategory := ""
	if len(decision.HardFailures) > 0 {
		failureCategory = decision.HardFailures[0]
	}
	recorder.RecordRetrievalEvaluation(ctx, telemetry.RetrievalEvaluationEvent{
		Status:          "decision",
		PolicyVersion:   decision.PolicyVersion,
		Decision:        status,
		FailureCategory: failureCategory,
	})
}
