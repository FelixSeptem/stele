CREATE TABLE IF NOT EXISTS provider_lifecycle_operations (
 tenant TEXT NOT NULL, project TEXT NOT NULL, namespace TEXT NOT NULL, principal_id TEXT NOT NULL,
 idempotency_key TEXT NOT NULL, request_fingerprint TEXT NOT NULL, status TEXT NOT NULL CHECK (status IN ('pending','completed')),
 outcome JSONB NOT NULL DEFAULT '{}'::jsonb, completed_at TIMESTAMPTZ NULL,
 PRIMARY KEY (tenant,project,namespace,principal_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS provider_lifecycle_operations_pending_idx ON provider_lifecycle_operations (status);
