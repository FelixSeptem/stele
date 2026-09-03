# runtime-memory-provider-contract Specification

## Purpose
TBD - created by archiving change agent-memory-benchmark-expansion. Update Purpose after archive.
## Requirements
### Requirement: BFCL memory operations replay offline
The system SHALL support offline replay of the BFCL-v4 `memory_kv`, `memory_rec_sum`, and `memory_vector` operation subsets or equivalent checksum-locked contract fixtures without requiring a remote model, search service, or judge.

#### Scenario: Replay a valid memory operation
- **WHEN** a contract fixture contains a valid memory read/write/search/update operation
- **THEN** the runner validates the operation name, arguments, expected scope, and result shape and records an operation-level outcome

#### Scenario: Handle malformed or irrelevant operations
- **WHEN** an operation has malformed arguments or is irrelevant to the supplied context
- **THEN** the runner records a contract failure or correct refusal without converting the case into a successful memory result

### Requirement: Provider contract preserves scope and lifecycle controls
The contract runner SHALL pass project, tenant, namespace, session, and lifecycle expectations through every memory operation and SHALL detect cross-scope or hidden-memory access.

#### Scenario: Reject a cross-tenant memory call
- **WHEN** a replayed operation requests a memory outside the run tenant
- **THEN** the operation fails with a scope-safety outcome and no foreign memory is returned

#### Scenario: Exclude forgotten memory by default
- **WHEN** a replayed search targets a forgotten, suppressed, or deleted record without an explicit debug allowance
- **THEN** the provider returns no hidden record and the report records zero must-not-return violations

### Requirement: Contract metrics remain separate from retrieval ranking
The system SHALL report operation accuracy, malformed-call rate, refusal correctness, scope-safety failures, and lifecycle-safety failures under a provider-contract family identity and SHALL NOT merge them into Recall@k, MRR, or nDCG.

#### Scenario: Produce a contract report
- **WHEN** all selected BFCL memory cases finish
- **THEN** the report contains family identity, subset counts, operation metrics, safety outcomes, and artifact provenance independent of retrieval reports

