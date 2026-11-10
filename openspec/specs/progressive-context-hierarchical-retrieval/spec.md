# progressive-context-hierarchical-retrieval Specification

## Purpose
Defines a replayable, bounded experiment contract for progressive context
levels and parent-first retrieval while keeping all derived output shadow-only
and subordinate to Stele's stable retrieval and isolation guarantees.

## Requirements

### Requirement: Progressive levels have explicit derived identities

The experiment capability SHALL represent L0 coarse retrieval projection, L1
session or context overview, and L2 canonical or chunk evidence as distinct
derived levels. Each level MUST carry a policy and renderer identity, source
watermark, exact-scope identity, lifecycle/freshness verdict, budget, citation
coverage, and deterministic content identity; no level may become canonical
memory.

#### Scenario: Level is eligible for comparison
- **WHEN** an exact-scope replay materializes a progressive level from visible
  PostgreSQL source records
- **THEN** the artifact contains all required identities and bounded metrics and
  can be compared with the corresponding flat-fusion baseline

#### Scenario: Level provenance is incomplete
- **WHEN** a level lacks a source watermark, scope identity, renderer identity,
  or lifecycle/freshness verdict
- **THEN** the level is marked non-comparable and cannot influence retrieval,
  context assembly, or release approval

### Requirement: Parent-first shadow planning is bounded and exact-scoped

The capability SHALL support parent-first planning that selects validated parent
projections or chunks before expanding to children or adjacent evidence. Planning
MUST enforce exact tenant, project, namespace, session, and temporal scope plus
fixed candidate, expansion, token/character, and latency limits.

#### Scenario: Parent-first plan expands visible lineage
- **WHEN** a shadow replay selects a visible parent with valid child lineage
- **THEN** it expands only the bounded in-scope children or adjacent chunks and
  records the expansion disposition for comparison

#### Scenario: Lineage or scope cannot be proven
- **WHEN** a parent, child, or adjacent candidate is foreign, hidden, stale, or
  missing a valid lineage snapshot
- **THEN** the candidate is omitted, the plan records a stable fail-closed
  category, and no broader lookup is attempted

### Requirement: Experimental output is shadow-only and reversible

Progressive and parent-first strategies SHALL run only in offline, diagnostic,
or shadow mode until an independently authorized release policy activates them.
Shadow candidates, ordering, summaries, and scores MUST NOT be returned by
ordinary public search or context responses. Every run MUST identify the stable
baseline and provide a disablement or rollback disposition.

#### Scenario: Shadow run completes
- **WHEN** an authorized offline or shadow run evaluates a strategy
- **THEN** it emits bounded comparison evidence while the default retrieval and
  context response remain unchanged

#### Scenario: Unapproved activation is requested
- **WHEN** a caller attempts to make an experimental strategy default without a
  compatible release handoff
- **THEN** activation is rejected or ignored and the stable baseline remains in
  effect

### Requirement: Replay fails closed on safety or budget violations

The capability SHALL treat stale, hidden, foreign, lifecycle-ineligible,
over-budget, over-candidate, timeout, and nondeterministic results as hard
non-pass outcomes. A quality gain MUST NOT compensate for an isolation,
integrity, freshness, or rollback failure.

#### Scenario: Strategy exceeds a configured bound
- **WHEN** expansion or context packing exceeds its candidate, token/character,
  or latency bound
- **THEN** the strategy falls back to the stable baseline for the run and records
  a bounded non-pass reason

#### Scenario: Safety failure accompanies quality gain
- **WHEN** a strategy improves recall but includes hidden or foreign evidence
- **THEN** the run is non-pass, produces no activation eligibility, and preserves
  the baseline decision

### Requirement: Experiment artifacts are redacted and retention-safe

The capability SHALL emit machine-readable and human-readable comparison
artifacts containing only bounded logical identities, aggregate quality and
resource metrics, freshness, citation, replay, rollback, and fallback
categories. Artifacts MUST exclude queries, prompts, source text, raw scores,
memory or scope identifiers, provider payloads, credentials, and DSNs.

#### Scenario: Authorized operator reads a report
- **WHEN** an authorized evaluation caller reads a completed experiment report
- **THEN** the caller receives redacted aggregates and stable reason categories
  without hidden content or identifying values

#### Scenario: Incomplete run is retained
- **WHEN** a replay fails, times out, or is cancelled before completion
- **THEN** temporary artifacts are removed or marked non-consumable and cannot be
  used for release approval

### Requirement: Task-family representation comparisons retain source retrieval

The capability SHALL compare the stable `flat-fusion-v1` baseline with declared
derived representations for exact-fact, cross-document, profile-preference,
and procedural-lesson task families.  A candidate representation SHALL be one
of exact excerpts plus search, topic projection plus search, or bounded
hierarchical/parent-first evidence plus search.  Every candidate SHALL retain
the authorized source-search channel, exact tenant/project/namespace and
lifecycle predicates, source watermark, renderer/policy identity, fixed
candidate and byte/character/token limits, and a deterministic replay
identity.  Derived output SHALL remain non-canonical and offline, diagnostic,
or shadow-only.

#### Scenario: A task-family candidate is compared
- **WHEN** an authorized evaluation supplies a compatible exact-scope fixture
  and a declared representation candidate for one task family
- **THEN** the report compares that candidate with `flat-fusion-v1` under the
  same visible source records and budgets, records bounded aggregate evidence,
  and leaves ordinary retrieval and context responses unchanged

#### Scenario: A candidate attempts to replace search or widen visibility
- **WHEN** a representation candidate omits its authorized source-search
  companion, has an incompatible source watermark, or reaches foreign, hidden,
  stale, or lifecycle-ineligible evidence
- **THEN** the candidate is a stable non-pass, the run records only a bounded
  category, and the experiment retains the flat baseline without broader lookup

### Requirement: Representation comparison reports preserve task-family safety

Representation comparison artifacts SHALL report only task-family and
representation identities, aggregate critical-fact retention, source/citation
coverage, broken-reference and stale-result categories, bounded representation
size and maintenance-cost observations, task-utility observation coverage,
replay identity, fallback, and rollback disposition.  The artifacts MUST NOT
contain task text, source text, queries, answer text, raw scores, scope values,
memory identifiers, source identifiers, provider payloads, credentials, or
per-stage costs.  Quality or observational task-utility differences MUST NOT
override an isolation, lifecycle, freshness, deterministic-replay,
broken-reference, or resource-bound failure.

#### Scenario: A comparable candidate produces an aggregate report
- **WHEN** all declared source, representation, and task observations are
  compatible and remain within the configured bounds
- **THEN** the report emits deterministic, redacted, family-level aggregates
  and labels condition differences as observational rather than causal or
  activation-authorizing

#### Scenario: A representation loses a critical fact or exceeds a bound
- **WHEN** a candidate loses required exact evidence, produces a broken
  reference, exceeds a representation bound, or cannot reproduce its declared
  replay identity
- **THEN** the run is non-pass, records a bounded failure/fallback category,
  requires rollback to the baseline for that run, and cannot influence ordinary
  retrieval
