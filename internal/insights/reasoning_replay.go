package insights

import (
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

// PlanReasoningCandidateReplay evaluates one normalized reasoning envelope
// against a policy in shadow mode. It returns the same bounded replay report
// shape as durable insight replay while leaving all stores untouched.
func PlanReasoningCandidateReplay(envelope reasoning.InsightReplayEnvelope, policy ReservedInsightActivationPolicy, authorizedEvidence []memory.DerivedInsightEvidenceRef, now time.Time) (memory.DerivedInsightReplayReport, error) {
	result, err := reasoning.ReplayInsightCandidate(envelope)
	if err != nil {
		return memory.DerivedInsightReplayReport{}, err
	}
	report := memory.DerivedInsightReplayReport{
		RunID:       envelope.ReplayID,
		Scope:       envelope.Request.Scope,
		GeneratedAt: now,
		Counters:    memory.DerivedInsightReplayCounters{EvidenceEvaluated: len(envelope.Request.Evidence)},
	}
	if now.IsZero() {
		report.GeneratedAt = time.Now().UTC()
	}
	if result.Disposition == reasoning.InsightDispositionQuarantined {
		report.Counters.Quarantined++
		report.Decisions = append(report.Decisions, memory.DerivedInsightReplayDecision{
			InsightType: envelope.Request.InsightType, Fingerprint: envelope.ReplayID,
			Decision: memory.DerivedInsightReplayDecisionQuarantine,
			Reason:   memory.DerivedInsightReplayReasonActivationQuarantined, Message: result.Reason,
		})
		return report, report.Validate()
	}
	activation := AdmitReasoningCandidate(envelope.Candidate, policy, authorizedEvidence, ActivationInput{Policy: policy, Shadow: true, PolicyVersion: envelope.Request.PolicyVersion, SourceWatermark: envelope.Request.SourceWatermark, CandidateFingerprint: envelope.ReplayID, Now: report.GeneratedAt})
	decision := memory.DerivedInsightReplayDecision{InsightID: envelope.Candidate.ID, InsightType: envelope.Candidate.InsightType, Fingerprint: envelope.ReplayID, EvidenceCount: len(envelope.Candidate.Evidence), Reason: memory.DerivedInsightReplayReasonActivationQuarantined, Message: activation.Reason}
	switch activation.Disposition {
	case ActivationDispositionWouldActivate:
		decision.Decision = memory.DerivedInsightReplayDecisionWouldActivate
		decision.Reason = memory.DerivedInsightReplayReasonActivationWouldApply
		report.Counters.WouldActivate++
	case ActivationDispositionStale:
		decision.Decision = memory.DerivedInsightReplayDecisionStale
		decision.Reason = memory.DerivedInsightReplayReasonActivationPolicyStale
		report.Counters.Stale++
	case ActivationDispositionTypeDisabled, ActivationDispositionRejected:
		decision.Decision = memory.DerivedInsightReplayDecisionReject
		report.Counters.Skipped++
	default:
		decision.Decision = memory.DerivedInsightReplayDecisionQuarantine
		report.Counters.Quarantined++
	}
	report.Decisions = append(report.Decisions, decision)
	if err := report.Validate(); err != nil {
		return memory.DerivedInsightReplayReport{}, fmt.Errorf("validate reasoning replay report: %w", err)
	}
	return report, nil
}
