package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	policy.Scope = policy.Scope.Normalized()
	policy.QueryAnalysisSelector = policy.QueryAnalysisSelector.Normalized()
	policy.RetrievalPlannerSelector = policy.RetrievalPlannerSelector.Normalized()
	fusionChannelWeights, err := marshalOptionalFusionChannelWeights(policy.FusionChannelWeights)
	if err != nil {
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
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions,
	diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37)
ON CONFLICT (id) DO UPDATE SET id = ranking_rollout_policies.id
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions,
	diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
`
	if policy.QueryAnalysis != nil {
		payload, marshalErr := json.Marshal(policy.QueryAnalysis)
		if marshalErr != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("marshal query-analysis rollout policy: %w", marshalErr)
		}
		const queryWithAnalysis = `
INSERT INTO ranking_rollout_policies (
	id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions,
	diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37)
ON CONFLICT (id) DO UPDATE SET id = ranking_rollout_policies.id
RETURNING id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions,
	diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at`
		created, scanErr := scanRankingRolloutPolicy(tx.QueryRow(ctx, queryWithAnalysis,
			policy.ID, policy.Scope.Tenant, policy.Scope.Project, policy.Scope.Namespace, policy.Status, policy.Mode,
			rankingRolloutSurfaceStrings(policy.Surfaces), rankingRolloutSignalSourceStrings(policy.SignalSources), policy.ThresholdStatus,
			policy.EvidenceMinimum, policy.Actor, policy.Reason, nullableString(policy.LatestDryRunID), nullableString(string(policy.LatestDryRunStatus)),
			nullableString(policy.FusionStrategy), nullableString(policy.FusionVersion), nullableRankingInt(policy.FusionRankConstant), fusionChannelWeights,
			nullableRankingInt(policy.FusionPerChannelCandidate), nullableRankingInt(policy.FusionTotalCandidates), nullableString(policy.DiversityPolicyName), nullableString(policy.DiversityPolicyVersion),
			nullableDiversityFloat(policy.DiversityMMRLambda, policy.DiversityPolicyName != ""), nullableDiversityFloat(policy.DiversitySemanticThreshold, policy.DiversityPolicyName != ""),
			nullableRankingInt(policy.DiversityMaxCandidates), nullableRankingInt(policy.DiversityMaxPairwiseComparisons), nullableRankingInt(policy.DiversityMaxEmbeddingDimensions), nullableRankingInt(policy.DiversityMaxCitationsPerCandidate), marshalOptionalDiversityCoverageWeights(policy.DiversityCoverageWeights),
			nullableString(policy.QueryAnalysisSelector.SessionID), nullableString(policy.QueryAnalysisSelector.UserID), payload,
			nullableTime(policy.ActivatedAt), nullableTime(policy.DisabledAt), nullableTime(policy.RolledBackAt), policy.CreatedAt, policy.UpdatedAt))
		if scanErr != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("create ranking rollout policy: %w", scanErr)
		}
		created.QueryAnalysis = policy.QueryAnalysis
		created.QueryAnalysisSelector = policy.QueryAnalysisSelector
		if err := persistRetrievalPlannerRollout(ctx, tx, policy); err != nil {
			return memory.RankingRolloutPolicy{}, err
		}
		created.RetrievalPlanner = policy.RetrievalPlanner
		created.RetrievalPlannerSelector = policy.RetrievalPlannerSelector
		if err := persistContextCalibrationRollout(ctx, tx, policy); err != nil {
			return memory.RankingRolloutPolicy{}, err
		}
		created.ContextCalibration = policy.ContextCalibration
		created.ContextCalibrationSelector = policy.ContextCalibrationSelector
		if err := upsertRankingRolloutPolicyState(ctx, tx, created, created.Status, created.Actor, created.Reason, created.UpdatedAt); err != nil {
			return memory.RankingRolloutPolicy{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout policy transaction: %w", err)
		}
		return created, nil
	}
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
		nullableString(policy.FusionStrategy),
		nullableString(policy.FusionVersion),
		nullableRankingInt(policy.FusionRankConstant),
		fusionChannelWeights,
		nullableRankingInt(policy.FusionPerChannelCandidate),
		nullableRankingInt(policy.FusionTotalCandidates),
		nullableString(policy.DiversityPolicyName), nullableString(policy.DiversityPolicyVersion), nullableDiversityFloat(policy.DiversityMMRLambda, policy.DiversityPolicyName != ""), nullableDiversityFloat(policy.DiversitySemanticThreshold, policy.DiversityPolicyName != ""),
		nullableRankingInt(policy.DiversityMaxCandidates), nullableRankingInt(policy.DiversityMaxPairwiseComparisons), nullableRankingInt(policy.DiversityMaxEmbeddingDimensions), nullableRankingInt(policy.DiversityMaxCitationsPerCandidate), marshalOptionalDiversityCoverageWeights(policy.DiversityCoverageWeights),
		nil, nil, nil,
		nullableTime(policy.ActivatedAt),
		nullableTime(policy.DisabledAt),
		nullableTime(policy.RolledBackAt),
		policy.CreatedAt,
		policy.UpdatedAt,
	))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("create ranking rollout policy: %w", err)
	}
	if err := persistRetrievalPlannerRollout(ctx, tx, policy); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	created.RetrievalPlanner = policy.RetrievalPlanner
	created.RetrievalPlannerSelector = policy.RetrievalPlannerSelector
	if err := persistContextCalibrationRollout(ctx, tx, policy); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	created.ContextCalibration = policy.ContextCalibration
	created.ContextCalibrationSelector = policy.ContextCalibrationSelector
	if err := upsertRankingRolloutPolicyState(ctx, tx, created, created.Status, created.Actor, created.Reason, created.UpdatedAt); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("commit ranking rollout policy transaction: %w", err)
	}
	return created, nil
}

func persistRetrievalPlannerRollout(ctx context.Context, tx pgx.Tx, policy memory.RankingRolloutPolicy) error {
	if policy.RetrievalPlanner == nil {
		return nil
	}
	payload, err := json.Marshal(policy.RetrievalPlanner)
	if err != nil {
		return fmt.Errorf("marshal retrieval-planner rollout policy: %w", err)
	}
	const query = `
UPDATE ranking_rollout_policies
SET retrieval_planner_session_id = $1,
    retrieval_planner_user_id = $2,
    retrieval_planner_policy = $3
WHERE id = $4 AND tenant = $5 AND project = $6 AND namespace = $7`
	result, err := tx.Exec(ctx, query, nullableString(policy.RetrievalPlannerSelector.SessionID), nullableString(policy.RetrievalPlannerSelector.UserID), payload, policy.ID, policy.Scope.Tenant, policy.Scope.Project, policy.Scope.Namespace)
	if err != nil {
		return fmt.Errorf("persist retrieval-planner rollout policy: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("persist retrieval-planner rollout policy: exact policy row not found")
	}
	return nil
}

// persistContextCalibrationRollout keeps the optional calibration payload in
// its own additive table. The base ranking policy remains compatible with
// pre-calibration rows, while the payload retains exact scope and selector
// identity for the hot-path policy read.
func persistContextCalibrationRollout(ctx context.Context, q queryRower, policy memory.RankingRolloutPolicy) error {
	if policy.ContextCalibration == nil {
		return nil
	}
	payload, err := json.Marshal(policy.ContextCalibration)
	if err != nil {
		return fmt.Errorf("marshal context-calibration rollout policy: %w", err)
	}
	const query = `
INSERT INTO context_calibration_rollout_policies
 (policy_id, tenant, project, namespace, session_id, user_id, payload, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (policy_id) DO UPDATE SET tenant=EXCLUDED.tenant, project=EXCLUDED.project,
 namespace=EXCLUDED.namespace, session_id=EXCLUDED.session_id, user_id=EXCLUDED.user_id,
 payload=EXCLUDED.payload, updated_at=EXCLUDED.updated_at`
	if _, err := q.Exec(ctx, query, policy.ID, policy.Scope.Tenant, policy.Scope.Project, policy.Scope.Namespace,
		nullableString(policy.ContextCalibrationSelector.SessionID), nullableString(policy.ContextCalibrationSelector.UserID), payload, policy.CreatedAt, policy.UpdatedAt); err != nil {
		return fmt.Errorf("persist context-calibration rollout policy: %w", err)
	}
	return nil
}

// attachContextCalibrationRollout loads calibration only for a context policy.
// A missing row is a normal pre-calibration baseline; malformed payloads are
// rejected so they cannot accidentally activate a partial policy.
func attachContextCalibrationRollout(ctx context.Context, q queryRower, policy *memory.RankingRolloutPolicy) error {
	if policy == nil || !rankingRolloutPolicyIncludesSurface(*policy, memory.RankingRolloutSurfaceContext) {
		return nil
	}
	const query = `
SELECT session_id, user_id, payload
FROM context_calibration_rollout_policies
WHERE policy_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4`
	var sessionID, userID sql.NullString
	var payload []byte
	if err := q.QueryRow(ctx, query, policy.ID, policy.Scope.Tenant, policy.Scope.Project, policy.Scope.Namespace).Scan(&sessionID, &userID, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("read context-calibration rollout policy: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var calibration memory.ContextCalibrationRolloutPolicy
	if err := decoder.Decode(&calibration); err != nil {
		return fmt.Errorf("decode context-calibration rollout payload: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode context-calibration rollout payload: trailing JSON value")
		}
		return fmt.Errorf("decode context-calibration rollout payload: trailing data: %w", err)
	}
	if err := calibration.Validate(); err != nil {
		return fmt.Errorf("validate context-calibration rollout payload: %w", err)
	}
	policy.ContextCalibration = &calibration
	if sessionID.Valid {
		policy.ContextCalibrationSelector.SessionID = sessionID.String
	}
	if userID.Valid {
		policy.ContextCalibrationSelector.UserID = userID.String
	}
	return nil
}

func (r *Repository) ReadRankingRolloutPolicy(ctx context.Context, input memory.ReadRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	input.Scope = input.Scope.Normalized()

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
	activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	policy, err := scanRankingRolloutPolicy(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.PolicyID))
	if err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read ranking rollout policy: %w", err)
	}
	if err := attachContextCalibrationRollout(ctx, r.db, &policy); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	return policy, nil
}

func (r *Repository) ReadActiveRankingRolloutPolicy(ctx context.Context, input memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	input.Scope = input.Scope.Normalized()

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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
	if input.Surface == memory.RankingRolloutSurfaceContext {
		if err := attachContextCalibrationRollout(ctx, r.db, &policy); err != nil {
			return memory.RankingRolloutPolicy{}, err
		}
	}
	return policy, nil
}

// ReadEffectiveQueryAnalysisRolloutPolicy selects only a query-analysis
// payload whose complete exact scope (including nullable session/user
// selectors) matches the request. It intentionally has no broader fallback.
func (r *Repository) ReadEffectiveQueryAnalysisRolloutPolicy(ctx context.Context, input memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	scope := input.Scope.Normalized()
	selectorSession := strings.TrimSpace(input.SessionID)
	selectorUser := strings.TrimSpace(input.UserID)
	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces,
       query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
       activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND $4 = ANY(surfaces)
  AND query_analysis_policy IS NOT NULL
  AND query_analysis_session_id IS NOT DISTINCT FROM $5
  AND query_analysis_user_id IS NOT DISTINCT FROM $6
ORDER BY activated_at DESC NULLS LAST, updated_at DESC, created_at DESC, id DESC
LIMIT 1`
	var p memory.RankingRolloutPolicy
	var surfaces []string
	var sessionID, userID sql.NullString
	var payload []byte
	var activated, disabled, rolledBack, created, updated sql.NullTime
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, string(input.Surface), nullableString(selectorSession), nullableString(selectorUser)).Scan(
		&p.ID, &p.Scope.Tenant, &p.Scope.Project, &p.Scope.Namespace, &p.Status, &p.Mode, &surfaces,
		&sessionID, &userID, &payload, &activated, &disabled, &rolledBack, &created, &updated); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read effective query-analysis rollout policy: %w", err)
	}
	p.Surfaces = rankingRolloutSurfaces(surfaces)
	if sessionID.Valid {
		p.QueryAnalysisSelector.SessionID = sessionID.String
	}
	if userID.Valid {
		p.QueryAnalysisSelector.UserID = userID.String
	}
	if len(payload) == 0 {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("query-analysis rollout payload is empty")
	}
	var qa memory.QueryAnalysisRolloutPolicy
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&qa); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: trailing JSON value")
	} else if err != io.EOF {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: trailing data: %w", err)
	}
	if err := qa.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("validate query-analysis rollout payload: %w", err)
	}
	p.QueryAnalysis = &qa
	if activated.Valid {
		p.ActivatedAt = activated.Time
	}
	if disabled.Valid {
		p.DisabledAt = disabled.Time
	}
	if rolledBack.Valid {
		p.RolledBackAt = rolledBack.Time
	}
	if created.Valid {
		p.CreatedAt = created.Time
	}
	if updated.Valid {
		p.UpdatedAt = updated.Time
	}
	return p, nil
}

func (r *Repository) ReadEffectiveRetrievalPlannerRolloutPolicy(ctx context.Context, input memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, err
	}
	scope := input.Scope.Normalized()
	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces,
       retrieval_planner_session_id, retrieval_planner_user_id, retrieval_planner_policy,
       activated_at, disabled_at, rolled_back_at, created_at, updated_at
FROM ranking_rollout_policies
WHERE tenant = $1 AND project = $2 AND namespace = $3
  AND $4 = ANY(surfaces)
  AND retrieval_planner_policy IS NOT NULL
  AND retrieval_planner_session_id IS NOT DISTINCT FROM $5
  AND retrieval_planner_user_id IS NOT DISTINCT FROM $6
ORDER BY activated_at DESC NULLS LAST, updated_at DESC, created_at DESC, id DESC
LIMIT 1`
	var policy memory.RankingRolloutPolicy
	var surfaces []string
	var sessionID, userID sql.NullString
	var payload []byte
	var activated, disabled, rolledBack, created, updated sql.NullTime
	if err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, string(input.Surface), nullableString(strings.TrimSpace(input.SessionID)), nullableString(strings.TrimSpace(input.UserID))).Scan(
		&policy.ID, &policy.Scope.Tenant, &policy.Scope.Project, &policy.Scope.Namespace, &policy.Status, &policy.Mode, &surfaces,
		&sessionID, &userID, &payload, &activated, &disabled, &rolledBack, &created, &updated,
	); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("read effective retrieval-planner rollout policy: %w", err)
	}
	policy.Surfaces = rankingRolloutSurfaces(surfaces)
	if sessionID.Valid {
		policy.RetrievalPlannerSelector.SessionID = sessionID.String
	}
	if userID.Valid {
		policy.RetrievalPlannerSelector.UserID = userID.String
	}
	if len(payload) == 0 {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("retrieval-planner rollout payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var planner memory.RetrievalPlannerRolloutPolicy
	if err := decoder.Decode(&planner); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("decode retrieval-planner rollout payload: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("decode retrieval-planner rollout payload: trailing JSON value")
		}
		return memory.RankingRolloutPolicy{}, fmt.Errorf("decode retrieval-planner rollout payload: trailing data: %w", err)
	}
	if err := planner.Validate(); err != nil {
		return memory.RankingRolloutPolicy{}, fmt.Errorf("validate retrieval-planner rollout payload: %w", err)
	}
	policy.RetrievalPlanner = &planner
	if activated.Valid {
		policy.ActivatedAt = activated.Time
	}
	if disabled.Valid {
		policy.DisabledAt = disabled.Time
	}
	if rolledBack.Valid {
		policy.RolledBackAt = rolledBack.Time
	}
	if created.Valid {
		policy.CreatedAt = created.Time
	}
	if updated.Valid {
		policy.UpdatedAt = updated.Time
	}
	return policy, nil
}

func (r *Repository) ListRankingRolloutPolicies(ctx context.Context, input memory.ListRankingRolloutPoliciesInput) ([]memory.RankingRolloutPolicy, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	input.Scope = input.Scope.Normalized()

	const query = `
SELECT id, tenant, project, namespace, status, mode, surfaces, signal_sources, threshold_status,
	evidence_minimum, actor, reason, latest_dry_run_id, latest_dry_run_status,
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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
		if err := attachContextCalibrationRollout(ctx, r.db, &policy); err != nil {
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
	input.Scope = input.Scope.Normalized()
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
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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
	input.Scope = input.Scope.Normalized()

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
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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
	input.Scope = input.Scope.Normalized()

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
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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
	var fusionStrategy sql.NullString
	var fusionVersion sql.NullString
	var fusionRankConstant sql.NullInt64
	var fusionChannelWeights []byte
	var fusionPerChannelCandidate sql.NullInt64
	var fusionTotalCandidates sql.NullInt64
	var diversityPolicyName sql.NullString
	var diversityPolicyVersion sql.NullString
	var diversityMMRLambda sql.NullFloat64
	var diversitySemanticThreshold sql.NullFloat64
	var diversityMaxCandidates sql.NullInt64
	var diversityMaxPairwiseComparisons sql.NullInt64
	var diversityMaxEmbeddingDimensions sql.NullInt64
	var diversityMaxCitationsPerCandidate sql.NullInt64
	var diversityCoverageWeights []byte
	var queryAnalysisSessionID sql.NullString
	var queryAnalysisUserID sql.NullString
	var queryAnalysisPayload []byte
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
		&fusionStrategy,
		&fusionVersion,
		&fusionRankConstant,
		&fusionChannelWeights,
		&fusionPerChannelCandidate,
		&fusionTotalCandidates,
		&diversityPolicyName,
		&diversityPolicyVersion,
		&diversityMMRLambda,
		&diversitySemanticThreshold,
		&diversityMaxCandidates,
		&diversityMaxPairwiseComparisons,
		&diversityMaxEmbeddingDimensions,
		&diversityMaxCitationsPerCandidate,
		&diversityCoverageWeights,
		&queryAnalysisSessionID,
		&queryAnalysisUserID,
		&queryAnalysisPayload,
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
	if fusionStrategy.Valid {
		policy.FusionStrategy = fusionStrategy.String
	}
	if fusionVersion.Valid {
		policy.FusionVersion = fusionVersion.String
	}
	if fusionRankConstant.Valid {
		policy.FusionRankConstant = int(fusionRankConstant.Int64)
	}
	if len(fusionChannelWeights) > 0 {
		if err := json.Unmarshal(fusionChannelWeights, &policy.FusionChannelWeights); err != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("decode ranking rollout fusion channel weights: %w", err)
		}
	}
	if fusionPerChannelCandidate.Valid {
		policy.FusionPerChannelCandidate = int(fusionPerChannelCandidate.Int64)
	}
	if fusionTotalCandidates.Valid {
		policy.FusionTotalCandidates = int(fusionTotalCandidates.Int64)
	}
	if diversityPolicyName.Valid {
		policy.DiversityPolicyName = diversityPolicyName.String
	}
	if diversityPolicyVersion.Valid {
		policy.DiversityPolicyVersion = diversityPolicyVersion.String
	}
	if diversityMMRLambda.Valid {
		policy.DiversityMMRLambda = diversityMMRLambda.Float64
	}
	if diversitySemanticThreshold.Valid {
		policy.DiversitySemanticThreshold = diversitySemanticThreshold.Float64
	}
	if diversityMaxCandidates.Valid {
		policy.DiversityMaxCandidates = int(diversityMaxCandidates.Int64)
	}
	if diversityMaxPairwiseComparisons.Valid {
		policy.DiversityMaxPairwiseComparisons = int(diversityMaxPairwiseComparisons.Int64)
	}
	if diversityMaxEmbeddingDimensions.Valid {
		policy.DiversityMaxEmbeddingDimensions = int(diversityMaxEmbeddingDimensions.Int64)
	}
	if diversityMaxCitationsPerCandidate.Valid {
		policy.DiversityMaxCitationsPerCandidate = int(diversityMaxCitationsPerCandidate.Int64)
	}
	if len(diversityCoverageWeights) > 0 {
		if err := json.Unmarshal(diversityCoverageWeights, &policy.DiversityCoverageWeights); err != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("decode diversity coverage weights: %w", err)
		}
	}
	if queryAnalysisSessionID.Valid {
		policy.QueryAnalysisSelector.SessionID = queryAnalysisSessionID.String
	}
	if queryAnalysisUserID.Valid {
		policy.QueryAnalysisSelector.UserID = queryAnalysisUserID.String
	}
	if len(queryAnalysisPayload) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(queryAnalysisPayload))
		decoder.DisallowUnknownFields()
		var qa memory.QueryAnalysisRolloutPolicy
		if err := decoder.Decode(&qa); err != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: trailing JSON value")
			}
			return memory.RankingRolloutPolicy{}, fmt.Errorf("decode query-analysis rollout payload: trailing data: %w", err)
		}
		if err := qa.Validate(); err != nil {
			return memory.RankingRolloutPolicy{}, fmt.Errorf("validate query-analysis rollout payload: %w", err)
		}
		policy.QueryAnalysis = &qa
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
	fusion_strategy, fusion_version, fusion_rank_constant, fusion_channel_weights,
	fusion_per_channel_candidate, fusion_total_candidates,
	diversity_policy_name, diversity_policy_version, diversity_mmr_lambda, diversity_semantic_threshold,
	diversity_max_candidates, diversity_max_pairwise_comparisons, diversity_max_embedding_dimensions, diversity_max_citations_per_candidate, diversity_coverage_weights,
	query_analysis_session_id, query_analysis_user_id, query_analysis_policy,
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

func nullableRankingInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableDiversityFloat(value float64, configured bool) any {
	if !configured {
		return nil
	}
	return value
}

func marshalOptionalDiversityCoverageWeights(value map[string]float64) any {
	if len(value) == 0 {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return payload
}

func marshalOptionalFusionChannelWeights(value map[string]float64) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal ranking rollout fusion channel weights: %w", err)
	}
	return payload, nil
}
