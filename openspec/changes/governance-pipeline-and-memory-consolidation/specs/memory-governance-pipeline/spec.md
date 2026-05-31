## ADDED Requirements

### Requirement: Worker-driven governance pipeline
The service SHALL process raw events into governed memory through an asynchronous worker-driven pipeline rather than the synchronous ingest request path.

#### Scenario: Worker claims ungoverned raw events
- **WHEN** the worker loop runs and raw events exist without completed governance processing
- **THEN** the worker can claim eligible raw events for candidate extraction without requiring a new client request

#### Scenario: Ingest path remains lightweight
- **WHEN** a client submits `POST /v1/events`
- **THEN** the request persists the raw event and returns without performing full candidate extraction or consolidation inline

### Requirement: Candidate memory persistence
The service MUST persist candidate memory as a first-class lifecycle state with governance metadata and source event linkage.

#### Scenario: Candidate extracted from raw event
- **WHEN** the worker extracts a memory candidate from a raw event
- **THEN** the service stores a candidate record with class, content, governance metadata, and linkage to the source raw event

#### Scenario: Candidate retains governance audit context
- **WHEN** a candidate memory is written
- **THEN** the service stores enough provenance and governance fields to explain later promotion, suppression, or expiry decisions
