ALTER TABLE governed_reasoning_insight_candidates
    DROP CONSTRAINT IF EXISTS governed_reasoning_goal_metadata_object;

ALTER TABLE governed_reasoning_insight_candidates
    DROP COLUMN IF EXISTS goal_metadata;
