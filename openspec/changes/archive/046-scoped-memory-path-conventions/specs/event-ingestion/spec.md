## ADDED Requirements

### Requirement: Event ingestion carries a governed memory path
The event ingestion contract SHALL accept an optional normalized memory path, persist it with the raw event and derived governed intent, and include it in normalized idempotency and provenance data without changing exact scope authorization.

#### Scenario: Event includes a valid path
- **WHEN** an authorized client submits a valid event with `memory_path=agents/research`
- **THEN** the event, its governed downstream intent, and its provenance retain the normalized path

#### Scenario: Event path is invalid
- **WHEN** an event contains a malformed or overlong memory path
- **THEN** ingestion rejects it before creating an event, idempotency result, or downstream intent

#### Scenario: Retried event preserves path identity
- **WHEN** an equivalent event with the same principal, scope, idempotency key, and normalized path is retried
- **THEN** ingestion returns the original outcome without creating a duplicate event
