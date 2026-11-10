## ADDED Requirements

### Requirement: Task-family representation evidence is a future release prerequisite

Before a derived representation can be considered for any future scoped
activation, the release evaluator SHALL require fresh, compatible, exact-scope
task-family representation evidence against `flat-fusion-v1`.  The evidence
MUST identify the fixture, source and qrel checksums, task family, baseline and
candidate representation identities, source watermark/freshness, policy and
renderer versions, equal resource budgets, aggregate critical-fact retention,
source/citation coverage, stale/broken-reference categories, bounded
maintenance observations, deterministic replay, and tested baseline rollback.
Synthetic-only, skipped, stale, incomplete, incompatible, or unsafe evidence
MUST remain non-pass and MUST NOT authorize activation.

#### Scenario: A candidate has complete compatible evidence
- **WHEN** a candidate has fresh owned PostgreSQL + pgvector evidence for every
  declared protected task family, preserves required facts and scope/lifecycle
  safety, satisfies declared resource bounds, and includes a compatible rollback
  result
- **THEN** the release evaluator records only eligibility for a separate,
  explicitly authorized future scoped rollout stage and leaves the current
  baseline selected

#### Scenario: A task-family result regresses or is incomplete
- **WHEN** a candidate loses a protected critical fact, has a broken/stale or
  safety result, exceeds a bound, lacks deterministic replay or rollback, or is
  missing a required family observation
- **THEN** the release evaluator records a stable non-pass category regardless
  of aggregate gains and keeps the candidate offline or shadow-only

### Requirement: Representation release review keeps observational evidence separate

The release evaluator SHALL preserve externally supplied task-utility,
repeated-error, and maintenance observations as separate bounded evidence.
Representation comparison reports MUST label cross-condition differences as
observational and MUST NOT claim a representation caused a task outcome or
derive activation eligibility from task utility alone.

#### Scenario: Candidate utility differs from the baseline
- **WHEN** compatible reports contain externally judged task observations that
  differ between baseline and candidate conditions
- **THEN** the release evidence presents the difference separately from safety,
critical-fact, and resource gates and does not convert it into a causal claim
or automatic activation
