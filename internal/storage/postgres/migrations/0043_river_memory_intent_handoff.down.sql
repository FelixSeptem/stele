DROP INDEX IF EXISTS derived_work_memory_intent_handoff_idx;
-- Keep the River job references, handoff timestamp, and append-only table for
-- audit-safe rollback.
