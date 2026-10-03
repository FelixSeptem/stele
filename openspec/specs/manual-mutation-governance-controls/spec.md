# manual-mutation-governance-controls Specification

## Purpose
Protect privileged canonical-memory mutations with concurrency, authorization, and audit controls.

## Requirements

### Requirement: Manual mutations are optimistic-concurrency-aware

The service SHALL protect manual canonical memory mutation from silent operator overwrite by applying the governed-operation precedence contract before the concurrency guard and durable mutation. Exact scope, lifecycle visibility, privileged principal grant, explicit mutation approval, and idempotency/replay checks MUST pass first; a stale expected version then rejects the mutation with a conflict instead of overwriting newer canonical state.

#### Scenario: Expected version does not match current version

- **WHEN** a privileged caller submits an update, merge, or reclassification request against a stale expected version after all earlier precedence gates pass
- **THEN** the service rejects the mutation with a conflict outcome instead of overwriting the newer canonical state

#### Scenario: Earlier governance gate fails

- **WHEN** a manual mutation has foreign scope, hidden target, missing grant, disabled policy, or conflicting replay metadata
- **THEN** the service rejects it before reading the current version or writing canonical or projection records

### Requirement: Manual mutations record durable audit and provenance metadata
The service MUST record stable attribution and operator intent for every manual canonical memory mutation.

#### Scenario: Operator performs a manual governance action
- **WHEN** a privileged caller creates, updates, merges, or reclassifies canonical memory
- **THEN** the resulting history includes actor identity, reason, request attribution, operation type, and applied timestamp in durable audit and provenance records

### Requirement: Manual mutations preserve retrieval projection consistency
The service MUST keep retrieval projections consistent when manual mutation changes canonical content or class.

#### Scenario: Manual mutation materially changes canonical content
- **WHEN** a manual mutation changes the retrievable text or class of canonical memory
- **THEN** the service refreshes lexical or relation projections as needed, prevents stale semantic embeddings from continuing to participate in default retrieval, and marks the current canonical projection eligible for durable semantic rebuild

#### Scenario: Manual mutation preserves vector audit continuity
- **WHEN** a manual mutation invalidates the previously active semantic projection
- **THEN** the service keeps the prior vector revision auditable, records that the new canonical projection requires rebuild, and does not silently overwrite semantic lineage in place
