CREATE TABLE IF NOT EXISTS provider_sync_cursors (
    binding_id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    agent_id text NOT NULL,
    session_id text NOT NULL,
    provider_instance_id text NOT NULL,
    contract_version text NOT NULL,
    snapshot_watermark text NOT NULL,
    acknowledged_sequence bigint NOT NULL DEFAULT 0,
    issued_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT provider_sync_cursors_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT provider_sync_cursors_sequence_check CHECK (acknowledged_sequence >= 0)
);

CREATE INDEX IF NOT EXISTS provider_sync_cursors_scope_updated_idx
    ON provider_sync_cursors (tenant, project, namespace, updated_at DESC);
