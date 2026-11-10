-- Redacted, append-only EV3 representation-comparison evidence. This table
-- holds no canonical memory, source content, scope hash, query, or credential.
CREATE TABLE representation_evaluation_reports (
    run_identity text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_identity text NOT NULL,
    fixture_identity text NOT NULL,
    baseline_identity text NOT NULL,
    candidate_identity text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('offline', 'shadow')),
    verdict text NOT NULL CHECK (verdict IN ('shadow', 'non_pass')),
    comparable boolean NOT NULL DEFAULT false,
    rollback_required boolean NOT NULL DEFAULT false,
    aggregate jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    CONSTRAINT representation_evaluation_reports_scope_check CHECK (tenant <> '' AND project <> '' AND namespace <> ''),
    CONSTRAINT representation_evaluation_reports_identity_check CHECK (
        run_identity <> '' AND policy_identity <> '' AND fixture_identity <> '' AND
        baseline_identity <> '' AND candidate_identity <> ''
    )
);

CREATE INDEX representation_evaluation_reports_scope_created_idx
    ON representation_evaluation_reports (tenant, project, namespace, created_at DESC);
CREATE INDEX representation_evaluation_reports_retention_idx
    ON representation_evaluation_reports (expires_at, tenant, project, namespace)
    WHERE expires_at IS NOT NULL;

CREATE TRIGGER representation_evaluation_reports_append_only
    BEFORE UPDATE ON representation_evaluation_reports
    FOR EACH ROW EXECUTE FUNCTION prevent_governed_audit_mutation();
