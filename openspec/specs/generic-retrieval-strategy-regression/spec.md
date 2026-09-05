# generic-retrieval-strategy-regression Specification

## Purpose
TBD - created by archiving change agent-memory-benchmark-expansion. Update Purpose after archive.
## Requirements
### Requirement: Generic retrieval datasets are explicitly identified
The system SHALL support selected C-MTEB/MTEB and BEIR retrieval or reranking subsets only with a `generic_retrieval` family identity, independent manifest, and independent report namespace.

#### Scenario: Run a selected Chinese retrieval subset
- **WHEN** a user selects a checksum-locked C-MTEB retrieval subset and a fixed embedding profile
- **THEN** the runner imports it under benchmark scope and reports the subset, profile, corpus checksum, and retrieval metrics

### Requirement: Strategies are comparable on the same corpus
The system SHALL allow lexical, semantic, hybrid, chunk, hybrid-rank, and reranker profiles to run against the same normalized corpus and qrels with deterministic configuration identity.

#### Scenario: Compare retrieval strategies
- **WHEN** two or more strategy profiles run over identical corpus and qrels checksums
- **THEN** the report compares Recall@k, MRR, nDCG, latency, candidate pool size, and failures without changing the input identity

#### Scenario: Reject incomparable results
- **WHEN** strategy runs use different corpus, qrels, embedding dimensions, or normalization settings
- **THEN** the comparison is rejected or marked incomparable rather than aggregated as a single regression

### Requirement: Generic IR does not alter production isolation
The generic retrieval runner SHALL use benchmark-only project, tenant, and namespace scopes and SHALL verify that no production or foreign-run memory is returned.

#### Scenario: Complete a generic IR run
- **WHEN** a generic retrieval run imports and queries its corpus
- **THEN** all returned records belong to the run scope and the report includes isolation and lifecycle safety outcomes
