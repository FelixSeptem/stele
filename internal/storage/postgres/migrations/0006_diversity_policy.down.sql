ALTER TABLE ranking_rollout_policies DROP CONSTRAINT IF EXISTS ranking_rollout_policies_diversity_parameters_check;
ALTER TABLE ranking_rollout_policies
    DROP COLUMN IF EXISTS diversity_policy_name,
    DROP COLUMN IF EXISTS diversity_policy_version,
    DROP COLUMN IF EXISTS diversity_mmr_lambda,
    DROP COLUMN IF EXISTS diversity_semantic_threshold,
    DROP COLUMN IF EXISTS diversity_max_candidates,
    DROP COLUMN IF EXISTS diversity_max_pairwise_comparisons,
    DROP COLUMN IF EXISTS diversity_max_embedding_dimensions,
    DROP COLUMN IF EXISTS diversity_max_citations_per_candidate,
    DROP COLUMN IF EXISTS diversity_coverage_weights;
