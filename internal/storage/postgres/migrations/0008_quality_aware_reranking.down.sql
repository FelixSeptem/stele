ALTER TABLE ranking_rollout_policies
    DROP CONSTRAINT IF EXISTS ranking_rollout_policies_reranker_parameters_check;

ALTER TABLE ranking_rollout_policies
    DROP COLUMN IF EXISTS reranker_mode,
    DROP COLUMN IF EXISTS reranker_version,
    DROP COLUMN IF EXISTS reranker_provider,
    DROP COLUMN IF EXISTS quality_feature_version;
