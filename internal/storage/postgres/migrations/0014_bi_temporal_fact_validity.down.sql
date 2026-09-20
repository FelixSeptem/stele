DROP INDEX IF EXISTS temporal_corrections_scope_fact_created_at_idx;
DROP INDEX IF EXISTS canonical_memories_temporal_scope_validity_idx;
DROP INDEX IF EXISTS memory_versions_temporal_scope_validity_idx;
DROP TABLE IF EXISTS temporal_corrections;

ALTER TABLE embedding_rebuilds
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS temporal_fact_id;
ALTER TABLE context_projection_items
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS temporal_fact_id;
ALTER TABLE memory_chunk_derivations
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS temporal_fact_id;
ALTER TABLE relation_projections
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS temporal_fact_id,
    DROP COLUMN IF EXISTS source_version;
ALTER TABLE memory_versions
    DROP COLUMN IF EXISTS validity_source,
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS ingested_at,
    DROP COLUMN IF EXISTS temporal_fact_id;
ALTER TABLE canonical_memories
    DROP COLUMN IF EXISTS validity_source,
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS ingested_at,
    DROP COLUMN IF EXISTS temporal_fact_id;
