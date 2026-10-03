ALTER TABLE governed_reasoning_insight_candidates
    ADD COLUMN IF NOT EXISTS goal_metadata jsonb;

ALTER TABLE governed_reasoning_insight_candidates
    DROP CONSTRAINT IF EXISTS governed_reasoning_goal_metadata_object;

ALTER TABLE governed_reasoning_insight_candidates
    ADD CONSTRAINT governed_reasoning_goal_metadata_object
    CHECK (goal_metadata IS NULL OR jsonb_typeof(goal_metadata) = 'object');
