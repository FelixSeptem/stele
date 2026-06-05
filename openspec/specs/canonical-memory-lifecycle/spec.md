# canonical-memory-lifecycle Specification

## Purpose
TBD - created by archiving change governance-pipeline-and-memory-consolidation. Update Purpose after archive.
## Requirements
### Requirement: Consolidation into canonical memory
The service SHALL consolidate candidate memory into canonical memory using class-aware lifecycle rules.

#### Scenario: Profile candidate supersedes prior canonical memory
- **WHEN** a candidate for mutable profile memory is accepted and conflicts with the latest active canonical version
- **THEN** the service writes a new canonical version and marks the prior canonical representation as superseded according to lifecycle rules without erasing history

#### Scenario: Episodic contradiction coexists
- **WHEN** a candidate episodic memory conflicts with an earlier episodic fact but both remain valid evidence
- **THEN** the service preserves both records and resolves visibility through lifecycle metadata rather than destructive overwrite

### Requirement: Candidate suppression
The service MUST support suppression of candidates that lose consolidation or fail governance acceptance while preserving auditability.

#### Scenario: Losing candidate is suppressed
- **WHEN** a candidate loses dedupe or consolidation
- **THEN** the service records the candidate as suppressed and excludes it from default read paths while preserving provenance

### Requirement: Append-only canonical versioning
Canonical memory updates MUST be append-only and MUST preserve material history.

#### Scenario: Canonical memory receives a material update
- **WHEN** consolidation changes the canonical content or state of a memory
- **THEN** the service writes a new memory version instead of mutating the previous version in place

