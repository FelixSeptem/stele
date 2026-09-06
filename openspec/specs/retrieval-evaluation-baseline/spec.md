# retrieval-evaluation-baseline Specification

## Purpose
TBD - created by archiving change retrieval-evaluation-baseline. Update Purpose after archive.
## Requirements
### Requirement: Versioned internal retrieval fixtures
The service SHALL define a repository-owned, versioned retrieval-evaluation fixture
format for controlled source data, scoped query cases, expected evidence aliases,
acceptable multi-hop evidence groups, and lifecycle/isolation assertions.

#### Scenario: Fixture declares scoped evidence expectations
- **WHEN** an evaluator loads a valid fixture case
- **THEN** the case identifies an explicit tenant, project, namespace, query, expected
  evidence alias or group, and any expected exclusion without relying on generated
  database identifiers

#### Scenario: Fixture uses multiple evidence groups for multi-hop recall
- **WHEN** a query requires independent supporting facts
- **THEN** the fixture can declare the required evidence groups separately and the
  evaluator reports coverage for each group without requiring a generated answer record

#### Scenario: Fixture is malformed or unsafe
- **WHEN** a fixture has an invalid scope, duplicate alias, missing expected evidence,
  or an assertion that cannot be evaluated safely
- **THEN** the evaluator rejects the fixture before seeding or querying any database

### Requirement: Deterministic retrieval replay and report
The service SHALL execute a selected fixture through the real scoped retrieval path and
produce a deterministic, bounded report identifying fixture, representation, ranking,
and compatible embedding-revision metadata.

#### Scenario: Replay evaluates the current retrieval implementation
- **WHEN** an operator or CI runs a valid fixture against an owned PostgreSQL harness
- **THEN** the evaluator seeds only the fixture scope, executes its queries through the
  lexical, semantic, and enabled relation retrieval path, and emits machine-readable and
  human-readable reports

#### Scenario: Replay compares a candidate with baseline
- **WHEN** an evaluator is given a baseline and candidate report with compatible fixture
  and representation versions
- **THEN** it reports per-metric deltas, protected-category regressions, and the ranking
  versions that produced both reports

#### Scenario: Real database prerequisite is absent locally
- **WHEN** the local replay command is invoked without an explicitly owned PostgreSQL
  test DSN
- **THEN** it exits with a stable non-pass skip category and does not connect to a
  default, ambient, or operator database

### Requirement: Retrieval quality metrics
The evaluator SHALL calculate recall, ranking, coverage, duplication, candidate-pool,
and latency metrics from lifecycle-visible scoped results.

#### Scenario: Expected evidence is retrieved
- **WHEN** a query returns one or more required evidence aliases within the configured
  cutoff
- **THEN** the report includes the applicable Recall@k, MRR, nDCG@k, final rank, and
  multi-hop evidence coverage contribution

#### Scenario: Similar evidence crowds a result set
- **WHEN** multiple returned hits map to the same fixture fact cluster or source group
- **THEN** the report records duplicate-rate evidence separately from recall and rank
  quality

#### Scenario: Replay has bounded execution
- **WHEN** a replay run completes or fails
- **THEN** the report includes bounded candidate-pool and latency measurements without
  serializing raw fixture payloads

### Requirement: Safety failures override quality scores
The evaluator MUST treat scope isolation and lifecycle visibility violations as hard
failures independent of aggregate retrieval quality.

#### Scenario: Foreign scope evidence is returned
- **WHEN** a query result contains evidence outside the case tenant, project, or
  namespace
- **THEN** the run fails with a stable isolation category even if its recall metrics are
  otherwise higher

#### Scenario: Hidden lifecycle evidence is returned
- **WHEN** a default retrieval result contains suppressed, forgotten, expired, or
  deleted memory
- **THEN** the run fails with a stable lifecycle-visibility category

#### Scenario: Expected hidden evidence remains excluded
- **WHEN** a fixture intentionally includes matching hidden memory alongside visible
  evidence
- **THEN** the report records successful exclusion without disclosing hidden content or
  identifiers

### Requirement: Redacted and bounded evaluation diagnostics
The evaluator SHALL expose diagnostics only through local, CI, or authorized
administrative paths and SHALL redact sensitive or hidden material.

#### Scenario: Diagnostic records a visible candidate disposition
- **WHEN** a lifecycle-visible expected or returned memory is evaluated
- **THEN** diagnostics can record fixture alias, candidate channel, channel rank, final
  rank, and bounded inclusion or omission reason

#### Scenario: Diagnostic encounters hidden or foreign evidence
- **WHEN** an evaluator detects a hidden or foreign candidate
- **THEN** the report records only a stable aggregate failure or exclusion category and
  does not include content, memory ID, source ID, or foreign scope values

#### Scenario: Report is rendered
- **WHEN** a machine-readable or human-readable report is generated
- **THEN** it excludes credentials, DSNs, raw source event payloads, full database
  errors, and unbounded query plans

### Requirement: Versioned quality release policy
The service SHALL define a versioned policy that distinguishes hard safety gates,
protected quality thresholds, advisory metrics, and bounded latency budgets for retrieval
changes.

#### Scenario: Candidate meets the release policy
- **WHEN** a candidate report has zero safety failures and satisfies protected quality
  and latency thresholds against a compatible baseline
- **THEN** the evaluator marks the candidate eligible for the next scoped rollout stage

#### Scenario: Candidate regresses protected evidence coverage
- **WHEN** a candidate lowers protected Recall@k or required multi-hop evidence coverage
  beyond the approved policy tolerance
- **THEN** the evaluator rejects the candidate regardless of aggregate metric gains

#### Scenario: Policy threshold changes
- **WHEN** an operator changes a release threshold
- **THEN** the policy version changes and subsequent reports identify the policy version
  used for their decision

