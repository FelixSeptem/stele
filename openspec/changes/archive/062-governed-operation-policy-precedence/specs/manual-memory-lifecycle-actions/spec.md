## MODIFIED Requirements

### Requirement: Idempotent lifecycle action behavior

Repeated lifecycle action requests MUST remain safe for retry and operator re-entry. The service MUST evaluate exact scope, target lifecycle visibility, privileged principal grant, and any explicit lifecycle policy approval before idempotency lookup; identical authorized requests MUST replay the original outcome and conflicting reuse MUST fail closed without a second transition.

#### Scenario: Duplicate suppress or delete request is retried

- **WHEN** the same authorized lifecycle action is submitted more than once for the same memory with equivalent scope, policy, and idempotency metadata
- **THEN** the service avoids conflicting durable mutations and returns a stable post-action lifecycle outcome

#### Scenario: Lifecycle action reuses a key with a conflict

- **WHEN** an authorized caller reuses a lifecycle idempotency key with a different action, target fingerprint, or policy version
- **THEN** the service returns a bounded idempotency conflict and preserves the first lifecycle history
