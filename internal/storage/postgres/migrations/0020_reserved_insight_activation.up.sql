CREATE TABLE IF NOT EXISTS reserved_insight_activation_policies (
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_version text NOT NULL,
    owner text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    enabled_types jsonb NOT NULL DEFAULT '{}'::jsonb,
    provider_contract_version text,
    schema_version text,
    min_evidence integer NOT NULL,
    min_confidence double precision NOT NULL,
    max_evidence integer NOT NULL,
    max_candidate_bytes integer NOT NULL,
    expires_at timestamptz,
    source_watermark text,
    rolled_back boolean NOT NULL DEFAULT false,
    reason text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant, project, namespace, policy_version)
);

CREATE TABLE IF NOT EXISTS reserved_insight_activation_decisions (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_version text NOT NULL,
    candidate_fingerprint text NOT NULL,
    idempotency_key text,
    insight_type text NOT NULL,
    disposition text NOT NULL,
    insight_id text,
    reason text NOT NULL,
    source_watermark text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS reserved_insight_activation_evidence (
    decision_id text NOT NULL REFERENCES reserved_insight_activation_decisions(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    evidence_kind text NOT NULL,
    evidence_id text NOT NULL,
    relation text NOT NULL,
    observed_at timestamptz,
    PRIMARY KEY (decision_id, evidence_kind, evidence_id, relation)
);

CREATE UNIQUE INDEX IF NOT EXISTS reserved_insight_activation_scope_fingerprint_idx
    ON reserved_insight_activation_decisions (tenant, project, namespace, candidate_fingerprint);
CREATE UNIQUE INDEX IF NOT EXISTS reserved_insight_activation_scope_idempotency_idx
    ON reserved_insight_activation_decisions (tenant, project, namespace, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS reserved_insight_activation_scope_created_at_idx
    ON reserved_insight_activation_decisions (tenant, project, namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS reserved_insight_activation_evidence_scope_idx
    ON reserved_insight_activation_evidence (tenant, project, namespace, evidence_kind, evidence_id);
CREATE INDEX IF NOT EXISTS reserved_insight_activation_policy_scope_updated_at_idx
    ON reserved_insight_activation_policies (tenant, project, namespace, updated_at DESC);
