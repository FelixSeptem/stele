package retrieval

import (
	"context"
	"errors"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (s *Service) applyContextCalibrationRollout(ctx context.Context, input AssembleContextInput, hits []SearchHit, policy *memory.RankingRolloutPolicy) ([]SearchHit, []ContextDiagnostic) {
	if !s.contextCalibrationEnabled {
		return hits, []ContextDiagnostic{{Section: "context_calibration", Status: "baseline_fallback", Reason: "calibration is disabled by deployment"}}
	}
	if policy == nil || policy.ContextCalibration == nil || s.contextCalibrationSummaryReader == nil {
		return hits, nil
	}
	if !s.contextCalibrationLimits.valid() || !contextCalibrationPolicyWithinLimits(*policy.ContextCalibration, s.contextCalibrationLimits) {
		return hits, []ContextDiagnostic{{Section: "context_calibration", Status: "baseline_fallback", Reason: "calibration policy exceeds deployment limits"}}
	}
	now := s.evaluationClock()
	resolution := memory.ResolveContextCalibrationRollout(policy, memory.ResolveContextCalibrationRolloutInput{
		Scope: input.Scope, Surface: memory.RankingRolloutSurfaceContext, SessionID: input.SessionID, UserID: input.UserID,
		SummaryVersion: policy.ContextCalibration.SummaryVersion, Now: now,
	})
	if resolution.Stage == memory.ContextCalibrationRolloutStageBaseline {
		return hits, []ContextDiagnostic{{Section: "context_calibration", Status: "baseline_fallback", Reason: "calibration policy is absent, expired, foreign, or incompatible"}}
	}
	summary, err := s.contextCalibrationSummaryReader.ReadContextCalibrationSummary(ctx, memory.ReadContextCalibrationSummaryInput{Scope: input.Scope, PolicyVersion: policy.ContextCalibration.PolicyVersion, SummaryVersion: policy.ContextCalibration.SummaryVersion, Now: now, MaxAge: s.contextCalibrationLimits.MaxSummaryAge})
	if err != nil {
		reason := "calibration summary unavailable"
		if !errors.Is(err, pgx.ErrNoRows) {
			reason = "calibration summary unavailable or stale"
		}
		return hits, []ContextDiagnostic{{Section: "context_calibration", Status: "baseline_fallback", Reason: reason}}
	}
	calibrated, diagnostics := ApplyContextCalibration(input.Scope, hits, summary, policy.ContextCalibration.MaxCandidates)
	result := make([]ContextDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		status := diagnostic.Status
		if resolution.Stage != memory.ContextCalibrationRolloutStageActive {
			status = "shadow"
			calibrated = hits
		}
		result = append(result, ContextDiagnostic{Section: "context_calibration", Status: status, Reason: diagnostic.Reason})
	}
	return calibrated, result
}

func contextCalibrationPolicyWithinLimits(policy memory.ContextCalibrationRolloutPolicy, limits ContextCalibrationLimits) bool {
	return policy.MinimumEvidence >= limits.MinimumEvidence && policy.ConfidenceThreshold >= limits.ConfidenceThreshold &&
		policy.DecayWindow <= limits.DecayWindow && policy.ContributionCap <= limits.ContributionCap &&
		policy.MaxCandidates <= limits.MaxCandidates && policy.MaxContextItems <= limits.MaxContextItems && policy.MaxElapsed <= limits.MaxElapsed
}

func filterCalibratedHits(hits []SearchHit, rank map[string]int) []SearchHit {
	filtered := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		if _, ok := rank[hit.Memory.ID]; ok {
			filtered = append(filtered, hit)
		}
	}
	return filtered
}
