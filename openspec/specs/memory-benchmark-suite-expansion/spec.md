# memory-benchmark-suite-expansion Specification

## Purpose
TBD - created by archiving change agent-memory-benchmark-expansion. Update Purpose after archive.
## Requirements
### Requirement: LongMemEval adapter preserves session and memory semantics
The system SHALL normalize a checksum-locked LongMemEval source into ordered sessions, turns, timestamps, question metadata, answer-session evidence, abstention state, and update/conflict relationships without overwriting canonical memory in place.

#### Scenario: Normalize a LongMemEval sample
- **WHEN** a valid LongMemEval sample is normalized with a fixed conversion version
- **THEN** the output contains stable conversation IDs, source turn provenance, benchmark queries, evidence groups, and qrels that are deterministic across repeated runs

#### Scenario: Preserve update and abstention metadata
- **WHEN** a sample represents a knowledge update, conflict, or abstention question
- **THEN** the normalized query records the question date, expected current/obsolete evidence state, and abstention expectation

### Requirement: Dataset registry exposes honest support state
The registry SHALL distinguish `runnable`, `metadata-only`, and `planned` support for LongMemEval, profile/preference datasets, and any future public dataset.

#### Scenario: List expansion datasets
- **WHEN** a user lists benchmark datasets
- **THEN** the output includes family, license status, local prerequisites, support state, and the adapter or reason it is unavailable

#### Scenario: Refuse an unavailable adapter
- **WHEN** a user requests a metadata-only or planned dataset for execution
- **THEN** the command returns a stable prerequisite status and does not silently substitute another dataset

### Requirement: External data remains locally locked and non-redistributed
The system SHALL require a manifest containing dataset version, license, upstream revision, source checksum, conversion version, split identity, qrels checksum, and redistribution status before a non-synthetic run can start.

#### Scenario: Reject checksum drift
- **WHEN** the cached source or normalized artifact does not match its manifest checksum
- **THEN** the run fails with a checksum status before importing or querying data

#### Scenario: Keep restricted data out of the repository
- **WHEN** a user fetches a redistribution-restricted dataset
- **THEN** raw and normalized files are stored only under the configured local cache and the repository receives metadata or instructions only
