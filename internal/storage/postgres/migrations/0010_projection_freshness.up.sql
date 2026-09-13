ALTER TABLE context_projections
    ADD COLUMN IF NOT EXISTS freshness_category text,
    ADD COLUMN IF NOT EXISTS freshness_slo text,
    ADD COLUMN IF NOT EXISTS freshness_age_ms bigint,
    ADD COLUMN IF NOT EXISTS freshness_duration_ms bigint,
    ADD COLUMN IF NOT EXISTS freshness_eligible boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS rebuild_checkpoint text,
    ADD COLUMN IF NOT EXISTS rebuild_required boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS context_projections_freshness_scope_idx
    ON context_projections (tenant, project, namespace, kind, freshness_eligible, updated_at DESC);
