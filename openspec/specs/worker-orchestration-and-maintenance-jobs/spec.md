# worker-orchestration-and-maintenance-jobs Specification

## Purpose
Define the worker and scheduler orchestration behaviors that drive governed background processing, maintenance execution, and retry-safe job handling.
## Requirements
### Requirement: Reliable worker orchestration
The service SHALL run governance work through a stable worker orchestration path rather than a single fire-once execution model.

#### Scenario: Worker retries a failed eligible job
- **WHEN** a governance or maintenance job fails after being claimed
- **THEN** the service records the failure state and can retry the job according to bounded retry rules without requiring a new client request

#### Scenario: Lease expiry allows recovery
- **WHEN** a worker crashes or loses its lease before completing a claimed job
- **THEN** the service can make that job eligible for later reclaim by another worker without duplicating the committed side effects

### Requirement: Idempotent maintenance execution
The service MUST keep repeated governance and maintenance execution idempotent enough for restart and retry safety.

#### Scenario: A maintenance job is re-run after partial progress
- **WHEN** a retention, compaction, or cleanup task is executed more than once for the same eligible target set
- **THEN** the service avoids creating duplicate durable mutations that would violate lifecycle or provenance expectations

### Requirement: Scheduled maintenance dispatch
The service SHALL provide a scheduler-driven path for periodic maintenance jobs that stays separate from public request handling.

#### Scenario: Scheduler trigger fires on cadence
- **WHEN** the configured maintenance cadence is reached
- **THEN** the scheduler can dispatch retention, expiry, compaction, or cleanup work without requiring traffic on the public API
