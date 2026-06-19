## MODIFIED Requirements

### Requirement: Reliable worker orchestration
The service SHALL run governance work through a durable worker orchestration path that persists lease, failure, and retry state rather than relying on a fire-once execution model.

#### Scenario: Worker records retryable failure
- **WHEN** processing a claimed governance raw event fails before completion
- **THEN** the service persists attempt count, failure time, error summary, and the next eligible retry time so the event can be retried without requiring a new client request

#### Scenario: Worker exhausts retry budget
- **WHEN** a governance raw event reaches the configured automatic retry limit
- **THEN** the service marks that event as exhausted or quarantined for audit and excludes it from further automatic claim attempts until a later explicit recovery path intervenes

#### Scenario: Lease renewal protects long-running processing
- **WHEN** a valid governance raw event processing attempt outlives the initial lease window
- **THEN** the active worker can renew the lease before expiry so another worker does not concurrently reclaim the same event

#### Scenario: Lease expiry still allows crash recovery
- **WHEN** a worker crashes or loses its lease before completing a claimed job
- **THEN** the service can make that unfinished and non-exhausted event eligible for later reclaim by another worker without duplicating committed side effects

#### Scenario: Operator requeues an exhausted raw event
- **WHEN** an authorized recovery action clears the exhausted terminal state for a governance raw event
- **THEN** the service returns that event to the ordinary worker claim path instead of executing it through a separate recovery-only processor

#### Scenario: Recovery action does not seize leased ownership
- **WHEN** an operator targets a governance raw event that is still under an active worker lease
- **THEN** the service rejects the recovery action rather than bypassing the durable worker ownership contract
