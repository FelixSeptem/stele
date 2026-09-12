ALTER TABLE ranking_rollout_policies
    ADD COLUMN IF NOT EXISTS quality_feature_version text,
    ADD COLUMN IF NOT EXISTS reranker_provider text,
    ADD COLUMN IF NOT EXISTS reranker_version text,
    ADD COLUMN IF NOT EXISTS reranker_mode text;

ALTER TABLE ranking_rollout_policies
    ADD CONSTRAINT ranking_rollout_policies_reranker_parameters_check
    CHECK (
        (quality_feature_version IS NULL OR length(quality_feature_version) BETWEEN 1 AND 64)
        AND (reranker_provider IS NULL OR length(reranker_provider) BETWEEN 1 AND 64)
        AND (reranker_version IS NULL OR length(reranker_version) BETWEEN 1 AND 64)
        AND (reranker_mode IS NULL OR reranker_mode IN ('disabled', 'diagnostics_only', 'shadow', 'active_for_scope'))
    );
