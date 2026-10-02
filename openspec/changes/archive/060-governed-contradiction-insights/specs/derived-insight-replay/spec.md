## ADDED Requirements

### Requirement: Replay reports contradiction temporal dispositions

Bounded replay SHALL preserve contradiction keys, both source-version
identities, valid-time intervals, source watermarks, and policy versions, and
SHALL distinguish `contradiction`, `temporal_coexistence`,
`unresolved_temporal`, `stale_evidence`, `review_required`, and
`would_activate` outcomes.

#### Scenario: Replay finds a temporal coexistence

- **WHEN** replay evaluates incompatible fact values whose valid-time intervals do not overlap
- **THEN** the report records temporal coexistence and does not schedule activation

#### Scenario: Replay finds an overlapping contradiction

- **WHEN** replay evaluates two visible, mutually exclusive fact versions with overlapping intervals
- **THEN** the report records a deterministic contradiction candidate and either review-required or would-activate according to policy

#### Scenario: Replay evidence is stale

- **WHEN** either source version or its watermark no longer satisfies the replay contract
- **THEN** the report records stale evidence and performs no activation or source refresh
