-- Immutable verified association from a corrected Provider projection revision
-- to the normal governed memory intent ledger. The projection remains evidence
-- only; it does not become a canonical-memory write authority.
CREATE TABLE memory_intent_correction_attributions (
    intent_id uuid PRIMARY KEY REFERENCES memory_intents(id),
    projection_id uuid NOT NULL,
    logical_id uuid NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_revision integer NOT NULL CHECK (source_revision > 1),
    binding_id text NOT NULL CHECK (octet_length(binding_id) BETWEEN 1 AND 256),
    principal_id text NOT NULL CHECK (octet_length(principal_id) BETWEEN 1 AND 256),
    target_memory_id uuid NOT NULL,
    target_version bigint NOT NULL CHECK (target_version > 0),
    attribution_state text NOT NULL CHECK (attribution_state = 'verified'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY (projection_id, logical_id, tenant, project, namespace)
        REFERENCES provider_projection_revisions(projection_id, logical_id, tenant, project, namespace),
    CHECK (projection_id <> '00000000-0000-0000-0000-000000000000'),
    CHECK (logical_id <> '00000000-0000-0000-0000-000000000000'),
    CHECK (target_memory_id <> '00000000-0000-0000-0000-000000000000')
);

CREATE INDEX memory_intent_correction_attribution_scope_target_idx
    ON memory_intent_correction_attributions (tenant, project, namespace, target_memory_id, target_version);
CREATE INDEX memory_intent_correction_attribution_binding_idx
    ON memory_intent_correction_attributions (binding_id, principal_id, created_at DESC);

CREATE OR REPLACE FUNCTION verify_memory_intent_correction_attribution_scope()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM memory_intents i
        WHERE i.id = NEW.intent_id
          AND i.tenant = NEW.tenant AND i.project = NEW.project AND i.namespace = NEW.namespace
          AND i.intent_type = 'update'
          AND i.target_memory_id::text = NEW.target_memory_id::text
          AND i.target_version = NEW.target_version
    ) THEN
        RAISE EXCEPTION 'correction attribution intent scope or target mismatch';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER memory_intent_correction_attribution_scope_guard
    BEFORE INSERT ON memory_intent_correction_attributions
    FOR EACH ROW EXECUTE FUNCTION verify_memory_intent_correction_attribution_scope();

CREATE TRIGGER memory_intent_correction_attribution_append_only
    BEFORE UPDATE OR DELETE ON memory_intent_correction_attributions
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
