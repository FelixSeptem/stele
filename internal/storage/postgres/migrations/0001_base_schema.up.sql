CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS raw_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    event_type text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_timestamp timestamptz,
    governance_worker_id text,
    governance_claimed_at timestamptz,
    governance_lease_until timestamptz,
    governance_processed_at timestamptz,
    governance_attempt integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE raw_events
    ADD COLUMN IF NOT EXISTS governance_last_failed_at timestamptz,
    ADD COLUMN IF NOT EXISTS governance_last_error text,
    ADD COLUMN IF NOT EXISTS governance_next_attempt_at timestamptz,
    ADD COLUMN IF NOT EXISTS governance_exhausted_at timestamptz,
    ADD COLUMN IF NOT EXISTS memory_session_id text,
    ADD COLUMN IF NOT EXISTS memory_session_turn_id text,
    ADD COLUMN IF NOT EXISTS memory_session_outcome_id text,
    ADD COLUMN IF NOT EXISTS memory_session_source text;

CREATE TABLE IF NOT EXISTS candidate_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_raw_event_id uuid NOT NULL REFERENCES raw_events(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    content text NOT NULL,
    confidence double precision NOT NULL,
    importance double precision NOT NULL,
    freshness double precision NOT NULL,
    sensitivity text NOT NULL,
    mutability text NOT NULL,
    retention_class text NOT NULL,
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS canonical_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    state text NOT NULL,
    retention_class text NOT NULL DEFAULT 'durable',
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    search_text tsvector,
    embedding vector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE canonical_memories
    ADD COLUMN IF NOT EXISTS retention_class text NOT NULL DEFAULT 'durable';

CREATE TABLE IF NOT EXISTS memory_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    version bigint NOT NULL,
    state text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    modified_by text NOT NULL,
    UNIQUE (memory_id, version)
);

CREATE TABLE IF NOT EXISTS embedding_rebuilds (
    memory_id uuid PRIMARY KEY REFERENCES canonical_memories(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_version bigint NOT NULL,
    content_hash text NOT NULL,
    requested_provider text NOT NULL,
    requested_model text NOT NULL,
    requested_dimensions integer NOT NULL DEFAULT 0,
    status text NOT NULL,
    failure_reason text,
    requested_at timestamptz NOT NULL,
    last_attempted_at timestamptz,
    active_vector_revision_id uuid
);

CREATE TABLE IF NOT EXISTS vector_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_version bigint NOT NULL,
    content_hash text NOT NULL,
    provider text NOT NULL,
    model text NOT NULL,
    dimensions integer NOT NULL,
    embedding vector,
    status text NOT NULL,
    failure_reason text,
    superseded_by uuid,
    generated_at timestamptz NOT NULL,
    activated_at timestamptz,
    last_rebuild_request_at timestamptz
);

CREATE TABLE IF NOT EXISTS provenance_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id uuid REFERENCES raw_events(id),
    candidate_memory_id uuid REFERENCES candidate_memories(id),
    memory_id uuid REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    operation text NOT NULL,
    request_id text,
    actor text,
    source_context jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS governance_recovery_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id uuid NOT NULL REFERENCES raw_events(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    action text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    after_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS embedding_cutover_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    status text NOT NULL,
    target_provider text NOT NULL,
    target_model text NOT NULL,
    target_dimensions integer NOT NULL,
    class_filters text[] NOT NULL DEFAULT '{}'::text[],
    wave_size integer NOT NULL,
    reason text NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_action_by text,
    last_action_reason text,
    last_action_at timestamptz,
    activated_at timestamptz,
    paused_at timestamptz,
    cancelled_at timestamptz,
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS embedding_cutover_items (
    plan_id uuid NOT NULL REFERENCES embedding_cutover_plans(id) ON DELETE CASCADE,
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    class text NOT NULL,
    status text NOT NULL,
    failure_reason text,
    active_vector_revision_id uuid,
    active_provider text,
    active_model text,
    active_dimensions integer,
    requested_at timestamptz,
    last_attempted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, memory_id)
);

CREATE TABLE IF NOT EXISTS embedding_recovery_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid NOT NULL REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    cutover_plan_id uuid REFERENCES embedding_cutover_plans(id),
    action text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    before_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    after_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deletion_markers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    memory_id uuid REFERENCES canonical_memories(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS relation_projections (
    memory_id uuid PRIMARY KEY REFERENCES canonical_memories(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    source_entity text NOT NULL,
    relation_type text NOT NULL,
    target_entity text NOT NULL,
    relation_text text NOT NULL,
    search_text tsvector,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS job_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name text NOT NULL,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    trigger_source text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL,
    attempt integer NOT NULL DEFAULT 1,
    processed_count integer NOT NULL DEFAULT 0,
    error_message text,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE TABLE IF NOT EXISTS derived_insights (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    type text NOT NULL,
    lifecycle_state text NOT NULL,
    title text NOT NULL,
    summary text NOT NULL,
    confidence double precision NOT NULL,
    confidence_method text,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    lesson jsonb,
    derivation_source text NOT NULL,
    derivation_fingerprint text NOT NULL,
    derivation_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    evidence_window_start timestamptz,
    evidence_window_end timestamptz,
    derived_at timestamptz NOT NULL,
    last_observed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS derived_insight_evidence (
    insight_id uuid NOT NULL REFERENCES derived_insights(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    evidence_kind text NOT NULL,
    evidence_id text NOT NULL,
    relation text NOT NULL,
    observed_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (insight_id, evidence_kind, evidence_id, relation)
);

CREATE TABLE IF NOT EXISTS derived_insight_lifecycle_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    insight_id uuid NOT NULL REFERENCES derived_insights(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    from_state text,
    to_state text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS derived_insight_feedback (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    insight_id uuid NOT NULL REFERENCES derived_insights(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    feedback_type text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    quality_score double precision,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    request_id text,
    superseded_at timestamptz,
    superseded_by_actor text,
    superseded_by_reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS derived_insight_replay_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    mode text NOT NULL,
    status text NOT NULL,
    insight_types text[] NOT NULL DEFAULT '{}'::text[],
    evidence_window_start timestamptz NOT NULL,
    evidence_window_end timestamptz NOT NULL,
    evidence_limit integer NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    idempotency_key text,
    request_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    report_counters jsonb,
    report_decisions jsonb,
    failure text,
    report_generated_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS quality_evaluation_runs (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    status text NOT NULL,
    checks text[] NOT NULL DEFAULT '{}'::text[],
    actor text,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

CREATE TABLE IF NOT EXISTS quality_evaluation_findings (
    id text PRIMARY KEY,
    evaluation_run_id text NOT NULL REFERENCES quality_evaluation_runs(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    code text NOT NULL,
    severity text NOT NULL,
    component text NOT NULL,
    category text NOT NULL,
    message text,
    suggested_action_category text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS repair_plans (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    evaluation_run_id text NOT NULL REFERENCES quality_evaluation_runs(id),
    baseline_run_id text REFERENCES quality_evaluation_runs(id),
    verification_run_id text REFERENCES quality_evaluation_runs(id),
    status text NOT NULL,
    verification_status text,
    dry_run boolean NOT NULL DEFAULT false,
    actor text NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    approved_at timestamptz,
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS repair_actions (
    id text PRIMARY KEY,
    plan_id text NOT NULL REFERENCES repair_plans(id) ON DELETE CASCADE,
    evaluation_run_id text NOT NULL REFERENCES quality_evaluation_runs(id),
    finding_id text REFERENCES quality_evaluation_findings(id),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    category text NOT NULL,
    status text NOT NULL,
    target_kind text,
    target_id text,
    reason_code text,
    attempt integer NOT NULL DEFAULT 0,
    worker_id text,
    lease_until timestamptz,
    last_error text,
    next_attempt_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS admission_pressure_audit (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    operation text NOT NULL,
    decision text NOT NULL,
    findings jsonb NOT NULL DEFAULT '[]'::jsonb,
    snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    observed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scope_proof_runs (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    status text NOT NULL,
    verdict text NOT NULL,
    checks text[] NOT NULL DEFAULT '{}'::text[],
    fixture_mode text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    rerun_of text REFERENCES scope_proof_runs(id),
    linked_session_id text,
    failure_category text,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

CREATE TABLE IF NOT EXISTS scope_proof_steps (
    id text PRIMARY KEY,
    proof_id text NOT NULL REFERENCES scope_proof_runs(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    step text NOT NULL,
    status text NOT NULL,
    verdict text,
    failure_category text,
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    attempt integer NOT NULL DEFAULT 0,
    worker_id text,
    lease_until timestamptz,
    last_error text,
    next_attempt_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS memory_session_runs (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    status text NOT NULL,
    verdict text NOT NULL,
    actor text,
    reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    failure_category text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

CREATE TABLE IF NOT EXISTS memory_session_turns (
    id text PRIMARY KEY,
    session_id text NOT NULL REFERENCES memory_session_runs(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    idempotency_key text,
    outcome_idempotency_key text,
    status text NOT NULL,
    query text NOT NULL,
    context_evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    outcome_event_ids text[] NOT NULL DEFAULT '{}'::text[],
    expected_recall text[] NOT NULL DEFAULT '{}'::text[],
    verification_status text,
    failure_category text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz
);

CREATE TABLE IF NOT EXISTS memory_session_verifications (
    id text PRIMARY KEY,
    session_id text NOT NULL REFERENCES memory_session_runs(id) ON DELETE CASCADE,
    turn_id text REFERENCES memory_session_turns(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    status text NOT NULL,
    verdict text NOT NULL,
    expected_recall text[] NOT NULL DEFAULT '{}'::text[],
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    failure_category text,
    attempt integer NOT NULL DEFAULT 0,
    worker_id text,
    lease_until timestamptz,
    last_error text,
    next_attempt_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS memory_loop_evidence_links (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    owner_kind text NOT NULL,
    owner_id text NOT NULL,
    evidence_kind text NOT NULL,
    evidence_id text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usefulness_feedback (
    id text PRIMARY KEY,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    feedback_type text NOT NULL,
    source_surface text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    idempotency_key text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    superseded_at timestamptz,
    superseded_by_actor text,
    superseded_by_reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usefulness_feedback_subjects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    feedback_id text NOT NULL REFERENCES usefulness_feedback(id) ON DELETE CASCADE,
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    subject_kind text NOT NULL,
    subject_id text,
    expected_recall_kind text,
    expected_recall_id text,
    opaque_token text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usefulness_feedback_supersessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    feedback_id text NOT NULL REFERENCES usefulness_feedback(id) ON DELETE CASCADE,
    actor text NOT NULL,
    reason text NOT NULL,
    superseded_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usefulness_feedback_summaries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant text NOT NULL,
    project text NOT NULL,
    namespace text NOT NULL,
    subject_kind text NOT NULL,
    subject_id text,
    expected_recall_kind text,
    expected_recall_id text,
    opaque_token text,
    total_active integer NOT NULL DEFAULT 0,
    counts jsonb NOT NULL DEFAULT '{}'::jsonb,
    effective_quality text NOT NULL DEFAULT 'unknown',
    dominant_categories text[] NOT NULL DEFAULT '{}'::text[],
    last_feedback_at timestamptz,
    rebuilt_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS raw_events_scope_created_at_idx
    ON raw_events (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS raw_events_session_outcome_idx
    ON raw_events (tenant, project, namespace, memory_session_id, memory_session_turn_id, created_at DESC)
    WHERE memory_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS raw_events_governance_claim_idx
    ON raw_events (
        governance_processed_at,
        governance_exhausted_at,
        governance_next_attempt_at,
        governance_lease_until,
        created_at
    );

CREATE INDEX IF NOT EXISTS candidate_memories_source_raw_event_idx
    ON candidate_memories (source_raw_event_id, created_at DESC);

CREATE INDEX IF NOT EXISTS candidate_memories_scope_status_idx
    ON candidate_memories (tenant, project, namespace, status, created_at DESC);

CREATE INDEX IF NOT EXISTS canonical_memories_scope_state_idx
    ON canonical_memories (tenant, project, namespace, state);

CREATE INDEX IF NOT EXISTS canonical_memories_updated_at_idx
    ON canonical_memories (updated_at DESC);

CREATE INDEX IF NOT EXISTS canonical_memories_search_text_idx
    ON canonical_memories
    USING GIN (search_text);

CREATE INDEX IF NOT EXISTS memory_versions_memory_created_at_idx
    ON memory_versions (memory_id, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_rebuilds_status_requested_at_idx
    ON embedding_rebuilds (status, requested_at ASC);

CREATE INDEX IF NOT EXISTS embedding_rebuilds_scope_status_idx
    ON embedding_rebuilds (tenant, project, namespace, status, requested_at ASC);

CREATE INDEX IF NOT EXISTS provenance_links_scope_created_at_idx
    ON provenance_links (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS vector_revisions_memory_generated_at_idx
    ON vector_revisions (memory_id, generated_at DESC);

CREATE INDEX IF NOT EXISTS vector_revisions_scope_status_generated_at_idx
    ON vector_revisions (tenant, project, namespace, status, generated_at DESC);

CREATE INDEX IF NOT EXISTS governance_recovery_ledger_scope_created_at_idx
    ON governance_recovery_ledger (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS governance_recovery_ledger_raw_event_created_at_idx
    ON governance_recovery_ledger (raw_event_id, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_scope_created_at_idx
    ON embedding_recovery_ledger (tenant, project, namespace, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_memory_created_at_idx
    ON embedding_recovery_ledger (memory_id, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_cutover_plan_created_at_idx
    ON embedding_recovery_ledger (cutover_plan_id, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_cutover_plans_scope_status_created_at_idx
    ON embedding_cutover_plans (tenant, project, namespace, status, created_at DESC);

CREATE INDEX IF NOT EXISTS embedding_cutover_items_plan_status_updated_at_idx
    ON embedding_cutover_items (plan_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS embedding_cutover_items_scope_status_updated_at_idx
    ON embedding_cutover_items (tenant, project, namespace, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_scope_updated_at_idx
    ON relation_projections (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS relation_projections_search_text_idx
    ON relation_projections
    USING GIN (search_text);

CREATE INDEX IF NOT EXISTS job_executions_scope_started_at_idx
    ON job_executions (tenant, project, namespace, started_at DESC);

CREATE INDEX IF NOT EXISTS job_executions_job_name_started_at_idx
    ON job_executions (job_name, started_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS derived_insights_scope_type_fingerprint_idx
    ON derived_insights (tenant, project, namespace, type, derivation_fingerprint);

CREATE INDEX IF NOT EXISTS derived_insights_scope_state_type_idx
    ON derived_insights (tenant, project, namespace, lifecycle_state, type, updated_at DESC);

CREATE INDEX IF NOT EXISTS derived_insights_scope_confidence_idx
    ON derived_insights (tenant, project, namespace, confidence DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS derived_insight_evidence_insight_idx
    ON derived_insight_evidence (insight_id, created_at DESC);

CREATE INDEX IF NOT EXISTS derived_insight_lifecycle_ledger_insight_idx
    ON derived_insight_lifecycle_ledger (insight_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS derived_insight_feedback_insight_created_at_idx
    ON derived_insight_feedback (insight_id, created_at DESC);

CREATE INDEX IF NOT EXISTS derived_insight_feedback_scope_type_active_idx
    ON derived_insight_feedback (tenant, project, namespace, feedback_type, created_at DESC)
    WHERE superseded_at IS NULL;

CREATE INDEX IF NOT EXISTS derived_insight_replay_runs_scope_status_updated_at_idx
    ON derived_insight_replay_runs (tenant, project, namespace, status, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS derived_insight_replay_runs_scope_idempotency_idx
    ON derived_insight_replay_runs (tenant, project, namespace, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS quality_evaluation_runs_scope_updated_at_idx
    ON quality_evaluation_runs (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS quality_evaluation_findings_run_created_at_idx
    ON quality_evaluation_findings (evaluation_run_id, created_at ASC);

CREATE INDEX IF NOT EXISTS repair_plans_scope_updated_at_idx
    ON repair_plans (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS repair_actions_scope_status_next_attempt_idx
    ON repair_actions (tenant, project, namespace, status, next_attempt_at, updated_at ASC);

CREATE INDEX IF NOT EXISTS admission_pressure_audit_scope_observed_at_idx
    ON admission_pressure_audit (tenant, project, namespace, observed_at DESC);

CREATE INDEX IF NOT EXISTS scope_proof_runs_scope_updated_at_idx
    ON scope_proof_runs (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS scope_proof_runs_scope_status_updated_at_idx
    ON scope_proof_runs (tenant, project, namespace, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS scope_proof_steps_scope_status_next_attempt_idx
    ON scope_proof_steps (tenant, project, namespace, status, next_attempt_at, updated_at ASC);

CREATE INDEX IF NOT EXISTS scope_proof_steps_proof_created_at_idx
    ON scope_proof_steps (proof_id, created_at ASC);

CREATE INDEX IF NOT EXISTS memory_session_runs_scope_updated_at_idx
    ON memory_session_runs (tenant, project, namespace, updated_at DESC);

CREATE INDEX IF NOT EXISTS memory_session_runs_scope_status_updated_at_idx
    ON memory_session_runs (tenant, project, namespace, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS memory_session_turns_session_created_at_idx
    ON memory_session_turns (session_id, created_at ASC);

CREATE UNIQUE INDEX IF NOT EXISTS memory_session_turns_scope_idempotency_idx
    ON memory_session_turns (tenant, project, namespace, session_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS memory_session_turns_scope_outcome_idempotency_idx
    ON memory_session_turns (tenant, project, namespace, session_id, id, outcome_idempotency_key)
    WHERE outcome_idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS memory_session_verifications_scope_status_next_attempt_idx
    ON memory_session_verifications (tenant, project, namespace, status, next_attempt_at, updated_at ASC);

CREATE INDEX IF NOT EXISTS memory_loop_evidence_links_owner_idx
    ON memory_loop_evidence_links (tenant, project, namespace, owner_kind, owner_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS usefulness_feedback_scope_idempotency_idx
    ON usefulness_feedback (tenant, project, namespace, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS usefulness_feedback_scope_type_created_at_idx
    ON usefulness_feedback (tenant, project, namespace, feedback_type, created_at DESC);

CREATE INDEX IF NOT EXISTS usefulness_feedback_subjects_subject_idx
    ON usefulness_feedback_subjects (tenant, project, namespace, subject_kind, subject_id, expected_recall_kind, expected_recall_id, opaque_token);

CREATE INDEX IF NOT EXISTS usefulness_feedback_supersessions_feedback_idx
    ON usefulness_feedback_supersessions (tenant, project, namespace, feedback_id, superseded_at DESC);

CREATE INDEX IF NOT EXISTS usefulness_feedback_active_subject_idx
    ON usefulness_feedback_subjects (tenant, project, namespace, subject_kind, subject_id, expected_recall_kind, expected_recall_id, opaque_token, feedback_id);
