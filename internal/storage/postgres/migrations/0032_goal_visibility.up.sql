CREATE TABLE IF NOT EXISTS goal_visibility_policies (
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    version text NOT NULL,
    owner text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    principal_grant text NOT NULL,
    scope_proof text NOT NULL,
    require_review boolean NOT NULL DEFAULT true,
    max_evidence_age_seconds bigint NOT NULL,
    source_watermark text NOT NULL,
    expires_at timestamptz NOT NULL,
    rolled_back boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant, project, namespace, version)
);

CREATE TABLE IF NOT EXISTS goal_visibility_decisions (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    goal_id text NOT NULL,
    policy_version text NOT NULL,
    replay_id text NOT NULL,
    disposition text NOT NULL,
    review_state text NOT NULL,
    freshness text NOT NULL,
    evidence_count integer NOT NULL,
    reason text NOT NULL,
    actor text,
    created_at timestamptz NOT NULL,
    UNIQUE (tenant, project, namespace, replay_id)
);

CREATE TABLE IF NOT EXISTS goal_visibility_lifecycle (
    id bigserial PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    decision_id text NOT NULL,
    from_disposition text,
    to_disposition text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS goal_visibility_decisions_scope_created_idx ON goal_visibility_decisions (tenant, project, namespace, created_at DESC);
