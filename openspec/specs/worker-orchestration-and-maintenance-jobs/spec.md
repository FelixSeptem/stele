# worker-orchestration-and-maintenance-jobs Specification

## Purpose
Define the worker and scheduler orchestration behaviors that drive governed background processing, maintenance execution, and retry-safe job handling.
## Requirements
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

### Requirement: Asynchronous embedding reindex execution
The service SHALL execute semantic backfill, rebuild, and provider-rotation work through the existing durable worker or scheduler runtime model rather than inline write paths.

#### Scenario: Background runtime claims eligible embedding work
- **WHEN** canonical memories are marked eligible for semantic backfill, rebuild, or provider-target drift correction
- **THEN** the service can claim and process that work asynchronously without requiring a foreground memory mutation or retrieval request

#### Scenario: Reindex execution retries safely after failure
- **WHEN** embedding generation or activation fails for a claimed reindex target
- **THEN** the service records durable failure state and can retry later without duplicating active-vector promotion or corrupting semantic lineage

#### Scenario: Provider rotation reuses the same durable execution path
- **WHEN** the desired embedding provider or model target changes for eligible canonical memory
- **THEN** the service schedules provider-drift rebuild work through the same durable asynchronous execution path used for missing or stale embeddings

### Requirement: Embedding operator recovery reuses durable rebuild execution
The service SHALL treat operator-triggered embedding retry or requeue actions as state transitions back into the ordinary durable rebuild path rather than as ad hoc execution shortcuts.

#### Scenario: Requeue returns work to normal claim flow
- **WHEN** an authorized operator requeues eligible embedding rebuild work
- **THEN** the rebuild record becomes claimable by the existing background rebuild job instead of being executed inline by the admin request

#### Scenario: Retry preserves idempotent execution semantics
- **WHEN** operator remediation restores failed embedding work to pending eligibility
- **THEN** later background execution still uses the same append, compare, and promote safeguards as scheduler-discovered rebuild work

### Requirement: Embedding recovery actions do not seize active leases
The service MUST reject embedding remediation actions that would bypass active background ownership of a rebuild record.

#### Scenario: Operator targets rebuilding work
- **WHEN** an operator attempts to retry or requeue a rebuild record that is currently rebuilding under an active lease
- **THEN** the action is rejected and the existing worker ownership remains unchanged

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
