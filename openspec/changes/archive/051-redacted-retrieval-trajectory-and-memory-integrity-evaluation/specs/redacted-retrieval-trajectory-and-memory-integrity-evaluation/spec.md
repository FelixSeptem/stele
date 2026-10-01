## Purpose

Provide bounded, redacted evidence about retrieval paths and memory-organization integrity so operators can evaluate quality and safety without exposing sensitive records or changing production behavior.

## ADDED Requirements

### Requirement: Retrieval trajectories are bounded and redacted

The service SHALL persist and expose retrieval trajectories only as bounded,
low-cardinality aggregates for authorized evaluation or administrative callers.
Trajectory data MUST be linked to compatible report, policy, strategy, and
source-watermark identities and MUST exclude raw queries, scope values,
memory/event identifiers, hidden candidates, raw scores, provider payloads,
credentials, prompts, and unbounded plans.

#### Scenario: Authorized trajectory is inspected

- **WHEN** an authorized operator requests a completed trajectory report for an exact scope
- **THEN** the service returns channel availability, candidate-count buckets, expansion buckets, disposition categories, fallback categories, freshness, budget, and latency aggregates without sensitive identifiers

#### Scenario: Public caller requests trajectory internals

- **WHEN** an ordinary search or context caller requests trajectory or diagnostic details
- **THEN** the service returns no trajectory internals and preserves the existing public response contract

### Requirement: Memory organization integrity is independent from action success

The service SHALL report action success separately from information-integrity
success for consolidation, merge, reclassification, reflection, and
context-projection operations. Required evidence loss, alteration, unexpected
duplication, wrong placement, scope leakage, or lifecycle leakage MUST be a
hard integrity failure regardless of retrieval-quality or action-success gains.

#### Scenario: Organization preserves protected evidence

- **WHEN** an organization operation completes and protected evidence remains correctly placed within the exact scope
- **THEN** the report records action success and integrity success as separate deterministic outcomes

#### Scenario: Organization succeeds but integrity fails

- **WHEN** an operation reports success but required evidence is missing, altered, duplicated beyond policy, foreign, or lifecycle-hidden
- **THEN** the integrity verdict is non-pass and release eligibility is denied even if aggregate retrieval quality improves

### Requirement: Replay and retention are deterministic and non-authoritative

The service SHALL support deterministic replay and retention cleanup for
trajectory and integrity artifacts. Replay MUST preserve append-only history,
and cleanup MUST remove only expired derived artifacts while leaving canonical
source records and default retrieval behavior unchanged.

#### Scenario: Identical evidence is replayed

- **WHEN** the same source records, policy versions, and renderer identities are replayed
- **THEN** the service produces stable aggregate identities and verdict categories without changing canonical memory or active retrieval

#### Scenario: Retention window expires

- **WHEN** a configured retention window expires for a trajectory or integrity artifact
- **THEN** cleanup removes only the expired derived artifact, records a bounded cleanup result, and preserves canonical source history

### Requirement: Safety failures override quality gains

The service SHALL classify missing, stale, incompatible, nondeterministic,
scope-unsafe, lifecycle-unsafe, or rollback-incomplete evidence as a stable
non-pass result. These failures MUST NOT authorize activation of progressive
context, parent-first retrieval, reranking, reserved insight types, or any
other experimental strategy.

#### Scenario: Quality improves with an isolation failure

- **WHEN** a report shows improved quality but contains scope or lifecycle leakage
- **THEN** the final verdict is non-pass and no rollout or activation eligibility is emitted

#### Scenario: Evidence is incompatible

- **WHEN** fixture, policy, strategy, provider, renderer, or source-watermark identities are incompatible
- **THEN** the service records an incompatible result and prevents comparison or rollout approval
