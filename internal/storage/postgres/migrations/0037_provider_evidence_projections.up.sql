-- Runtime-reported evidence only; no task, lease, commit or publication state.
CREATE TABLE provider_projection_identities (
    logical_id uuid PRIMARY KEY CHECK (logical_id <> '00000000-0000-0000-0000-000000000000'),
    binding_id text NOT NULL, principal_id text NOT NULL,
    tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    agent_id text NOT NULL, session_id text NOT NULL,
    conversation_id text NOT NULL, provider_instance_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('turn','outcome','feedback')),
    effect_id text NOT NULL CHECK (octet_length(effect_id) BETWEEN 1 AND 128 AND effect_id ~ '^[A-Za-z0-9_.:-]+$'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (binding_id,kind,effect_id),
    UNIQUE (logical_id,tenant,project,namespace),
    FOREIGN KEY (binding_id,tenant,project,namespace)
        REFERENCES provider_runtime_bindings(binding_id,tenant,project,namespace)
);
CREATE TABLE provider_projection_revisions (
    projection_id uuid PRIMARY KEY CHECK (projection_id <> '00000000-0000-0000-0000-000000000000'),
    logical_id uuid NOT NULL,
    tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    source_revision integer NOT NULL CHECK (source_revision > 0),
    idempotency_key text NOT NULL CHECK (octet_length(idempotency_key) BETWEEN 1 AND 256),
    fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    supersedes_id uuid,
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object' AND octet_length(payload::text)<=32768),
    admission jsonb NOT NULL CHECK (jsonb_typeof(admission)='object' AND octet_length(admission::text)<=16384),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(logical_id,source_revision),
    UNIQUE(logical_id,idempotency_key),
    UNIQUE(projection_id,logical_id,tenant,project,namespace),
    FOREIGN KEY(logical_id,tenant,project,namespace)
        REFERENCES provider_projection_identities(logical_id,tenant,project,namespace),
    FOREIGN KEY(supersedes_id,logical_id,tenant,project,namespace)
        REFERENCES provider_projection_revisions(projection_id,logical_id,tenant,project,namespace),
    CHECK (supersedes_id IS NULL OR supersedes_id <> projection_id)
);
CREATE INDEX provider_projection_history ON provider_projection_revisions(logical_id,source_revision DESC);
CREATE TRIGGER provider_projection_identity_append_only BEFORE UPDATE OR DELETE
    ON provider_projection_identities FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
CREATE TRIGGER provider_projection_revision_append_only BEFORE UPDATE OR DELETE
    ON provider_projection_revisions FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();

-- Mapping is an asynchronous, immutable source attribution, not a ranking or
-- insight activation. Checkpoints are scheduling state, never evidence.
CREATE UNIQUE INDEX usefulness_feedback_scoped_identity ON usefulness_feedback(id,tenant,project,namespace);
CREATE TABLE provider_projection_feedback_links (
    projection_id uuid PRIMARY KEY, logical_id uuid NOT NULL,
    tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
    feedback_id text NOT NULL UNIQUE,
    policy_version text NOT NULL CHECK(policy_version='projection-feedback-v1'),
    lineage jsonb NOT NULL CHECK(jsonb_typeof(lineage)='array' AND jsonb_array_length(lineage) BETWEEN 1 AND 8),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    FOREIGN KEY(projection_id,logical_id,tenant,project,namespace)
      REFERENCES provider_projection_revisions(projection_id,logical_id,tenant,project,namespace),
    FOREIGN KEY(feedback_id,tenant,project,namespace)
      REFERENCES usefulness_feedback(id,tenant,project,namespace)
);
CREATE TRIGGER provider_projection_feedback_append_only BEFORE UPDATE OR DELETE
    ON provider_projection_feedback_links FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
CREATE TABLE provider_projection_feedback_checks (
    logical_id uuid PRIMARY KEY REFERENCES provider_projection_identities(logical_id),
    checked_at timestamptz NOT NULL
);

-- Active feedback consumers use this view, not cached eligibility. It closes
-- the interval between correction/revocation/lifecycle changes and reconciliation.
CREATE VIEW eligible_usefulness_feedback AS
SELECT uf.* FROM usefulness_feedback uf
WHERE NOT EXISTS(SELECT 1 FROM provider_projection_feedback_links l WHERE l.feedback_id=uf.id)
OR EXISTS(
 SELECT 1 FROM provider_projection_feedback_links l
 JOIN provider_projection_revisions r USING(projection_id,logical_id,tenant,project,namespace)
 JOIN provider_projection_identities i USING(logical_id,tenant,project,namespace)
 JOIN provider_runtime_bindings b ON b.binding_id=i.binding_id AND b.principal_id=i.principal_id
  AND b.tenant=i.tenant AND b.project=i.project AND b.namespace=i.namespace
  AND b.agent_id=i.agent_id AND b.session_id=i.session_id
  AND b.conversation_id=i.conversation_id AND b.provider_instance_id=i.provider_instance_id
 JOIN access_principals ap ON ap.id=b.principal_id
 JOIN LATERAL (SELECT g.* FROM access_scope_grants g
  WHERE g.principal_id=i.principal_id AND g.tenant=i.tenant AND g.project=i.project AND g.namespace=i.namespace
   AND g.status='active' AND g.revoked_at IS NULL
  ORDER BY g.created_at DESC,g.id DESC LIMIT 1) g ON true
 WHERE l.feedback_id=uf.id AND l.tenant=uf.tenant AND l.project=uf.project AND l.namespace=uf.namespace
  AND l.policy_version='projection-feedback-v1'
  AND b.revoked_at IS NULL AND b.expires_at>statement_timestamp()
  AND ap.status='active' AND ap.disabled_at IS NULL AND (ap.expires_at IS NULL OR ap.expires_at>statement_timestamp())
  AND g.access_mode='read_write' AND g.default_disclosure_profile=ANY(g.allowed_disclosure_profiles)
  AND g.default_disclosure_profile IN ('memory-reference-v1','memory-content-v1')
  AND NOT EXISTS(SELECT 1 FROM provider_projection_revisions newer WHERE newer.logical_id=r.logical_id AND newer.source_revision>r.source_revision)
  AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(l.lineage) ref WHERE NOT EXISTS(
   SELECT 1 FROM raw_events e
   JOIN LATERAL (SELECT c.* FROM candidate_memories c WHERE c.source_raw_event_id=e.id
    AND c.tenant=e.tenant AND c.project=e.project AND c.namespace=e.namespace
    ORDER BY c.created_at DESC,c.id DESC LIMIT 1) c ON true
   JOIN LATERAL (SELECT p.* FROM provenance_links p WHERE p.raw_event_id=e.id AND p.candidate_memory_id=c.id
    AND p.tenant=e.tenant AND p.project=e.project AND p.namespace=e.namespace
    AND p.operation='promote_candidate' AND p.memory_id IS NOT NULL
    ORDER BY p.created_at DESC,p.id DESC LIMIT 1) p ON true
   JOIN canonical_memories m ON m.id=p.memory_id AND m.tenant=e.tenant AND m.project=e.project AND m.namespace=e.namespace
   JOIN LATERAL (SELECT v.* FROM memory_versions v WHERE v.memory_id=m.id ORDER BY v.version DESC LIMIT 1) v ON true
   WHERE e.id::text=ref->>'event_id' AND e.tenant=l.tenant AND e.project=l.project AND e.namespace=l.namespace
    AND e.metadata->>'reference_binding_id'=i.binding_id AND e.metadata->>'reference_principal_id'=i.principal_id
    AND e.metadata->>'reference_contract_version'='durable-reference-v1'
    AND e.event_type IN ('provider.reference.task','provider.reference.run','provider.reference.handoff','provider.reference.compilation')
    AND e.governance_processed_at IS NOT NULL AND c.status='promoted' AND m.state='active' AND v.state='active'
    AND c.id::text=ref->'lineage'->>'candidate_id' AND m.id::text=ref->'lineage'->>'memory_id'
    AND v.id::text=ref->'lineage'->>'version_id' AND p.id::text=ref->'lineage'->>'provenance_id'
    AND v.version::text=ref->'lineage'->>'version' AND v.version>=1
    AND c.memory_path=e.memory_path AND m.memory_path=e.memory_path AND v.memory_path=e.memory_path
    -- Mirror PC5 validReferenceRecord, including NormalizeMemoryPath. PostgreSQL
    -- UTF8 text already enforces valid UTF8; the bounds here are bytes, not chars.
    AND '00000000-0000-0000-0000-000000000000'::uuid NOT IN (e.id,c.id,m.id,v.id,p.id)
    AND octet_length(v.content)<=4096
    AND octet_length(e.metadata->>'reference_id') BETWEEN 1 AND 128
    AND (e.metadata->>'reference_id') COLLATE "C" ~ '^[A-Za-z0-9_.:-]+$'
    AND octet_length(e.memory_path) BETWEEN 1 AND 512
    AND left(e.memory_path,1)<>'/' AND right(e.memory_path,1)<>'/'
    -- Go strings.TrimSpace uses Unicode White_Space. Control whitespace is
    -- covered below; list the remaining codepoints without restricting Unicode.
    AND ascii(left(e.memory_path,1)) NOT IN (32,160,5760,8192,8193,8194,8195,8196,8197,8198,8199,8200,8201,8202,8232,8233,8239,8287,12288)
    AND ascii(right(e.memory_path,1)) NOT IN (32,160,5760,8192,8193,8194,8195,8196,8197,8198,8199,8200,8201,8202,8232,8233,8239,8287,12288)
    AND translate(e.memory_path,chr(92)||'*?[]','')=e.memory_path
    AND strpos(lower(e.memory_path),'%2f')=0 AND strpos(lower(e.memory_path),'%5c')=0
    AND NOT EXISTS(SELECT 1 FROM regexp_split_to_table(e.memory_path,'/') segment
      WHERE segment IN ('','.','..') OR octet_length(segment)>128)
    -- unicode.IsControl includes C0 and C1; locale-dependent regex character
    -- classes need not recognize every C1 character. UTF8 PostgreSQL forbids NUL.
    AND NOT EXISTS(SELECT 1 FROM regexp_split_to_table(e.memory_path,'') character
      WHERE ascii(character) BETWEEN 1 AND 31 OR ascii(character) BETWEEN 127 AND 159)
    AND e.source_metadata->>'version'='source-metadata-v1' AND e.source_metadata->>'trust' IN ('untrusted','attested_data') AND e.source_metadata->>'classification' IN ('low','medium')
    AND c.source_metadata->>'version'='source-metadata-v1' AND c.source_metadata->>'trust' IN ('untrusted','attested_data') AND c.source_metadata->>'classification' IN ('low','medium')
    AND m.source_metadata->>'version'='source-metadata-v1' AND m.source_metadata->>'trust' IN ('untrusted','attested_data') AND m.source_metadata->>'classification' IN ('low','medium')
    AND v.source_metadata->>'version'='source-metadata-v1' AND v.source_metadata->>'trust' IN ('untrusted','attested_data') AND v.source_metadata->>'classification' IN ('low','medium')
  ))
);
