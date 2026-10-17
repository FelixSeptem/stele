CREATE TABLE memory_repositories (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    name text NOT NULL CHECK (octet_length(name) BETWEEN 1 AND 256),
    source_url text NOT NULL DEFAULT '' CHECK (octet_length(source_url) <= 2048),
    generation bigint NOT NULL DEFAULT 0 CHECK (generation >= 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(id, tenant, project, namespace)
);
CREATE TABLE memory_repository_revisions (
    repository_id uuid NOT NULL, tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    claimed_revision text NOT NULL CHECK (octet_length(claimed_revision) BETWEEN 1 AND 256),
    snapshot_digest text NOT NULL CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
    observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(repository_id, claimed_revision),
    FOREIGN KEY(repository_id,tenant,project,namespace) REFERENCES memory_repositories(id,tenant,project,namespace)
);
CREATE TABLE memory_repository_previews (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    repository_id uuid NOT NULL, tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    generation bigint NOT NULL CHECK (generation >= 0), snapshot_digest text NOT NULL CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
    plan_digest text NOT NULL CHECK (plan_digest ~ '^[0-9a-f]{64}$'), warning_digest text NOT NULL CHECK (warning_digest ~ '^[0-9a-f]{64}$'),
    parser_version text NOT NULL CHECK (parser_version='memory-repo-parser-v1'),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object' AND octet_length(payload::text) <= 2097152),
    expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY(repository_id,tenant,project,namespace) REFERENCES memory_repositories(id,tenant,project,namespace)
);
CREATE TABLE memory_repository_imports (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    repository_id uuid NOT NULL, tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    preview_id uuid NOT NULL, generation bigint NOT NULL CHECK (generation > 0), idempotency_key text NOT NULL CHECK (octet_length(idempotency_key) BETWEEN 1 AND 256),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    snapshot_digest text NOT NULL CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
    plan_digest text NOT NULL CHECK (plan_digest ~ '^[0-9a-f]{64}$'),
    warning_digest text NOT NULL CHECK (warning_digest ~ '^[0-9a-f]{64}$'),
    plan_payload jsonb NOT NULL CHECK (jsonb_typeof(plan_payload)='object' AND octet_length(plan_payload::text) <= 2097152),
    status text NOT NULL CHECK (status IN ('queued','running','completed')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), completed_at timestamptz,
    UNIQUE(repository_id,idempotency_key), UNIQUE(repository_id,generation),
    FOREIGN KEY(repository_id,tenant,project,namespace) REFERENCES memory_repositories(id,tenant,project,namespace),
    FOREIGN KEY(preview_id) REFERENCES memory_repository_previews(id)
);
CREATE TABLE memory_repository_import_entries (
    import_id uuid NOT NULL, entry_key text NOT NULL CHECK (entry_key ~ '^[0-9a-f]{64}$'),
    dispatch_status text NOT NULL CHECK (dispatch_status IN ('queued','submitted','duplicate','rejected','failed')),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object'), intent_id uuid,
    PRIMARY KEY(import_id,entry_key), FOREIGN KEY(import_id) REFERENCES memory_repository_imports(id)
);
CREATE TABLE memory_repository_import_attempts (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    import_id uuid NOT NULL, entry_key text NOT NULL CHECK (entry_key ~ '^[0-9a-f]{64}$'),
    sequence integer NOT NULL CHECK (sequence > 0),
    outcome text NOT NULL CHECK (outcome IN ('queued','submitted','duplicate','rejected','failed')),
    diagnostic text NOT NULL DEFAULT '' CHECK (octet_length(diagnostic) <= 1024),
    intent_id uuid, occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(import_id,entry_key,sequence),
    FOREIGN KEY(import_id,entry_key) REFERENCES memory_repository_import_entries(import_id,entry_key)
);
CREATE TABLE memory_repository_entry_tombstones (
    repository_id uuid NOT NULL, tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    entry_key text NOT NULL CHECK (entry_key ~ '^[0-9a-f]{64}$'), accepted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(repository_id,entry_key), FOREIGN KEY(repository_id,tenant,project,namespace) REFERENCES memory_repositories(id,tenant,project,namespace)
);
CREATE INDEX memory_repository_previews_expiry ON memory_repository_previews(expires_at);
CREATE INDEX memory_repository_imports_scope_created ON memory_repository_imports(tenant,project,namespace,created_at DESC);
CREATE INDEX memory_repository_import_attempts_import ON memory_repository_import_attempts(import_id,entry_key,sequence);
