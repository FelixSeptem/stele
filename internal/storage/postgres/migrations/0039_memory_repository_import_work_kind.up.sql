-- Repository imports use the durable queue but remain independently claimable
-- from governed memory intent work.
ALTER TABLE derived_work_items
    DROP CONSTRAINT IF EXISTS derived_work_items_kind_check;

ALTER TABLE derived_work_items
    ADD CONSTRAINT derived_work_items_kind_check
    CHECK (kind IN (
        'reflection',
        'compaction',
        'projection_rebuild',
        'insight_maintenance',
        'freshness_retention',
        'memory_intent',
        'memory_repository_import'
    ));
