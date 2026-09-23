package postgres

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ContextCalibrationSummaryStore interface {
	ReadContextCalibrationSummary(context.Context, memory.ReadContextCalibrationSummaryInput) (memory.ContextCalibrationSummary, error)
	UpsertContextCalibrationSummary(context.Context, memory.ContextCalibrationSummary) (memory.ContextCalibrationSummary, error)
}

func (r *Repository) ReadContextCalibrationSummary(ctx context.Context, input memory.ReadContextCalibrationSummaryInput) (memory.ContextCalibrationSummary, error) {
	if err := input.Validate(); err != nil {
		return memory.ContextCalibrationSummary{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, policy_version, summary_version, source_watermark,
       evidence_count, confidence_sum, priority_sum, freshness, created_at, updated_at
FROM context_calibration_summaries
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND policy_version = $4 AND summary_version = $5
  AND updated_at >= $6
ORDER BY updated_at DESC
LIMIT 1`
	var summary memory.ContextCalibrationSummary
	err := r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyVersion, input.SummaryVersion, input.Now.Add(-input.MaxAge)).Scan(
		&summary.ID, &summary.Scope.Tenant, &summary.Scope.Project, &summary.Scope.Namespace, &summary.PolicyVersion, &summary.SummaryVersion, &summary.SourceWatermark,
		&summary.EvidenceCount, &summary.ConfidenceSum, &summary.PrioritySum, &summary.Freshness, &summary.CreatedAt, &summary.UpdatedAt)
	if err != nil {
		return memory.ContextCalibrationSummary{}, fmt.Errorf("read context calibration summary: %w", err)
	}
	if err := summary.Validate(); err != nil {
		return memory.ContextCalibrationSummary{}, err
	}
	if summary.Freshness != memory.ContextCalibrationSummaryFresh {
		return memory.ContextCalibrationSummary{}, fmt.Errorf("context calibration summary is not fresh")
	}
	return summary, nil
}

func (r *Repository) UpsertContextCalibrationSummary(ctx context.Context, summary memory.ContextCalibrationSummary) (memory.ContextCalibrationSummary, error) {
	if err := summary.Validate(); err != nil {
		return memory.ContextCalibrationSummary{}, err
	}
	const query = `
INSERT INTO context_calibration_summaries
 (id, tenant, project, namespace, policy_version, summary_version, source_watermark, evidence_count, confidence_sum, priority_sum, freshness, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (tenant, project, namespace, policy_version, summary_version)
DO UPDATE SET id=EXCLUDED.id, source_watermark=EXCLUDED.source_watermark, evidence_count=EXCLUDED.evidence_count,
 confidence_sum=EXCLUDED.confidence_sum, priority_sum=EXCLUDED.priority_sum, freshness=EXCLUDED.freshness,
 updated_at=EXCLUDED.updated_at
RETURNING id, tenant, project, namespace, policy_version, summary_version, source_watermark, evidence_count, confidence_sum, priority_sum, freshness, created_at, updated_at`
	var created memory.ContextCalibrationSummary
	err := r.db.QueryRow(ctx, query, summary.ID, summary.Scope.Tenant, summary.Scope.Project, summary.Scope.Namespace, summary.PolicyVersion, summary.SummaryVersion, summary.SourceWatermark, summary.EvidenceCount, summary.ConfidenceSum, summary.PrioritySum, summary.Freshness, summary.CreatedAt, summary.UpdatedAt).Scan(
		&created.ID, &created.Scope.Tenant, &created.Scope.Project, &created.Scope.Namespace, &created.PolicyVersion, &created.SummaryVersion, &created.SourceWatermark,
		&created.EvidenceCount, &created.ConfidenceSum, &created.PrioritySum, &created.Freshness, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return memory.ContextCalibrationSummary{}, fmt.Errorf("upsert context calibration summary: %w", err)
	}
	return created, nil
}
