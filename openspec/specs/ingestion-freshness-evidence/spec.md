# ingestion-freshness-evidence Specification

## Purpose
Provide bounded, exact-scope and replay-safe evidence for the time and
eligibility transitions between durable event admission and public memory
retrieval without exposing protected records or changing memory behavior.

## Requirements

### Requirement: Freshness evidence has a closed versioned contract

The service SHALL define a versioned ingestion freshness evidence contract with
closed stage identities for `admitted`, `governance_completed`,
`canonical_active`, and `public_retrievable`, plus bounded disposition,
freshness bucket, SLO bucket and observed-time fields.

#### Scenario: Complete lifecycle evidence is available
- **WHEN** an authorized assurance run observes all required stages for one
  exact-scope fixture
- **THEN** it returns the closed versioned stage contract with stable ordering,
  bounded latency/SLO buckets and no raw payload fields

#### Scenario: Unknown stage or field is supplied
- **WHEN** a fixture or report contains an unsupported stage, disposition,
  version, null, duplicate or unknown field
- **THEN** validation rejects the evidence rather than silently interpreting it

### Requirement: Evidence resolves one exact scope before lookup

Freshness evidence SHALL resolve and authorize one tenant/project/namespace
scope before reading or correlating event, governance, canonical or retrieval
state. Foreign, missing, hidden or unavailable records SHALL share a bounded
unavailable outcome without existence disclosure.

#### Scenario: Authorized scope observes a fixture
- **WHEN** an assurance caller presents an active grant for the fixture's exact
  scope
- **THEN** the service can correlate only records in that resolved scope

#### Scenario: Foreign scope requests the same opaque reference
- **WHEN** a caller presents a reference belonging to another scope
- **THEN** the service returns a bounded denied/unavailable outcome without
  revealing whether the reference exists

### Requirement: Stage completion is evidence-based

The service MUST distinguish durable event admission, governance completion,
canonical active promotion and an authorized public retrieval observation.
Acceptance of an HTTP write, candidate creation, or aggregate backlog health
MUST NOT be reported as a later stage.

#### Scenario: Event is accepted but governance is pending
- **WHEN** durable admission exists while worker governance is pending, leased,
  retrying or exhausted
- **THEN** the report marks later stages incomplete with the bounded disposition
  and does not claim public retrievability

#### Scenario: Canonical memory is active but disclosure excludes it
- **WHEN** governance and canonical promotion complete but current lifecycle,
  source classification or disclosure policy prevents a public read
- **THEN** `canonical_active` may be complete while `public_retrievable` is
  ineligible or unavailable, with no hidden identity, path or content disclosed

#### Scenario: Authorized public read succeeds
- **WHEN** the same exact-scope authorized public retrieval observes the current
  active eligible memory
- **THEN** `public_retrievable` is complete and attributable to the observed
  read, not inferred from database state alone

### Requirement: Freshness classification is deterministic and bounded

Given identical stage times, configured freshness windows and observation time,
the service SHALL produce identical nonnegative latency buckets, freshness
categories and SLO outcomes. Missing, negative, stale or divergent timestamps
MUST fail closed.

#### Scenario: Repeated evaluation uses identical inputs
- **WHEN** the same evidence and policy are evaluated twice
- **THEN** stage ordering, latency buckets, freshness categories and SLO result
  are identical

#### Scenario: Stage timestamp is invalid or regresses
- **WHEN** a later stage precedes its prerequisite or a timestamp is missing,
  future-dated beyond the allowed observation envelope, or invalid
- **THEN** the report records an invalid/stale disposition and does not report a
  passing freshness result

### Requirement: Replay and restart retain evidence identity

Freshness evaluation SHALL use a stable exact-scope fixture/report identity and
idempotent replay semantics. API or worker restart, retry and duplicate
observation MUST not create conflicting successful evidence for one logical
fixture.

#### Scenario: Same fixture is replayed
- **WHEN** an authorized run repeats an identical fixture and policy identity
- **THEN** it returns the original bounded result or an equivalent deterministic
  result without duplicate logical evidence

#### Scenario: Retry changes the stage result
- **WHEN** a retry supplies a different stage, timestamp, policy or scope under
  the same identity
- **THEN** the service returns a bounded conflict and preserves the original
  evidence history

### Requirement: Reports and telemetry are redacted and low cardinality

Freshness reports and metrics MUST omit raw queries, content, paths, scope
values, event/memory/version identifiers, credentials, ranking details and
hidden-candidate counts. They SHALL retain only bounded stage, disposition,
latency, freshness, SLO, retry/recovery and lifecycle reason categories.

#### Scenario: Operator inspects an authorized report
- **WHEN** an administrator reads freshness evidence for an authorized scope
- **THEN** the response contains bounded categories and reproducibility metadata
  without protected payload or unbounded identifiers

#### Scenario: Metrics are exported
- **WHEN** a stage or freshness evaluation emits telemetry
- **THEN** labels remain low-cardinality and exclude scope and record identity

### Requirement: Freshness evidence cannot mutate memory or authorize business state

The service MUST keep freshness evaluation read-only with respect to events,
canonical memory, versions, lifecycle, disclosure grants and external task
state. A passing freshness result SHALL NOT imply task success, commit,
publication, lease, tool or role authority.

#### Scenario: Evaluation completes
- **WHEN** freshness evidence is generated or replayed
- **THEN** canonical records and governance history remain unchanged and the
  report is usable only as bounded operational evidence

#### Scenario: Caller treats freshness as completion
- **WHEN** a consumer attempts to use a freshness result as a task or artifact
  completion receipt
- **THEN** the contract provides no such authority and the result remains a
  non-authoritative diagnostic
