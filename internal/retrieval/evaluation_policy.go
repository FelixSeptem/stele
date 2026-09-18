package retrieval

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/telemetry"
)

const (
	evaluationDecisionSafetyFailure        = "safety_failure"
	evaluationDecisionProtectedRecall      = "protected_recall_regression"
	evaluationDecisionProtectedMultiHop    = "protected_multihop_regression"
	evaluationDecisionProtectedCoverage    = "protected_evidence_coverage_regression"
	evaluationDecisionBudgetRegression     = "budget_omission_regression"
	evaluationDecisionLatencyExceeded      = "latency_budget_exceeded"
	evaluationDecisionLatencyRegression    = "latency_regression"
	evaluationDecisionPlannerCompatibility = "planner_compatibility_failure"
	evaluationDecisionPlannerSafety        = "planner_safety_failure"
	evaluationDecisionPlannerResource      = "planner_resource_failure"
	evaluationDecisionPlannerFallback      = "planner_fallback_failure"
	evaluationDecisionPlannerReranker      = "planner_reranker_failure"
	evaluationDecisionPlannerRollback      = "planner_rollback_failure"
	evaluationDecisionProtectedFamily      = "protected_family_regression"
	evaluationDecisionProtectedCategory    = "protected_category_regression"
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
	protectedCategories := append([]string(nil), policy.ProtectedCategories...)
	for _, item := range candidate.Cases {
		if item.PlannerProtected && item.Category != "" && !containsEvaluationString(protectedCategories, item.Category) {
			protectedCategories = append(protectedCategories, item.Category)
		}
	}
	comparison, err := CompareEvaluationReports(baseline, candidate, protectedCategories)
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
	if candidate.Metrics.EvidenceCoverage < baseline.Metrics.EvidenceCoverage-policy.MaxEvidenceCoverageRegression {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionProtectedCoverage)
	}
	if candidate.Metrics.BudgetOmissionRate > baseline.Metrics.BudgetOmissionRate+policy.MaxBudgetOmissionIncrease {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionBudgetRegression)
	}
	if policy.MaxP95LatencyRegressionMS > 0 && candidate.Metrics.P95LatencyMS > baseline.Metrics.P95LatencyMS+float64(policy.MaxP95LatencyRegressionMS) {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionLatencyRegression)
	}
	if candidate.Metrics.P95LatencyMS > float64(policy.MaxP95LatencyMS) {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionLatencyExceeded)
	}
	plannerPolicyEnabled := policy.Planner.enabled()
	if plannerPolicyEnabled && !candidate.PlannerEvidence.Compatible {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerCompatibility)
	}
	if plannerPolicyEnabled && candidate.PlannerEvidence.SafetyFailures > 0 {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerSafety)
	}
	if plannerPolicyEnabled && (candidate.PlannerEvidence.ResourceFailure || (policy.Planner.MaxPasses > 0 && candidate.PlannerEvidence.MaxPasses > policy.Planner.MaxPasses) || (policy.Planner.MaxCandidateCount > 0 && candidate.PlannerEvidence.MaxCandidateCount > policy.Planner.MaxCandidateCount)) {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerResource)
	}
	if plannerPolicyEnabled && candidate.PlannerEvidence.FallbackRate > policy.Planner.MaxFallbackRate {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerFallback)
	}
	if plannerPolicyEnabled && !candidate.PlannerEvidence.RerankerSafe {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerReranker)
	}
	if policy.Planner.RequireRollbackTested && !candidate.PlannerEvidence.RollbackTested {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionPlannerRollback)
	}
	if len(comparison.ProtectedRegressions) > 0 {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionProtectedCategory)
		decision.Advisories = append(decision.Advisories, "protected_category_regression_observed")
	}
	protectedFamilies := append([]RetrievalQueryFamily(nil), policy.ProtectedFamilies...)
	for _, item := range candidate.Cases {
		if item.PlannerProtected && !containsRetrievalQueryFamily(protectedFamilies, item.QueryFamily) {
			protectedFamilies = append(protectedFamilies, item.QueryFamily)
		}
	}
	if len(evaluationProtectedFamilyRegressions(baseline.Cases, candidate.Cases, protectedFamilies, policy.MaxRecallRegression, policy.MaxMultiHopCoverageRegression)) > 0 {
		decision.Eligible = false
		decision.HardFailures = append(decision.HardFailures, evaluationDecisionProtectedFamily)
	}
	return decision, nil
}

func containsRetrievalQueryFamily(families []RetrievalQueryFamily, want RetrievalQueryFamily) bool {
	for _, family := range families {
		if family == want {
			return true
		}
	}
	return false
}

func containsEvaluationString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p EvaluationPlannerReleasePolicy) enabled() bool {
	return p.RequireCompatible || p.MaxPasses > 0 || p.MaxCandidateCount > 0 || p.MaxFallbackRate > 0 || p.RequireRollbackTested
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
