ALTER TABLE canonical_memories
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS temporal_head_version bigint,
    ADD COLUMN IF NOT EXISTS ingested_at timestamptz,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz,
    ADD COLUMN IF NOT EXISTS validity_source text NOT NULL DEFAULT 'legacy_current_compatible';

ALTER TABLE memory_versions
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS ingested_at timestamptz,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz,
    ADD COLUMN IF NOT EXISTS validity_source text NOT NULL DEFAULT 'legacy_current_compatible';

UPDATE memory_versions
SET temporal_fact_id = COALESCE(temporal_fact_id, memory_id::text),
    ingested_at = COALESCE(ingested_at, created_at),
    valid_from = COALESCE(valid_from, created_at),
    validity_source = COALESCE(NULLIF(validity_source, ''), 'legacy_current_compatible')
WHERE temporal_fact_id IS NULL OR ingested_at IS NULL OR valid_from IS NULL OR validity_source IS NULL OR validity_source = '';

UPDATE canonical_memories cm
SET temporal_fact_id = COALESCE(cm.temporal_fact_id, cm.id::text),
    ingested_at = COALESCE(cm.ingested_at, cm.created_at),
    valid_from = COALESCE(cm.valid_from, cm.created_at),
    validity_source = COALESCE(NULLIF(cm.validity_source, ''), 'legacy_current_compatible')
WHERE cm.temporal_fact_id IS NULL OR cm.ingested_at IS NULL OR cm.valid_from IS NULL OR cm.validity_source IS NULL OR cm.validity_source = '';

CREATE TABLE IF NOT EXISTS temporal_corrections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    temporal_fact_id text NOT NULL,
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    predecessor_version bigint,
    successor_version bigint NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    disposition text NOT NULL CHECK (disposition IN ('none', 'rejected', 'conflict_open', 'resolved')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant, project, namespace, memory_id, successor_version)
);

ALTER TABLE relation_projections
    ADD COLUMN IF NOT EXISTS source_version bigint,
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz;

ALTER TABLE memory_chunk_derivations
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz;

ALTER TABLE context_projection_items
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz;

ALTER TABLE embedding_rebuilds
    ADD COLUMN IF NOT EXISTS temporal_fact_id text,
    ADD COLUMN IF NOT EXISTS valid_from timestamptz,
    ADD COLUMN IF NOT EXISTS valid_to timestamptz;

CREATE INDEX IF NOT EXISTS memory_versions_temporal_scope_validity_idx
    ON memory_versions (temporal_fact_id, valid_from, valid_to, created_at DESC);
CREATE INDEX IF NOT EXISTS canonical_memories_temporal_scope_validity_idx
    ON canonical_memories (tenant, project, namespace, temporal_fact_id, valid_from, valid_to);
CREATE INDEX IF NOT EXISTS temporal_corrections_scope_fact_created_at_idx
    ON temporal_corrections (tenant, project, namespace, temporal_fact_id, created_at DESC);
