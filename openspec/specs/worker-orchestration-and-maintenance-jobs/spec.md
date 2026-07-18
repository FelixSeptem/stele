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

### Requirement: Derived insight derivation runs asynchronously
The service SHALL derive governed experience insights through the worker or scheduler runtime rather than foreground API request paths.

#### Scenario: Scheduler runs insight derivation
- **WHEN** the configured maintenance cadence reaches a scope eligible for derived insight processing
- **THEN** the scheduler can run a bounded derivation job that evaluates repeated failure evidence for that scope

#### Scenario: Ingest path remains lightweight
- **WHEN** a client submits a raw event or mutates memory
- **THEN** the request does not synchronously derive failure patterns or lessons

### Requirement: Derived insight derivation is idempotent
The service MUST keep repeated insight derivation safe across retries, restarts, and duplicate scheduler triggers.

#### Scenario: Duplicate derivation sees same evidence window
- **WHEN** the same derivation job runs again for the same scope and evidence window
- **THEN** the service updates the same derived insight fingerprint instead of creating duplicate active failure patterns

#### Scenario: Derivation job fails
- **WHEN** derived insight processing fails before completion
- **THEN** the service records job failure through the existing durable job execution path without partially activating unsupported insights

### Requirement: Insight derivation consumes feedback idempotently
The service SHALL allow background insight derivation and maintenance jobs to consume quality feedback summaries in an idempotent, retry-safe manner.

#### Scenario: Derivation consumes same feedback twice
- **WHEN** a derivation job is retried for the same scope, evidence window, and feedback state
- **THEN** the service applies the same effective insight update or lifecycle decision without creating duplicate feedback-driven transitions

#### Scenario: Feedback changes between derivation runs
- **WHEN** new feedback is recorded after a prior derivation run
- **THEN** a later derivation or maintenance run can evaluate the updated quality summary and record any resulting insight change with fresh audit attribution

### Requirement: Feedback-driven suppression is scheduler-safe
The service MUST perform feedback-driven suppression or review marking through durable background execution semantics rather than foreground feedback write shortcuts.

#### Scenario: Operator records negative feedback
- **WHEN** an operator records negative feedback for an active insight
- **THEN** the feedback write path does not synchronously execute broad insight derivation or maintenance work

#### Scenario: Scheduler processes feedback-driven lifecycle work
- **WHEN** scheduled insight maintenance evaluates feedback that meets a suppression or review policy
- **THEN** it records the lifecycle or review decision through the same durable, auditable job path used for derived insight maintenance

### Requirement: Replay apply uses durable worker execution
The service SHALL execute derived insight replay apply and backfill work through the durable worker or scheduler model with leases, retry state, idempotency, and failure summaries.

#### Scenario: Worker claims replay work
- **WHEN** a replay apply run is pending and eligible
- **THEN** a worker can claim it with durable ownership and process it without requiring the admin request to remain open

#### Scenario: Replay worker fails
- **WHEN** replay execution fails before completion
- **THEN** the service records attempt count, failure summary, next eligibility, and partial report state so the run can be retried or inspected

### Requirement: Replay scheduling remains scope-bound and bounded
The service MUST keep scheduled or operator-triggered replay execution limited to the requested scope, evidence window, insight types, and configured execution limits.

#### Scenario: Replay backfill is scheduled
- **WHEN** a bounded replay backfill is queued for one authorized scope
- **THEN** the scheduler or worker processes only that scope and window rather than enumerating unrelated scopes

#### Scenario: Replay reaches execution limit
- **WHEN** replay processing reaches the configured evidence or decision limit before the window is exhausted
- **THEN** the service records a bounded completion or continuation-required status instead of silently scanning beyond the limit

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

### Requirement: Assurance jobs use durable worker orchestration
The service SHALL execute periodic health evaluations, incident refresh, alert candidate generation, and recovery verification through durable scheduler or worker orchestration.

#### Scenario: Scheduler dispatches health evaluation
- **WHEN** the configured assurance cadence is reached for an eligible scope
- **THEN** the scheduler can dispatch a bounded health evaluation job without requiring traffic on the public API

#### Scenario: Scheduler dispatches operational proof checks
- **WHEN** configured capacity/load or backup/restore proof freshness windows require evaluation for an eligible scope
- **THEN** the scheduler can dispatch bounded proof checks or proof freshness evaluation without executing external backup tooling or unbounded load generation

#### Scenario: Worker processes alert delivery
- **WHEN** an alert candidate is eligible for delivery
- **THEN** a worker can claim the delivery attempt with durable ownership, record result, and retry later without duplicating successful deliveries

#### Scenario: Assurance job fails retryably
- **WHEN** health evaluation, alert delivery, incident refresh, or recovery verification fails before completion
- **THEN** the service records attempt count, failure category, next eligibility, and bounded error summary

#### Scenario: Scheduler dispatches assurance cleanup
- **WHEN** configured assurance or conformance history retention windows have elapsed
- **THEN** the scheduler dispatches cleanup work that removes eligible high-volume records while preserving incident records and incident transition audit history

### Requirement: Conformance jobs use durable worker orchestration
The service SHALL execute scheduled conformance runs and stale integration checks through durable scheduler or worker orchestration.

#### Scenario: Scheduler dispatches conformance run
- **WHEN** a conformance profile is active and its cadence window arrives
- **THEN** the scheduler can dispatch a scoped conformance run using the durable job model

#### Scenario: Duplicate conformance dispatch occurs
- **WHEN** the same profile and cadence window are dispatched more than once across retries or restarts
- **THEN** the service resumes or skips idempotently without creating duplicate active diagnostics

#### Scenario: Conformance job reaches bounds
- **WHEN** conformance processing reaches configured evidence, time, or diagnostic limits
- **THEN** the service records bounded degraded or continuation-required status instead of scanning beyond limits

