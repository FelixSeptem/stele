CREATE TABLE IF NOT EXISTS context_projections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('always_visible', 'session', 'retrieval', 'archival_history')),
    version bigint NOT NULL CHECK (version > 0),
    schema_version text NOT NULL,
    policy_version text NOT NULL,
    renderer_version text NOT NULL,
    source_watermark jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_watermark_hash text NOT NULL,
    status text NOT NULL CHECK (status IN ('building', 'active', 'superseded', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    superseded_at timestamptz,
    UNIQUE (tenant, project, namespace, kind, version),
    UNIQUE (tenant, project, namespace, kind, policy_version, renderer_version, source_watermark_hash)
);

CREATE TABLE IF NOT EXISTS context_projection_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    projection_id uuid NOT NULL REFERENCES context_projections(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_kind text NOT NULL CHECK (source_kind IN ('canonical_version', 'raw_event')),
    source_id uuid NOT NULL,
    source_version bigint,
    memory_id uuid,
    class text NOT NULL,
    lifecycle_state text NOT NULL,
    rendered_text text NOT NULL,
    sort_key text NOT NULL,
    citation jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (projection_id, source_kind, source_id, source_version)
);

CREATE INDEX IF NOT EXISTS context_projections_scope_kind_status_idx
    ON context_projections (tenant, project, namespace, kind, status, version DESC);
CREATE INDEX IF NOT EXISTS context_projection_items_scope_sort_idx
    ON context_projection_items (tenant, project, namespace, projection_id, sort_key, id);
CREATE INDEX IF NOT EXISTS context_projection_items_source_idx
    ON context_projection_items (source_kind, source_id, source_version);

CREATE OR REPLACE FUNCTION prevent_context_projection_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id OR NEW.tenant <> OLD.tenant OR NEW.project <> OLD.project OR NEW.namespace <> OLD.namespace
       OR NEW.kind <> OLD.kind OR NEW.version <> OLD.version OR NEW.schema_version <> OLD.schema_version
       OR NEW.policy_version <> OLD.policy_version OR NEW.renderer_version <> OLD.renderer_version
       OR NEW.source_watermark <> OLD.source_watermark OR NEW.source_watermark_hash <> OLD.source_watermark_hash
       OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'context projection records are append-only';
    END IF;
    IF OLD.status = 'superseded' AND NEW.status <> OLD.status THEN
        RAISE EXCEPTION 'superseded context projection status is immutable';
    END IF;
    IF OLD.status = 'failed' AND NEW.status NOT IN ('failed', 'active') THEN
        RAISE EXCEPTION 'failed context projection has invalid status transition';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS context_projections_append_only ON context_projections;
CREATE TRIGGER context_projections_append_only
    BEFORE UPDATE ON context_projections
    FOR EACH ROW EXECUTE FUNCTION prevent_context_projection_mutation();

CREATE OR REPLACE FUNCTION prevent_context_projection_item_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'context projection items are append-only';
END;
$$;

DROP TRIGGER IF EXISTS context_projection_items_append_only ON context_projection_items;
CREATE TRIGGER context_projection_items_append_only
    BEFORE UPDATE OR DELETE ON context_projection_items
    FOR EACH ROW EXECUTE FUNCTION prevent_context_projection_item_mutation();
