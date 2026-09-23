## Purpose

Define a deterministic, scope-safe context-efficiency evidence and feedback-calibration capability that improves useful evidence per context budget without changing lifecycle visibility, provenance, or the approved baseline by default.

## ADDED Requirements

### Requirement: Context efficiency evidence is versioned and bounded

The service SHALL produce a versioned context-efficiency evidence record for an
authorized evaluation or governed rollout that identifies the fixture,
representation, packing, feedback, and policy versions and reports only bounded
aggregates for relevant-token ratio, evidence density, duplicate-token rate,
stale-token rate, quality-per-budget, candidate/context cost, and latency.

#### Scenario: Authorized evaluation records efficiency evidence

- **WHEN** an authorized evaluation runs a fixed fixture under one exact scope,
  clock, context budget, and policy identity
- **THEN** the report contains bounded efficiency aggregates and compatible
  logical identities without query text, content, raw scores, scopes, or hidden
  identifiers

#### Scenario: Efficiency value exceeds its envelope

- **WHEN** a metric, count, budget, or latency value is negative, unbounded, or
  outside its configured maximum
- **THEN** the evaluator rejects the evidence or records a bounded resource
  failure and cannot mark the calibration eligible

### Requirement: Efficiency evidence is replayable and comparable

The service MUST compare the approved baseline and an optional calibrated
strategy using the same exact scope, lifecycle and temporal constraints, graph
policy, candidate envelope, context budget, clock, fixture, and renderer
identity. Equivalent inputs MUST produce the same metric values, item order,
omission categories, and evidence identity.

#### Scenario: Fixed context replay is repeated

- **WHEN** the same source snapshot, request, feedback snapshot, clock, and
  policy versions are evaluated repeatedly
- **THEN** baseline and calibrated reports have identical bounded ordering,
  efficiency metrics, omission categories, and replay identity

#### Scenario: Baseline and candidate use incompatible identities

- **WHEN** the candidate changes the fixture, representation, packing,
  embedding, graph, feedback, or policy identity required for comparison
- **THEN** comparison is rejected as incompatible and no quality delta is
  considered release evidence

### Requirement: Feedback calibration is weak, scoped, and decayed

Feedback calibration SHALL consume only active, exact-scope, non-superseded
aggregate feedback or task-quality evidence that meets configured minimum
counts and confidence thresholds. Its influence MUST be capped, decayed over
time, versioned, asynchronously rebuildable, and unable to override lifecycle,
scope, provenance, protected-recall, or context-budget rules.

#### Scenario: Insufficient feedback is available

- **WHEN** a candidate has fewer than the configured minimum active signals or
  the signals are below the confidence threshold
- **THEN** the calibration records an insufficient-evidence category and keeps
  baseline ordering for that candidate

#### Scenario: Feedback is stale or superseded

- **WHEN** feedback has expired beyond the configured decay window or has been
  superseded by a newer record
- **THEN** the calibration excludes it from the active signal summary while
  retaining the durable history for authorized inspection

#### Scenario: Feedback would promote hidden evidence

- **WHEN** a positive signal references suppressed, forgotten, deleted,
  expired, foreign, or otherwise ineligible evidence
- **THEN** the evidence remains excluded and the signal cannot affect a visible
  candidate's rank or context inclusion

### Requirement: Calibration rollout is explicit and reversible

The service SHALL support diagnostics-only, shadow, and active-for-exact-scope
calibration stages with versioned policy identity, dry-run evidence, bounded
resource limits, disablement, and rollback. Ordinary search and context
responses MUST remain baseline-equivalent unless an explicit matching request or
active governed policy enables calibration.

#### Scenario: Shadow calibration runs

- **WHEN** a compatible exact-scope calibration policy is in shadow
- **THEN** the service computes bounded candidate and efficiency deltas but
  returns the approved baseline response

#### Scenario: Calibration policy is rolled back

- **WHEN** an operator disables or rolls back an active calibration policy
- **THEN** subsequent requests use the approved baseline without rewriting
  canonical memory, feedback history, provenance, or derived source records

### Requirement: Efficiency hard failures override quality gains

The release gate MUST mark a calibration non-pass when it detects protected
recall loss, scope or lifecycle leakage, citation mismatch, stale or duplicate
budget waste beyond policy, context-budget overflow, nondeterministic replay,
unbounded resource use, or failed rollback, even when aggregate quality or
quality-per-budget improves.

#### Scenario: Quality improves but protected recall regresses

- **WHEN** the candidate reports a positive quality delta but loses protected
  evidence or violates a hard safety/resource gate
- **THEN** the release decision remains non-pass and records only bounded failure
  categories

#### Scenario: Calibration exceeds the context budget

- **WHEN** calibrated packing would exceed the caller or service context budget
- **THEN** the candidate omits bounded evidence or falls back to baseline and
  records a budget category without increasing the budget or widening recall

### Requirement: Ordinary APIs do not expose calibration internals

Ordinary search and context responses MUST preserve existing public shapes and
MUST NOT expose feedback history, calibration weights, raw efficiency metrics,
candidate pools, hidden evidence, policy internals, or trajectory details.

#### Scenario: Ordinary caller receives calibrated context

- **WHEN** an active exact-scope calibration policy affects a normal context
  request
- **THEN** the response contains only the existing lifecycle-visible sections,
  bounded citations, and approved content fields

#### Scenario: Authorized diagnostics inspect calibration

- **WHEN** an authorized evaluation or administrative path requests calibration
  diagnostics
- **THEN** it receives only allowlisted aggregates, version identities, and
  bounded reason categories without raw feedback, content, scope values, or IDs
