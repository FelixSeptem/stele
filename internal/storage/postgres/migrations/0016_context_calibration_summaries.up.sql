CREATE TABLE IF NOT EXISTS context_calibration_summaries (
    id uuid PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    policy_version text NOT NULL,
    summary_version text NOT NULL,
    source_watermark timestamptz NOT NULL,
    evidence_count integer NOT NULL CHECK (evidence_count >= 0 AND evidence_count <= 10000),
    confidence_sum double precision NOT NULL CHECK (confidence_sum >= 0 AND confidence_sum <= 10000),
    priority_sum double precision NOT NULL CHECK (priority_sum >= -10000 AND priority_sum <= 10000),
    freshness text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant, project, namespace, policy_version, summary_version)
);

CREATE INDEX IF NOT EXISTS context_calibration_summaries_scope_updated_at_idx
    ON context_calibration_summaries (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS context_calibration_summaries_scope_watermark_idx
    ON context_calibration_summaries (tenant, project, namespace, source_watermark DESC);
