ALTER TABLE ranking_rollout_policies
    ADD COLUMN IF NOT EXISTS retrieval_planner_session_id text,
    ADD COLUMN IF NOT EXISTS retrieval_planner_user_id text,
    ADD COLUMN IF NOT EXISTS retrieval_planner_policy jsonb;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ranking_rollout_policies_retrieval_planner_payload_check') THEN
        ALTER TABLE ranking_rollout_policies ADD CONSTRAINT ranking_rollout_policies_retrieval_planner_payload_check CHECK (
            retrieval_planner_policy IS NULL
            OR (
                jsonb_typeof(retrieval_planner_policy) = 'object'
                AND retrieval_planner_policy->>'schema_version' = 'retrieval-planner-rollout-v1'
                AND retrieval_planner_policy->>'planner_version' = 'retrieval-planner-v1'
                AND retrieval_planner_policy->>'policy_version' = 'retrieval-plan-policy-v1'
            )
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ranking_rollout_policies_retrieval_planner_selector_check') THEN
        ALTER TABLE ranking_rollout_policies ADD CONSTRAINT ranking_rollout_policies_retrieval_planner_selector_check CHECK (
            retrieval_planner_policy IS NOT NULL
            OR (retrieval_planner_session_id IS NULL AND retrieval_planner_user_id IS NULL)
        );
    END IF;
END $$;
