## ADDED Requirements

### Requirement: Scheduler dispatches provider cutover waves through the durable rebuild path
The service SHALL advance active provider cutovers through scheduler-driven waves that reuse the existing embedding rebuild execution flow.

#### Scenario: Active cutover schedules the next wave
- **WHEN** an active provider cutover has remaining eligible items and the next cadence window arrives
- **THEN** the scheduler advances only the next bounded wave of cutover items into ordinary rebuild eligibility instead of executing embeddings inline

#### Scenario: Cutover wave failure remains retry-safe
- **WHEN** one or more items in a cutover wave fail embedding generation or activation
- **THEN** those failures remain visible through the normal durable rebuild and recovery path without corrupting plan progress or vector lineage

### Requirement: Cutover controls do not seize active rebuild ownership
The service MUST keep cutover pause or cancel behavior lease-safe for already rebuilding embedding work.

#### Scenario: Operator pauses a plan with active rebuilds in flight
- **WHEN** an operator pauses or cancels a cutover plan while some linked memories are already rebuilding
- **THEN** the service stops future waves from advancing but leaves already rebuilding items under their current worker ownership until they complete or fail
