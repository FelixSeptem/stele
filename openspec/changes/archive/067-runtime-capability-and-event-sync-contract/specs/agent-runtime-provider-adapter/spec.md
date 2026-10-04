## ADDED Requirements

### Requirement: Provider exposes resumable synchronization

The provider adapter SHALL expose a bounded synchronization operation that uses the authenticated runtime binding, supports initial snapshot and cursor-based continuation, and returns machine-readable completion, retry, compatibility, scope, and resynchronization outcomes.

#### Scenario: Provider starts a synchronization

- **WHEN** a bound runtime requests synchronization with a supported schema version
- **THEN** the adapter returns the scoped snapshot or next event batch with the current cursor and completion state

#### Scenario: Provider reports a retention gap

- **WHEN** the runtime cursor is older than the provider's retained replay window
- **THEN** the adapter returns the provider error category for resynchronization and does not claim that the runtime is synchronized

### Requirement: Provider synchronization preserves runtime identity

Synchronization requests SHALL retain the provider binding's tenant, project, namespace, agent, session, and provider-instance identity across initial sync, continuation, and retry. The adapter MUST reject cursor reuse with another binding or scope.

#### Scenario: Cursor is reused by another session

- **WHEN** a runtime presents a cursor issued to a different session or provider instance
- **THEN** the adapter returns a bounded scope or compatibility error and does not reveal the cursor owner's state

### Requirement: Provider conformance verifies synchronization recovery

The provider conformance profile SHALL include initial snapshot, ordered replay, cursor acknowledgment, duplicate retry, restart recovery, retention-gap resynchronization, scope rejection, schema incompatibility, redaction, and transport-neutral equivalence cases. A failed or incomplete synchronization case MUST make the conformance result ineligible for provider readiness.

#### Scenario: Conformance passes synchronization recovery

- **WHEN** all required synchronization fixtures complete with deterministic replay and exact-scope evidence
- **THEN** the conformance report records synchronization compatibility and recovery evidence as separate bounded categories

#### Scenario: Conformance detects nondeterministic replay

- **WHEN** the same binding and cursor produce different event identity, ordering, or completion results across replay
- **THEN** the conformance run records a synchronization failure and does not claim provider readiness
