DROP INDEX IF EXISTS job_executions_scope_job_window_idx;
DROP INDEX IF EXISTS job_executions_scope_lease_idx;
ALTER TABLE job_executions
    DROP COLUMN IF EXISTS error_category,
    DROP COLUMN IF EXISTS disposition,
    DROP COLUMN IF EXISTS source_watermark,
    DROP COLUMN IF EXISTS checkpoint,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS worker_id;
