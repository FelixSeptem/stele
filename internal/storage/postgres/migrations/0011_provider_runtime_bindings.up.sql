CREATE TABLE IF NOT EXISTS provider_runtime_bindings (
    binding_id TEXT PRIMARY KEY,
    principal_id TEXT NOT NULL REFERENCES access_principals(id),
    tenant TEXT NOT NULL,
    project TEXT NOT NULL,
    namespace TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL DEFAULT '',
    provider_instance_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    CONSTRAINT provider_runtime_bindings_scope_identity UNIQUE (binding_id, tenant, project, namespace),
    CONSTRAINT provider_runtime_bindings_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS provider_runtime_bindings_principal_scope_idx
    ON provider_runtime_bindings (principal_id, tenant, project, namespace);
