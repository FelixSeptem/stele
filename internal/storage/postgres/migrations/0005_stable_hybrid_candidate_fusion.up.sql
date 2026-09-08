ALTER TABLE ranking_rollout_policies
    ADD COLUMN IF NOT EXISTS fusion_strategy text,
    ADD COLUMN IF NOT EXISTS fusion_version text,
    ADD COLUMN IF NOT EXISTS fusion_rank_constant integer,
    ADD COLUMN IF NOT EXISTS fusion_channel_weights jsonb,
    ADD COLUMN IF NOT EXISTS fusion_per_channel_candidate integer,
    ADD COLUMN IF NOT EXISTS fusion_total_candidates integer;

ALTER TABLE ranking_rollout_policies
    ADD CONSTRAINT ranking_rollout_policies_fusion_parameters_check
    CHECK (
        fusion_strategy IS NULL
        OR (
            fusion_strategy IN ('rrf', 'normalized_weighted')
            AND fusion_version IS NOT NULL
            AND fusion_rank_constant BETWEEN 1 AND 10000
            AND fusion_per_channel_candidate BETWEEN 1 AND 1000
            AND fusion_total_candidates BETWEEN 1 AND 5000
            AND jsonb_typeof(fusion_channel_weights) = 'object'
        )
    );
