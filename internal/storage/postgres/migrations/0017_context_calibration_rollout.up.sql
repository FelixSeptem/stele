CREATE TABLE IF NOT EXISTS context_calibration_rollout_policies (
    policy_id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    session_id text,
    user_id text,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT context_calibration_rollout_policies_payload_check CHECK (
        jsonb_typeof(payload) = 'object'
        AND payload->>'schema_version' = 'context-calibration-rollout-v1'
        AND payload->>'policy_version' = 'context-calibration-v1'
    )
);

CREATE INDEX IF NOT EXISTS context_calibration_rollout_policies_scope_selector_idx
    ON context_calibration_rollout_policies (tenant, project, namespace, session_id, user_id, updated_at DESC);
