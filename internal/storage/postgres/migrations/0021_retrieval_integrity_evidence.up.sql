CREATE TABLE IF NOT EXISTS retrieval_integrity_reports (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    fixture_version text NOT NULL,
    policy_version text NOT NULL,
    strategy_version text NOT NULL,
    renderer_version text NOT NULL,
    provider_version text NOT NULL,
    source_watermark text NOT NULL,
    verdict text NOT NULL,
    action_category text NOT NULL,
    action_success boolean NOT NULL,
    integrity_success boolean NOT NULL,
    aggregate jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS retrieval_integrity_trajectories (
    id text PRIMARY KEY,
    report_id text REFERENCES retrieval_integrity_reports(id) ON DELETE RESTRICT,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    fixture_version text NOT NULL,
    policy_version text NOT NULL,
    strategy_version text NOT NULL,
    renderer_version text NOT NULL,
    provider_version text NOT NULL,
    source_watermark text NOT NULL,
    aggregate jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS retrieval_integrity_retention_outcomes (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    artifact_category text NOT NULL,
    result text NOT NULL,
    deleted_count integer NOT NULL DEFAULT 0,
    reason_category text NOT NULL DEFAULT 'expired',
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS retrieval_integrity_reports_scope_created_at_idx
    ON retrieval_integrity_reports (tenant, project, namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS retrieval_integrity_trajectories_scope_created_at_idx
    ON retrieval_integrity_trajectories (tenant, project, namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS retrieval_integrity_retention_scope_created_at_idx
    ON retrieval_integrity_retention_outcomes (tenant, project, namespace, created_at DESC);
