package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// CreateRepresentationEvaluationReport stores one redacted append-only EV3
// aggregate. Canonical memory and public retrieval state are not touched.
func (r *Repository) CreateRepresentationEvaluationReport(ctx context.Context, scope memory.Scope, report retrieval.RepresentationEvaluationReport, expiresAt *time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if report.ScopeHash != representationExperimentScopeHash(scope) {
		return fmt.Errorf("representation evaluation report scope mismatch")
	}
	aggregate, err := retrieval.MarshalRepresentationEvaluationReport(report)
	if err != nil {
		return fmt.Errorf("marshal representation evaluation report: %w", err)
	}
	const query = `
INSERT INTO representation_evaluation_reports (
 run_identity, tenant, project, namespace, policy_identity, fixture_identity,
 baseline_identity, candidate_identity, mode, verdict, comparable,
 rollback_required, aggregate, expires_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (run_identity) DO NOTHING`
	_, err = r.db.Exec(ctx, query, report.RunIdentity, scope.Tenant, scope.Project, scope.Namespace,
		report.PolicyIdentity, report.FixtureIdentity, report.Baseline, report.Candidate,
		report.Mode, report.Verdict, report.Comparable, report.RollbackRequired, aggregate, nullableTimeValue(expiresAt))
	if err != nil {
		return fmt.Errorf("create representation evaluation report: %w", err)
	}
	return nil
}

func (r *Repository) ListRepresentationEvaluationReports(ctx context.Context, scope memory.Scope, limit int) ([]retrieval.RepresentationEvaluationReport, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `SELECT aggregate, expires_at FROM representation_evaluation_reports WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP) ORDER BY created_at DESC, run_identity DESC LIMIT $4`, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list representation evaluation reports: %w", err)
	}
	defer rows.Close()
	reports := make([]retrieval.RepresentationEvaluationReport, 0)
	for rows.Next() {
		var payload []byte
		var expiresAt sql.NullTime
		if err := rows.Scan(&payload, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan representation evaluation report: %w", err)
		}
		var report retrieval.RepresentationEvaluationReport
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&report); err != nil {
			return nil, fmt.Errorf("unmarshal representation evaluation report: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("representation evaluation report must contain exactly one JSON value")
		}
		if report.ScopeHash != representationExperimentScopeHash(scope) {
			return nil, fmt.Errorf("representation evaluation report scope mismatch")
		}
		if _, err := retrieval.MarshalRepresentationEvaluationReport(report); err != nil {
			return nil, fmt.Errorf("validate representation evaluation report: %w", err)
		}
		if expiresAt.Valid {
			expiry := expiresAt.Time.UTC()
			report.PersistenceExpiresAt = &expiry
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate representation evaluation reports: %w", err)
	}
	return reports, nil
}

func (r *Repository) CleanupRepresentationEvaluationReports(ctx context.Context, scope memory.Scope, now time.Time, limit int) (int64, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const query = `
WITH expired AS (
 SELECT run_identity FROM representation_evaluation_reports
 WHERE tenant=$1 AND project=$2 AND namespace=$3 AND expires_at IS NOT NULL AND expires_at <= $4
 ORDER BY expires_at ASC, run_identity ASC LIMIT $5
), deleted AS (
 DELETE FROM representation_evaluation_reports WHERE run_identity IN (SELECT run_identity FROM expired)
 RETURNING run_identity
)
SELECT COUNT(*) FROM deleted`
	var deleted int64
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, now.UTC(), limit).Scan(&deleted); err != nil {
		return 0, fmt.Errorf("cleanup representation evaluation reports: %w", err)
	}
	return deleted, nil
}

func representationExperimentScopeHash(scope memory.Scope) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{scope.Tenant, scope.Project, scope.Namespace}, "\x00")))
	return fmt.Sprintf("scope:%x", sum[:])
}
