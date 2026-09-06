# Memory Intent Ledger

## ADDED Requirements

### Requirement: Accept governed memory intents
The service MUST accept `remember`, `update`, `forget`, `contradiction`, and `feedback` intents with explicit project, tenant, namespace, actor, reason, provenance, request ID, operation ID, and idempotency key.

#### Scenario: New intent
- **WHEN** a valid intent is submitted
- **THEN** the service appends one intent record and returns its durable identity and governance status

#### Scenario: Idempotent retry
- **WHEN** the same scoped idempotency key is submitted again
- **THEN** the service returns the original result without appending a second effect

### Requirement: Prevent direct canonical mutation
The intent path MUST route accepted requests through the existing candidate/governance workflow and MUST NOT overwrite canonical memory directly.

#### Scenario: Invalid target version
- **WHEN** an update or forget intent references a missing, foreign-scope, suppressed, forgotten, or stale target
- **THEN** validation rejects it before any mutation or enqueue side effect
