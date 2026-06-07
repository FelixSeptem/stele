# admin-inspection-surface Specification

## Purpose
Define the privileged inspection APIs that let operators review governed memory internals, history, and maintenance state without direct database access.
## Requirements
### Requirement: Admin inspection remains separate from public APIs
The service SHALL expose operational inspection surfaces through an admin-only route namespace and auth boundary separate from public product APIs.

#### Scenario: Operator accesses runtime diagnostics
- **WHEN** a caller requests admin inspection endpoints
- **THEN** the request is handled through an admin-specific surface rather than the standard public API contract

### Requirement: Job and backlog inspection
The service MUST support inspection of worker and scheduler execution state without requiring direct database access.

#### Scenario: Operator checks maintenance health
- **WHEN** an operator requests job or backlog state
- **THEN** the service can return current or recent status for job execution, retry state, queue or backlog pressure, and maintenance cadence health

### Requirement: Memory history and lifecycle diagnostics
The service MUST support operator inspection of governed memory history and hidden lifecycle states without weakening public retrieval safety defaults.

#### Scenario: Operator investigates a hidden memory
- **WHEN** a memory was suppressed, forgotten, expired, or deleted
- **THEN** the admin surface can expose the relevant history, lifecycle state transitions, and provenance diagnostics while public retrieval remains lifecycle-safe by default
