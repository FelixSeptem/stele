ALTER TABLE job_executions
    ADD COLUMN IF NOT EXISTS worker_id text,
    ADD COLUMN IF NOT EXISTS lease_until timestamptz,
    ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz,
    ADD COLUMN IF NOT EXISTS checkpoint text,
    ADD COLUMN IF NOT EXISTS source_watermark text,
    ADD COLUMN IF NOT EXISTS disposition text,
    ADD COLUMN IF NOT EXISTS error_category text;

CREATE INDEX IF NOT EXISTS job_executions_scope_lease_idx
    ON job_executions (tenant, project, namespace, lease_until, started_at);

CREATE INDEX IF NOT EXISTS job_executions_scope_job_window_idx
    ON job_executions (tenant, project, namespace, job_name, started_at DESC);
