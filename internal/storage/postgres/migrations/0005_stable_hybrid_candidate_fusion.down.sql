ALTER TABLE ranking_rollout_policies
    DROP CONSTRAINT IF EXISTS ranking_rollout_policies_fusion_parameters_check;

ALTER TABLE ranking_rollout_policies
    DROP COLUMN IF EXISTS fusion_strategy,
    DROP COLUMN IF EXISTS fusion_version,
    DROP COLUMN IF EXISTS fusion_rank_constant,
    DROP COLUMN IF EXISTS fusion_channel_weights,
    DROP COLUMN IF EXISTS fusion_per_channel_candidate,
    DROP COLUMN IF EXISTS fusion_total_candidates;
