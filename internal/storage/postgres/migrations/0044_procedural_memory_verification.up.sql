-- Procedural claim verification is an append-only audit ledger. Revalidation
-- execution is deliberately separate: it owns lease/retry state but never
-- rewrites a verification result.
CREATE TABLE procedural_memory_verifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    target_version bigint NOT NULL CHECK (target_version > 0),
    fingerprint text NOT NULL CHECK (length(fingerprint) = 64),
    claim_fingerprint text NOT NULL CHECK (length(claim_fingerprint) = 64),
    applicability_fingerprint text NOT NULL CHECK (length(applicability_fingerprint) = 64),
    disposition text NOT NULL CHECK (disposition IN ('supported', 'refuted', 'inconclusive', 'needs_revalidation')),
    evidence jsonb NOT NULL CHECK (jsonb_typeof(evidence) = 'array' AND jsonb_array_length(evidence) > 0),
    source_watermark text NOT NULL CHECK (octet_length(source_watermark) BETWEEN 1 AND 256),
    policy_version text NOT NULL CHECK (octet_length(policy_version) BETWEEN 1 AND 256),
    actor text NOT NULL CHECK (octet_length(actor) BETWEEN 1 AND 256),
    reason text NOT NULL CHECK (octet_length(reason) BETWEEN 1 AND 4096),
    corrected_content text,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (corrected_content IS NULL OR octet_length(corrected_content) <= 65536),
    CHECK (disposition = 'refuted' OR corrected_content IS NULL)
);

CREATE UNIQUE INDEX procedural_memory_verifications_scope_fingerprint_idx
    ON procedural_memory_verifications (tenant, project, namespace, fingerprint);
CREATE INDEX procedural_memory_verifications_scope_target_created_idx
    ON procedural_memory_verifications (tenant, project, namespace, memory_id, target_version, created_at DESC, id DESC);

CREATE FUNCTION verify_procedural_memory_verification_scope()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM canonical_memories m
        JOIN memory_versions v ON v.memory_id = m.id AND v.version = NEW.target_version
        WHERE m.id = NEW.memory_id
          AND m.tenant = NEW.tenant AND m.project = NEW.project AND m.namespace = NEW.namespace
          AND m.class = 'procedural'
    ) THEN
        RAISE EXCEPTION 'procedural verification target scope or version mismatch';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER procedural_memory_verifications_scope_guard
    BEFORE INSERT ON procedural_memory_verifications
    FOR EACH ROW EXECUTE FUNCTION verify_procedural_memory_verification_scope();

CREATE TRIGGER procedural_memory_verifications_append_only
    BEFORE UPDATE OR DELETE ON procedural_memory_verifications
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

CREATE TABLE procedural_memory_revalidation_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    verification_id uuid NOT NULL REFERENCES procedural_memory_verifications(id),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    target_version bigint NOT NULL CHECK (target_version > 0),
    source_watermark text NOT NULL CHECK (octet_length(source_watermark) BETWEEN 1 AND 256),
    trigger text NOT NULL CHECK (trigger IN ('administrator', 'source_drift', 'correction_signal', 'stale_feedback', 'needs_review_feedback')),
    work_identity text NOT NULL CHECK (octet_length(work_identity) BETWEEN 1 AND 256),
    status text NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'exhausted')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts BETWEEN 1 AND 100),
    lease_owner text,
    lease_expires_at timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    terminal_verification_id uuid REFERENCES procedural_memory_verifications(id),
    failure_category text,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    terminal_at timestamptz,
    CHECK (attempt_count <= max_attempts),
    CHECK ((status = 'running') = (lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)),
    CHECK ((status IN ('completed', 'failed', 'exhausted')) = (terminal_at IS NOT NULL))
);

CREATE UNIQUE INDEX procedural_memory_revalidation_runs_scope_work_identity_idx
    ON procedural_memory_revalidation_runs (tenant, project, namespace, work_identity);
CREATE INDEX procedural_memory_revalidation_runs_claim_idx
    ON procedural_memory_revalidation_runs (tenant, project, namespace, status, next_attempt_at, lease_expires_at, created_at, id);
CREATE INDEX procedural_memory_revalidation_runs_scope_target_created_idx
    ON procedural_memory_revalidation_runs (tenant, project, namespace, memory_id, target_version, created_at DESC, id DESC);

CREATE FUNCTION verify_procedural_revalidation_run_scope()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM procedural_memory_verifications v
        WHERE v.id = NEW.verification_id
          AND v.tenant = NEW.tenant AND v.project = NEW.project AND v.namespace = NEW.namespace
          AND v.memory_id = NEW.memory_id AND v.target_version = NEW.target_version
    ) THEN
        RAISE EXCEPTION 'procedural revalidation run scope or target mismatch';
    END IF;
    IF NEW.terminal_verification_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM procedural_memory_verifications v
        WHERE v.id = NEW.terminal_verification_id
          AND v.tenant = NEW.tenant AND v.project = NEW.project AND v.namespace = NEW.namespace
          AND v.memory_id = NEW.memory_id AND v.target_version = NEW.target_version
    ) THEN
        RAISE EXCEPTION 'procedural revalidation terminal verification scope mismatch';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER procedural_memory_revalidation_runs_scope_guard
    BEFORE INSERT OR UPDATE ON procedural_memory_revalidation_runs
    FOR EACH ROW EXECUTE FUNCTION verify_procedural_revalidation_run_scope();
