ALTER TABLE raw_events ADD COLUMN IF NOT EXISTS memory_path text NOT NULL DEFAULT '/';
ALTER TABLE candidate_memories ADD COLUMN IF NOT EXISTS memory_path text NOT NULL DEFAULT '/';
ALTER TABLE canonical_memories ADD COLUMN IF NOT EXISTS memory_path text NOT NULL DEFAULT '/';
ALTER TABLE memory_versions ADD COLUMN IF NOT EXISTS memory_path text NOT NULL DEFAULT '/';
ALTER TABLE memory_intents ADD COLUMN IF NOT EXISTS memory_path text NOT NULL DEFAULT '/';

UPDATE raw_events SET memory_path = '/' WHERE memory_path IS NULL OR btrim(memory_path) = '';
UPDATE candidate_memories SET memory_path = '/' WHERE memory_path IS NULL OR btrim(memory_path) = '';
UPDATE canonical_memories SET memory_path = '/' WHERE memory_path IS NULL OR btrim(memory_path) = '';
UPDATE memory_versions SET memory_path = '/' WHERE memory_path IS NULL OR btrim(memory_path) = '';
UPDATE memory_intents SET memory_path = '/' WHERE memory_path IS NULL OR btrim(memory_path) = '';

CREATE INDEX IF NOT EXISTS raw_events_scope_path_created_at_idx
    ON raw_events (tenant, project, namespace, memory_path text_pattern_ops, created_at DESC);
CREATE INDEX IF NOT EXISTS candidate_memories_scope_path_updated_at_idx
    ON candidate_memories (tenant, project, namespace, memory_path text_pattern_ops, updated_at DESC);
CREATE INDEX IF NOT EXISTS canonical_memories_scope_path_updated_at_idx
    ON canonical_memories (tenant, project, namespace, memory_path text_pattern_ops, updated_at DESC);
CREATE INDEX IF NOT EXISTS memory_versions_memory_path_created_at_idx
    ON memory_versions (memory_id, memory_path text_pattern_ops, created_at DESC);
CREATE INDEX IF NOT EXISTS memory_intents_scope_path_created_at_idx
    ON memory_intents (tenant, project, namespace, memory_path text_pattern_ops, created_at DESC);
