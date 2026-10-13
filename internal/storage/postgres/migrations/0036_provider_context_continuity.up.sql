CREATE TABLE provider_context_receipts (
    reference text PRIMARY KEY CHECK (reference ~ '^ctxc_[0-9a-f]{32}$'),
    binding_id text NOT NULL,
    principal_id text NOT NULL,
    tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    agent_id text NOT NULL, session_id text NOT NULL,
    conversation_id text NOT NULL, provider_instance_id text NOT NULL,
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    digest_contract text NOT NULL, canonical_version text NOT NULL,
    digest_algorithm text NOT NULL, selection_version text NOT NULL,
    selection_fingerprint text NOT NULL CHECK (selection_fingerprint ~ '^[0-9a-f]{64}$'),
    digest_value text NOT NULL CHECK (digest_value ~ '^[0-9a-f]{64}$'),
    issued_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
    CHECK (expires_at > issued_at)
);
CREATE INDEX provider_context_receipt_lookup ON provider_context_receipts
 (binding_id, principal_id, tenant, project, namespace, agent_id, session_id, conversation_id, provider_instance_id, reference);
CREATE INDEX provider_context_receipt_dedup ON provider_context_receipts
 (binding_id, request_fingerprint, digest_contract, canonical_version, digest_value, expires_at);
CREATE INDEX provider_context_receipt_expiry ON provider_context_receipts (expires_at, reference);
-- One durable cross-process resource lock; no process-local capacity authority.
CREATE TABLE provider_context_receipt_capacity (singleton boolean PRIMARY KEY CHECK(singleton));
INSERT INTO provider_context_receipt_capacity VALUES (true);
