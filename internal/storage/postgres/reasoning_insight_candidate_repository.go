package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/reasoning"
)

// PersistReasoningInsightCandidate stores only the normalized, non-authoritative
// envelope. Active insight admission remains owned by the reserved activation
// service and never happens as a side effect of this write.
func (r *Repository) PersistReasoningInsightCandidate(ctx context.Context, candidate reasoning.InsightCandidate, disposition reasoning.InsightDisposition, reason string) error {
	if err := candidate.Validate(reasoning.DefaultLimits()); err != nil {
		return err
	}
	if !validReasoningDisposition(disposition) {
		return fmt.Errorf("reasoning candidate disposition %q is invalid", disposition)
	}
	evidence, err := json.Marshal(candidate.Evidence)
	if err != nil {
		return fmt.Errorf("marshal reasoning candidate evidence: %w", err)
	}
	const query = `
INSERT INTO governed_reasoning_insight_candidates (
    id, tenant, project, namespace, insight_type, mode, title, summary,
    evidence, evidence_digest, source_watermark, scope_proof, lifecycle_visibility,
    redaction_policy, provider_version, schema_version, policy_version, replay_id,
    uncertainty, direct_activation, canonical_mutation, disposition, reason, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
ON CONFLICT (tenant, project, namespace, replay_id) DO NOTHING`
	_, err = r.db.Exec(ctx, query,
		candidate.ID, candidate.Scope.Tenant, candidate.Scope.Project, candidate.Scope.Namespace,
		candidate.InsightType, candidate.Mode, candidate.Title, candidate.Summary, evidence,
		candidate.EvidenceDigest, candidate.SourceWatermark, candidate.ScopeProof,
		candidate.LifecycleVisibility, candidate.RedactionPolicy, candidate.ProviderVersion, candidate.SchemaVersion, candidate.PolicyVersion,
		candidate.ReplayID, candidate.Uncertainty, candidate.DirectActivation,
		candidate.CanonicalMutation, disposition, strings.TrimSpace(reason), candidate.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("persist reasoning insight candidate: %w", err)
	}
	return nil
}

func validReasoningDisposition(disposition reasoning.InsightDisposition) bool {
	switch disposition {
	case reasoning.InsightDispositionCandidate,
		reasoning.InsightDispositionRejected,
		reasoning.InsightDispositionQuarantined,
		reasoning.InsightDispositionWouldActivate,
		reasoning.InsightDispositionStale,
		reasoning.InsightDispositionFallback:
		return true
	default:
		return false
	}
}
