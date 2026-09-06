-- Governed memory intent, durable reflection, review, and compaction evidence
-- records. These tables are append-only at their audit boundaries; mutable
-- worker state is restricted to lease/checkpoint columns.

CREATE TABLE IF NOT EXISTS memory_intents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    intent_type text NOT NULL CHECK (intent_type IN ('remember', 'update', 'forget', 'contradiction', 'feedback')),
    actor text NOT NULL,
    reason text NOT NULL,
    provenance jsonb NOT NULL DEFAULT '{}'::jsonb,
    request_id text NOT NULL,
    operation_id text NOT NULL,
    idempotency_key text NOT NULL,
    request_fingerprint text NOT NULL,
    target_memory_id text,
    target_version bigint,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('pending', 'accepted', 'candidate', 'active', 'suppressed', 'rejected', 'failed')),
    candidate_memory_id uuid REFERENCES candidate_memories(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

CREATE TABLE IF NOT EXISTS reflection_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('session_completion', 'event_threshold', 'compaction_pressure', 'schedule', 'operator')),
    input_watermark jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_watermark_hash text NOT NULL,
    transcript_schema_version text NOT NULL,
    processed_offset bigint NOT NULL DEFAULT 0 CHECK (processed_offset >= 0),
    lease_owner text,
    lease_until timestamptz,
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    retry_budget integer NOT NULL DEFAULT 3 CHECK (retry_budget >= 0),
    status text NOT NULL CHECK (status IN ('pending', 'leased', 'running', 'completed', 'failed', 'exhausted', 'cancelled')),
    failure_category text,
    outputs jsonb NOT NULL DEFAULT '{}'::jsonb,
    evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    replay_of uuid REFERENCES reflection_runs(id),
    replay_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS reflection_run_checkpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES reflection_runs(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    processed_offset bigint NOT NULL CHECK (processed_offset >= 0),
    input_watermark jsonb NOT NULL DEFAULT '{}'::jsonb,
    output_hash text,
    committed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, processed_offset)
);

CREATE TABLE IF NOT EXISTS reflection_review_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid REFERENCES reflection_runs(id),
    candidate_memory_id uuid REFERENCES candidate_memories(id),
    intent_id uuid REFERENCES memory_intents(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    decision text NOT NULL CHECK (decision IN ('accept', 'suppress', 'reject', 'request_more_evidence')),
    reviewer text NOT NULL,
    reason text NOT NULL,
    policy_version text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (run_id IS NOT NULL OR candidate_memory_id IS NOT NULL OR intent_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS compaction_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('token_limit', 'manual', 'scheduled', 'session_completion', 'reflection')),
    source_session_id text,
    source_conversation_id text,
    source_watermark jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_watermark_hash text NOT NULL,
    source_raw_event_start uuid,
    source_raw_event_end uuid,
    canonical_version_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    derivation_version text NOT NULL,
    input_token_estimate integer NOT NULL DEFAULT 0 CHECK (input_token_estimate >= 0),
    output_token_estimate integer NOT NULL DEFAULT 0 CHECK (output_token_estimate >= 0),
    evidence_coverage jsonb NOT NULL DEFAULT '{}'::jsonb,
    recent_tail_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    summary_version bigint NOT NULL DEFAULT 1 CHECK (summary_version > 0),
    state text NOT NULL CHECK (state IN ('building', 'active', 'stale', 'superseded', 'failed')),
    reflection_run_id uuid REFERENCES reflection_runs(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS memory_intents_scope_idempotency_idx
    ON memory_intents (tenant, project, namespace, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS memory_intents_scope_operation_idx
    ON memory_intents (tenant, project, namespace, operation_id);
CREATE INDEX IF NOT EXISTS memory_intents_scope_status_created_idx
    ON memory_intents (tenant, project, namespace, status, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS reflection_runs_scope_replay_key_idx
    ON reflection_runs (tenant, project, namespace, replay_key)
    WHERE replay_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS reflection_runs_scope_input_dedup_idx
    ON reflection_runs (tenant, project, namespace, trigger, input_watermark_hash, transcript_schema_version)
    WHERE replay_of IS NULL;
CREATE INDEX IF NOT EXISTS reflection_runs_scope_status_lease_idx
    ON reflection_runs (tenant, project, namespace, status, lease_until, created_at ASC);
CREATE INDEX IF NOT EXISTS reflection_run_checkpoints_run_offset_idx
    ON reflection_run_checkpoints (run_id, processed_offset DESC);
CREATE INDEX IF NOT EXISTS reflection_review_decisions_scope_created_idx
    ON reflection_review_decisions (tenant, project, namespace, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS compaction_evidence_scope_source_dedup_idx
    ON compaction_evidence (tenant, project, namespace, source_watermark_hash, derivation_version, summary_version);
CREATE INDEX IF NOT EXISTS compaction_evidence_scope_state_updated_idx
    ON compaction_evidence (tenant, project, namespace, state, updated_at DESC);
CREATE INDEX IF NOT EXISTS compaction_evidence_reflection_run_idx
    ON compaction_evidence (reflection_run_id, created_at DESC);

CREATE OR REPLACE FUNCTION prevent_governed_audit_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% records are append-only', TG_TABLE_NAME;
END;
$$;

DROP TRIGGER IF EXISTS memory_intents_append_only ON memory_intents;
CREATE TRIGGER memory_intents_append_only
    BEFORE UPDATE OR DELETE ON memory_intents
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

DROP TRIGGER IF EXISTS reflection_run_checkpoints_append_only ON reflection_run_checkpoints;
CREATE TRIGGER reflection_run_checkpoints_append_only
    BEFORE UPDATE OR DELETE ON reflection_run_checkpoints
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

DROP TRIGGER IF EXISTS reflection_review_decisions_append_only ON reflection_review_decisions;
CREATE TRIGGER reflection_review_decisions_append_only
    BEFORE UPDATE OR DELETE ON reflection_review_decisions
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

CREATE OR REPLACE FUNCTION prevent_reflection_run_identity_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id OR NEW.tenant <> OLD.tenant OR NEW.project <> OLD.project
       OR NEW.namespace <> OLD.namespace OR NEW.trigger <> OLD.trigger
       OR NEW.input_watermark <> OLD.input_watermark
       OR NEW.input_watermark_hash <> OLD.input_watermark_hash
       OR NEW.transcript_schema_version <> OLD.transcript_schema_version
       OR NEW.replay_of IS DISTINCT FROM OLD.replay_of
       OR NEW.replay_key IS DISTINCT FROM OLD.replay_key
       OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'reflection run identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS reflection_runs_identity_guard ON reflection_runs;
CREATE TRIGGER reflection_runs_identity_guard
    BEFORE UPDATE ON reflection_runs
    FOR EACH ROW EXECUTE FUNCTION prevent_reflection_run_identity_mutation();

CREATE OR REPLACE FUNCTION prevent_compaction_evidence_identity_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id OR NEW.tenant <> OLD.tenant OR NEW.project <> OLD.project
       OR NEW.namespace <> OLD.namespace OR NEW.trigger <> OLD.trigger
       OR NEW.source_watermark <> OLD.source_watermark
       OR NEW.source_watermark_hash <> OLD.source_watermark_hash
       OR NEW.derivation_version <> OLD.derivation_version
       OR NEW.summary_version <> OLD.summary_version
       OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'compaction evidence identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS compaction_evidence_identity_guard ON compaction_evidence;
CREATE TRIGGER compaction_evidence_identity_guard
    BEFORE UPDATE ON compaction_evidence
    FOR EACH ROW EXECUTE FUNCTION prevent_compaction_evidence_identity_mutation();
