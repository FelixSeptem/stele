## MODIFIED Requirements

### Requirement: Merged ranked retrieval output
The service MUST merge lexical, semantic, enabled relation, and authorized
chunk-derived candidates into one lifecycle-safe ranked output rather than
returning isolated recall streams. The merge MUST use the selected versioned
stable fusion strategy over bounded channel ranks, not implicit cross-channel raw
score addition, and MUST preserve one canonical memory identity per result.

#### Scenario: Query hits multiple recall paths
- **WHEN** a query produces candidates from two or more enabled recall paths
- **THEN** the service deduplicates overlapping canonical memories, fuses their
  bounded channel ranks through the selected strategy, and returns one unified
  deterministic ranked result list

#### Scenario: One optional recall path is unavailable
- **WHEN** semantic, relation, or authorized chunk recall is unavailable while
  at least one validated required recall path remains available
- **THEN** the service returns a fused result from the remaining validated paths
  without using incomparable raw scores or widening scope

#### Scenario: No fusion rollout is approved for the scope
- **WHEN** no approved fusion strategy rollout applies to the resolved scope
- **THEN** the service uses the current safe baseline strategy and preserves the
  ordinary public result shape

### Requirement: Authorized retrieval evaluation diagnostics
The service SHALL support bounded candidate-channel, channel-rank, selected
fusion-strategy, and final-disposition diagnostics for controlled retrieval
evaluation without changing ordinary public result shape or exposing hidden
evidence.

#### Scenario: Evaluation captures visible candidate channels
- **WHEN** an authorized evaluator runs a scoped retrieval fixture
- **THEN** it can identify the selected fusion strategy version, whether a
  lifecycle-visible result was supplied by lexical, semantic, enabled relation,
  or enabled chunk recall, and record bounded channel-rank and final-rank data

#### Scenario: Ordinary retrieval runs
- **WHEN** a client uses ordinary hybrid retrieval without an authorized
  evaluation or admin diagnostic path
- **THEN** the service preserves the existing result shape and does not expose
  fusion configuration, channel ranks, internal candidate pools, or
  channel-failure details

#### Scenario: Candidate is hidden or out of scope
- **WHEN** a candidate would violate lifecycle visibility or resolved scope
  boundaries
- **THEN** evaluation diagnostics record only a stable aggregate exclusion or
  failure category, and the candidate does not contribute to a fused score or
  disclose content or identifiers
