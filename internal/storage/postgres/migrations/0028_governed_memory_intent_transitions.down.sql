DROP TABLE IF EXISTS memory_intent_transitions;
ALTER TABLE memory_intents
    DROP CONSTRAINT IF EXISTS memory_intents_payload_bounded_check;
ALTER TABLE memory_intents
    DROP CONSTRAINT IF EXISTS memory_intents_status_check;
ALTER TABLE memory_intents
    ADD CONSTRAINT memory_intents_status_check
    CHECK (status IN ('pending', 'accepted', 'candidate', 'active', 'suppressed', 'rejected', 'failed'));
ALTER TABLE memory_intents
    DROP COLUMN IF EXISTS target_insight_id,
    DROP COLUMN IF EXISTS evidence_refs,
    DROP COLUMN IF EXISTS outcome_reference,
    DROP COLUMN IF EXISTS policy_version;
