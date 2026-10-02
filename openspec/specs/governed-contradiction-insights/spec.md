# governed-contradiction-insights Specification

## Purpose
Define a bounded, evidence-backed contradiction insight that compares
scope-authorized fact versions without treating model output as canonical truth.

## Requirements

### Requirement: Contradiction candidates require two eligible evidence sides

The service SHALL create a contradiction candidate only when both sides resolve
to lifecycle-visible, exact-scope canonical versions with source-version
identity, provenance, and an evidence digest. A single claim, unbound citation,
or semantic similarity without a fact identity SHALL be insufficient.

#### Scenario: Two mutually exclusive facts are eligible

- **WHEN** two visible fact versions share a contradiction key, have authorized provenance, and satisfy the configured evidence bound
- **THEN** the service creates a bounded contradiction candidate containing both citations, their source versions, the contradiction key, and the candidate replay identity

#### Scenario: One side is hidden or foreign

- **WHEN** either fact version is suppressed, forgotten, deleted, redacted, stale outside the request watermark, or outside the exact scope
- **THEN** the service rejects or quarantines the candidate without disclosing the excluded version

### Requirement: Temporal coexistence is distinct from contradiction

The service SHALL evaluate valid-time intervals before classifying a fact pair.
Mutually exclusive facts with overlapping valid intervals MAY be classified as
contradictory; facts with disjoint intervals SHALL be classified as temporal
coexistence, and facts with unknown or invalid intervals SHALL remain
unresolved unless an explicit policy permits review.

#### Scenario: Facts are valid in disjoint intervals

- **WHEN** two otherwise incompatible facts have non-overlapping valid-time intervals
- **THEN** the service records temporal coexistence and does not create an active contradiction candidate

#### Scenario: Facts overlap in valid time

- **WHEN** two mutually exclusive facts have overlapping valid-time intervals and complete provenance
- **THEN** the service may create a contradiction candidate with the overlap interval and both temporal identities

#### Scenario: A validity interval is missing

- **WHEN** a fact pair cannot be assigned a valid-time relationship
- **THEN** the service records an unresolved-temporal disposition and does not make the pair eligible for activation

### Requirement: Contradiction derivation is bounded and deterministic

The service SHALL bound the authorized scope, evidence window, pair or group
count, provider budget, and output size for contradiction derivation. The
normalized contradiction key, sorted evidence identities, source watermarks,
provider/schema identity, and policy versions SHALL determine a stable replay
identity.

#### Scenario: Derivation exceeds its pair budget

- **WHEN** eligible facts would produce more contradiction pairs than the run limit
- **THEN** the service stops at the bound and records a stable budget-exhausted disposition without scanning unbounded evidence

#### Scenario: Identical inputs are replayed

- **WHEN** the same fact versions, temporal intervals, policy, provider contract, and source watermark are replayed
- **THEN** the service produces the same contradiction identity and bounded disposition

### Requirement: Contradiction results are non-authoritative by default

Offline and shadow contradiction runs MUST NOT mutate canonical memory, active
insight state, default retrieval, or ordinary context. A candidate MAY be
handed to activation only after it satisfies an independently versioned,
exact-scope contradiction policy.

#### Scenario: Shadow detects an overlapping conflict

- **WHEN** a shadow run finds a candidate that would pass the configured policy
- **THEN** it records a would-activate result and leaves active insight state and ordinary behavior unchanged

#### Scenario: No policy is enabled

- **WHEN** contradiction detection produces an otherwise valid candidate without an enabled contradiction policy
- **THEN** the service retains a reviewable candidate or quarantine record and creates no active contradiction insight

### Requirement: Contradiction evidence remains append-only and reviewable

An admitted contradiction SHALL be stored as a derived, versioned record with
both source citations, overlap or coexistence analysis, uncertainty,
derivation provenance, policy decision, review state, and lifecycle history.
Updating confidence, evidence, or review disposition SHALL append an auditable
version or transition.

#### Scenario: Operator reviews a contradiction

- **WHEN** an authorized operator accepts, rejects, or marks a contradiction as unresolved
- **THEN** the service records the review decision and attribution without changing either canonical fact version

#### Scenario: A source fact is corrected

- **WHEN** a source version or validity interval changes through an append-only correction
- **THEN** the service marks affected contradiction evidence stale or schedules a bounded rebuild while preserving the prior contradiction history
