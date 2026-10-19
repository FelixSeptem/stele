-- Provider-created turns receive an immutable full-binding ownership record.
-- Generic turns remain unassociated and cannot be claimed retroactively.
CREATE TABLE provider_context_turn_bindings (
    turn_id text PRIMARY KEY REFERENCES memory_session_turns(id) ON DELETE CASCADE,
    binding_id text NOT NULL,
    principal_id text NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    agent_id text NOT NULL,
    session_id text NOT NULL,
    conversation_id text NOT NULL,
    provider_instance_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (turn_id, binding_id, principal_id, tenant, project, namespace,
        agent_id, session_id, conversation_id, provider_instance_id)
);
CREATE INDEX provider_context_turn_bindings_exact_idx ON provider_context_turn_bindings
    (turn_id, binding_id, principal_id, tenant, project, namespace, agent_id,
     session_id, conversation_id, provider_instance_id);

ALTER TABLE provider_context_receipts ADD COLUMN turn_id text NULL;
CREATE INDEX provider_context_receipts_turn_idx ON provider_context_receipts
    (turn_id, binding_id, session_id, tenant, project, namespace)
    WHERE turn_id IS NOT NULL;

CREATE TABLE provider_context_turn_attributions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    turn_id text NOT NULL,
    binding_id text NOT NULL,
    principal_id text NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    agent_id text NOT NULL,
    session_id text NOT NULL,
    conversation_id text NOT NULL,
    provider_instance_id text NOT NULL,
    request_id text NOT NULL,
    operation_id text NOT NULL,
    digest_contract text NOT NULL,
    canonical_version text NOT NULL,
    digest_value text NOT NULL,
    continuity_reference text NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX provider_context_turn_attributions_lookup_idx
    ON provider_context_turn_attributions (turn_id, binding_id, created_at, id);
