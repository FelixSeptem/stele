# offline-benchmark-execution Specification

## Purpose
TBD - created by archiving change local-agent-memory-benchmark-suite. Update Purpose after archive.
## Requirements
### Requirement: Run is offline by default
The benchmark run operation SHALL default `STELE_BENCHMARK_OFFLINE=true` and SHALL read data, normalized records, and embeddings only from the configured local cache.

#### Scenario: Complete local prerequisites run without network
- **WHEN** the cache and embedding profile are present and offline mode is enabled
- **THEN** the run completes without network access and records offline mode in the run manifest

#### Scenario: Missing local data does not trigger fallback
- **WHEN** the requested dataset or split is absent while offline mode is enabled
- **THEN** the run exits with `prerequisite_missing` and does not contact a remote dataset, embedding, or judge service

### Requirement: Three run modes are supported
The system SHALL support `smoke`, `local-full`, and `reproducible-extended` modes with explicit split, query budget, and reproducibility metadata.

#### Scenario: Smoke mode is bounded
- **WHEN** a user runs smoke mode
- **THEN** the runner uses only the declared smoke split and reports its case count and checksum

#### Scenario: Extended mode records all determinism inputs
- **WHEN** a user runs reproducible-extended mode
- **THEN** the run manifest records dataset/qrels checksums, embedding model and revision, dimensions, normalization, chunk/rank configuration, and random seed

### Requirement: Embedding profiles are validated before retrieval
The runner SHALL validate model identity, revision, dimensions, normalization, and vector availability against the manifest before executing semantic or hybrid retrieval.

#### Scenario: Dimension mismatch is rejected
- **WHEN** cached vectors have a dimension different from the declared profile
- **THEN** the run exits with `prerequisite_missing` or `invalid_manifest` and does not execute retrieval

#### Scenario: Lexical-only mode is explicit
- **WHEN** semantic prerequisites are unavailable and the user explicitly selects lexical-only smoke
- **THEN** the run may proceed and labels the report with the lexical-only profile; it SHALL not silently downgrade other modes

### Requirement: Benchmark runs use isolated Stele scope
Every run SHALL use an explicit benchmark project, tenant, namespace, and run id, and retrieval SHALL exclude suppressed, forgotten, and deleted memories by default.

#### Scenario: Cross-run leakage is prevented
- **WHEN** two runs contain identical evidence under different namespaces
- **THEN** a query in one run cannot retrieve evidence from the other run

#### Scenario: Suppressed memory is excluded
- **WHEN** a qrel or corpus contains a suppressed or forgotten memory
- **THEN** default retrieval excludes it and the report records any attempted forbidden return as a safety failure

### Requirement: CLI outcomes are machine-readable and stable
The benchmark CLI SHALL expose `list`, `fetch`, `normalize`, `run`, and `report` operations and SHALL return structured status values and distinct exit classes for success, quality-gate failure, missing prerequisites, invalid input, and internal errors.

#### Scenario: Missing prerequisite is diagnosable
- **WHEN** a run cannot find a required cache or model
- **THEN** stdout or the report contains `status=prerequisite_missing`, the missing paths, and remediation text

### Requirement: Change completion requires a real local retrieval run
The benchmark change SHALL NOT be considered complete solely because unit tests, synthetic smoke fixtures, or report serialization succeed. Completion SHALL require at least one complete benchmark run through Stele's PostgreSQL + pgvector retrieval path using a checksum-locked dataset manifest and a retained report artifact.

#### Scenario: Synthetic smoke does not satisfy completion
- **WHEN** only repository-owned synthetic smoke runs have succeeded
- **THEN** the change remains incomplete until a real PostgreSQL + pgvector benchmark report is recorded

#### Scenario: Real local run satisfies execution evidence
- **WHEN** a locked benchmark corpus completes through the local PostgreSQL + pgvector retrieval path
- **THEN** the runner retains a machine-readable report with input checksums, database runtime identity, scope, strategy, metrics, and quality/safety outcome

