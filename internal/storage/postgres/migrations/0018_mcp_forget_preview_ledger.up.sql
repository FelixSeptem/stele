ALTER TABLE access_scope_grants
    ADD COLUMN IF NOT EXISTS access_mode TEXT NOT NULL DEFAULT 'read_write';

ALTER TABLE access_scope_grants
    DROP CONSTRAINT IF EXISTS access_scope_grants_access_mode_check;

ALTER TABLE access_scope_grants
    ADD CONSTRAINT access_scope_grants_access_mode_check CHECK (access_mode IN ('read_only', 'read_write'));

CREATE TABLE IF NOT EXISTS mcp_forget_previews (
    preview_id TEXT PRIMARY KEY,
    principal_id TEXT NOT NULL,
    tenant TEXT NOT NULL,
    project TEXT NOT NULL,
    namespace TEXT NOT NULL,
    memory_ids JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS mcp_forget_previews_expiry_idx
    ON mcp_forget_previews (expires_at);

CREATE TABLE IF NOT EXISTS mcp_forget_apply_operations (
    tenant TEXT NOT NULL,
    project TEXT NOT NULL,
    namespace TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    claim_id TEXT NOT NULL,
    lease_until TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed')),
    outcome JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    PRIMARY KEY (tenant, project, namespace, principal_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS mcp_forget_apply_operations_pending_idx
    ON mcp_forget_apply_operations (lease_until) WHERE status = 'pending';
