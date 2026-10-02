DROP INDEX IF EXISTS scheduler_run_attempts_retention_idx;
DROP INDEX IF EXISTS scheduler_run_summaries_scope_state_idx;
DROP INDEX IF EXISTS scheduler_run_summaries_scope_observed_idx;
DROP TABLE IF EXISTS scheduler_run_attempts;
DROP TABLE IF EXISTS scheduler_run_summaries;
