## ADDED Requirements

### Requirement: Provider reference events declare supported promotion semantics

Provider reference writes SHALL use an allowlisted event or intent type and
document whether that type can produce a searchable canonical memory. Arbitrary
raw event types MUST NOT be treated as reference writes by convention.

#### Scenario: Allowlisted event enters governance
- **WHEN** a supported Provider event passes ordinary admission and idempotency
- **THEN** it enters the existing governed event lifecycle with its reference
  intent and promotion semantics preserved

#### Scenario: Unsupported event type is archived only
- **WHEN** an event type is not allowlisted for reference promotion
- **THEN** the service returns or records its archival outcome without claiming
  a canonical reference

### Requirement: Event status exposes durable governance completion

Provider event status used for references SHALL distinguish accepted, pending,
failed and completed governance, and completed SHALL require durable persistence
of all required downstream records.

#### Scenario: Status observes completed governance
- **WHEN** candidate, canonical and provenance records are durably committed
- **THEN** status reports completed governance with stable bounded identities

#### Scenario: Status observes a failed worker attempt
- **WHEN** governance has a retryable or terminal error
- **THEN** status reports a bounded failure category and does not return a
  completed reference

### Requirement: Reference path attribution remains part of event identity

Provider event ingestion SHALL include normalized `memory_path` in the durable
event, downstream intent, provenance and idempotency fingerprint used by the
reference contract.

#### Scenario: Non-root path is ingested
- **WHEN** an authorized event uses a valid non-root path
- **THEN** all governed records retain the normalized path for exact lookup

#### Scenario: Path changes on replay
- **WHEN** the same idempotency key is reused with a different normalized path
- **THEN** the service returns a bounded conflict and preserves the original
  event/path identity
