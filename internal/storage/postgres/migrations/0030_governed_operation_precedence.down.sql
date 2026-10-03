DROP INDEX IF EXISTS memory_intents_scope_precedence_idx;
ALTER TABLE memory_intents
    DROP CONSTRAINT IF EXISTS memory_intents_precedence_metadata_bounded_check;
ALTER TABLE memory_intents
    DROP COLUMN IF EXISTS precedence_outcome,
    DROP COLUMN IF EXISTS precedence_stage,
    DROP COLUMN IF EXISTS precedence_version;
