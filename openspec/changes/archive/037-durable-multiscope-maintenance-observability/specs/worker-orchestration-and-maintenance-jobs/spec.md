## ADDED Requirements

### Requirement: Maintenance executions expose stable recovery state

The service SHALL expose durable execution state for maintenance jobs,
including stable identity, attempt number, lease owner/expiry, retry outcome,
checkpoint or source watermark, and terminal disposition, so restarts and
operator inspection do not depend on process memory.

#### Scenario: Execution state is inspected after restart

- **WHEN** a scheduler or worker starts after an interrupted maintenance run
- **THEN** it can determine whether to resume, reclaim, retry, or suppress the run from durable execution state

#### Scenario: Terminal execution is replayed

- **WHEN** the same stable identity is submitted after a successful terminal execution
- **THEN** the scheduler records a bounded duplicate disposition and does not execute the maintenance work again
