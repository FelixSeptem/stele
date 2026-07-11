## ADDED Requirements

### Requirement: Scope proof jobs use durable worker orchestration
The service SHALL execute scope proof steps through the durable worker or scheduler orchestration model.

#### Scenario: Worker claims proof step
- **WHEN** a proof run has a pending eligible step
- **THEN** a worker can claim it with durable ownership and process it without requiring the admin request to remain open

#### Scenario: Proof step fails retryably
- **WHEN** proof step execution fails before completion
- **THEN** the service persists attempt count, failure summary, next eligible retry time, and bounded failure category

#### Scenario: Duplicate proof dispatch occurs
- **WHEN** the same proof step is dispatched more than once across retries or restarts
- **THEN** the service resumes or skips idempotently without duplicating fixture writes, quality evaluations, replay runs, or report evidence

### Requirement: Memory session verification jobs use durable worker orchestration
The service SHALL execute asynchronous memory session verification through durable worker or scheduler orchestration.

#### Scenario: Worker claims session verification
- **WHEN** a session turn or session run is pending verification
- **THEN** a worker can claim verification with durable lease ownership and process only the authorized scope

#### Scenario: Session verification reaches bounded wait limit
- **WHEN** governed processing has not completed within the configured verification window
- **THEN** the worker records a degraded or failed verification result instead of blocking indefinitely
