-- Immutable association from one refuted procedural verification to an
-- ordinary governed update intent. Verification remains evidence only; the
-- existing intent worker remains the only canonical-successor path.
CREATE TABLE memory_intent_verification_attributions (
    intent_id uuid PRIMARY KEY REFERENCES memory_intents(id),
    verification_id uuid NOT NULL REFERENCES procedural_memory_verifications(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    target_memory_id uuid NOT NULL,
    target_version bigint NOT NULL CHECK (target_version > 0),
    source_watermark text NOT NULL CHECK (octet_length(source_watermark) BETWEEN 1 AND 256),
    policy_version text NOT NULL CHECK (octet_length(policy_version) BETWEEN 1 AND 256),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (target_memory_id <> '00000000-0000-0000-0000-000000000000')
);

CREATE UNIQUE INDEX memory_intent_verification_attribution_scope_verification_idx
    ON memory_intent_verification_attributions (tenant, project, namespace, verification_id);
CREATE INDEX memory_intent_verification_attribution_scope_target_idx
    ON memory_intent_verification_attributions (tenant, project, namespace, target_memory_id, target_version);

CREATE FUNCTION verify_memory_intent_verification_attribution_scope()
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
        RAISE EXCEPTION 'verification attribution intent scope or target mismatch';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM procedural_memory_verifications v
        WHERE v.id = NEW.verification_id
          AND v.tenant = NEW.tenant AND v.project = NEW.project AND v.namespace = NEW.namespace
          AND v.memory_id::text = NEW.target_memory_id::text
          AND v.target_version = NEW.target_version
          AND v.disposition = 'refuted'
          AND v.corrected_content IS NOT NULL
          AND v.source_watermark = NEW.source_watermark
          AND v.policy_version = NEW.policy_version
    ) THEN
        RAISE EXCEPTION 'verification attribution source scope, disposition, or target mismatch';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER memory_intent_verification_attribution_scope_guard
    BEFORE INSERT ON memory_intent_verification_attributions
    FOR EACH ROW EXECUTE FUNCTION verify_memory_intent_verification_attribution_scope();

CREATE TRIGGER memory_intent_verification_attribution_append_only
    BEFORE UPDATE OR DELETE ON memory_intent_verification_attributions
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
