# agent-memory-benchmark-adapter Specification

## Purpose
TBD - created by archiving change local-agent-memory-benchmark-suite. Update Purpose after archive.
## Requirements
### Requirement: Adapters produce a versioned normalized corpus
Each dataset adapter SHALL convert source data into versioned `ConversationRecord`, `MemoryEventRecord`, `BenchmarkQuery`, and `QREL` records without requiring PostgreSQL access.

#### Scenario: LoCoMo normalization is deterministic
- **WHEN** the LoCoMo adapter receives the same locked source and conversion version twice
- **THEN** it emits byte-equivalent normalized records and a stable normalization checksum

### Requirement: Conversation and event provenance is preserved
Normalized records SHALL retain conversation/session identifiers, ordered turns, timestamps when available, speaker, source file/offset, and the mapping from each memory event to its originating turn.

#### Scenario: Evidence is traceable to a source turn
- **WHEN** a qrel references an evidence record
- **THEN** the report can resolve that record to its session and original source offset

### Requirement: Dataset layers have explicit support states
The registry SHALL identify Layer 0 internal fixture, Layer 1 LoCoMo, Layer 2 LongMemEval, Layer 3 Multi-Session Chat/PersonaChat, and Layer 4 HotpotQA/TimeQA/BEIR, with each dataset marked as runnable, metadata-only, or planned.

#### Scenario: Unsupported layer is listed but not runnable
- **WHEN** a user lists datasets before an adapter is implemented
- **THEN** the dataset appears with its support state and run returns `prerequisite_missing` instead of attempting an ad hoc import

### Requirement: Adapters map lifecycle and scope explicitly
Adapters SHALL assign memory class, expected lifecycle, project, tenant, namespace, and session scope for every event or query; they SHALL NOT overwrite canonical memory records during normalization.

#### Scenario: Session isolation survives normalization
- **WHEN** two source sessions contain identical text under different scopes
- **THEN** their normalized ids and scope fields remain distinct and can be imported into separate benchmark namespaces

### Requirement: Normalization validates evidence and negatives
Adapters SHALL report unmapped evidence, duplicate ids, malformed turns, and must-not-return references as structured validation errors before a benchmark run is allowed.

#### Scenario: Missing supporting evidence blocks a full run
- **WHEN** a query references an evidence id not emitted by the adapter
- **THEN** validation fails with the query id and missing evidence id

