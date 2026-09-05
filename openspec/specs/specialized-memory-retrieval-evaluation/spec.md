# specialized-memory-retrieval-evaluation Specification

## Purpose
TBD - created by archiving change agent-memory-benchmark-expansion. Update Purpose after archive.
## Requirements
### Requirement: Profile and preference cases are session-aware
The system SHALL normalize PersonaChat or Multi-Session Chat cases into explicit profile facts, preference updates, sessions, evidence groups, and expected current/obsolete states.

#### Scenario: Retrieve a profile preference
- **WHEN** a later query asks for a preference established in an earlier session
- **THEN** the relevant profile evidence is returned with source session provenance and no unrelated tenant data

#### Scenario: Apply a preference update
- **WHEN** a later session changes a previously recorded preference
- **THEN** current-state retrieval ranks the new fact ahead of the old fact while historical queries can still identify the old version

### Requirement: Temporal and multi-hop fixtures expose evidence groups
The system SHALL support TimeQA-style temporal cases and HotpotQA-style multi-hop cases with graded qrels, evidence-group completeness, and must-not-return metadata.

#### Scenario: Prefer the current temporal fact
- **WHEN** a query asks for the state at a specified date and both old and new facts exist
- **THEN** the result includes the fact valid at that date and does not treat a later or obsolete fact as current

#### Scenario: Require complete multi-hop evidence
- **WHEN** a query requires two or more supporting facts
- **THEN** the report distinguishes partial evidence from a complete evidence-group hit and records the source IDs of each fact

### Requirement: Specialized reports expose targeted metrics
The system SHALL report profile recall/consistency, temporal update precedence, stale-fact suppression, evidence-group hit/completeness, and scope-safety outcomes separately from generic retrieval metrics.

#### Scenario: Report a specialized regression run
- **WHEN** a profile, temporal, or multi-hop fixture run completes
- **THEN** the machine-readable report identifies the subfamily, qrels version, targeted metrics, unmapped evidence count, and safety result
