## ADDED Requirements

### Requirement: Explicit manual lifecycle actions
The service SHALL expose explicit API actions for manual lifecycle management of canonical memory.

#### Scenario: Privileged caller suppresses a memory
- **WHEN** a privileged caller applies a suppress action to a canonical memory
- **THEN** the memory becomes hidden from default reads while remaining available for audit and controlled inspection

#### Scenario: Privileged caller expires a memory
- **WHEN** a privileged caller applies an expire action to a canonical memory
- **THEN** the service updates lifecycle visibility according to forgetting semantics without requiring destructive payload deletion

#### Scenario: Privileged caller deletes a memory
- **WHEN** a privileged caller applies a delete action to a canonical memory
- **THEN** the service removes payload and read projections according to deletion policy while preserving a durable audit marker

### Requirement: Lifecycle actions remain on a privileged management surface
Manual lifecycle actions MUST remain separated from ordinary public memory reads.

#### Scenario: Standard reader accesses memory APIs
- **WHEN** a standard product caller uses canonical memory read APIs
- **THEN** the caller can read lifecycle-safe memory resources but cannot invoke manual suppress, expire, or delete actions without privileged authorization

### Requirement: Lifecycle action audit attribution
Manual lifecycle actions MUST record stable attribution and operator intent.

#### Scenario: Action reason and actor are captured
- **WHEN** a lifecycle action is applied through the API
- **THEN** the resulting audit trail includes actor identity, action type, reason, and applied timestamp

### Requirement: Idempotent lifecycle action behavior
Repeated lifecycle action requests MUST remain safe for retry and operator re-entry.

#### Scenario: Duplicate suppress or delete request is retried
- **WHEN** the same lifecycle action is submitted more than once for the same memory
- **THEN** the service avoids conflicting durable mutations and returns a stable post-action lifecycle outcome
