CREATE TABLE IF NOT EXISTS progressive_experiment_reports (
    run_identity text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_identity text NOT NULL,
    baseline_identity text NOT NULL,
    strategy_identity text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('offline', 'shadow')),
    verdict text NOT NULL CHECK (verdict IN ('shadow', 'non_pass')),
    eligible boolean NOT NULL DEFAULT false,
    fallback_category text NOT NULL DEFAULT 'none',
    rollback_required boolean NOT NULL DEFAULT false,
    source_watermark_hash text NOT NULL DEFAULT '',
    aggregate jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    CONSTRAINT progressive_experiment_reports_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT progressive_experiment_reports_identity_check CHECK (run_identity <> '' AND policy_identity <> '' AND baseline_identity <> '' AND strategy_identity <> '')
);

CREATE INDEX IF NOT EXISTS progressive_experiment_reports_scope_created_idx
    ON progressive_experiment_reports (tenant, project, namespace, created_at DESC);
CREATE INDEX IF NOT EXISTS progressive_experiment_reports_retention_idx
    ON progressive_experiment_reports (expires_at, tenant, project, namespace)
    WHERE expires_at IS NOT NULL;

DROP TRIGGER IF EXISTS progressive_experiment_reports_append_only ON progressive_experiment_reports;
CREATE TRIGGER progressive_experiment_reports_append_only
    BEFORE UPDATE OR DELETE ON progressive_experiment_reports
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
