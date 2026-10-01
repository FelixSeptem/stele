## MODIFIED Requirements

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
