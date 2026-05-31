## ADDED Requirements

### Requirement: Base PostgreSQL schema for memory service
The service SHALL define a PostgreSQL schema that supports raw event storage, canonical memory storage, version history, and provenance links.

#### Scenario: Fresh database bootstrap
- **WHEN** migrations are applied to a clean PostgreSQL database
- **THEN** the database contains base tables for raw events, canonical memories, memory versions, and provenance links

### Requirement: Memory schema indexing
The base schema MUST include indexes supporting scope filtering, lifecycle state filtering, and time-based access patterns.

#### Scenario: Scope and state indexes exist
- **WHEN** base migrations complete
- **THEN** the resulting schema includes indexes required for scope-constrained and state-constrained access patterns

### Requirement: Future retrieval extension points
The base schema SHALL reserve extension points for future semantic and lexical retrieval without requiring a foundational redesign.

#### Scenario: Retrieval extension compatibility
- **WHEN** later changes add vector or full-text retrieval capabilities
- **THEN** they can extend the base schema without replacing raw event or canonical memory tables

### Requirement: Append-only raw event storage
Raw event storage MUST preserve ingested event payloads as append-only records.

#### Scenario: Repeated ingestion creates distinct records
- **WHEN** two separate raw events are ingested for the same scope
- **THEN** the service stores them as distinct raw event records rather than updating a prior event in place
