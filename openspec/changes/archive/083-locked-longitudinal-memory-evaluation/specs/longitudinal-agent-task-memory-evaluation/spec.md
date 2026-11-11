## ADDED Requirements

### Requirement: V3 experiments lock conditions and stage-visible sources

Opt-in v3 task series SHALL declare exactly the five memory conditions in
fixed order, one no-memory baseline, equal ordered tasks and held-out flags,
model/runtime/criteria digests, seeds, independent item/UTF-8-byte/whitespace-
token/citation budgets and a service-latency bound. Every source snapshot SHALL
bind its literal content digest, immutable version, disclosure metadata and
first available/first withdrawn stage. Replay MUST physically omit future or
withdrawn evidence from the stage's owned database scope. No-memory SHALL
contain no sources. Later held-out tasks MUST NOT feed evidence into earlier
stages. Serialized reports SHALL contain hashes and aggregates only.

#### Scenario: Correction is published between tasks
- **WHEN** a later correction snapshot replaces an earlier rule
- **THEN** earlier replay contains only the earlier snapshot and later replay
  contains only snapshots available and not withdrawn at that later stage

#### Scenario: Manifest or replay contains future evidence
- **WHEN** availability, source lineage, scope, task ordering, condition coverage
  or budget identities are missing or incompatible
- **THEN** the experiment fails closed without claiming benefit or activation

### Requirement: V3 costs and release blockers remain independent

V3 reports SHALL aggregate independently supplied model-token and tool-call
observations with coverage and nullable averages separately from task utility,
repeated errors and maintenance cost. Coverage SHALL use non-superseded stages
even when utility is inconclusive; the same denominator correction SHALL apply
to v2 repeated-error and maintenance metrics. Measured service latency,
scope/lifecycle/disclosure eligibility, future-evidence exclusion and tested
baseline replay SHALL remain independent gates. A missing observation or gate
MUST be inconclusive and MUST NOT become a zero observation or pass. Utility
differences MUST remain observational; synthetic input MUST stay labeled.

#### Scenario: Costs are observed while utility is missing
- **WHEN** an inconclusive stage supplies zero tool calls but no model tokens
- **THEN** tool coverage includes that stage, token coverage remains missing,
  utility stays inconclusive and no coverage can exceed one

#### Scenario: Utility improves while a release blocker fails
- **WHEN** task utility improves but a latency, isolation, lifecycle,
  disclosure, leakage or rollback gate fails or is absent
- **THEN** the report preserves separate aggregates and is non-passing

#### Scenario: Legacy input is replayed
- **WHEN** a supported v1 or v2 source with existing checksum locks is loaded
- **THEN** default schemas and input identities remain valid, v3 fields are
  refused, and documented v2 denominator correction is the only legacy change
