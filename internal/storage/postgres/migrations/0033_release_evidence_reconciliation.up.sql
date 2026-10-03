CREATE TABLE IF NOT EXISTS release_evidence_reconciliation_runs (
    id uuid PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_id text NOT NULL,
    replay_key text NOT NULL,
    source_watermark_hash text NOT NULL,
    checkpoint text NOT NULL DEFAULT '',
    status text NOT NULL,
    attempt integer NOT NULL DEFAULT 1,
    processed_count integer NOT NULL DEFAULT 0,
    reason_category text,
    actor text,
    reason text,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    retention_until timestamptz,
    UNIQUE (tenant, project, namespace, replay_key),
    CONSTRAINT release_evidence_reconciliation_runs_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT release_evidence_reconciliation_runs_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed', 'revoked'))
);

CREATE INDEX IF NOT EXISTS release_evidence_reconciliation_runs_scope_started_idx
    ON release_evidence_reconciliation_runs (tenant, project, namespace, started_at DESC);

CREATE TABLE IF NOT EXISTS release_evidence_reconciliation_history (
    id uuid PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_id text NOT NULL,
    handoff_identity text NOT NULL,
    replay_key text NOT NULL,
    scope_hash text NOT NULL,
    source_watermark_hash text NOT NULL,
    policy_version text NOT NULL,
    fixture_version text NOT NULL,
    representation_identity text NOT NULL,
    verdict text NOT NULL,
    reason_category text NOT NULL,
    observed_at timestamptz NOT NULL,
    actor text,
    job_run_id uuid REFERENCES release_evidence_reconciliation_runs(id) ON DELETE SET NULL,
    retention_until timestamptz,
    UNIQUE (tenant, project, namespace, replay_key),
    CONSTRAINT release_evidence_reconciliation_history_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT release_evidence_reconciliation_history_verdict_check CHECK (verdict IN ('eligible', 'revoked'))
);

CREATE INDEX IF NOT EXISTS release_evidence_reconciliation_history_scope_observed_idx
    ON release_evidence_reconciliation_history (tenant, project, namespace, observed_at DESC);

CREATE TABLE IF NOT EXISTS release_evidence_reconciliation_current (
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_id text NOT NULL,
    handoff_identity text NOT NULL,
    eligible boolean NOT NULL DEFAULT false,
    reason_category text NOT NULL,
    latest_history_id uuid NOT NULL REFERENCES release_evidence_reconciliation_history(id),
    observed_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant, project, namespace, policy_id, handoff_identity),
    CONSTRAINT release_evidence_reconciliation_current_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> '')
);

