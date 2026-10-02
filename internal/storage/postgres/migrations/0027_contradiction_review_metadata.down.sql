DROP INDEX IF EXISTS governed_reasoning_candidates_scope_review_idx;
ALTER TABLE governed_reasoning_insight_candidates
    DROP CONSTRAINT IF EXISTS governed_reasoning_insight_candidates_contradiction_temporal_check,
    DROP CONSTRAINT IF EXISTS governed_reasoning_insight_candidates_review_state_check,
    DROP COLUMN IF EXISTS contradiction_temporal_disposition,
    DROP COLUMN IF EXISTS contradiction_review_state,
    DROP COLUMN IF EXISTS contradiction_overlap_from,
    DROP COLUMN IF EXISTS contradiction_overlap_to,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS review_reason;
