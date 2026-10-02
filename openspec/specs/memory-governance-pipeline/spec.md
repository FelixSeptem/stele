# memory-governance-pipeline Specification

## Purpose
Process raw events into governed memory through an asynchronous worker pipeline.

## Requirements

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

### Requirement: Accepted intents enter asynchronous governance
The governance pipeline MUST be able to consume accepted memory intents as durable work items and MUST apply the same scope, lifecycle, provenance, candidate, retry, and audit controls used for raw-event governance.

#### Scenario: Worker claims an accepted intent
- **WHEN** an accepted intent is queued and its lease is available
- **THEN** a worker claims it within the owning scope and emits candidate or lifecycle work through the existing governance path

#### Scenario: Intent processing exceeds retry budget
- **WHEN** an intent repeatedly fails a bounded dependency or processing step
- **THEN** the worker records a failed or suppressed outcome and stops retrying after the configured budget
