-- No implicit public disclosure or source attestation on existing grants.
ALTER TABLE access_scope_grants
    ADD COLUMN allowed_disclosure_profiles text[] NOT NULL DEFAULT '{}',
    ADD COLUMN default_disclosure_profile text NOT NULL DEFAULT '',
    ADD COLUMN source_attestation boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT disclosure_profiles_valid CHECK (
        (cardinality(allowed_disclosure_profiles)=0 AND default_disclosure_profile='' AND NOT source_attestation)
        OR (cardinality(allowed_disclosure_profiles) BETWEEN 1 AND 2
            AND allowed_disclosure_profiles <@ ARRAY['memory-reference-v1','memory-content-v1']::text[]
            AND default_disclosure_profile=ANY(allowed_disclosure_profiles)
            AND (cardinality(allowed_disclosure_profiles)=1 OR allowed_disclosure_profiles[1]<>allowed_disclosure_profiles[2]))
    );
ALTER TABLE access_audit_records
    ADD COLUMN actor text NOT NULL DEFAULT '',
    ADD COLUMN reason text NOT NULL DEFAULT '';

-- Legacy snapshots remain untrusted/restricted; no historical attestation backfill.
ALTER TABLE raw_events ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE candidate_memories ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE canonical_memories ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE memory_versions ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE derived_insights ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE derived_insight_evidence
    ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}',
    ADD COLUMN source_version bigint;
ALTER TABLE memory_chunk_derivations ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE memory_chunk_items ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE context_projections ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';
ALTER TABLE context_projection_items ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}';

-- The same restrictive join is used by every persistence path. SQL derives
-- snapshots from exact-scope durable sources rather than caller supplied labels.
CREATE FUNCTION stele_source_join(sources jsonb[], floor_class text DEFAULT 'low')
RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    labels jsonb;
    classes text[] := ARRAY['low','medium','high','restricted'];
    severity integer := array_position(classes, floor_class);
    trusted boolean := true;
    rank integer;
BEGIN
    IF severity IS NULL OR coalesce(cardinality(sources),0)=0 THEN
        RETURN '{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}'::jsonb;
    END IF;
    FOREACH labels IN ARRAY sources LOOP
        rank := array_position(classes, labels->>'classification');
        IF labels IS NULL OR labels->>'version' IS DISTINCT FROM 'source-metadata-v1'
           OR rank IS NULL OR coalesce(labels->>'trust','') NOT IN ('untrusted','attested_data') THEN
            rank := 4;
            trusted := false;
        END IF;
        severity := greatest(severity, rank);
        trusted := trusted AND labels->>'trust' IS NOT DISTINCT FROM 'attested_data';
    END LOOP;
    RETURN jsonb_build_object('version','source-metadata-v1','trust',
        CASE WHEN trusted THEN 'attested_data' ELSE 'untrusted' END,
        'classification',classes[severity]);
END;
$$;

CREATE FUNCTION stele_candidate_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE labels jsonb;
BEGIN
    SELECT source_metadata INTO labels FROM raw_events
      WHERE id=NEW.source_raw_event_id AND tenant=NEW.tenant
        AND project=NEW.project AND namespace=NEW.namespace;
    NEW.source_metadata := stele_source_join(ARRAY[labels], NEW.sensitivity);
    RETURN NEW;
END;
$$;
CREATE TRIGGER candidate_source_snapshot BEFORE INSERT ON candidate_memories
    FOR EACH ROW EXECUTE FUNCTION stele_candidate_source_snapshot();

CREATE FUNCTION stele_canonical_source_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.source_metadata := stele_source_join(ARRAY[OLD.source_metadata,NEW.source_metadata]);
    RETURN NEW;
END;
$$;
CREATE TRIGGER canonical_source_guard BEFORE UPDATE ON canonical_memories
    FOR EACH ROW EXECUTE FUNCTION stele_canonical_source_guard();

CREATE FUNCTION stele_version_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SELECT source_metadata INTO NEW.source_metadata FROM canonical_memories WHERE id=NEW.memory_id;
    NEW.source_metadata := stele_source_join(ARRAY[NEW.source_metadata]);
    RETURN NEW;
END;
$$;
CREATE TRIGGER version_source_snapshot BEFORE INSERT ON memory_versions
    FOR EACH ROW EXECUTE FUNCTION stele_version_source_snapshot();

-- Immutable source snapshots are resolved by identity, version and exact scope.
CREATE FUNCTION stele_source_snapshot(kind text, source text, source_version bigint,
    t text, p text, n text) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE labels jsonb;
BEGIN
    IF kind='raw_event' AND source_version=1 THEN
        SELECT source_metadata INTO labels FROM raw_events
          WHERE id::text=source AND tenant=t AND project=p AND namespace=n;
    ELSIF kind='canonical_version' THEN
        SELECT v.source_metadata INTO labels FROM memory_versions v
          JOIN canonical_memories c ON c.id=v.memory_id
          WHERE v.id::text=source AND v.version=source_version
            AND c.tenant=t AND c.project=p AND c.namespace=n;
    END IF;
    RETURN stele_source_join(ARRAY[labels]);
END;
$$;

CREATE FUNCTION stele_derived_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE labels jsonb;
BEGIN
    IF TG_TABLE_NAME='memory_chunk_derivations' OR TG_TABLE_NAME='context_projection_items' THEN
        NEW.source_metadata := stele_source_snapshot(NEW.source_kind,NEW.source_id::text,
            coalesce(NEW.source_version,1),NEW.tenant,NEW.project,NEW.namespace);
    ELSE
        SELECT source_metadata INTO labels FROM memory_chunk_derivations
          WHERE id=NEW.derivation_id AND tenant=NEW.tenant AND project=NEW.project AND namespace=NEW.namespace;
        NEW.source_metadata := stele_source_join(ARRAY[labels]);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER chunk_derivation_source_snapshot BEFORE INSERT ON memory_chunk_derivations
    FOR EACH ROW EXECUTE FUNCTION stele_derived_source_snapshot();
CREATE TRIGGER chunk_item_source_snapshot BEFORE INSERT ON memory_chunk_items
    FOR EACH ROW EXECUTE FUNCTION stele_derived_source_snapshot();
CREATE TRIGGER projection_item_source_snapshot BEFORE INSERT ON context_projection_items
    FOR EACH ROW EXECUTE FUNCTION stele_derived_source_snapshot();

CREATE FUNCTION stele_projection_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE sources jsonb[];
BEGIN
    SELECT array_agg(source_metadata) INTO sources FROM context_projection_items
      WHERE projection_id=NEW.id AND tenant=NEW.tenant AND project=NEW.project AND namespace=NEW.namespace;
    IF OLD.status='building' THEN
        NEW.source_metadata := stele_source_join(sources);
    ELSE
        NEW.source_metadata := stele_source_join(ARRAY[OLD.source_metadata,stele_source_join(sources)]);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER projection_source_snapshot BEFORE UPDATE ON context_projections
    FOR EACH ROW EXECUTE FUNCTION stele_projection_source_snapshot();

CREATE FUNCTION stele_immutable_source_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.source_metadata IS DISTINCT FROM OLD.source_metadata THEN
        RAISE EXCEPTION 'source metadata snapshots are immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER raw_source_snapshot_immutable BEFORE UPDATE ON raw_events
    FOR EACH ROW EXECUTE FUNCTION stele_immutable_source_snapshot();
CREATE TRIGGER candidate_source_snapshot_immutable BEFORE UPDATE ON candidate_memories
    FOR EACH ROW EXECUTE FUNCTION stele_immutable_source_snapshot();
CREATE TRIGGER version_source_snapshot_immutable BEFORE UPDATE ON memory_versions
    FOR EACH ROW EXECUTE FUNCTION stele_immutable_source_snapshot();

CREATE FUNCTION stele_insight_evidence_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE labels jsonb;
BEGIN
    IF NEW.evidence_kind='raw_event' THEN
        SELECT source_metadata,1 INTO labels,NEW.source_version FROM raw_events
          WHERE id::text=NEW.evidence_id AND tenant=NEW.tenant AND project=NEW.project AND namespace=NEW.namespace;
    ELSIF NEW.evidence_kind IN ('canonical_memory','procedural_memory','summary_memory','relation_memory') THEN
        SELECT v.source_metadata,v.version INTO labels,NEW.source_version FROM memory_versions v
          JOIN canonical_memories c ON c.id=v.memory_id
          WHERE c.id::text=NEW.evidence_id AND c.tenant=NEW.tenant AND c.project=NEW.project AND c.namespace=NEW.namespace
          ORDER BY v.version DESC LIMIT 1;
    END IF;
    NEW.source_metadata := stele_source_join(ARRAY[labels]);
    RETURN NEW;
END;
$$;
CREATE TRIGGER insight_evidence_source_snapshot BEFORE INSERT ON derived_insight_evidence
    FOR EACH ROW EXECUTE FUNCTION stele_insight_evidence_snapshot();

CREATE FUNCTION stele_insight_source_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.source_metadata := stele_source_join(ARRAY[OLD.source_metadata,NEW.source_metadata]);
    RETURN NEW;
END;
$$;
CREATE TRIGGER insight_source_guard BEFORE UPDATE ON derived_insights
    FOR EACH ROW EXECUTE FUNCTION stele_insight_source_guard();

-- Reject malformed snapshots at the system of record as well as at ingestion.
DO $$
DECLARE target text;
BEGIN
    FOREACH target IN ARRAY ARRAY['raw_events','candidate_memories','canonical_memories',
        'memory_versions','derived_insights','derived_insight_evidence','memory_chunk_derivations',
        'memory_chunk_items','context_projections','context_projection_items'] LOOP
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT source_metadata_valid CHECK (
            jsonb_typeof(source_metadata)=''object'' AND
            source_metadata ?& ARRAY[''version'',''trust'',''classification''] AND
            source_metadata - ARRAY[''version'',''trust'',''classification''] = ''{}''::jsonb AND
            source_metadata->>''version''=''source-metadata-v1'' AND
            source_metadata->>''trust'' IN (''untrusted'',''attested_data'') AND
            source_metadata->>''classification'' IN (''low'',''medium'',''high'',''restricted'') AND
            NOT source_metadata @> ''{"version":null}''::jsonb AND
            NOT source_metadata @> ''{"trust":null}''::jsonb AND
            NOT source_metadata @> ''{"classification":null}''::jsonb)', target);
    END LOOP;
END;
$$;
