# benchmark-family-reporting-and-governance Specification

## Purpose
TBD - created by archiving change agent-memory-benchmark-expansion. Update Purpose after archive.
## Requirements
### Requirement: Reports are family-scoped and auditable
Every benchmark report SHALL contain dataset and split identity, manifest and normalized corpus checksums, qrels version/checksum, Stele revision, PostgreSQL and pgvector identity when applicable, embedding/strategy profile, run scope, metrics, errors, safety outcomes, and retained artifact paths.

#### Scenario: Reproduce a LongMemEval report
- **WHEN** a report consumer reads a completed LongMemEval run
- **THEN** the consumer can identify the exact inputs, conversion, database runtime, strategy, scope, metrics, and quality/safety decision without consulting mutable external services

### Requirement: Offline and prerequisite failures are stable
The runner SHALL distinguish success, quality-gate failure, prerequisite missing, invalid manifest, checksum mismatch, capacity refusal, and internal error in both machine-readable output and process status.

#### Scenario: Run without a cached dataset
- **WHEN** offline mode is enabled and the requested dataset is absent
- **THEN** the runner returns `prerequisite_missing`, performs no network fallback, and identifies the missing artifact

#### Scenario: Detect report input incompatibility
- **WHEN** a report is requested with incompatible manifest, qrels, or conversion versions
- **THEN** the runner refuses to combine them and reports the exact incompatibility

### Requirement: Benchmark artifacts are cleanable and isolated
The system SHALL provide family/run-level cleanup that removes benchmark data and temporary database scope while preserving explicitly retained reports and manifests.

#### Scenario: Clean a completed run
- **WHEN** a user removes a benchmark run
- **THEN** run-scoped corpus, embeddings, and database records are deleted or tombstoned according to policy, retained audit artifacts remain addressable, and production scopes are unchanged

### Requirement: Product-ready gate requires real core evidence
The expansion SHALL not be marked complete until LongMemEval has completed at least one non-synthetic checksum-locked local PostgreSQL + pgvector retrieval run, BFCL memory subsets have an offline contract result, and profile/temporal/multi-hop regressions have retained reports.

#### Scenario: Evaluate completion readiness
- **WHEN** the change completion command checks benchmark artifacts
- **THEN** it passes only when all required family evidence and provenance are present and synthetic smoke alone is rejected

