## ADDED Requirements

### Requirement: First-ten-minutes smoke loop covers ingest to context
The service MUST provide a documented first-ten-minutes smoke loop that proves a self-hosted deployment can start, ingest scoped events, process background work, retrieve memory, assemble context, inspect admin state, and observe runtime signals.

#### Scenario: Operator runs full smoke loop
- **WHEN** an operator follows the smoke loop for a fresh self-hosted deployment
- **THEN** the documented flow verifies readiness, event ingestion, worker processing, scheduler maintenance, memory search, context assembly, admin inspection, and metrics without requiring source-code inspection

#### Scenario: Smoke loop fails
- **WHEN** a smoke-loop step fails
- **THEN** the documentation identifies the next admin inspection, readiness diagnostic, job status, or metric check needed to distinguish configuration, database, worker, scheduler, retrieval, or replay failure

### Requirement: Smoke loop includes derived insight replay verification
The service SHALL include derived insight dry-run and bounded apply verification in the self-hosted smoke loop when derived insight processing is enabled.

#### Scenario: Operator verifies replay dry-run
- **WHEN** the smoke fixture has generated eligible insight evidence
- **THEN** the documented smoke loop can run an admin replay dry-run and show expected replay decisions without mutating insight state

#### Scenario: Operator verifies replay apply
- **WHEN** an operator executes the bounded smoke replay apply step
- **THEN** the documented flow shows how to inspect replay status, replay report counters, resulting derived insight visibility, and context assembly output

### Requirement: Smoke fixtures remain scope-safe
The service MUST keep smoke fixtures and examples scoped to explicit tenant, project, and namespace values and SHALL NOT rely on global or cross-scope retrieval behavior.

#### Scenario: Smoke data is ingested
- **WHEN** the smoke loop writes test events or evaluates replay
- **THEN** every request carries explicit scope and later verification reads only from that same authorized scope
