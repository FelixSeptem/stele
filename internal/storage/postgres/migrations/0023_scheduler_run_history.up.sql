CREATE TABLE IF NOT EXISTS scheduler_run_summaries (
    run_key text PRIMARY KEY,
    job_class text NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    cadence_window timestamptz NOT NULL,
    state text NOT NULL,
    attempt_count integer NOT NULL DEFAULT 0,
    terminal_disposition text,
    recovery text NOT NULL DEFAULT 'none',
    checkpoint text,
    source_watermark text,
    freshness text NOT NULL DEFAULT 'unknown',
    slo text NOT NULL DEFAULT 'unknown',
    retry_exhausted boolean NOT NULL DEFAULT false,
    cleanup_state text NOT NULL DEFAULT 'retained',
    observed_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE TABLE IF NOT EXISTS scheduler_run_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_key text NOT NULL REFERENCES scheduler_run_summaries(run_key) ON DELETE CASCADE,
    attempt integer NOT NULL,
    state text NOT NULL,
    disposition text,
    worker_id text,
    lease_until timestamptz,
    checkpoint text,
    source_watermark text,
    retry_at timestamptz,
    recovery text NOT NULL DEFAULT 'none',
    error_category text,
    observed_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    detail_expires_at timestamptz,
    UNIQUE (run_key, attempt, state, observed_at)
);

CREATE INDEX IF NOT EXISTS scheduler_run_summaries_scope_observed_idx
    ON scheduler_run_summaries (tenant, project, namespace, observed_at DESC, run_key DESC);

CREATE INDEX IF NOT EXISTS scheduler_run_summaries_scope_state_idx
    ON scheduler_run_summaries (tenant, project, namespace, state, observed_at DESC, run_key DESC);

CREATE INDEX IF NOT EXISTS scheduler_run_attempts_retention_idx
    ON scheduler_run_attempts (detail_expires_at, run_key);
