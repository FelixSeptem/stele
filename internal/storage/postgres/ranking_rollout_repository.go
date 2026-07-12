package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateRankingRolloutPolicy(ctx context.Context, policy memory.RankingRolloutPolicy) (memory.RankingRolloutPolicy, error) {
	if err := policy.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("begin ranking rollout policy transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
INSERT INTO ranking_rollout_policies (
	id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
`
	created, err := scanRankingRolloutPolicy(tx.QueryRow(
		ctx,
		query,
		policy.ID,
		policy.Scope.Tenant,
		policy.Scope.Project,
		policy.Scope.Namespace,
		policy.Status,
		policy.Mode,
		rankingRolloutSurfaceStrings(policy.Surfaces),
		rankingRolloutSignalSourceStrings(policy.SignalSources),
		policy.ThresholdStatus,
		policy.EvidenceMinimum,
		policy.Actor,
		policy.Reason,
		nullableString(policy.LatestDryRunID),
		nullableString(string(policy.LatestDryRunStatus)),
		nullableTime(policy.ActivatedAt),
		nullableTime(policy.DisabledAt),
		nullableTime(policy.RolledBackAt),
		policy.CreatedAt,
		policy.UpdatedAt,
	))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("create ranking rollout policy: %w", err)
	}

	if err := upsertRankingRolloutPolicyState(ctx, tx, created, created.Status, created.Actor, created.Reason, created.UpdatedAt); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout policy transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) ReadRankingRolloutPolicy(ctx context.Context, input memory.ReadRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	policy, err := scanRankingRolloutPolicy(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read ranking rollout policy: %w", err)
	}
	return policy, nil
}

func (r *Repository) ReadActiveRankingRolloutPolicy(ctx context.Context, input memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND status = $4
	AND $5 = ANY(surfaces)
ORDER BY activated_at DESC NULLS LAST, updated_at DESC, created_at DESC, id DESC
LIMIT 1
`
	policy, err := scanRankingRolloutPolicy(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, string(input.Surface)))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read active ranking rollout policy: %w", err)
	}
	return policy, nil
}

func (r *Repository) ListRankingRolloutPolicies(ctx context.Context, input memory.ListRankingRolloutPoliciesInput) ([]memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3
ORDER BY created_at DESC, id DESC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list ranking rollout policies: %w", err)
	}
	defer rows.Close()

	policies := make([]memory.RankingRolloutPolicy, 0)
	for rows.Next() {
		policy, err := scanRankingRolloutPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ranking rollout policies: %w", err)
	}
	return policies, nil
}

func (r *Repository) RecordRankingRolloutDryRun(ctx context.Context, input memory.RecordRankingRolloutDryRunInput) (memory.RankingRolloutDryRun, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutDryRun{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("begin ranking rollout dry-run transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	policy, err := readRankingRolloutPolicyForUpdate(ctx, tx, input.Scope, input.PolicyID)
	if err != nil {
		return memory.RankingRolloutDryRun{}, err
	}
	if !rankingRolloutPolicyIncludesSurface(policy, input.Surface) {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("ranking rollout dry-run surface %q is not configured on policy", input.Surface)
	}
	if !rankingRolloutPolicyIncludesSignalSource(policy, input.SignalSource) {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("ranking rollout dry-run signal source %q is not configured on policy", input.SignalSource)
	}

	impact := append([]memory.RankingRolloutImpactEntry(nil), input.ImpactEntries...)
	if len(impact) == 0 && input.EvidenceCount == 0 && len(input.ChangedSubjectIDs) == 0 && len(input.ReasonCodes) == 0 {
		impact, err = buildRankingRolloutDryRunImpact(ctx, tx, policy, input)
		if err != nil {
			return memory.RankingRolloutDryRun{}, err
		}
	}
	dryRun := summarizeRankingRolloutDryRun(policy, input, impact)
	dryRun.ID = uuid.NewString()
	dryRun.PolicyID = input.PolicyID
	dryRun.Scope = input.Scope.Normalized()
	dryRun.Surface = input.Surface
	dryRun.SignalSource = input.SignalSource
	dryRun.CreatedAt = input.CreatedAt

	const query = `
INSERT INTO ranking_rollout_dry_runs (
	id, policy_id, tenant, project, namespace, surface, signal_source, threshold_status,
	baseline_rank, adjusted_rank, changed_subject_ids, reason_codes, signal_categories, evidence_count, hidden_evidence_count, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, policy_id, tenant, project, namespace, surface, signal_source, threshold_status,
	baseline_rank, adjusted_rank, changed_subject_ids, reason_codes, signal_categories, evidence_count, hidden_evidence_count, created_at
`
	created, err := scanRankingRolloutDryRun(tx.QueryRow(
		ctx,
		query,
		dryRun.ID,
		dryRun.PolicyID,
		dryRun.Scope.Tenant,
		dryRun.Scope.Project,
		dryRun.Scope.Namespace,
		dryRun.Surface,
		dryRun.SignalSource,
		dryRun.ThresholdStatus,
		dryRun.BaselineRank,
		dryRun.AdjustedRank,
		dryRun.ChangedSubjectIDs,
		rankingRolloutReasonCodeStrings(dryRun.ReasonCodes),
		dryRun.SignalCategories,
		dryRun.EvidenceCount,
		dryRun.HiddenEvidenceCount,
		dryRun.CreatedAt,
	))
	if err != nil {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("record ranking rollout dry-run: %w", err)
	}
	for _, entry := range impact {
		entry.ID = uuid.NewString()
		entry.DryRunID = created.ID
		entry.PolicyID = created.PolicyID
		entry.Scope = created.Scope
		entry.Surface = created.Surface
		entry.SignalSource = created.SignalSource
		entry.CreatedAt = created.CreatedAt
		if len(entry.SignalCategories) == 0 {
			entry.SignalCategories = append([]string(nil), created.SignalCategories...)
		}
		if err := insertRankingRolloutImpactEntry(ctx, tx, entry); err != nil {
			return memory.RankingRolloutDryRun{}, err
		}
	}

	const updatePolicy = `
UPDATE ranking_rollout_policies
SET latest_dry_run_id = $5,
	latest_dry_run_status = $6,
	threshold_status = $6,
	updated_at = $7
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	if _, err := tx.Exec(ctx, updatePolicy, created.Scope.Tenant, created.Scope.Project, created.Scope.Namespace, created.PolicyID, created.ID, created.ThresholdStatus, created.CreatedAt); err != nil {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("update ranking rollout latest dry-run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("commit ranking rollout dry-run transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) ActivateRankingRolloutPolicy(ctx context.Context, input memory.ActivateRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	if !input.Gate.CanActivate() {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("ranking rollout activation gate not satisfied")
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("begin ranking rollout activation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
UPDATE ranking_rollout_policies
SET status = $5,
	actor = $6,
	reason = $7,
	activated_at = $8,
	updated_at = $8,
	latest_dry_run_status = $9,
	mode = $10
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
	AND latest_dry_run_id IS NOT NULL
	AND latest_dry_run_status = $9
	AND status NOT IN ($11, $12)
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
`
	policy, err := scanRankingRolloutPolicy(tx.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID, memory.RankingRolloutPolicyStatusActiveForScope, input.Actor, input.Reason, input.ActivatedAt, input.Gate.EvidenceThresholdStatus, memory.RankingRolloutModeActiveForScope, memory.RankingRolloutPolicyStatusDisabled, memory.RankingRolloutPolicyStatusRolledBack))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("activate ranking rollout policy: %w", err)
	}
	if err := upsertRankingRolloutPolicyState(ctx, tx, policy, policy.Status, input.Actor, input.Reason, input.ActivatedAt); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout activation transaction: %w", err)
	}
	return policy, nil
}

func (r *Repository) DisableRankingRolloutPolicy(ctx context.Context, input memory.DisableRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("begin ranking rollout disable transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
UPDATE ranking_rollout_policies
SET status = $5,
	actor = $6,
	reason = $7,
	disabled_at = $8,
	updated_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
`
	policy, err := scanRankingRolloutPolicy(tx.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID, memory.RankingRolloutPolicyStatusDisabled, input.Actor, input.Reason, input.DisabledAt))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("disable ranking rollout policy: %w", err)
	}
	if err := upsertRankingRolloutPolicyState(ctx, tx, policy, policy.Status, input.Actor, input.Reason, input.DisabledAt); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout disable transaction: %w", err)
	}
	return policy, nil
}

func (r *Repository) RollbackRankingRolloutPolicy(ctx context.Context, input memory.RollbackRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("begin ranking rollout rollback transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := readRankingRolloutPolicyForUpdate(ctx, tx, input.Scope, input.PolicyID)
	if err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	const query = `
UPDATE ranking_rollout_policies
SET status = $5,
	actor = $6,
	reason = $7,
	rolled_back_at = $8,
	updated_at = $8
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
`
	policy, err := scanRankingRolloutPolicy(tx.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID, memory.RankingRolloutPolicyStatusRolledBack, input.Actor, input.Reason, input.RolledBackAt))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("rollback ranking rollout policy: %w", err)
	}
	if err := upsertRankingRolloutPolicyState(ctx, tx, policy, policy.Status, input.Actor, input.Reason, input.RolledBackAt); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	const auditQuery = `
INSERT INTO ranking_rollout_rollback_audit (
	policy_id, tenant, project, namespace, from_status, to_status, actor, reason, rolled_back_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`
	if _, err := tx.Exec(ctx, auditQuery, input.PolicyID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, current.Status, memory.RankingRolloutPolicyStatusRolledBack, input.Actor, input.Reason, input.RolledBackAt); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("insert ranking rollout rollback audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout rollback transaction: %w", err)
	}
	return policy, nil
}

func (r *Repository) ListRankingRolloutPolicyImpact(ctx context.Context, input memory.ListRankingRolloutPolicyImpactInput) ([]memory.RankingRolloutImpactEntry, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	const query = `
SELECT id, dry_run_id, policy_id, tenant, project, namespace, surface, signal_source, signal_categories, subject_kind,
	subject_id, opaque_token, candidate_priority, included, budget_impact, baseline_rank, adjusted_rank, reason_code, evidence_count, hidden_evidence, created_at
FROM ranking_rollout_impact_entries
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND policy_id = $4
ORDER BY created_at ASC, id ASC
`
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("list ranking rollout policy impact: %w", err)
	}
	defer rows.Close()

	items := make([]memory.RankingRolloutImpactEntry, 0)
	for rows.Next() {
		item, err := scanRankingRolloutImpactEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ranking rollout policy impact: %w", err)
	}
	return items, nil
}

func scanRankingRolloutPolicy(scanner provenanceScanner) (memory.RankingRolloutPolicy, error) {
	var policy memory.RankingRolloutPolicy
	var surfaces []string
	var signalSources []string
	var latestDryRunID sql.NullString
	var latestDryRunStatus sql.NullString
	var activatedAt sql.NullTime
	var disabledAt sql.NullTime
	var rolledBackAt sql.NullTime
	if err := scanner.Scan(
		&policy.ID,
		&policy.Scope.Tenant,
		&policy.Scope.Project,
		&policy.Scope.Namespace,
		&policy.Status,
		&policy.Mode,
		&surfaces,
		&signalSources,
		&policy.ThresholdStatus,
		&policy.EvidenceMinimum,
		&policy.Actor,
		&policy.Reason,
		&latestDryRunID,
		&latestDryRunStatus,
		&activatedAt,
		&disabledAt,
		&rolledBackAt,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("scan ranking rollout policy: %w", err)
	}
	policy.Surfaces = rankingRolloutSurfaces(surfaces)
	policy.SignalSources = rankingRolloutSignalSources(signalSources)
	if latestDryRunID.Valid {
		policy.LatestDryRunID = latestDryRunID.String
	}
	if latestDryRunStatus.Valid {
		policy.LatestDryRunStatus = memory.RankingRolloutThresholdStatus(latestDryRunStatus.String)
	}
	if activatedAt.Valid {
		policy.ActivatedAt = activatedAt.Time
	}
	if disabledAt.Valid {
		policy.DisabledAt = disabledAt.Time
	}
	if rolledBackAt.Valid {
		policy.RolledBackAt = rolledBackAt.Time
	}
	return policy, nil
}

func scanRankingRolloutDryRun(scanner provenanceScanner) (memory.RankingRolloutDryRun, error) {
	var dryRun memory.RankingRolloutDryRun
	var changedSubjectIDs []string
	var reasonCodes []string
	var signalCategories []string
	if err := scanner.Scan(
		&dryRun.ID,
		&dryRun.PolicyID,
		&dryRun.Scope.Tenant,
		&dryRun.Scope.Project,
		&dryRun.Scope.Namespace,
		&dryRun.Surface,
		&dryRun.SignalSource,
		&dryRun.ThresholdStatus,
		&dryRun.BaselineRank,
		&dryRun.AdjustedRank,
		&changedSubjectIDs,
		&reasonCodes,
		&signalCategories,
		&dryRun.EvidenceCount,
		&dryRun.HiddenEvidenceCount,
		&dryRun.CreatedAt,
	); err != nil {
		return memory.RankingRolloutDryRun{}, fmt.Errorf("scan ranking rollout dry run: %w", err)
	}
	dryRun.ChangedSubjectIDs = changedSubjectIDs
	dryRun.ReasonCodes = rankingRolloutReasonCodes(reasonCodes)
	dryRun.SignalCategories = signalCategories
	return dryRun, nil
}

func scanRankingRolloutImpactEntry(scanner provenanceScanner) (memory.RankingRolloutImpactEntry, error) {
	var item memory.RankingRolloutImpactEntry
	var dryRunID sql.NullString
	var subjectID sql.NullString
	var opaqueToken sql.NullString
	var reasonCode sql.NullString
	var signalCategories []string
	if err := scanner.Scan(
		&item.ID,
		&dryRunID,
		&item.PolicyID,
		&item.Scope.Tenant,
		&item.Scope.Project,
		&item.Scope.Namespace,
		&item.Surface,
		&item.SignalSource,
		&signalCategories,
		&item.SubjectKind,
		&subjectID,
		&opaqueToken,
		&item.CandidatePriority,
		&item.Included,
		&item.BudgetImpact,
		&item.BaselineRank,
		&item.AdjustedRank,
		&reasonCode,
		&item.EvidenceCount,
		&item.HiddenEvidence,
		&item.CreatedAt,
	); err != nil {
		return memory.RankingRolloutImpactEntry{}, fmt.Errorf("scan ranking rollout impact entry: %w", err)
	}
	if dryRunID.Valid {
		item.DryRunID = dryRunID.String
	}
	if subjectID.Valid {
		item.SubjectID = subjectID.String
	}
	if opaqueToken.Valid {
		item.OpaqueToken = opaqueToken.String
	}
	if reasonCode.Valid {
		item.ReasonCode = memory.RankingRolloutImpactReasonCode(reasonCode.String)
	}
	item.SignalCategories = signalCategories
	return item, nil
}

type rankingRolloutSignalCandidate struct {
	SubjectKind         string
	SubjectID           string
	OpaqueToken         string
	EvidenceCount       int
	HiddenEvidenceCount int
	PositiveSignals     int
	NegativeSignals     int
	BlockerSignals      int
	SignalCategories    []string
}

func readRankingRolloutPolicyForUpdate(ctx context.Context, db queryRower, scope memory.Scope, policyID string) (memory.RankingRolloutPolicy, error) {
	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
FOR UPDATE
`
	policy, err := scanRankingRolloutPolicy(db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, policyID))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read ranking rollout policy for update: %w", err)
	}
	return policy, nil
}

func buildRankingRolloutDryRunImpact(ctx context.Context, db queryRower, policy memory.RankingRolloutPolicy, input memory.RecordRankingRolloutDryRunInput) ([]memory.RankingRolloutImpactEntry, error) {
	candidates, err := readRankingRolloutSignalCandidates(ctx, db, input.Scope, input.SignalSource)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return []memory.RankingRolloutImpactEntry{{
			PolicyID:         input.PolicyID,
			Scope:            input.Scope.Normalized(),
			Surface:          input.Surface,
			SignalSource:     input.SignalSource,
			SignalCategories: []string{string(input.SignalSource)},
			SubjectKind:      "scope",
			BaselineRank:     0,
			AdjustedRank:     0,
			ReasonCode:       memory.RankingRolloutImpactReasonCodeInsufficientEvidence,
			EvidenceCount:    0,
			CreatedAt:        input.CreatedAt,
		}}, nil
	}

	baseline := append([]rankingRolloutSignalCandidate(nil), candidates...)
	sort.SliceStable(baseline, func(i, j int) bool {
		return rankingRolloutCandidateKey(baseline[i]) < rankingRolloutCandidateKey(baseline[j])
	})
	baselineRank := make(map[string]int, len(baseline))
	for i, candidate := range baseline {
		baselineRank[rankingRolloutCandidateKey(candidate)] = i + 1
	}

	adjusted := append([]rankingRolloutSignalCandidate(nil), candidates...)
	sort.SliceStable(adjusted, func(i, j int) bool {
		left := rankingRolloutCandidateScore(adjusted[i], policy)
		right := rankingRolloutCandidateScore(adjusted[j], policy)
		if left == right {
			return rankingRolloutCandidateKey(adjusted[i]) < rankingRolloutCandidateKey(adjusted[j])
		}
		return left > right
	})

	entries := make([]memory.RankingRolloutImpactEntry, 0, len(adjusted))
	for i, candidate := range adjusted {
		key := rankingRolloutCandidateKey(candidate)
		baseRank := baselineRank[key]
		adjustedRank := i + 1
		reason := rankingRolloutImpactReason(baseRank, adjustedRank, candidate, policy)
		entry := memory.RankingRolloutImpactEntry{
			PolicyID:          input.PolicyID,
			Scope:             input.Scope.Normalized(),
			Surface:           input.Surface,
			SignalSource:      input.SignalSource,
			SignalCategories:  candidate.SignalCategories,
			SubjectKind:       candidate.SubjectKind,
			SubjectID:         candidate.SubjectID,
			OpaqueToken:       candidate.OpaqueToken,
			CandidatePriority: adjustedRank,
			Included:          input.Surface == memory.RankingRolloutSurfaceSearch || adjustedRank <= maxRankingRolloutContextDryRunBudget(policy),
			BudgetImpact:      maxInt(baseRank-adjustedRank, adjustedRank-baseRank),
			BaselineRank:      baseRank,
			AdjustedRank:      adjustedRank,
			ReasonCode:        reason,
			EvidenceCount:     candidate.EvidenceCount,
			HiddenEvidence:    candidate.HiddenEvidenceCount > 0 || candidate.BlockerSignals > 0,
			CreatedAt:         input.CreatedAt,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func readRankingRolloutSignalCandidates(ctx context.Context, db queryRower, scope memory.Scope, source memory.RankingRolloutSignalSource) ([]rankingRolloutSignalCandidate, error) {
	switch source {
	case memory.RankingRolloutSignalSourceUsefulnessFeedback:
		return readUsefulnessRankingRolloutCandidates(ctx, db, scope)
	case memory.RankingRolloutSignalSourceTaskEvaluations:
		return readTaskRankingRolloutCandidates(ctx, db, scope)
	case memory.RankingRolloutSignalSourceQualityFindings:
		return readQualityRankingRolloutCandidates(ctx, db, scope)
	case memory.RankingRolloutSignalSourceSessionVerification:
		return readVerificationRankingRolloutCandidates(ctx, db, scope)
	default:
		return nil, fmt.Errorf("ranking rollout signal source %q is not supported", source)
	}
}

func readUsefulnessRankingRolloutCandidates(ctx context.Context, db queryRower, scope memory.Scope) ([]rankingRolloutSignalCandidate, error) {
	const query = `
SELECT ufs.subject_kind,
	COALESCE(ufs.subject_id, ''),
	COALESCE(ufs.opaque_token, ''),
	COUNT(*)::int,
	COUNT(*) FILTER (WHERE uf.feedback_type = 'useful')::int,
	COUNT(*) FILTER (WHERE uf.feedback_type IN ('irrelevant', 'noisy', 'stale', 'missing_expected'))::int,
	COUNT(*) FILTER (WHERE uf.feedback_type IN ('unsafe_or_hidden', 'needs_review'))::int
FROM usefulness_feedback uf
JOIN usefulness_feedback_subjects ufs
	ON ufs.feedback_id = uf.id
	AND ufs.tenant = uf.tenant
	AND ufs.project = uf.project
	AND ufs.namespace = uf.namespace
WHERE uf.tenant = $1
	AND uf.project = $2
	AND uf.namespace = $3
	AND uf.superseded_at IS NULL
GROUP BY ufs.subject_kind, COALESCE(ufs.subject_id, ''), COALESCE(ufs.opaque_token, '')
ORDER BY ufs.subject_kind ASC, COALESCE(ufs.subject_id, '') ASC, COALESCE(ufs.opaque_token, '') ASC
LIMIT 100
`
	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("read usefulness ranking rollout candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]rankingRolloutSignalCandidate, 0)
	for rows.Next() {
		var candidate rankingRolloutSignalCandidate
		if err := rows.Scan(&candidate.SubjectKind, &candidate.SubjectID, &candidate.OpaqueToken, &candidate.EvidenceCount, &candidate.PositiveSignals, &candidate.NegativeSignals, &candidate.BlockerSignals); err != nil {
			return nil, fmt.Errorf("scan usefulness ranking rollout candidate: %w", err)
		}
		candidate.HiddenEvidenceCount = candidate.BlockerSignals
		candidate.SignalCategories = []string{string(memory.RankingRolloutSignalSourceUsefulnessFeedback)}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usefulness ranking rollout candidates: %w", err)
	}
	return candidates, nil
}

func readTaskRankingRolloutCandidates(ctx context.Context, db queryRower, scope memory.Scope) ([]rankingRolloutSignalCandidate, error) {
	const query = `
SELECT tel.evidence_kind,
	COALESCE(tel.evidence_id, ''),
	COALESCE(tel.opaque_token, ''),
	COUNT(*)::int,
	COUNT(*) FILTER (WHERE te.verdict = 'succeeded')::int,
	COUNT(*) FILTER (WHERE te.verdict IN ('failed', 'partial'))::int,
	COUNT(*) FILTER (WHERE te.verdict = 'inconclusive' OR 'hidden_memory' = ANY(te.contribution_categories))::int
FROM task_evaluations te
JOIN task_evidence_links tel
	ON tel.task_evaluation_id = te.id
	AND tel.tenant = te.tenant
	AND tel.project = te.project
	AND tel.namespace = te.namespace
WHERE te.tenant = $1
	AND te.project = $2
	AND te.namespace = $3
	AND te.superseded_at IS NULL
GROUP BY tel.evidence_kind, COALESCE(tel.evidence_id, ''), COALESCE(tel.opaque_token, '')
ORDER BY tel.evidence_kind ASC, COALESCE(tel.evidence_id, '') ASC, COALESCE(tel.opaque_token, '') ASC
LIMIT 100
`
	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("read task ranking rollout candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]rankingRolloutSignalCandidate, 0)
	for rows.Next() {
		var candidate rankingRolloutSignalCandidate
		if err := rows.Scan(&candidate.SubjectKind, &candidate.SubjectID, &candidate.OpaqueToken, &candidate.EvidenceCount, &candidate.PositiveSignals, &candidate.NegativeSignals, &candidate.BlockerSignals); err != nil {
			return nil, fmt.Errorf("scan task ranking rollout candidate: %w", err)
		}
		candidate.HiddenEvidenceCount = candidate.BlockerSignals
		candidate.SignalCategories = []string{string(memory.RankingRolloutSignalSourceTaskEvaluations)}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task ranking rollout candidates: %w", err)
	}
	return candidates, nil
}

func readQualityRankingRolloutCandidates(ctx context.Context, db queryRower, scope memory.Scope) ([]rankingRolloutSignalCandidate, error) {
	const query = `
SELECT code,
	'',
	'',
	COUNT(*)::int,
	0,
	COUNT(*) FILTER (WHERE severity = 'warning')::int,
	COUNT(*) FILTER (WHERE severity = 'blocker')::int
FROM quality_evaluation_findings
WHERE tenant = $1 AND project = $2 AND namespace = $3
GROUP BY code
ORDER BY code ASC
LIMIT 100
`
	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("read quality ranking rollout candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]rankingRolloutSignalCandidate, 0)
	for rows.Next() {
		var candidate rankingRolloutSignalCandidate
		var unusedSubjectKind, unusedOpaqueToken string
		if err := rows.Scan(&candidate.SubjectID, &unusedSubjectKind, &unusedOpaqueToken, &candidate.EvidenceCount, &candidate.PositiveSignals, &candidate.NegativeSignals, &candidate.BlockerSignals); err != nil {
			return nil, fmt.Errorf("scan quality ranking rollout candidate: %w", err)
		}
		candidate.SubjectKind = "quality_finding"
		candidate.HiddenEvidenceCount = candidate.BlockerSignals
		candidate.SignalCategories = []string{string(memory.RankingRolloutSignalSourceQualityFindings)}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quality ranking rollout candidates: %w", err)
	}
	return candidates, nil
}

func readVerificationRankingRolloutCandidates(ctx context.Context, db queryRower, scope memory.Scope) ([]rankingRolloutSignalCandidate, error) {
	const query = `
SELECT COALESCE(turn_id, session_id),
	'',
	'',
	COUNT(*)::int,
	COUNT(*) FILTER (WHERE verdict = 'passed')::int,
	COUNT(*) FILTER (WHERE verdict = 'passed_degraded')::int,
	COUNT(*) FILTER (WHERE verdict IN ('failed', 'pending'))::int
FROM memory_session_verifications
WHERE tenant = $1 AND project = $2 AND namespace = $3
GROUP BY COALESCE(turn_id, session_id)
ORDER BY COALESCE(turn_id, session_id) ASC
LIMIT 100
`
	rows, err := db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("read session verification ranking rollout candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]rankingRolloutSignalCandidate, 0)
	for rows.Next() {
		var candidate rankingRolloutSignalCandidate
		var unusedSubjectKind, unusedOpaqueToken string
		if err := rows.Scan(&candidate.SubjectID, &unusedSubjectKind, &unusedOpaqueToken, &candidate.EvidenceCount, &candidate.PositiveSignals, &candidate.NegativeSignals, &candidate.BlockerSignals); err != nil {
			return nil, fmt.Errorf("scan session verification ranking rollout candidate: %w", err)
		}
		candidate.SubjectKind = "verification"
		candidate.HiddenEvidenceCount = candidate.BlockerSignals
		candidate.SignalCategories = []string{string(memory.RankingRolloutSignalSourceSessionVerification)}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session verification ranking rollout candidates: %w", err)
	}
	return candidates, nil
}

func summarizeRankingRolloutDryRun(policy memory.RankingRolloutPolicy, input memory.RecordRankingRolloutDryRunInput, impact []memory.RankingRolloutImpactEntry) memory.RankingRolloutDryRun {
	dryRun := memory.RankingRolloutDryRun{
		ThresholdStatus:     input.ThresholdStatus,
		BaselineRank:        input.BaselineRank,
		AdjustedRank:        input.AdjustedRank,
		ChangedSubjectIDs:   boundedUniqueStrings(input.ChangedSubjectIDs, 100),
		ReasonCodes:         uniqueRankingRolloutReasonCodes(input.ReasonCodes),
		SignalCategories:    boundedUniqueStrings(input.SignalCategories, 16),
		EvidenceCount:       input.EvidenceCount,
		HiddenEvidenceCount: input.HiddenEvidenceCount,
	}
	if len(dryRun.SignalCategories) == 0 {
		dryRun.SignalCategories = []string{string(input.SignalSource)}
	}
	if len(impact) == 0 {
		return dryRun
	}
	evidence := 0
	hidden := 0
	changed := make([]string, 0)
	reasons := make([]memory.RankingRolloutImpactReasonCode, 0)
	for _, entry := range impact {
		evidence += entry.EvidenceCount
		if entry.HiddenEvidence {
			hidden++
		}
		reasons = append(reasons, entry.ReasonCode)
		if entry.BaselineRank != entry.AdjustedRank {
			if strings.TrimSpace(entry.SubjectID) != "" {
				changed = append(changed, entry.SubjectID)
			} else if strings.TrimSpace(entry.OpaqueToken) != "" {
				changed = append(changed, "opaque")
			}
		}
		if entry.BaselineRank > dryRun.BaselineRank {
			dryRun.BaselineRank = entry.BaselineRank
		}
		if entry.AdjustedRank > dryRun.AdjustedRank {
			dryRun.AdjustedRank = entry.AdjustedRank
		}
		dryRun.SignalCategories = append(dryRun.SignalCategories, entry.SignalCategories...)
	}
	dryRun.EvidenceCount = evidence
	dryRun.HiddenEvidenceCount = hidden
	dryRun.ChangedSubjectIDs = boundedUniqueStrings(append(dryRun.ChangedSubjectIDs, changed...), 100)
	dryRun.ReasonCodes = uniqueRankingRolloutReasonCodes(append(dryRun.ReasonCodes, reasons...))
	dryRun.SignalCategories = boundedUniqueStrings(dryRun.SignalCategories, 16)
	switch {
	case hidden > 0:
		dryRun.ThresholdStatus = memory.RankingRolloutThresholdStatusBlocked
	case evidence < policy.EvidenceMinimum:
		dryRun.ThresholdStatus = memory.RankingRolloutThresholdStatusInsufficient
	default:
		dryRun.ThresholdStatus = memory.RankingRolloutThresholdStatusSatisfied
	}
	return dryRun
}

func insertRankingRolloutImpactEntry(ctx context.Context, db queryRower, entry memory.RankingRolloutImpactEntry) error {
	const query = `
INSERT INTO ranking_rollout_impact_entries (
	id, dry_run_id, policy_id, tenant, project, namespace, surface, signal_source, signal_categories,
	subject_kind, subject_id, opaque_token, candidate_priority, included, budget_impact,
	baseline_rank, adjusted_rank, reason_code, evidence_count, hidden_evidence, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
`
	if _, err := db.Exec(ctx, query, entry.ID, entry.DryRunID, entry.PolicyID, entry.Scope.Tenant, entry.Scope.Project, entry.Scope.Namespace, entry.Surface, entry.SignalSource, entry.SignalCategories, entry.SubjectKind, nullableString(entry.SubjectID), nullableString(entry.OpaqueToken), entry.CandidatePriority, entry.Included, entry.BudgetImpact, entry.BaselineRank, entry.AdjustedRank, entry.ReasonCode, entry.EvidenceCount, entry.HiddenEvidence, entry.CreatedAt); err != nil {
		return fmt.Errorf("insert ranking rollout impact entry: %w", err)
	}
	return nil
}

func upsertRankingRolloutPolicyState(ctx context.Context, db queryRower, policy memory.RankingRolloutPolicy, status memory.RankingRolloutPolicyStatus, actor, reason string, updatedAt time.Time) error {
	const query = `
INSERT INTO ranking_rollout_policy_states (
	policy_id, tenant, project, namespace, status, actor, reason, activated_at, disabled_at, rolled_back_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (policy_id) DO UPDATE
SET status = EXCLUDED.status,
	actor = EXCLUDED.actor,
	reason = EXCLUDED.reason,
	activated_at = EXCLUDED.activated_at,
	disabled_at = EXCLUDED.disabled_at,
	rolled_back_at = EXCLUDED.rolled_back_at,
	updated_at = EXCLUDED.updated_at
`
	if _, err := db.Exec(ctx, query, policy.ID, policy.Scope.Tenant, policy.Scope.Project, policy.Scope.Namespace, status, actor, reason, nullableTime(policy.ActivatedAt), nullableTime(policy.DisabledAt), nullableTime(policy.RolledBackAt), updatedAt); err != nil {
		return fmt.Errorf("upsert ranking rollout policy state: %w", err)
	}
	return nil
}

func rankingRolloutPolicyIncludesSurface(policy memory.RankingRolloutPolicy, surface memory.RankingRolloutSurface) bool {
	for _, value := range policy.Surfaces {
		if value == surface {
			return true
		}
	}
	return false
}

func rankingRolloutPolicyIncludesSignalSource(policy memory.RankingRolloutPolicy, source memory.RankingRolloutSignalSource) bool {
	for _, value := range policy.SignalSources {
		if value == source {
			return true
		}
	}
	return false
}

func rankingRolloutCandidateKey(candidate rankingRolloutSignalCandidate) string {
	return candidate.SubjectKind + ":" + candidate.SubjectID + ":" + candidate.OpaqueToken
}

func rankingRolloutCandidateScore(candidate rankingRolloutSignalCandidate, policy memory.RankingRolloutPolicy) int {
	if candidate.EvidenceCount < policy.EvidenceMinimum || candidate.HiddenEvidenceCount > 0 || candidate.BlockerSignals > 0 {
		return -1000 - candidate.NegativeSignals
	}
	return candidate.PositiveSignals*10 - candidate.NegativeSignals*10
}

func rankingRolloutImpactReason(baselineRank, adjustedRank int, candidate rankingRolloutSignalCandidate, policy memory.RankingRolloutPolicy) memory.RankingRolloutImpactReasonCode {
	if candidate.HiddenEvidenceCount > 0 || candidate.BlockerSignals > 0 {
		return memory.RankingRolloutImpactReasonCodeBlockerPresent
	}
	if candidate.EvidenceCount < policy.EvidenceMinimum {
		return memory.RankingRolloutImpactReasonCodeInsufficientEvidence
	}
	if adjustedRank < baselineRank {
		return memory.RankingRolloutImpactReasonCodeSubjectBoosted
	}
	if adjustedRank > baselineRank {
		return memory.RankingRolloutImpactReasonCodeSubjectPenalized
	}
	return memory.RankingRolloutImpactReasonCodeBaselineRetained
}

func maxRankingRolloutContextDryRunBudget(policy memory.RankingRolloutPolicy) int {
	if policy.EvidenceMinimum <= 0 {
		return 4
	}
	return maxInt(policy.EvidenceMinimum, 1)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func boundedUniqueStrings(values []string, limit int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	return out
}

func uniqueRankingRolloutReasonCodes(values []memory.RankingRolloutImpactReasonCode) []memory.RankingRolloutImpactReasonCode {
	seen := make(map[memory.RankingRolloutImpactReasonCode]struct{}, len(values))
	out := make([]memory.RankingRolloutImpactReasonCode, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func rankingRolloutSurfaceStrings(values []memory.RankingRolloutSurface) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, string(value))
	}
	return items
}

func rankingRolloutSurfaces(values []string) []memory.RankingRolloutSurface {
	items := make([]memory.RankingRolloutSurface, 0, len(values))
	for _, value := range values {
		items = append(items, memory.RankingRolloutSurface(value))
	}
	return items
}

func rankingRolloutSignalSourceStrings(values []memory.RankingRolloutSignalSource) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, string(value))
	}
	return items
}

func rankingRolloutSignalSources(values []string) []memory.RankingRolloutSignalSource {
	items := make([]memory.RankingRolloutSignalSource, 0, len(values))
	for _, value := range values {
		items = append(items, memory.RankingRolloutSignalSource(value))
	}
	return items
}

func rankingRolloutReasonCodes(values []string) []memory.RankingRolloutImpactReasonCode {
	items := make([]memory.RankingRolloutImpactReasonCode, 0, len(values))
	for _, value := range values {
		items = append(items, memory.RankingRolloutImpactReasonCode(value))
	}
	return items
}

func rankingRolloutReasonCodeStrings(values []memory.RankingRolloutImpactReasonCode) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, string(value))
	}
	return items
}
