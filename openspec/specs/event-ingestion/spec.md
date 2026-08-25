# event-ingestion Specification

## Purpose
Define the public raw event ingestion contract, including request validation, durable event persistence, stable identifiers, and provenance capture.
## Requirements
### Requirement: Event ingestion API
The service SHALL expose `POST /v1/events` for writing raw events into the memory system.

#### Scenario: Successful event ingestion
- **WHEN** a client submits a valid event ingestion request to `POST /v1/events`
- **THEN** the service persists the raw event and returns a stable event identifier

### Requirement: Event request validation
The service MUST validate event type, content, timestamps, metadata shape, and resolved request scope before accepting an event.

#### Scenario: Missing required event content
- **WHEN** a client submits an event ingestion request without required content
- **THEN** the service rejects the request as invalid

#### Scenario: Invalid scope on event ingestion
- **WHEN** a client submits an event ingestion request with unresolved or invalid scope
- **THEN** the service rejects the request before writing the event

### Requirement: Event provenance capture
The service SHALL record provenance for ingested events, including creation time, request lineage, and the authenticated scope context used for the write.

#### Scenario: Auditable ingested event
- **WHEN** a raw event is successfully ingested
- **THEN** the service stores enough provenance metadata to trace when and under which resolved scope the event was created

### Requirement: Stable event write contract
Successful event ingestion responses MUST return a durable event identifier that can be used by later workflows and diagnostics.

#### Scenario: Returned identifier can be used for later inspection
- **WHEN** a client receives a successful event ingestion response
- **THEN** the response contains a stable event identifier associated with the persisted event

### Requirement: Event ingestion surfaces admission pressure metadata
Successful event ingestion responses SHALL include pressure-aware admission metadata without removing the stable event identifier contract.

#### Scenario: Event is accepted normally
- **WHEN** a client submits a valid event and admission returns `accept`
- **THEN** the service persists the raw event, returns the stable event identifier, and includes admission metadata with decision `accept`

#### Scenario: Event is accepted while downstream work is degraded
- **WHEN** a client submits a valid event and admission returns `accept_degraded`
- **THEN** the service persists the raw event, returns the stable event identifier, and includes warning finding codes that describe downstream degradation

#### Scenario: Event is queued under pressure
- **WHEN** a client submits a valid event and admission returns `queue`
- **THEN** the service durably records the event or write intent according to the ingestion contract and returns admission metadata that identifies queued processing state

#### Scenario: Event is rejected before write
- **WHEN** a client submits an event and admission returns `reject`
- **THEN** the service rejects the request before writing a raw event and returns stable blocker finding codes

### Requirement: Ingestion accepts proof and session attribution
The event ingestion contract SHALL support optional proof and memory-session attribution metadata without weakening validation or scope isolation.

#### Scenario: Proof run writes fixture event
- **WHEN** proof execution ingests a smoke fixture event for a scope
- **THEN** the event is persisted through the ordinary ingestion contract with proof attribution and admission metadata

#### Scenario: Session records turn outcome event
- **WHEN** a memory session records a turn outcome that should enter the memory pipeline
- **THEN** the service writes the outcome through ordinary event ingestion with session and turn attribution

#### Scenario: Attribution scope mismatches event scope
- **WHEN** proof or session attribution references a run outside the resolved event scope
- **THEN** ingestion rejects the request before writing the event

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

### Requirement: Public event ingestion is idempotent per principal and scope
The service SHALL require a bounded idempotency key for `POST /v1/events` and
SHALL persist a durable idempotency decision scoped to the authenticated
principal and authorized tenant, project, and namespace.

#### Scenario: Exact event retry returns original result
- **WHEN** the same authenticated principal retries an event request in the same scope with the same idempotency key and equivalent normalized payload
- **THEN** the service returns the original durable event identifier and admission result without creating another raw event or provenance record

#### Scenario: Key is reused for another payload
- **WHEN** the same authenticated principal and scope reuse an idempotency key with a different normalized event payload
- **THEN** the service returns a conflict response and does not create another raw event

#### Scenario: Same key is used by another principal or scope
- **WHEN** a different authorized principal or a different authorized scope uses an idempotency key already used elsewhere
- **THEN** the service treats it as an independent idempotency domain and does not disclose the prior event or caller

#### Scenario: Retry follows interrupted write
- **WHEN** an event write is interrupted after idempotency claim acquisition but before completion
- **THEN** a later retry safely resumes after bounded durable lease recovery or receives a bounded retryable response without duplicate event creation

### Requirement: Ingestion idempotency preserves governance lifecycle
The service SHALL apply idempotency before creating a raw event and SHALL not
allow a retry to bypass admission, provenance, or the ordinary
event-to-candidate-to-active lifecycle.

#### Scenario: First event write is accepted
- **WHEN** an authorized event request with a new idempotency key passes admission validation
- **THEN** the service persists one raw event, one provenance chain, and one completed idempotency result transactionally

#### Scenario: Admission rejection is retried
- **WHEN** an event request is rejected before a raw event is written because admission returns a blocking decision
- **THEN** the service returns the stable rejection category and does not persist a completed event idempotency result

