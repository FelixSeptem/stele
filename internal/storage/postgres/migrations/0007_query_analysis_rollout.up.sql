ALTER TABLE ranking_rollout_policies
    ADD COLUMN IF NOT EXISTS query_analysis_session_id text,
    ADD COLUMN IF NOT EXISTS query_analysis_user_id text,
    ADD COLUMN IF NOT EXISTS query_analysis_policy jsonb;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ranking_rollout_policies_query_analysis_payload_check') THEN
        ALTER TABLE ranking_rollout_policies ADD CONSTRAINT ranking_rollout_policies_query_analysis_payload_check CHECK (
            query_analysis_policy IS NULL OR (jsonb_typeof(query_analysis_policy) = 'object' AND query_analysis_policy ? 'schema_version' AND query_analysis_policy->>'schema_version' = 'query-analysis-rollout-v1' AND query_analysis_policy ? 'policy_version' AND query_analysis_policy->>'policy_version' = 'query-analysis-v1' AND query_analysis_policy ? 'limits_version' AND query_analysis_policy->>'limits_version' = 'query-analysis-limits-v1')
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ranking_rollout_policies_query_analysis_selector_check') THEN
        ALTER TABLE ranking_rollout_policies ADD CONSTRAINT ranking_rollout_policies_query_analysis_selector_check CHECK (query_analysis_policy IS NOT NULL OR (query_analysis_session_id IS NULL AND query_analysis_user_id IS NULL));
    END IF;
END $$;
