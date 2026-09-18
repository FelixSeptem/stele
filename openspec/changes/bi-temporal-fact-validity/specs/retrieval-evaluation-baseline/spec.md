## ADDED Requirements

### Requirement: Temporal fixtures compare current and historical evidence
Repository-owned fixtures SHALL cover current-valid, expired, as-of, interval,
retroactive-correction, legacy-compatibility, stale-similarity, and temporal
scope-isolation cases using aliases rather than generated identifiers.

#### Scenario: Current and historical cases are replayed
- **WHEN** an evaluator runs compatible temporal fixtures
- **THEN** it reports deterministic selected-version identities, validity
  dispositions, stale-fact exclusions, provenance coverage, and protected
  isolation/lifecycle outcomes

### Requirement: Temporal safety failures override quality
The evaluator MUST treat a stale-fact win, validity ambiguity, temporal
provenance mismatch, foreign-scope temporal evidence, or hidden-version leak as
hard failures independent of ranking or recall gains.

#### Scenario: Stale fact ranks first
- **WHEN** a current fixture returns an expired version above a current-valid
  successor
- **THEN** the evaluation fails with a stable temporal lifecycle category

#### Scenario: Historical replay has no owned database
- **WHEN** temporal replay lacks an explicitly owned PostgreSQL + pgvector DSN
- **THEN** it records the stable non-pass prerequisite category and cannot
  authorize temporal rollout
