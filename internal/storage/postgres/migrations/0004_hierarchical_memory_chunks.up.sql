-- Versioned, derived memory chunks. These records are a PostgreSQL-only
-- retrieval projection; raw events and canonical memory versions remain the
-- source of truth.

CREATE TABLE IF NOT EXISTS memory_chunk_derivations (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_kind text NOT NULL CHECK (source_kind IN ('raw_event', 'canonical_version')),
    source_id text NOT NULL,
    source_version bigint NOT NULL CHECK (source_version > 0),
    parent_memory_id text,
    source_session_id text,
    source_user_id text,
    source_watermark jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_watermark_hash text NOT NULL,
    source_content_hash text NOT NULL,
    policy_version text NOT NULL,
    renderer_version text NOT NULL,
    counter_version text NOT NULL,
    lifecycle_state text NOT NULL CHECK (lifecycle_state = 'active'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant, project, namespace, source_kind, source_id, source_version,
            policy_version, renderer_version, counter_version, source_content_hash)
);

CREATE TABLE IF NOT EXISTS memory_chunk_items (
    id text PRIMARY KEY,
    derivation_id text NOT NULL REFERENCES memory_chunk_derivations(id) ON DELETE RESTRICT,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_session_id text,
    source_user_id text,
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    class text NOT NULL CHECK (class IN ('profile', 'episodic', 'procedural', 'summary', 'relation')),
    lifecycle_state text NOT NULL CHECK (lifecycle_state = 'active'),
    content text NOT NULL CHECK (length(content) > 0),
    source_start integer NOT NULL CHECK (source_start >= 0),
    source_end integer NOT NULL CHECK (source_end > source_start),
    character_count integer NOT NULL CHECK (character_count > 0),
    token_count integer NOT NULL CHECK (token_count > 0),
    content_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (derivation_id, ordinal),
    UNIQUE (derivation_id, source_start, source_end, content_hash)
);

CREATE INDEX IF NOT EXISTS memory_chunk_derivations_scope_source_idx
    ON memory_chunk_derivations (tenant, project, namespace, source_kind, source_id, source_version, created_at DESC);
CREATE INDEX IF NOT EXISTS memory_chunk_items_scope_derivation_ordinal_idx
    ON memory_chunk_items (tenant, project, namespace, derivation_id, ordinal ASC);
CREATE INDEX IF NOT EXISTS memory_chunk_items_scope_session_user_idx
    ON memory_chunk_items (tenant, project, namespace, source_session_id, source_user_id, created_at DESC);

CREATE OR REPLACE FUNCTION prevent_memory_chunk_derivation_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'memory chunk derivations are append-only';
END;
$$;

DROP TRIGGER IF EXISTS memory_chunk_derivations_append_only ON memory_chunk_derivations;
CREATE TRIGGER memory_chunk_derivations_append_only
    BEFORE UPDATE OR DELETE ON memory_chunk_derivations
    FOR EACH ROW EXECUTE FUNCTION prevent_memory_chunk_derivation_mutation();

CREATE OR REPLACE FUNCTION prevent_memory_chunk_item_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'memory chunk items are append-only';
END;
$$;

DROP TRIGGER IF EXISTS memory_chunk_items_append_only ON memory_chunk_items;
CREATE TRIGGER memory_chunk_items_append_only
    BEFORE UPDATE OR DELETE ON memory_chunk_items
    FOR EACH ROW EXECUTE FUNCTION prevent_memory_chunk_item_mutation();
