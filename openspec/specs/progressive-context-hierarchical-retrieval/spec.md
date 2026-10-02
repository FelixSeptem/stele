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
