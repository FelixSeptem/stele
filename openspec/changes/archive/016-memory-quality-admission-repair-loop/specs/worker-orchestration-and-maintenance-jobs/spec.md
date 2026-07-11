## ADDED Requirements

### Requirement: Quality repair jobs use durable worker orchestration
The service SHALL execute quality repair actions through the durable worker or scheduler orchestration model.

#### Scenario: Worker claims eligible repair work
- **WHEN** a repair action is approved, pending, and eligible for execution
- **THEN** a worker can claim the action with durable lease ownership and process it without requiring the admin request to remain open

#### Scenario: Repair job records retryable failure
- **WHEN** repair action execution fails before completion
- **THEN** the service persists attempt count, failure time, error summary, and next eligible retry time

#### Scenario: Repair job exhausts retry budget
- **WHEN** a repair action reaches the configured automatic retry limit
- **THEN** the service marks the action as exhausted or requiring manual review and excludes it from further automatic claim attempts

#### Scenario: Repair job resumes safely after duplicate dispatch
- **WHEN** the same repair action is dispatched or claimed more than once across retries or restarts
- **THEN** the service detects duplicate execution state and skips or safely resumes without duplicating durable side effects
