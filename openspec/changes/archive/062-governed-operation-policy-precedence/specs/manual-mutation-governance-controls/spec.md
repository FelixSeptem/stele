## MODIFIED Requirements

### Requirement: Manual mutations are optimistic-concurrency-aware

The service SHALL protect manual canonical memory mutation from silent operator overwrite by applying the governed-operation precedence contract before the concurrency guard and durable mutation. Exact scope, lifecycle visibility, privileged principal grant, explicit mutation approval, and idempotency/replay checks MUST pass first; a stale expected version then rejects the mutation with a conflict instead of overwriting newer canonical state.

#### Scenario: Expected version does not match current version

- **WHEN** a privileged caller submits an update, merge, or reclassification request against a stale expected version after all earlier precedence gates pass
- **THEN** the service rejects the mutation with a conflict outcome instead of overwriting the newer canonical state

#### Scenario: Earlier governance gate fails

- **WHEN** a manual mutation has foreign scope, hidden target, missing grant, disabled policy, or conflicting replay metadata
- **THEN** the service rejects it before reading the current version or writing canonical or projection records
