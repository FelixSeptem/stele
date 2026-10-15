## ADDED Requirements

### Requirement: Provider reference consumers validate the complete contract

Strict Provider consumers SHALL negotiate the durable-reference capability,
validate closed write/status/read DTOs, distinguish governance states and
lifecycle outcomes, and reject missing or incompatible lineage fields.

#### Scenario: Consumer accepts a completed reference
- **WHEN** a response contains the negotiated version, exact scope proof,
  completed status and required bounded lineage
- **THEN** the consumer records the reference without implementing local
  canonicalization, ranking or fallback lookup

#### Scenario: Consumer receives incomplete status
- **WHEN** a response omits required status, path, identity or governance fields
- **THEN** the consumer records a failed-contract outcome rather than treating it
  as an empty or completed reference

### Requirement: Reference errors remain bounded and non-disclosive

Provider reference errors SHALL use documented categories for validation,
compatibility, conflict, pending, failed, unavailable, lifecycle and scope
outcomes without exposing foreign existence, raw payloads or internal SQL data.

#### Scenario: Conflicting replay is returned
- **WHEN** an idempotency key is reused with a changed normalized payload
- **THEN** the consumer receives a bounded conflict and does not retry with a
  broader scope or alternate write shape
