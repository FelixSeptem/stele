CREATE TABLE IF NOT EXISTS governed_reasoning_insight_candidates (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    insight_type text NOT NULL CHECK (insight_type IN ('hypothesis', 'goal', 'contradiction', 'causal_link')),
    mode text NOT NULL CHECK (mode IN ('offline', 'shadow')),
    title text NOT NULL,
    summary text NOT NULL,
    evidence jsonb NOT NULL,
    evidence_digest text NOT NULL,
    source_watermark text NOT NULL,
    scope_proof text NOT NULL,
    lifecycle_visibility text NOT NULL CHECK (lifecycle_visibility = 'active_only'),
    redaction_policy text NOT NULL CHECK (redaction_policy = 'references_only'),
    provider_version text NOT NULL,
    schema_version text NOT NULL,
    policy_version text NOT NULL,
    replay_id text NOT NULL,
    uncertainty double precision NOT NULL CHECK (uncertainty >= 0 AND uncertainty <= 1),
    direct_activation boolean NOT NULL DEFAULT false,
    canonical_mutation boolean NOT NULL DEFAULT false,
    disposition text NOT NULL CHECK (disposition IN ('candidate', 'rejected', 'quarantined', 'would_activate', 'stale', 'fallback')),
    reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    UNIQUE (tenant, project, namespace, replay_id)
);

CREATE INDEX IF NOT EXISTS governed_reasoning_insight_candidates_scope_created_idx
    ON governed_reasoning_insight_candidates (tenant, project, namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS governed_reasoning_insight_candidates_scope_disposition_idx
    ON governed_reasoning_insight_candidates (tenant, project, namespace, disposition, created_at DESC);

DROP TRIGGER IF EXISTS governed_reasoning_insight_candidates_append_only ON governed_reasoning_insight_candidates;
CREATE TRIGGER governed_reasoning_insight_candidates_append_only
    BEFORE UPDATE OR DELETE ON governed_reasoning_insight_candidates
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
