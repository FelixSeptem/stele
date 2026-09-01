## ADDED Requirements

### Requirement: QRELs support graded and grouped evidence
The benchmark model SHALL support relevance grades, evidence roles, multiple evidence groups per query, query types, and must-not-return ids, while retaining compatibility with binary relevance metrics.

#### Scenario: Multi-hop query requires a complete group
- **WHEN** a query declares a multi-hop evidence group
- **THEN** the report can distinguish partial evidence hits from a complete group hit

### Requirement: Reports include retrieval and safety metrics
Reports SHALL include Recall@k, MRR, nDCG, evidence-group/multi-hop hit rate, must-not-return violations, p50/p95 latency, safety failure count, query coverage, and unmapped evidence count.

#### Scenario: Strategy comparison is comparable
- **WHEN** the same corpus and qrels run with lexical, semantic, and hybrid-rank strategies
- **THEN** the report presents aligned metrics by strategy, top-k, dataset identity, and run mode

### Requirement: Reports are provenance-complete
Every report SHALL include dataset/version, manifest checksum, normalized corpus checksum, qrels version/checksum, embedding profile, Stele revision, run id, scope, and skipped prerequisites.

#### Scenario: Report can be reproduced
- **WHEN** a user receives a report and its referenced local artifacts
- **THEN** the report identifies the exact inputs and configuration needed to rerun the same evaluation

### Requirement: Quality gates distinguish quality from safety failures
The reporting layer SHALL preserve separate quality-gate and safety-gate outcomes and SHALL allow the existing retrieval release policy to reject unsafe results even when relevance metrics pass.

#### Scenario: Forbidden result fails the gate
- **WHEN** a strategy returns a must-not-return or suppressed memory
- **THEN** the report marks the safety gate failed and the overall outcome is not releasable

### Requirement: Reports are deterministic and exportable
Given identical normalized inputs, qrels, strategy configuration, and seed, the report SHALL produce deterministic case ordering and stable machine-readable JSON suitable for CI artifacts.

#### Scenario: Repeated run has stable ordering
- **WHEN** the same evaluation is run twice with identical inputs
- **THEN** query rows, aggregate keys, and provenance fields appear in the same order and values

### Requirement: Completion evidence report identifies a real runtime
The report retained for change completion SHALL identify PostgreSQL version, pgvector extension version, dataset and qrels checksums, normalized corpus checksum, embedding/strategy profile, run scope, and the quality and safety gate results. A report generated only from a synthetic candidate source SHALL NOT satisfy this requirement.

#### Scenario: Completion report is auditable
- **WHEN** a real local benchmark run finishes
- **THEN** its report contains the runtime and input identity needed to distinguish it from a unit-test or synthetic smoke report
