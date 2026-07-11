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

