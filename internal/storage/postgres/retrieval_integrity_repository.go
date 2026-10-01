package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/evaluation"
	"github.com/FelixSeptem/stele/internal/memory"
)

// CreateRetrievalIntegrityReport persists only bounded derived evidence. The
// scope columns are used for authorization predicates and never returned by
// the redacted report readers.
func (r *Repository) CreateRetrievalIntegrityReport(ctx context.Context, report evaluation.IntegrityReport, expiresAt *time.Time) error {
	if err := report.Scope.Validate(); err != nil {
		return err
	}
	if report.Fingerprint == "" || report.Identity.Validate() != nil {
		return fmt.Errorf("integrity report is not normalized")
	}
	aggregate, err := json.Marshal(report.Redacted())
	if err != nil {
		return fmt.Errorf("marshal integrity report: %w", err)
	}
	const query = `
INSERT INTO retrieval_integrity_reports (
 id, tenant, project, namespace, fixture_version, policy_version, strategy_version,
 renderer_version, provider_version, source_watermark, verdict, action_category,
 action_success, integrity_success, aggregate, created_at, expires_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
ON CONFLICT (id) DO NOTHING
`
	_, err = r.db.Exec(ctx, query, report.ID, report.Scope.Tenant, report.Scope.Project,
		report.Scope.Namespace, report.Identity.FixtureVersion, report.Identity.PolicyVersion,
		report.Identity.Strategy, report.Identity.Renderer, report.Identity.Provider,
		report.Identity.SourceWatermark, report.Verdict, report.Action, report.ActionSuccess,
		report.IntegritySuccess, aggregate, report.CreatedAt, nullableTimeValue(expiresAt))
	if err != nil {
		return fmt.Errorf("create retrieval integrity report: %w", err)
	}
	return nil
}

func (r *Repository) CreateRetrievalIntegrityTrajectory(ctx context.Context, scope memory.Scope, trajectory evaluation.TrajectoryAggregate, reportID string, expiresAt *time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if trajectory.ScopeHash == "" || reportID == "" {
		return fmt.Errorf("trajectory scope and report are required")
	}
	if err := evaluation.AuthorizeReportScope(scope, trajectory.ScopeHash); err != nil {
		return fmt.Errorf("trajectory scope: %w", err)
	}
	if err := trajectory.Identity.Validate(); err != nil {
		return fmt.Errorf("trajectory identity: %w", err)
	}
	aggregate, err := json.Marshal(trajectory)
	if err != nil {
		return fmt.Errorf("marshal retrieval trajectory: %w", err)
	}
	const query = `
INSERT INTO retrieval_integrity_trajectories (
 id, report_id, tenant, project, namespace, fixture_version, policy_version,
 strategy_version, renderer_version, provider_version, source_watermark,
 aggregate, created_at, expires_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (id) DO NOTHING
`
	_, err = r.db.Exec(ctx, query, retrievalIntegrityTrajectoryID(reportID, trajectory),
		reportID, scope.Tenant, scope.Project, scope.Namespace, trajectory.Identity.FixtureVersion,
		trajectory.Identity.PolicyVersion, trajectory.Identity.Strategy, trajectory.Identity.Renderer,
		trajectory.Identity.Provider, trajectory.Identity.SourceWatermark, aggregate, time.Now().UTC(), nullableTimeValue(expiresAt))
	if err != nil {
		return fmt.Errorf("create retrieval integrity trajectory: %w", err)
	}
	return nil
}

// retrievalIntegrityTrajectoryID is deterministic for one report and complete
// aggregate cell. It never stores report or scope identifiers in the ID.
func retrievalIntegrityTrajectoryID(reportID string, trajectory evaluation.TrajectoryAggregate) string {
	parts := []string{
		reportID, trajectory.ScopeHash,
		trajectory.Identity.FixtureVersion, trajectory.Identity.PolicyVersion, trajectory.Identity.Strategy,
		trajectory.Identity.Renderer, trajectory.Identity.Provider, trajectory.Identity.SourceWatermark,
		trajectory.Channel, trajectory.CandidateBucket, trajectory.ExpansionBucket, trajectory.Disposition,
		trajectory.Fallback, trajectory.Freshness, trajectory.BudgetBucket, trajectory.LatencyBucket,
		fmt.Sprintf("%d", trajectory.Count),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("trajectory:%x", sum[:])
}

func (r *Repository) ListRetrievalIntegrityReports(ctx context.Context, scope memory.Scope, limit int) ([]evaluation.RedactedIntegrityReport, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	const query = `
SELECT aggregate
FROM retrieval_integrity_reports
WHERE tenant=$1 AND project=$2 AND namespace=$3
ORDER BY created_at DESC, id DESC
LIMIT $4
`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list retrieval integrity reports: %w", err)
	}
	defer rows.Close()
	reports := make([]evaluation.RedactedIntegrityReport, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scan retrieval integrity report: %w", err)
		}
		var report evaluation.RedactedIntegrityReport
		if err := json.Unmarshal(payload, &report); err != nil {
			return nil, fmt.Errorf("unmarshal retrieval integrity report: %w", err)
		}
		if err := evaluation.AuthorizeReportScope(scope, report.ScopeHash); err != nil {
			return nil, fmt.Errorf("retrieval integrity report scope: %w", err)
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retrieval integrity reports: %w", err)
	}
	return reports, nil
}

// CreateRetrievalIntegrityRetentionOutcome appends a bounded audit outcome for
// derived-evidence cleanup. Canonical source records are never represented.
func (r *Repository) CreateRetrievalIntegrityRetentionOutcome(ctx context.Context, scope memory.Scope, outcome evaluation.RetentionOutcome) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	normalized, err := evaluation.BuildRetentionOutcome(evaluation.RetentionInput{
		ArtifactCategory: outcome.ArtifactCategory,
		Result:           outcome.Result,
		DeletedCount:     outcome.DeletedCount,
		ReasonCategory:   outcome.ReasonCategory,
		At:               outcome.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("retention outcome is not normalized: %w", err)
	}
	outcome = normalized
	const query = `
INSERT INTO retrieval_integrity_retention_outcomes (
 id, tenant, project, namespace, artifact_category, result, deleted_count, reason_category, created_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (id) DO NOTHING
`
	_, err = r.db.Exec(ctx, query, retrievalIntegrityRetentionOutcomeID(scope, outcome), scope.Tenant, scope.Project,
		scope.Namespace, outcome.ArtifactCategory, outcome.Result, outcome.DeletedCount, outcome.ReasonCategory,
		outcome.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("create retrieval integrity retention outcome: %w", err)
	}
	return nil
}

func retrievalIntegrityRetentionOutcomeID(scope memory.Scope, outcome evaluation.RetentionOutcome) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		scope.Tenant, scope.Project, scope.Namespace, outcome.ArtifactCategory, outcome.Result,
		fmt.Sprintf("%d", outcome.DeletedCount), outcome.ReasonCategory, outcome.CreatedAt.UTC().Format(time.RFC3339Nano),
	}, "\x00")))
	return fmt.Sprintf("retention:%x", sum[:])
}

func (r *Repository) CleanupRetrievalIntegrityEvidence(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	var deleted int64
	const query = `
WITH expired_candidates AS (
 SELECT artifact_type, id
 FROM (
  SELECT 'trajectory' AS artifact_type, id, expires_at
  FROM retrieval_integrity_trajectories
  WHERE expires_at IS NOT NULL AND expires_at <= $1
  UNION ALL
  SELECT 'integrity_report' AS artifact_type, report.id, report.expires_at
  FROM retrieval_integrity_reports AS report
  WHERE report.expires_at IS NOT NULL AND report.expires_at <= $1
   AND NOT EXISTS (
    SELECT 1 FROM retrieval_integrity_trajectories AS trajectory
    WHERE trajectory.report_id = report.id
   )
 ) AS expired
 ORDER BY expires_at ASC, artifact_type ASC, id ASC
 LIMIT $2
), expired_trajectories AS (
 DELETE FROM retrieval_integrity_trajectories
 WHERE id IN (SELECT id FROM expired_candidates WHERE artifact_type = 'trajectory')
 RETURNING id
), expired_reports AS (
 DELETE FROM retrieval_integrity_reports
 WHERE id IN (SELECT id FROM expired_candidates WHERE artifact_type = 'integrity_report')
 RETURNING id
)
SELECT (SELECT COUNT(*) FROM expired_trajectories)+(SELECT COUNT(*) FROM expired_reports)
`
	if err := r.db.QueryRow(ctx, query, now.UTC(), limit).Scan(&deleted); err != nil {
		return 0, fmt.Errorf("cleanup retrieval integrity evidence: %w", err)
	}
	return deleted, nil
}

func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
