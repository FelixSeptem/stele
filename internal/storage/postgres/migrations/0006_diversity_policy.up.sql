ALTER TABLE ranking_rollout_policies
    ADD COLUMN IF NOT EXISTS diversity_policy_name text,
    ADD COLUMN IF NOT EXISTS diversity_policy_version text,
    ADD COLUMN IF NOT EXISTS diversity_mmr_lambda double precision,
    ADD COLUMN IF NOT EXISTS diversity_semantic_threshold double precision,
    ADD COLUMN IF NOT EXISTS diversity_max_candidates integer,
    ADD COLUMN IF NOT EXISTS diversity_max_pairwise_comparisons integer,
    ADD COLUMN IF NOT EXISTS diversity_max_embedding_dimensions integer,
    ADD COLUMN IF NOT EXISTS diversity_max_citations_per_candidate integer,
    ADD COLUMN IF NOT EXISTS diversity_coverage_weights jsonb;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ranking_rollout_policies_diversity_parameters_check') THEN
        ALTER TABLE ranking_rollout_policies ADD CONSTRAINT ranking_rollout_policies_diversity_parameters_check CHECK (
            (diversity_policy_name IS NULL AND diversity_policy_version IS NULL AND diversity_mmr_lambda IS NULL AND diversity_semantic_threshold IS NULL AND diversity_max_candidates IS NULL AND diversity_max_pairwise_comparisons IS NULL AND diversity_max_embedding_dimensions IS NULL AND diversity_max_citations_per_candidate IS NULL AND diversity_coverage_weights IS NULL)
            OR (diversity_policy_name = 'mmr' AND diversity_policy_version IS NOT NULL AND diversity_mmr_lambda BETWEEN 0 AND 1 AND diversity_semantic_threshold BETWEEN 0 AND 1 AND diversity_max_candidates BETWEEN 1 AND 5000 AND diversity_max_pairwise_comparisons BETWEEN 1 AND 100000 AND diversity_max_embedding_dimensions BETWEEN 1 AND 4096 AND diversity_max_citations_per_candidate BETWEEN 1 AND 64 AND jsonb_typeof(diversity_coverage_weights) = 'object')
        );
    END IF;
END $$;
