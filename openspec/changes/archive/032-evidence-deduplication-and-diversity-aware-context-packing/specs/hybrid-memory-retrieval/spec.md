## MODIFIED Requirements

### Requirement: Merged ranked retrieval output
The service MUST merge lexical, semantic, enabled relation, and authorized
chunk-derived candidates into one lifecycle-safe ranked output rather than
returning isolated recall streams. The merge MUST use the selected versioned
stable fusion strategy over bounded channel ranks, not implicit cross-channel raw
score addition, and MUST preserve one canonical memory identity per result.
Before candidates are handed to context diversity selection or exposed as final
ranked evidence, the service MUST apply the selected deterministic identity and
validated-lineage deduplication policy without weakening scope, lifecycle,
source-lineage, citation, or canonical fallback behavior.

#### Scenario: Query hits multiple recall paths
- **WHEN** a query produces candidates from two or more enabled recall paths
- **THEN** the service deduplicates overlapping canonical memories, fuses their
  bounded channel ranks through the selected strategy, applies deterministic
  identity/lineage handling to the validated fused candidates, and returns one
  unified deterministic ranked result list

#### Scenario: Equivalent fused candidates share source evidence
- **WHEN** otherwise distinct fused candidates resolve to one validated source
  event or parent-memory lineage in the exact resolved scope
- **THEN** the service selects one stable representative before handing evidence
  to context diversity selection and retains bounded authorized citations for
  the equivalent sources

#### Scenario: One optional recall path is unavailable
- **WHEN** semantic, relation, or authorized chunk recall is unavailable while
  at least one validated required recall path remains available
- **THEN** the service returns a fused result from the remaining validated paths
  without using incomparable raw scores or widening scope

#### Scenario: No fusion rollout is approved for the scope
- **WHEN** no approved fusion strategy rollout applies to the resolved scope
- **THEN** the service uses the current safe baseline strategy and preserves the
  ordinary public result shape
