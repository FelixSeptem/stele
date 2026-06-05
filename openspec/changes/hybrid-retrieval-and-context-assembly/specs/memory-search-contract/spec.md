## ADDED Requirements

### Requirement: Public memory search contract
The service SHALL expose a stable memory search contract for governed retrieval over canonical memory and summary memory.

#### Scenario: Search request declares retrieval scope and filters
- **WHEN** a client submits a memory search request
- **THEN** the request supports query text, scope filters, class filters, time window, top-k, summary inclusion, and optional relation inclusion

#### Scenario: Search response returns ranked governed memory hits
- **WHEN** the service returns a successful search response
- **THEN** the response includes ranked hits with stable memory identifiers, citations, and score metadata

### Requirement: Structured search result metadata
The service MUST return enough retrieval metadata to support downstream SDK use without exposing raw internal storage details.

#### Scenario: Client inspects a ranked result
- **WHEN** a client receives a search result hit
- **THEN** the result includes memory class, lifecycle-safe state representation, timestamps, and citation-ready provenance references
