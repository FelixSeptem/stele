ALTER TABLE derived_work_items
    ADD COLUMN river_job_id bigint,
    ADD COLUMN river_handed_off_at timestamptz;

CREATE TABLE memory_intent_river_jobs (
    intent_id uuid PRIMARY KEY REFERENCES memory_intents(id),
    river_job_id bigint NOT NULL UNIQUE CHECK (river_job_id > 0),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE FUNCTION verify_memory_intent_river_job_scope()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM memory_intents i
        WHERE i.id = NEW.intent_id
          AND i.tenant = NEW.tenant AND i.project = NEW.project AND i.namespace = NEW.namespace
    ) THEN
        RAISE EXCEPTION 'memory-intent River job scope mismatch';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER memory_intent_river_job_scope_guard
    BEFORE INSERT ON memory_intent_river_jobs
    FOR EACH ROW EXECUTE FUNCTION verify_memory_intent_river_job_scope();

CREATE TRIGGER memory_intent_river_jobs_append_only
    BEFORE UPDATE OR DELETE ON memory_intent_river_jobs
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

CREATE INDEX derived_work_memory_intent_handoff_idx
    ON derived_work_items (created_at, id)
    WHERE kind = 'memory_intent' AND state IN ('queued', 'retry', 'claimed', 'running');
