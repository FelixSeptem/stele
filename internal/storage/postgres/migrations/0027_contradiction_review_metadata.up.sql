ALTER TABLE governed_reasoning_insight_candidates
    ADD COLUMN IF NOT EXISTS contradiction_temporal_disposition text,
    ADD COLUMN IF NOT EXISTS contradiction_review_state text NOT NULL DEFAULT 'review_required',
    ADD COLUMN IF NOT EXISTS contradiction_overlap_from timestamptz,
    ADD COLUMN IF NOT EXISTS contradiction_overlap_to timestamptz,
    ADD COLUMN IF NOT EXISTS reviewed_by text,
    ADD COLUMN IF NOT EXISTS reviewed_at timestamptz,
    ADD COLUMN IF NOT EXISTS review_reason text NOT NULL DEFAULT '';

ALTER TABLE governed_reasoning_insight_candidates
    DROP CONSTRAINT IF EXISTS governed_reasoning_insight_candidates_contradiction_temporal_check;
ALTER TABLE governed_reasoning_insight_candidates
    ADD CONSTRAINT governed_reasoning_insight_candidates_contradiction_temporal_check
    CHECK (contradiction_temporal_disposition IS NULL OR contradiction_temporal_disposition IN ('contradiction', 'temporal_coexistence', 'unresolved_temporal'));

ALTER TABLE governed_reasoning_insight_candidates
    DROP CONSTRAINT IF EXISTS governed_reasoning_insight_candidates_review_state_check;
ALTER TABLE governed_reasoning_insight_candidates
    ADD CONSTRAINT governed_reasoning_insight_candidates_review_state_check
    CHECK (contradiction_review_state IN ('review_required', 'confirmed', 'coexists', 'incorrect', 'stale'));

CREATE INDEX IF NOT EXISTS governed_reasoning_candidates_scope_review_idx
    ON governed_reasoning_insight_candidates (tenant, project, namespace, contradiction_review_state, created_at DESC);
