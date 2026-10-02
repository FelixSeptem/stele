CREATE TABLE IF NOT EXISTS ranking_rollout_evidence_attestations (
    policy_id uuid NOT NULL REFERENCES ranking_rollout_policies(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    run_identity text NOT NULL,
    scope_hash text NOT NULL,
    policy_version text NOT NULL,
    strategy_identity text NOT NULL,
    source_watermark_hash text NOT NULL,
    verdict text NOT NULL,
    freshness text NOT NULL,
    real_stack boolean NOT NULL,
    deterministic_replay boolean NOT NULL,
    rollback_tested boolean NOT NULL,
    evaluated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (policy_id, run_identity),
    CONSTRAINT ranking_rollout_evidence_attestations_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT ranking_rollout_evidence_attestations_verdict_check CHECK (verdict = 'passed' AND freshness = 'fresh' AND real_stack AND deterministic_replay AND rollback_tested),
    CONSTRAINT ranking_rollout_evidence_attestations_window_check CHECK (expires_at > evaluated_at)
);

CREATE INDEX IF NOT EXISTS ranking_rollout_evidence_attestations_scope_created_idx
    ON ranking_rollout_evidence_attestations (tenant, project, namespace, created_at DESC);
