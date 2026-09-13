DROP INDEX IF EXISTS context_projections_freshness_scope_idx;
ALTER TABLE context_projections
    DROP COLUMN IF EXISTS freshness_category,
    DROP COLUMN IF EXISTS freshness_slo,
    DROP COLUMN IF EXISTS freshness_age_ms,
    DROP COLUMN IF EXISTS freshness_duration_ms,
    DROP COLUMN IF EXISTS freshness_eligible,
    DROP COLUMN IF EXISTS rebuild_checkpoint,
    DROP COLUMN IF EXISTS rebuild_required;
