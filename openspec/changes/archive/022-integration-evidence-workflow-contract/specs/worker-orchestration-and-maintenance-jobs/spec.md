## ADDED Requirements

### Requirement: Workflow diagnostics use durable worker orchestration
The service SHALL execute stale workflow detection, gap materialization, next-action refresh, and workflow cleanup through durable scheduler or worker orchestration.

#### Scenario: Scheduler dispatches stale workflow scan
- **WHEN** configured workflow freshness windows or completion windows elapse for eligible scoped workflow runs
- **THEN** the scheduler can dispatch bounded stale workflow scan jobs without requiring traffic on public APIs

#### Scenario: Worker materializes workflow gaps
- **WHEN** a workflow run is eligible for diagnostic processing
- **THEN** a worker can claim the run or diagnostic job with durable ownership, record bounded gap diagnostics and next actions, and retry later without duplicating active diagnostics

#### Scenario: Duplicate workflow diagnostic dispatch occurs
- **WHEN** the same workflow run and diagnostic window are dispatched more than once across retries or restarts
- **THEN** the service resumes or skips idempotently without creating duplicate active diagnostics

#### Scenario: Workflow cleanup is dispatched
- **WHEN** configured workflow retention windows have elapsed
- **THEN** the scheduler dispatches cleanup work that removes eligible high-volume workflow records while preserving active templates and required audit transitions
