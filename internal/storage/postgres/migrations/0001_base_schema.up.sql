CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS raw_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    event_type text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_timestamp timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS canonical_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    state text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    search_text tsvector,
    embedding vector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memory_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    version bigint NOT NULL,
    state text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (memory_id, version)
);

CREATE TABLE IF NOT EXISTS provenance_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id uuid REFERENCES raw_events(id),
    memory_id uuid REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    operation text NOT NULL,
    request_id text,
    actor text,
    source_context jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deletion_markers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS raw_events_scope_created_at_idx
    ON raw_events (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS canonical_memories_scope_state_idx
    ON canonical_memories (tenant, project, namespace, state);

CREATE INDEX IF NOT EXISTS canonical_memories_updated_at_idx
    ON canonical_memories (updated_at DESC);

CREATE INDEX IF NOT EXISTS memory_versions_memory_created_at_idx
    ON memory_versions (memory_id, created_at DESC);

CREATE INDEX IF NOT EXISTS provenance_links_scope_created_at_idx
    ON provenance_links (tenant, project, namespace, created_at DESC);
