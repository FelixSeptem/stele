ALTER TABLE memory_intents
    ADD COLUMN IF NOT EXISTS precedence_version text NOT NULL DEFAULT 'governed-operation-precedence-v1',
    ADD COLUMN IF NOT EXISTS precedence_stage text NOT NULL DEFAULT 'mutation',
    ADD COLUMN IF NOT EXISTS precedence_outcome text NOT NULL DEFAULT 'accepted';

ALTER TABLE memory_intents
    ADD CONSTRAINT memory_intents_precedence_metadata_bounded_check
    CHECK (length(precedence_version) BETWEEN 1 AND 128 AND length(precedence_stage) BETWEEN 1 AND 64 AND length(precedence_outcome) BETWEEN 1 AND 64);

CREATE INDEX IF NOT EXISTS memory_intents_scope_precedence_idx
    ON memory_intents (tenant, project, namespace, precedence_stage, precedence_outcome, created_at DESC);
