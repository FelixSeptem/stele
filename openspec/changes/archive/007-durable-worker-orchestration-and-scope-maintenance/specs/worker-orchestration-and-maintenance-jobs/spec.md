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

### Requirement: Idempotent maintenance execution
The service MUST keep repeated governance and maintenance execution idempotent at the job, scope, and cadence-window level for restart and retry safety.

#### Scenario: A maintenance job is re-run after partial progress
- **WHEN** a retention, compaction, or cleanup task is executed more than once for the same eligible target set
- **THEN** the service avoids creating duplicate durable mutations that would violate lifecycle or provenance expectations

#### Scenario: Duplicate scheduler trigger hits the same scope window
- **WHEN** the same maintenance job is triggered again for the same scope and cadence window
- **THEN** the service detects the duplicate execution and skips or safely resumes it without applying duplicate durable mutations

### Requirement: Scheduled maintenance dispatch
The service SHALL provide a scheduler-driven, scope-aware path for periodic maintenance jobs that stays separate from public request handling.

#### Scenario: Scheduler trigger fires on cadence
- **WHEN** the configured maintenance cadence is reached
- **THEN** the scheduler can dispatch retention, expiry, compaction, or cleanup work without requiring traffic on the public API

#### Scenario: Scheduler dispatches scope-bound jobs per eligible scope
- **WHEN** scope-bound maintenance work is due
- **THEN** the scheduler enumerates eligible scopes and launches summary compaction or retention work per scope rather than relying only on a single static default scope

#### Scenario: Runtime-global cleanup remains singular
- **WHEN** a maintenance job is runtime-global rather than scope-bound
- **THEN** the scheduler executes it once per cadence window without multiplying the same cleanup work across all discovered scopes
