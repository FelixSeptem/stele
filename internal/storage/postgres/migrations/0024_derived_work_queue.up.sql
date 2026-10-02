-- Derived work is a reference-only queue. Raw content, canonical memory
-- payloads, checkpoints, and evidence remain in their owning tables.
CREATE TABLE IF NOT EXISTS derived_work_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_key text NOT NULL UNIQUE,
    kind text NOT NULL CHECK (kind IN ('reflection', 'compaction', 'projection_rebuild', 'insight_maintenance', 'freshness_retention')),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    watermark text NOT NULL,
    idempotency_key text NOT NULL,
    reference text NOT NULL,
    state text NOT NULL CHECK (state IN ('queued', 'claimed', 'running', 'retry', 'completed', 'exhausted', 'cancelled', 'dropped')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts > 0),
    lease_owner text,
    lease_until timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    failure_category text,
    loss_disposition text NOT NULL DEFAULT 'none' CHECK (loss_disposition IN ('none', 'overflow', 'evicted', 'flush_failure', 'process_loss')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    terminal_at timestamptz,
    detail_expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS derived_work_checkpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id uuid NOT NULL REFERENCES derived_work_items(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    checkpoint_seq bigint NOT NULL CHECK (checkpoint_seq > 0),
    processed_offset bigint NOT NULL CHECK (processed_offset >= 0),
    source_watermark text NOT NULL,
    committed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (work_id, checkpoint_seq)
);

CREATE TABLE IF NOT EXISTS derived_work_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id uuid NOT NULL REFERENCES derived_work_items(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    attempt integer NOT NULL CHECK (attempt > 0),
    state text NOT NULL,
    disposition text,
    worker_id text,
    lease_until timestamptz,
    failure_category text,
    observed_at timestamptz NOT NULL DEFAULT now(),
    detail_expires_at timestamptz
);

CREATE TABLE IF NOT EXISTS derived_work_terminal_summaries (
    work_id uuid PRIMARY KEY REFERENCES derived_work_items(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    state text NOT NULL CHECK (state IN ('completed', 'exhausted', 'cancelled', 'dropped')),
    disposition text NOT NULL,
    checkpoint_seq bigint,
    evidence_reference text,
    recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS derived_work_scope_identity_idx
    ON derived_work_items (tenant, project, namespace, kind, watermark, idempotency_key, reference);
CREATE INDEX IF NOT EXISTS derived_work_claim_idx
    ON derived_work_items (tenant, project, namespace, state, next_attempt_at, lease_until, created_at, id);
CREATE INDEX IF NOT EXISTS derived_work_detail_retention_idx
    ON derived_work_attempts (detail_expires_at, work_id);
CREATE INDEX IF NOT EXISTS derived_work_checkpoint_idx
    ON derived_work_checkpoints (work_id, checkpoint_seq DESC);
CREATE INDEX IF NOT EXISTS derived_work_terminal_scope_idx
    ON derived_work_terminal_summaries (tenant, project, namespace, recorded_at DESC, work_id);

DROP TRIGGER IF EXISTS derived_work_checkpoints_append_only ON derived_work_checkpoints;
CREATE TRIGGER derived_work_checkpoints_append_only
    BEFORE UPDATE OR DELETE ON derived_work_checkpoints
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

DROP TRIGGER IF EXISTS derived_work_attempts_append_only ON derived_work_attempts;
CREATE TRIGGER derived_work_attempts_append_only
    BEFORE UPDATE OR DELETE ON derived_work_attempts
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

DROP TRIGGER IF EXISTS derived_work_terminal_summaries_append_only ON derived_work_terminal_summaries;
CREATE TRIGGER derived_work_terminal_summaries_append_only
    BEFORE UPDATE OR DELETE ON derived_work_terminal_summaries
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
