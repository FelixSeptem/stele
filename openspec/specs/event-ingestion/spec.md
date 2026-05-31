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
