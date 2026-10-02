-- Governed intent lifecycle is recorded as immutable request rows plus an
-- append-only transition ledger. Payloads and diagnostics stay bounded.
ALTER TABLE memory_intents
    DROP CONSTRAINT IF EXISTS memory_intents_status_check;
ALTER TABLE memory_intents
    ADD CONSTRAINT memory_intents_status_check
    CHECK (status IN ('pending', 'accepted', 'candidate', 'active', 'suppressed', 'rejected', 'failed', 'replayed'));

ALTER TABLE memory_intents
    ADD COLUMN IF NOT EXISTS target_insight_id text,
    ADD COLUMN IF NOT EXISTS evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS outcome_reference text,
    ADD COLUMN IF NOT EXISTS policy_version text;

CREATE TABLE IF NOT EXISTS memory_intent_transitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    intent_id uuid NOT NULL REFERENCES memory_intents(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    from_status text,
    to_status text NOT NULL CHECK (to_status IN ('pending', 'accepted', 'candidate', 'active', 'suppressed', 'rejected', 'failed', 'replayed')),
    actor text NOT NULL,
    reason text NOT NULL,
    diagnostic_category text NOT NULL CHECK (diagnostic_category IN ('accepted', 'pending', 'rejected', 'suppressed', 'failed', 'replayed', 'scope_denied', 'target_stale', 'evidence_incomplete', 'policy_disabled', 'retry_exhausted', 'rolled_back')),
    work_reference text,
    outcome_reference text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (intent_id, sequence),
    CHECK (length(actor) <= 256),
    CHECK (length(reason) <= 2048),
    CHECK (work_reference IS NULL OR length(work_reference) <= 256),
    CHECK (outcome_reference IS NULL OR length(outcome_reference) <= 256)
);

CREATE INDEX IF NOT EXISTS memory_intent_transitions_scope_created_idx
    ON memory_intent_transitions (tenant, project, namespace, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS memory_intent_transitions_intent_sequence_idx
    ON memory_intent_transitions (intent_id, sequence ASC);

DROP TRIGGER IF EXISTS memory_intent_transitions_append_only ON memory_intent_transitions;
CREATE TRIGGER memory_intent_transitions_append_only
    BEFORE UPDATE OR DELETE ON memory_intent_transitions
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

ALTER TABLE memory_intents
    ADD CONSTRAINT memory_intents_payload_bounded_check
    CHECK (octet_length(payload::text) <= 131072 AND octet_length(provenance::text) <= 32768 AND octet_length(evidence_refs::text) <= 32768);
