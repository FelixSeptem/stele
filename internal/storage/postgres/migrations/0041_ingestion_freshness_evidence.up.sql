CREATE TABLE provider_memory_read_observations (
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    memory_id uuid NOT NULL,
    observed_at timestamptz NOT NULL,
    PRIMARY KEY (tenant, project, namespace, memory_id, observed_at),
    FOREIGN KEY (memory_id) REFERENCES canonical_memories(id) ON DELETE CASCADE
);
CREATE INDEX provider_memory_read_observations_scope_memory_idx
    ON provider_memory_read_observations (tenant, project, namespace, memory_id, observed_at DESC);

CREATE TABLE assurance_ingestion_freshness_evidence (
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    fixture_hash text NOT NULL CHECK (fixture_hash ~ '^[0-9a-f]{64}$'),
    identity_hash text NOT NULL CHECK (identity_hash ~ '^[0-9a-f]{64}$'),
    report jsonb NOT NULL,
    observed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant, project, namespace, fixture_hash),
    CHECK (jsonb_typeof(report) = 'object')
);
CREATE INDEX assurance_ingestion_freshness_scope_created_idx
    ON assurance_ingestion_freshness_evidence (tenant, project, namespace, created_at DESC);
