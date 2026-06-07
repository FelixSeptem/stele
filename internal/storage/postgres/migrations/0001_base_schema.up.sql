CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS raw_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    event_type text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_timestamp timestamptz,
    governance_worker_id text,
    governance_claimed_at timestamptz,
    governance_lease_until timestamptz,
    governance_processed_at timestamptz,
    governance_attempt integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS candidate_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_raw_event_id uuid NOT NULL REFERENCES raw_events(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    content text NOT NULL,
    confidence double precision NOT NULL,
    importance double precision NOT NULL,
    freshness double precision NOT NULL,
    sensitivity text NOT NULL,
    mutability text NOT NULL,
    retention_class text NOT NULL,
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS canonical_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    state text NOT NULL,
    retention_class text NOT NULL DEFAULT 'durable',
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    search_text tsvector,
    embedding vector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE canonical_memories
    ADD COLUMN IF NOT EXISTS retention_class text NOT NULL DEFAULT 'durable';

CREATE TABLE IF NOT EXISTS memory_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    version bigint NOT NULL,
    state text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    modified_by text NOT NULL,
    UNIQUE (memory_id, version)
);

CREATE TABLE IF NOT EXISTS provenance_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id uuid REFERENCES raw_events(id),
    candidate_memory_id uuid REFERENCES candidate_memories(id),
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

CREATE TABLE IF NOT EXISTS relation_projections (
    memory_id uuid PRIMARY KEY REFERENCES canonical_memories(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_entity text NOT NULL,
    relation_type text NOT NULL,
    target_entity text NOT NULL,
    relation_text text NOT NULL,
    search_text tsvector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS job_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name text NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    trigger_source text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL,
    attempt integer NOT NULL DEFAULT 1,
    processed_count integer NOT NULL DEFAULT 0,
    error_message text,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE INDEX IF NOT EXISTS raw_events_scope_created_at_idx
    ON raw_events (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS candidate_memories_source_raw_event_idx
    ON candidate_memories (source_raw_event_id, created_at DESC);

CREATE INDEX IF NOT EXISTS candidate_memories_scope_status_idx
    ON candidate_memories (tenant, project, namespace, status, created_at DESC);

CREATE INDEX IF NOT EXISTS canonical_memories_scope_state_idx
    ON canonical_memories (tenant, project, namespace, state);

CREATE INDEX IF NOT EXISTS canonical_memories_updated_at_idx
    ON canonical_memories (updated_at DESC);

CREATE INDEX IF NOT EXISTS canonical_memories_search_text_idx
    ON canonical_memories
    USING GIN (search_text);

CREATE INDEX IF NOT EXISTS memory_versions_memory_created_at_idx
    ON memory_versions (memory_id, created_at DESC);

CREATE INDEX IF NOT EXISTS provenance_links_scope_created_at_idx
    ON provenance_links (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_scope_updated_at_idx
    ON relation_projections (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_search_text_idx
    ON relation_projections
    USING GIN (search_text);

CREATE INDEX IF NOT EXISTS job_executions_scope_started_at_idx
    ON job_executions (tenant, project, namespace, started_at DESC);

CREATE INDEX IF NOT EXISTS job_executions_job_name_started_at_idx
    ON job_executions (job_name, started_at DESC);
