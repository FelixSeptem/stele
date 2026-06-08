# manual-mutation-governance-controls Specification

## Purpose
TBD - created by archiving change manual-memory-mutation-and-reclassification. Update Purpose after archive.
## Requirements
### Requirement: Manual mutations are optimistic-concurrency-aware
The service SHALL protect manual canonical memory mutation from silent operator overwrite by requiring an explicit concurrency guard for mutating existing records.

#### Scenario: Expected version does not match current version
- **WHEN** a privileged caller submits an update, merge, or reclassification request against a stale expected version
- **THEN** the service rejects the mutation with a conflict outcome instead of overwriting the newer canonical state

### Requirement: Manual mutations record durable audit and provenance metadata
The service MUST record stable attribution and operator intent for every manual canonical memory mutation.

#### Scenario: Operator performs a manual governance action
- **WHEN** a privileged caller creates, updates, merges, or reclassifies canonical memory
- **THEN** the resulting history includes actor identity, reason, request attribution, operation type, and applied timestamp in durable audit and provenance records

### Requirement: Manual mutations preserve retrieval projection consistency
The service MUST keep retrieval projections consistent when manual mutation changes canonical content or class.

#### Scenario: Manual mutation materially changes canonical content
- **WHEN** a manual mutation changes the retrievable text or class of canonical memory
- **THEN** the service refreshes lexical or relation projections as needed and prevents stale semantic embeddings from continuing to participate in default retrieval

