# retrieval-release-gate-and-progressive-context-evaluation Specification

## Purpose
This capability turns retrieval quality, context depth, and information
integrity into a reproducible, redacted, and reversible release contract for
Stele while keeping experimental strategies out of default production behavior.

## Requirements

### Requirement: Owned real-provider release evidence
The evaluator SHALL run a real-provider retrieval release gate only through an
explicitly owned PostgreSQL and pgvector evaluation DSN, provider profile,
exact scope, compatible fixture/policy identities, and a bounded evidence
freshness window. The report MUST include a stable redacted run identity,
source-watermark/freshness verdicts, compatible fixture, representation,
fusion, ranking, embedding, reranker, analysis, and release-policy versions,
and MUST exclude endpoints, credentials, DSNs, prompts, source text, raw
provider payloads, identifiers, and raw scores. Missing, stale, skipped,
degraded, incompatible, or unavailable prerequisites MUST be a stable
non-pass result and MUST NOT authorize activation. The evaluator MUST NOT
consult or fall back to the service DSN.

#### Scenario: Owned evaluation runs
- **WHEN** an operator supplies a valid explicitly owned evaluation DSN,
  compatible provider profiles, exact scope, and fresh compatible fixtures
- **THEN** the evaluator runs the scoped fixture against PostgreSQL + pgvector,
  emits bounded redacted evidence with a stable run identity, and records
  protected metrics, safety outcomes, freshness, latency, replay, and rollback
  results

#### Scenario: Evaluation DSN is absent
- **WHEN** the release command is invoked without an explicitly owned DSN or
  the source evidence is outside the configured freshness window
- **THEN** it returns a stable skipped/degraded non-pass category, does not
  fall back to any ambient service DSN, and cannot mark a candidate eligible

#### Scenario: Evidence freshness is absent
- **WHEN** the source evidence is outside the configured freshness window
- **THEN** the evaluator returns a stable stale non-pass category and cannot
  mark a candidate eligible or authorize activation

#### Scenario: Provider or fixture identity is incompatible
- **WHEN** a candidate report has incompatible fixture, representation, fusion,
  ranking, provider, analysis, watermark, or release-policy identity
- **THEN** comparison and activation are rejected as incompatible and no
  quality gain can be used as release evidence

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

### Requirement: Parent-first experiments remain reversible

The evaluator SHALL support an offline or shadow-only parent-first strategy that
first ranks validated projections or parent chunks and then performs bounded
child or adjacent expansion. The strategy MUST use exact-scope expansion,
explicit strategy identity, and the same protected-recall, multi-hop,
duplicate-rate, candidate-budget, latency, zero-leakage, and rollback gates as
the flat baseline. Shadow results MUST NOT alter ordinary search or context
responses.

#### Scenario: Parent-first shadow comparison succeeds

- **WHEN** a parent-first shadow run uses compatible baseline evidence and stays
  within candidate and latency budgets
- **THEN** the report includes per-category deltas, expansion aggregates,
  rollback evidence, and a bounded recommendation without changing production
  ranking

#### Scenario: Parent-first expansion crosses scope

- **WHEN** parent or child expansion would return a foreign tenant, project,
  namespace, or lifecycle-hidden record
- **THEN** the run records a hard isolation or lifecycle failure and the strategy
  remains ineligible regardless of aggregate quality

#### Scenario: Parent-first rollback is requested

- **WHEN** an operator disables or rolls back the parent-first strategy
- **THEN** subsequent retrieval uses the previously approved flat strategy and no
  canonical memory or source record is rewritten

### Requirement: Retrieval trajectories are redacted and retained safely

The service SHALL expose retrieval trajectories only through authorized
evaluation or administrative surfaces. Trajectory records MUST be bounded and
MUST contain only channel availability, candidate-count buckets,
parent/child-expansion buckets, aggregate dispositions, fallback categories,
and latency buckets. They MUST NOT contain query text, scope values, memory or
event identifiers, hidden candidates, raw scores, provider errors, credentials,
or unbounded plans. Retention and deletion MUST be deterministic and tested.

#### Scenario: Authorized trajectory is generated

- **WHEN** an authorized evaluation requests a trajectory for a completed run
- **THEN** the service returns bounded redacted aggregates linked to report and
  policy versions without exposing sensitive or high-cardinality fields

#### Scenario: Public caller requests trajectory details

- **WHEN** an ordinary public search or context caller requests trajectory or
  diagnostic internals
- **THEN** the service returns no trajectory internals and preserves the public
  response contract

#### Scenario: Retention cleanup runs

- **WHEN** the configured retention window expires for trajectories, diagnostics,
  reports, or evaluation fixtures
- **THEN** cleanup removes only expired derived artifacts, records a bounded
  deletion outcome, and leaves canonical source records intact

### Requirement: Memory organization integrity is measured separately

The evaluator SHALL produce a memory-organization integrity report that separates
action success from information integrity for consolidation, merge,
reclassification, reflection, and context-projection changes. It MUST report
fact/evidence recall, placement accuracy, duplicate, missing, altered, and
unexpected evidence counts by bounded category, and MUST treat loss of required
information, scope leakage, or lifecycle leakage as a hard failure independent
of action success or aggregate retrieval quality.

#### Scenario: Organization action succeeds with preserved evidence

- **WHEN** a consolidation, merge, reclassification, reflection, or projection
  operation completes and all protected evidence remains correctly placed
- **THEN** the report records action success and information-integrity success as
  separate outcomes with deterministic counts

#### Scenario: Action succeeds but evidence is missing or altered

- **WHEN** an organization operation reports success but required evidence is
  missing, altered, duplicated beyond policy, or placed in the wrong class
- **THEN** the integrity gate fails with bounded categories and prevents release
  approval even if retrieval ranking metrics improve

#### Scenario: Integrity report contains hidden evidence

- **WHEN** hidden or foreign records contribute to an integrity finding
- **THEN** the report exposes only aggregate categories and counts, never content,
  identifiers, or foreign scope values

### Requirement: Release policy is versioned, reviewable, and reversible
The service SHALL publish a versioned release checklist and policy that defines
hard safety gates, protected quality and efficiency thresholds, advisory
metrics, resource bounds, evidence freshness, prerequisite evidence, rebuild
and re-index procedures, rollback steps, retention ownership, and threshold
review cadence. Experimental strategies MUST remain disabled, diagnostics-only,
or shadow-only until a fresh compatible evidence run passes every required gate
and an authorized operator requests an exact-scope activation. No global or
implicit activation is permitted; activation state and its evidence identity
MUST be auditable and append-only.

#### Scenario: Candidate passes every required gate
- **WHEN** a candidate has fresh compatible evidence, zero safety failures,
  preserved protected coverage, acceptable efficiency and latency budgets,
  deterministic replay, and a tested rollback path
- **THEN** the policy records eligibility for the explicitly requested scoped
  rollout stage and identifies the evidence run and policy versions used

#### Scenario: Candidate has an efficiency, freshness, or isolation failure
- **WHEN** a report contains context-budget overflow, stale evidence,
  excessive stale or duplicate tokens, a scope or hidden-memory violation, or
  nondeterministic replay
- **THEN** the policy rejects activation regardless of aggregate quality or
  quality-per-budget gains and records a stable safety category

#### Scenario: Candidate has an efficiency or isolation failure
- **WHEN** a report contains context-budget overflow, excessive stale or
  duplicate tokens, or a scope or hidden-memory violation
- **THEN** the policy rejects the candidate regardless of aggregate quality or
  quality-per-budget gains and records a stable safety category

#### Scenario: Candidate has an isolation or lifecycle failure
- **WHEN** any report contains a scope or hidden-memory violation
- **THEN** the policy rejects the candidate regardless of aggregate quality
  gains and records a stable safety category

#### Scenario: Release policy threshold changes
- **WHEN** an operator changes an efficiency threshold, protected category,
  bound, prerequisite, retention rule, or rollback condition
- **THEN** the policy version increments and future reports cannot be compared
  as compatible with the previous policy without an explicit migration decision

#### Scenario: Scoped activation is disabled or rolled back
- **WHEN** an authorized operator disables or rolls back an active exact-scope
  strategy
- **THEN** subsequent requests use the previously approved baseline, the
  activation remains in append-only history, and canonical memory and source
  records are unchanged

### Requirement: Adaptive planning requires compatible release evidence
A planner policy MUST remain diagnostics-only or shadowed until compatible repository-owned evaluation and explicitly owned PostgreSQL and pgvector evidence demonstrate zero safety failures, preserved protected-family quality, bounded resources, deterministic replay, safe reranker behavior, and tested fallback/rollback.

#### Scenario: Real-stack evidence is unavailable
- **WHEN** the evaluation DSN is absent or the real-stack run is skipped
- **THEN** the stable non-pass result cannot authorize active planning and baseline remains unchanged

### Requirement: Temporal validity is a release gate
The release policy SHALL require compatible current/historical evidence proving
zero stale-fact wins, temporal provenance mismatches, validity ambiguity,
scope/lifecycle leakage, and unbounded temporal resource use before an
active-for-scope temporal retrieval rollout.

#### Scenario: Temporal candidate passes all gates
- **WHEN** owned PostgreSQL + pgvector evidence is compatible and all temporal,
  quality, safety, latency, fallback, and rollback gates pass
- **THEN** the policy may authorize the explicitly requested scoped rollout stage

#### Scenario: Temporal gate is skipped or fails
- **WHEN** temporal evidence is absent, skipped, incompatible, or contains a
  stale-fact or provenance failure
- **THEN** the policy keeps temporal behavior at diagnostics/shadow or approved
  current baseline and records a stable non-pass category

### Requirement: Trajectory and integrity evidence participate in release review

The release evaluator SHALL accept trajectory and memory-integrity artifacts
only when they are redacted, exact-scope, compatible with the release policy,
and fresh for the evaluated source watermark. These artifacts MAY explain or
block a release decision but MUST NOT authorize a rollout without all existing
quality, resource, lifecycle, isolation, replay, and rollback gates.

#### Scenario: Compatible safety evidence is supplied

- **WHEN** an evaluation includes fresh compatible trajectory and integrity reports
- **THEN** the release report includes their bounded categories and evaluates them with the existing required gates

#### Scenario: Integrity evidence is missing or stale

- **WHEN** a candidate lacks required integrity evidence or the evidence watermark is stale
- **THEN** the candidate remains non-pass or shadow-only and the baseline remains active
