## ADDED Requirements

### Requirement: Repository fixtures declare representation-task compatibility

Repository-owned retrieval fixtures SHALL declare a versioned task family,
compatible baseline and candidate representation identities, exact-scope source
records and expected visible evidence aliases, required critical facts,
representation-specific source/citation expectations, bounded representation
and maintenance-cost observations, and explicit exclusions.  Fixtures SHALL
cover exact terms/numbers, cross-document evidence, stable profile preferences,
and evidence-backed procedural lessons.  Fixture identities, source checksums,
qrel checksums, and comparable budgets SHALL be locked before replay.

#### Scenario: A valid task-family fixture loads
- **WHEN** a fixture declares a supported task family, compatible
representation identities, exact scope, locked checksums, visible evidence, and
equal budgets
- **THEN** the evaluator accepts it for deterministic baseline/candidate replay
without requiring generated database identifiers or an agent/model answer

#### Scenario: A fixture is unsafe or incomparable
- **WHEN** a fixture has an unknown family or representation, unequal budgets,
missing required critical evidence, conflicting source/qrel checksum, foreign
or hidden expectation, or unsupported maintenance observation
- **THEN** the evaluator rejects it before database seeding or retrieval and
emits no representation-quality pass

### Requirement: Representation replays measure retention and cost separately

The evaluator SHALL replay compatible representation candidates against the
same baseline fixture and report deterministic family-level critical-fact
retention, evidence/source/citation coverage, broken-reference and stale/noisy
harm categories, candidate/representation budget use, latency, maintenance-cost
coverage, and externally supplied task-utility observation coverage.  Missing
task verdicts or cost observations SHALL remain missing or inconclusive.  The
evaluator MUST NOT execute agents, invoke models, generate answers, infer costs
or task outcomes, mutate canonical memory, or change default retrieval.

#### Scenario: A candidate preserves required facts within equal budgets
- **WHEN** a compatible candidate replay returns all required visible evidence
  while staying within the fixture's declared representation and retrieval
  budgets
- **THEN** the bounded report includes the family aggregates and comparison
  deltas without exposing raw fixture content or changing the selected strategy

#### Scenario: An externally supplied observation is incomplete
- **WHEN** a fixture lacks a task verdict or maintenance observation for an
  evaluated representation stage
- **THEN** the report records incomplete coverage for that observation and does
  not synthesize a success, error, causal benefit, token cost, or tool cost
