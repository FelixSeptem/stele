## MODIFIED Requirements

### Requirement: Progressive context evidence is comparable and rebuildable

The evaluator SHALL compare the flat fusion baseline with progressive context
levels and any feedback-calibrated packing strategy. Each level MUST report
source watermark, freshness category, token or character budget,
citation/evidence coverage, duplicate/stale rates, relevant-token ratio,
quality-per-budget, deterministic rebuild identity, and lifecycle/scope safety.
Generated summaries MUST remain derived artifacts and MUST NOT become canonical
memory.

#### Scenario: Progressive levels are evaluated

- **WHEN** an evaluation run requests progressive context levels for an exact
  scope
- **THEN** the report contains separate level identities and bounded metrics for
  recall, evidence coverage, efficiency, cost, freshness, citations, and
  rebuildability

#### Scenario: Projection is stale or hidden

- **WHEN** a level has a missing or stale source watermark, or references hidden
  or foreign evidence
- **THEN** that level fails closed with a bounded freshness, lifecycle, or
  isolation category and cannot influence default retrieval or release approval

#### Scenario: Derived context is rebuilt

- **WHEN** the same PostgreSQL source records are rebuilt with the same policy
  and renderer versions
- **THEN** the level produces deterministic content identity, efficiency
  aggregates, and item order while preserving prior derived versions as
  append-only history

### Requirement: Release policy is versioned, reviewable, and reversible

The service SHALL publish a versioned release checklist and policy that defines
hard safety gates, protected quality and efficiency thresholds, advisory
metrics, resource bounds, prerequisite evidence, rebuild/re-index procedures,
rollback steps, retention ownership, and threshold review cadence. Experimental
strategies MUST remain disabled for default retrieval until all required gates
pass; efficiency gains MUST NOT offset protected recall, isolation, lifecycle,
citation, budget, or deterministic-replay failures.

#### Scenario: Candidate passes every required gate

- **WHEN** a candidate has compatible evidence, zero safety failures, preserved
  protected coverage, acceptable efficiency and latency budgets, fresh
  rebuildable context, and a tested rollback path
- **THEN** the policy records eligibility for the explicitly requested scoped
  rollout stage and identifies the evidence versions used

#### Scenario: Candidate has an efficiency or isolation failure

- **WHEN** a report contains context-budget overflow, excessive stale or
  duplicate tokens, a scope or hidden-memory violation, or nondeterministic
  replay
- **THEN** the policy rejects the candidate regardless of aggregate quality or
  quality-per-budget gains and records a stable safety category

#### Scenario: Candidate has an isolation or lifecycle failure

- **WHEN** any report contains a scope or hidden-memory violation
- **THEN** the policy rejects the candidate regardless of aggregate quality gains
  and records a stable safety category

#### Scenario: Release policy threshold changes

- **WHEN** an operator changes an efficiency threshold, protected category,
  bound, prerequisite, retention rule, or rollback condition
- **THEN** the policy version increments and future reports cannot be compared
  as compatible with the previous version without an explicit migration
  decision
