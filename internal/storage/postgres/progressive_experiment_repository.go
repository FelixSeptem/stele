package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func (r *Repository) CreateProgressiveExperimentReport(ctx context.Context, scope memory.Scope, report retrieval.ProgressiveExperimentReport, expiresAt *time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if report.ScopeHash != progressiveExperimentScopeHash(scope) {
		return fmt.Errorf("progressive experiment report scope mismatch")
	}
	aggregate, err := retrieval.MarshalProgressiveExperimentReport(report)
	if err != nil {
		return fmt.Errorf("marshal progressive experiment report: %w", err)
	}
	const query = `
INSERT INTO progressive_experiment_reports (
 run_identity, tenant, project, namespace, policy_identity, baseline_identity,
 strategy_identity, mode, verdict, eligible, fallback_category, rollback_required,
 source_watermark_hash, aggregate, expires_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (run_identity) DO NOTHING`
	_, err = r.db.Exec(ctx, query, report.RunIdentity, scope.Tenant, scope.Project, scope.Namespace,
		report.PolicyIdentity, report.BaselineIdentity, report.StrategyIdentity, report.Mode,
		report.Verdict, report.Eligible, report.FallbackCategory, report.RollbackRequired, "", aggregate, nullableTimeValue(expiresAt))
	if err != nil {
		return fmt.Errorf("create progressive experiment report: %w", err)
	}
	return nil
}

func (r *Repository) ListProgressiveExperimentReports(ctx context.Context, scope memory.Scope, limit int) ([]retrieval.ProgressiveExperimentReport, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `SELECT aggregate FROM progressive_experiment_reports WHERE tenant=$1 AND project=$2 AND namespace=$3 ORDER BY created_at DESC, run_identity DESC LIMIT $4`, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list progressive experiment reports: %w", err)
	}
	defer rows.Close()
	reports := make([]retrieval.ProgressiveExperimentReport, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scan progressive experiment report: %w", err)
		}
		var report retrieval.ProgressiveExperimentReport
		if err := json.Unmarshal(payload, &report); err != nil {
			return nil, fmt.Errorf("unmarshal progressive experiment report: %w", err)
		}
		if report.ScopeHash != progressiveExperimentScopeHash(scope) {
			return nil, fmt.Errorf("progressive experiment report scope mismatch")
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progressive experiment reports: %w", err)
	}
	return reports, nil
}

func (r *Repository) CleanupProgressiveExperimentReports(ctx context.Context, scope memory.Scope, now time.Time, limit int) (int64, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const query = `
WITH expired AS (
 SELECT run_identity FROM progressive_experiment_reports
 WHERE tenant=$1 AND project=$2 AND namespace=$3 AND expires_at IS NOT NULL AND expires_at <= $4
 ORDER BY expires_at ASC, run_identity ASC LIMIT $5
), deleted AS (
 DELETE FROM progressive_experiment_reports WHERE run_identity IN (SELECT run_identity FROM expired)
 RETURNING run_identity
)
SELECT COUNT(*) FROM deleted`
	var deleted int64
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, now.UTC(), limit).Scan(&deleted); err != nil {
		return 0, fmt.Errorf("cleanup progressive experiment reports: %w", err)
	}
	return deleted, nil
}

func progressiveExperimentScopeHash(scope memory.Scope) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{scope.Tenant, scope.Project, scope.Namespace}, "\x00")))
	return fmt.Sprintf("scope:%x", sum[:])
}
