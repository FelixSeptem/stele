package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	uncertainty, direct_activation, canonical_mutation, disposition, reason, created_at,
	contradiction_temporal_disposition, contradiction_review_state, contradiction_overlap_from, contradiction_overlap_to, review_reason
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
	$25, $26, $27, $28, $29)
ON CONFLICT (tenant, project, namespace, replay_id) DO NOTHING`
	_, err = r.db.Exec(ctx, query,
		candidate.ID, candidate.Scope.Tenant, candidate.Scope.Project, candidate.Scope.Namespace,
		candidate.InsightType, candidate.Mode, candidate.Title, candidate.Summary, evidence,
		candidate.EvidenceDigest, candidate.SourceWatermark, candidate.ScopeProof,
		candidate.LifecycleVisibility, candidate.RedactionPolicy, candidate.ProviderVersion, candidate.SchemaVersion, candidate.PolicyVersion,
		candidate.ReplayID, candidate.Uncertainty, candidate.DirectActivation,
		candidate.CanonicalMutation, disposition, strings.TrimSpace(reason), candidate.CreatedAt,
		metadataStringOrNil(candidate.Metadata, "contradiction_temporal_disposition"), metadataStringOrDefault(candidate.Metadata, "contradiction_review_state", "review_required"),
		metadataTime(candidate.Metadata, "contradiction_overlap_from"), metadataTime(candidate.Metadata, "contradiction_overlap_to"), strings.TrimSpace(reason),
	)
	if err != nil {
		return fmt.Errorf("persist reasoning insight candidate: %w", err)
	}
	return nil
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}
func metadataStringOrNil(metadata map[string]any, key string) any {
	value := metadataString(metadata, key)
	if value == "" {
		return nil
	}
	return value
}
func metadataStringOrDefault(metadata map[string]any, key, fallback string) string {
	if value := metadataString(metadata, key); value != "" {
		return value
	}
	return fallback
}
func metadataTime(metadata map[string]any, key string) *time.Time {
	value := metadataString(metadata, key)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
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
