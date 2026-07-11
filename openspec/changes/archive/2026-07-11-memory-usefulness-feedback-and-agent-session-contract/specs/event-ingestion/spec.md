## ADDED Requirements

### Requirement: Session outcome payloads use the event ingestion contract
The service SHALL ingest memory-session outcome payloads through the same governed event ingestion lifecycle as ordinary event writes.

#### Scenario: Session outcome includes event payload
- **WHEN** a memory session outcome request includes a bounded event payload
- **THEN** the service validates and persists it as a raw event with explicit session id, turn id, outcome, actor, reason, and source attribution metadata

#### Scenario: Session outcome is retried
- **WHEN** a session outcome event payload is retried with the same idempotency key
- **THEN** the service avoids duplicate raw event creation while preserving the original event and session attribution

#### Scenario: Payload would bypass governance
- **WHEN** a session outcome payload attempts to write canonical memory or lifecycle state directly
- **THEN** the service rejects the request and requires the ordinary event -> candidate -> active lifecycle
