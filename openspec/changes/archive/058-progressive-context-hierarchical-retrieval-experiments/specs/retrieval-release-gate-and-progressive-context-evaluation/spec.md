## ADDED Requirements

### Requirement: Progressive experiment approval requires protected baseline parity

The release evaluator MUST compare every progressive level and parent-first
strategy with the stable flat-fusion baseline using the same exact scope,
fixture, temporal policy, and evaluation window. Approval requires protected
recall and evidence integrity to be preserved, no isolation expansion, bounded
duplicate and latency impact, deterministic replay, and a tested rollback path.

#### Scenario: Candidate preserves protected gates
- **WHEN** a candidate report matches the baseline identities and satisfies all
  protected gates
- **THEN** the report may be marked eligible for a separately authorized
  rollout decision

#### Scenario: Candidate misses a protected gate
- **WHEN** a candidate loses protected recall, violates isolation, exceeds a
  bound, or lacks deterministic replay or rollback evidence
- **THEN** the evaluator returns a stable non-pass verdict and preserves the
  baseline

### Requirement: Progressive comparison is deterministic across rebuilds

The evaluator SHALL require identical level and parent-first comparison outputs
for identical source watermarks, policies, renderer versions, and fixed clocks,
while preserving older evidence as append-only history.

#### Scenario: Same inputs are replayed
- **WHEN** the same source records and policy identities are replayed
- **THEN** level identities, item ordering, aggregate metrics, and verdict
  categories are stable
