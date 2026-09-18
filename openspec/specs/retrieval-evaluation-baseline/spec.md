# retrieval-evaluation-baseline Specification

## Purpose
TBD - created by archiving change retrieval-evaluation-baseline. Update Purpose after archive.

## Requirements

### Requirement: Versioned internal retrieval fixtures
The service SHALL define a repository-owned, versioned retrieval-evaluation fixture
format for controlled source data, scoped query cases, expected evidence aliases,
acceptable multi-hop evidence groups, lifecycle/isolation assertions, and expected
query-analysis dispositions. Fixtures SHALL cover protected simple factual recall as
well as temporal, entity-centric, mixed-language, ambiguous, multi-hop, malformed,
and adversarial queries without depending on generated database identifiers.

#### Scenario: Fixture declares scoped evidence expectations
- **WHEN** an evaluator loads a valid fixture case
- **THEN** the case identifies an explicit tenant, project, namespace, query, expected evidence alias or group, expected analysis category where applicable, and any expected exclusion without relying on generated database identifiers

#### Scenario: Fixture uses multiple evidence groups for multi-hop recall
- **WHEN** a query requires independent supporting facts
- **THEN** the fixture can declare the required evidence groups separately and the evaluator reports coverage for each group without requiring a generated answer record

#### Scenario: Fixture protects original-query behavior
- **WHEN** a simple factual, malformed, adversarial, or analysis-unavailable case is evaluated
- **THEN** the fixture can require original-query retention, expected fallback disposition, protected evidence coverage, and bounded analysis output

#### Scenario: Fixture exercises bounded query categories
- **WHEN** the fixture suite is validated
- **THEN** it contains versioned cases for temporal, entity-centric, mixed-language, ambiguous, multi-hop, malformed, and adversarial queries with explicit safe expectations

#### Scenario: Fixture is malformed or unsafe
- **WHEN** a fixture has an invalid scope, duplicate alias, missing expected evidence, invalid analysis expectation, or an assertion that cannot be evaluated safely
- **THEN** the evaluator rejects the fixture before seeding or querying any database

### Requirement: Deterministic retrieval replay and report
The service SHALL execute a selected fixture through the real scoped retrieval
path and produce a deterministic, bounded report identifying fixture,
representation, selected fusion strategy name and version, ranking, compatible
embedding-revision metadata, query-analysis policy and limit version, rollout
disposition, and bounded fallback information. Query-analysis evaluation SHALL
compare the immutable original-query baseline with the analyzed candidate under
compatible fixture, representation, fusion, ranking, and analysis versions.

#### Scenario: Replay evaluates the current retrieval implementation
- **WHEN** an operator or CI runs a valid fixture against an owned PostgreSQL harness
- **THEN** the evaluator seeds only the fixture scope, executes its original and eligible derived signals through the lexical, semantic, enabled relation, and authorized chunk retrieval paths, and emits machine-readable and human-readable reports with the effective fusion, ranking, and query-analysis policy identities

#### Scenario: Replay compares a candidate with baseline
- **WHEN** an evaluator is given a baseline and candidate report with compatible fixture and representation versions
- **THEN** it reports per-metric deltas, protected-category regressions, the strategy versions, and the ranking versions that produced both reports

#### Scenario: Replay compares a query-analysis candidate with original-query baseline
- **WHEN** an evaluator is given compatible original-query baseline and query-analysis candidate reports
- **THEN** it reports per-metric deltas, protected-category regressions, analysis fallback categories, bounded signal and candidate counts, and the strategy, ranking, and analysis versions that produced both reports

#### Scenario: Equivalent replay is repeated
- **WHEN** the same fixture, representation, policies, limits, and owned database state are replayed
- **THEN** the ordered analysis dispositions, result identities, metric values, and bounded diagnostic categories are reproducible

#### Scenario: Real database prerequisite is absent locally
- **WHEN** the local replay command is invoked without an explicitly owned PostgreSQL test DSN
- **THEN** it exits with `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` as a stable non-pass category, does not connect to a default, ambient, or operator database, and cannot produce activation evidence

### Requirement: Retrieval quality metrics
The evaluator SHALL calculate recall, ranking, coverage, duplication,
candidate-pool, diversity-selection, query-analysis, and latency metrics from
lifecycle-visible scoped results. For an evaluated diversity policy version, it
SHALL compare the identity/lineage-deduplicated baseline with the policy result
and report protected evidence coverage, duplicate rate, diversity disposition
aggregates, candidate-pool size, and latency without serializing raw candidate
payloads. For an evaluated query-analysis policy version, it SHALL compare the
immutable original-query baseline with the analyzed candidate and report
protected simple-fact recall, temporal and multi-hop evidence coverage,
analysis and fallback category counts, signal and subquery counts,
candidate-pool bounds, duplicate rate, and latency without serializing raw
queries, subqueries, plans, or candidate payloads.

#### Scenario: Expected evidence is retrieved
- **WHEN** a query returns one or more required evidence aliases within the configured cutoff
- **THEN** the report includes the applicable Recall@k, MRR, nDCG@k, final rank, and multi-hop evidence coverage contribution

#### Scenario: Similar evidence crowds a result set
- **WHEN** multiple returned hits map to the same fixture fact cluster or source group
- **THEN** the report records duplicate-rate evidence separately from recall and rank quality and identifies whether the evaluated policy omitted duplicate or diversity-competing evidence through bounded aggregate dispositions

#### Scenario: Diversity policy is compared with its baseline
- **WHEN** an evaluator compares a named diversity-policy version with a compatible identity/lineage-deduplicated baseline
- **THEN** the report includes per-metric deltas for protected recall, evidence coverage, duplicate rate, candidate-pool size, and latency, and rejects the candidate if a hard safety failure occurs

#### Scenario: Query analysis is compared with original-query baseline
- **WHEN** an evaluator compares a named query-analysis policy version with its compatible immutable original-query baseline
- **THEN** the report includes per-category and aggregate deltas for protected simple-fact recall, temporal and multi-hop evidence coverage, duplicate rate, fallback categories, signal and subquery counts, candidate-pool size, and latency

#### Scenario: Query analysis encounters unsafe evidence
- **WHEN** any original or derived signal returns foreign-scope or lifecycle-hidden evidence
- **THEN** the evaluator records the applicable hard safety failure without allowing quality improvements to offset it or disclosing the unsafe evidence

#### Scenario: Replay has bounded execution
- **WHEN** a replay run completes or fails
- **THEN** the report includes bounded analysis work, signal, subquery, candidate-pool, and latency measurements without serializing raw fixture payloads, query plans, or derived query text

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
administrative paths and SHALL redact sensitive or hidden material while retaining
bounded strategy identity, candidate channel, channel rank, fusion disposition,
and final-rank evidence for visible evaluated results.

#### Scenario: Diagnostic records a visible candidate disposition
- **WHEN** a lifecycle-visible expected or returned memory is evaluated
- **THEN** diagnostics can record fixture alias, selected strategy name/version,
  candidate channel, channel rank, bounded fusion disposition, final rank, and
  bounded inclusion or omission reason

#### Scenario: Diagnostic encounters hidden or foreign evidence
- **WHEN** an evaluator detects a hidden or foreign candidate
- **THEN** the report records only a stable aggregate failure or exclusion
  category and does not include content, memory ID, source ID, fusion score, or
  foreign scope values

#### Scenario: Report is rendered
- **WHEN** a machine-readable or human-readable report is generated
- **THEN** it excludes credentials, DSNs, raw source event payloads, full
  database errors, unbounded query plans, and raw per-provider score values

### Requirement: Versioned quality release policy
The service SHALL define a versioned policy that distinguishes hard safety gates,
protected quality thresholds, advisory metrics, bounded analysis and retrieval
budgets, and prerequisite-stage evidence for retrieval changes. A query-analysis
candidate MUST NOT be eligible for active decomposition unless a compatible,
explicitly owned PostgreSQL and pgvector report first proves that the approved
identity/lineage duplicate-rate, protected evidence coverage, candidate-budget,
and latency gates are green, and then proves the analyzed candidate eligible
against the original-query baseline. Synthetic-only or skipped real-stack results
MUST NOT satisfy either gate.

#### Scenario: Candidate meets the release policy
- **WHEN** a candidate report has zero safety failures and satisfies protected quality, analysis, candidate-pool, and latency thresholds against a compatible baseline
- **THEN** the evaluator marks the candidate eligible for the next scoped rollout stage only when all declared prerequisite-stage gates are also satisfied

#### Scenario: Candidate regresses protected evidence coverage
- **WHEN** a candidate lowers protected simple-fact Recall@k or required temporal or multi-hop evidence coverage beyond the approved policy tolerance
- **THEN** the evaluator rejects the candidate regardless of aggregate metric gains

#### Scenario: Diversity prerequisite is not green
- **WHEN** the preceding real-stack duplicate-rate, protected evidence coverage, candidate-budget, or latency gate is absent, skipped, incompatible, or failing
- **THEN** the evaluator marks active query decomposition ineligible regardless of query-analysis metrics

#### Scenario: Real-stack evaluation is skipped
- **WHEN** no explicitly owned PostgreSQL and pgvector evaluation DSN is available
- **THEN** the stable non-pass skip result does not authorize diagnostics-only, shadow, or synthetic evidence to advance query decomposition into active rollout

#### Scenario: Policy threshold changes
- **WHEN** an operator changes a release threshold, bound, protected category, or prerequisite evidence rule
- **THEN** the policy version changes and subsequent reports identify the policy version used for their decision

### Requirement: Evaluation measures adaptive plans by query family and pass
The evaluator SHALL support versioned planner fixtures and compare baseline with the planned candidate using protected quality, safety, candidate, latency, fallback, reranker-use, and first- versus second-pass evidence metrics.

#### Scenario: Query-family regression is hidden by aggregate gain
- **WHEN** aggregate quality improves but a protected query family regresses beyond policy
- **THEN** the evaluator records a protected-family failure and does not mark the planner candidate eligible
