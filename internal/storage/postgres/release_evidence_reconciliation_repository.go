package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListReleaseEvidenceReconciliationInputs(ctx context.Context, scope memory.Scope, limit int) ([]retrieval.ReleaseEvidenceReconciliationInput, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const query = `
SELECT policy_id, run_identity, scope_hash, policy_version, strategy_identity,
       source_watermark_hash, verdict, freshness, real_stack,
       deterministic_replay, rollback_tested, evaluated_at, expires_at
FROM ranking_rollout_evidence_attestations
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY evaluated_at DESC
LIMIT $4`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list release evidence reconciliation inputs: %w", err)
	}
	defer rows.Close()
	inputs := make([]retrieval.ReleaseEvidenceReconciliationInput, 0)
	for rows.Next() {
		var policyID, runID, scopeHash, policyVersion, strategy, watermark, verdict, freshness string
		var realStack, deterministic, rollback bool
		var evaluatedAt, expiresAt time.Time
		if err := rows.Scan(&policyID, &runID, &scopeHash, &policyVersion, &strategy, &watermark, &verdict, &freshness, &realStack, &deterministic, &rollback, &evaluatedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan release evidence reconciliation input: %w", err)
		}
		report := retrieval.ReleaseEvidenceReport{
			RunIdentity: runID, ProviderProfile: strategy, PolicyVersion: policyVersion,
			FixtureVersion: "unknown", ScopeHash: scopeHash, SourceWatermarkHash: watermark,
			EvidenceFreshness: retrieval.ReleaseEvidenceFreshness(freshness), EvidenceExpiresAt: expiresAt,
			Verdict: retrieval.ReleaseEvidenceVerdict(verdict), ReleaseEligible: verdict == string(retrieval.ReleaseEvidencePassed),
			RealStack: realStack, DeterministicReplay: deterministic, RollbackTested: rollback,
			GeneratedAt: evaluatedAt, OperationalOutcome: retrieval.ReleaseEvidenceOperationalOutcome{State: retrieval.ReleaseEvidenceRunCompleted, Cleanup: retrieval.ReleaseEvidenceCleanupComplete, Consumable: true},
		}
		rollbackVerdict := retrieval.ReleaseEvidenceRollbackUnknown
		if rollback {
			rollbackVerdict = retrieval.ReleaseEvidenceRollbackPassed
		}
		report.Attestation = &retrieval.ReleaseEvidenceAttestation{RunIdentity: runID, ScopeHash: scopeHash, SourceWatermarkHash: watermark, PolicyVersion: policyVersion, FixtureVersion: report.FixtureVersion, RollbackVerdict: rollbackVerdict, Freshness: retrieval.ReleaseEvidenceFreshness(freshness), IssuedAt: evaluatedAt, ExpiresAt: expiresAt}
		inputs = append(inputs, retrieval.ReleaseEvidenceReconciliationInput{PolicyID: policyID, ExpectedScopeHash: scopeHash, ExpectedPolicyVersion: policyVersion, ExpectedFixtureVersion: report.FixtureVersion, ExpectedStrategyIdentity: strategy, ExpectedSourceWatermarkHash: watermark, Report: report})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate release evidence reconciliation inputs: %w", err)
	}
	return inputs, nil
}

func (r *Repository) ApplyReleaseEvidenceReconciliation(ctx context.Context, scope memory.Scope, replayKey string, result retrieval.ReleaseEvidenceReconciliationResult) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(result.PolicyID) == "" || strings.TrimSpace(result.HandoffIdentity) == "" || strings.TrimSpace(replayKey) == "" {
		return fmt.Errorf("reconciliation policy, handoff, and replay identities are required")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin release evidence reconciliation transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	now := result.ObservedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	verdict := "revoked"
	if result.Eligible {
		verdict = "eligible"
	}
	historyID := uuid.New()
	const historyQuery = `
INSERT INTO release_evidence_reconciliation_history
 (id, tenant, project, namespace, policy_id, handoff_identity, replay_key,
  scope_hash, source_watermark_hash, policy_version, fixture_version,
  representation_identity, verdict, reason_category, observed_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (tenant, project, namespace, replay_key) DO NOTHING`
	tag, err := tx.Exec(ctx, historyQuery, historyID, scope.Tenant, scope.Project, scope.Namespace, result.PolicyID, result.HandoffIdentity, replayKey, result.ScopeHash, result.SourceWatermarkHash, result.PolicyVersion, result.FixtureVersion, result.RepresentationIdentity, verdict, result.Reason, now)
	if err != nil {
		return fmt.Errorf("record release evidence reconciliation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	const currentQuery = `
INSERT INTO release_evidence_reconciliation_current
 (tenant, project, namespace, policy_id, handoff_identity, eligible,
  reason_category, latest_history_id, observed_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
ON CONFLICT (tenant, project, namespace, policy_id, handoff_identity)
DO UPDATE SET eligible = EXCLUDED.eligible, reason_category = EXCLUDED.reason_category,
 latest_history_id = EXCLUDED.latest_history_id, observed_at = EXCLUDED.observed_at,
 updated_at = EXCLUDED.updated_at`
	if _, err := tx.Exec(ctx, currentQuery, scope.Tenant, scope.Project, scope.Namespace, result.PolicyID, result.HandoffIdentity, result.Eligible, result.Reason, historyID, now); err != nil {
		return fmt.Errorf("update current release evidence eligibility: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit release evidence reconciliation: %w", err)
	}
	return nil
}

func (r *Repository) ListReleaseEvidenceReconciliation(ctx context.Context, scope memory.Scope, limit int) ([]retrieval.ReconciliationInspection, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT policy_id, handoff_identity, eligible, reason_category, observed_at FROM release_evidence_reconciliation_current WHERE tenant=$1 AND project=$2 AND namespace=$3 ORDER BY observed_at DESC LIMIT $4`, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list release evidence reconciliation status: %w", err)
	}
	defer rows.Close()
	result := make([]retrieval.ReconciliationInspection, 0)
	for rows.Next() {
		var item retrieval.ReconciliationInspection
		if err := rows.Scan(&item.PolicyID, &item.HandoffIdentity, &item.Eligible, &item.Reason, &item.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan release evidence reconciliation status: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate release evidence reconciliation status: %w", err)
	}
	return result, nil
}

func (r *Repository) ReadReleaseEvidenceReconciliationEligibility(ctx context.Context, scope memory.Scope, policyID string) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	if strings.TrimSpace(policyID) == "" {
		return false, fmt.Errorf("policy id is required")
	}
	var eligible bool
	err := r.db.QueryRow(ctx, `SELECT eligible FROM release_evidence_reconciliation_current WHERE tenant=$1 AND project=$2 AND namespace=$3 AND policy_id=$4 ORDER BY observed_at DESC LIMIT 1`, scope.Tenant, scope.Project, scope.Namespace, policyID).Scan(&eligible)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("read release evidence reconciliation eligibility: %w", err)
	}
	return eligible, nil
}

func (r *Repository) TriggerReleaseEvidenceReconciliation(ctx context.Context, scope memory.Scope, trigger retrieval.ReconciliationTrigger) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	if err := trigger.Validate(); err != nil {
		return "", err
	}
	runID := uuid.New()
	replayKey := fmt.Sprintf("manual:%s:%s:%s", scope.Tenant, scope.Project, scope.Namespace)
	var storedID uuid.UUID
	err := r.db.QueryRow(ctx, `INSERT INTO release_evidence_reconciliation_runs (id, tenant, project, namespace, policy_id, replay_key, source_watermark_hash, status, actor, reason, started_at) VALUES ($1,$2,$3,$4,'manual',$5,'','pending',$6,$7,$8) ON CONFLICT (tenant, project, namespace, replay_key) DO UPDATE SET actor=EXCLUDED.actor, reason=EXCLUDED.reason RETURNING id`, runID, scope.Tenant, scope.Project, scope.Namespace, replayKey, trigger.Actor, trigger.Reason, time.Now().UTC()).Scan(&storedID)
	if err != nil {
		return "", fmt.Errorf("queue release evidence reconciliation: %w", err)
	}
	return "run:" + strings.ReplaceAll(storedID.String(), "-", ""), nil
}

func (r *Repository) CheckpointReleaseEvidenceReconciliation(ctx context.Context, scope memory.Scope, replayKey, checkpoint string, processed int) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(replayKey) == "" || strings.TrimSpace(checkpoint) == "" || processed < 0 {
		return fmt.Errorf("reconciliation checkpoint is invalid")
	}
	_, err := r.db.Exec(ctx, `UPDATE release_evidence_reconciliation_runs SET checkpoint=$5, processed_count=$6, status='running' WHERE tenant=$1 AND project=$2 AND namespace=$3 AND replay_key=$4`, scope.Tenant, scope.Project, scope.Namespace, replayKey, checkpoint, processed)
	if err != nil {
		return fmt.Errorf("checkpoint release evidence reconciliation: %w", err)
	}
	return nil
}
