## MODIFIED Requirements

### Requirement: Merged ranked retrieval output
The service MUST merge lexical, semantic, enabled relation, and authorized
chunk-derived candidates recalled by the immutable original query and any
validated derived query signals into one lifecycle-safe ranked output rather
than returning isolated recall streams. The original query MUST remain a
mandatory signal, and each derived signal MUST use the same bounded channel
recall and resolved request constraints as the original query. The merge MUST
use the selected versioned stable fusion strategy over bounded channel ranks,
not implicit raw score addition within or across query signals, and MUST
preserve one canonical memory identity per result. Before candidates are handed
to context diversity selection or exposed as final ranked evidence, the service
MUST apply the selected deterministic identity and validated-lineage
deduplication policy without weakening scope, lifecycle, source-lineage,
citation, canonical fallback, or result-budget behavior. Rejecting or losing an
optional derived signal MUST preserve the original-query retrieval path and the
ordinary public result shape.

#### Scenario: Query hits multiple recall paths
- **WHEN** an original or validated derived query signal produces candidates from two or more enabled recall paths
- **THEN** the service deduplicates overlapping canonical memories, fuses their bounded channel ranks through the selected strategy, applies deterministic identity/lineage handling to the validated fused candidates, and returns one unified deterministic ranked result list

#### Scenario: Original and derived signals overlap
- **WHEN** the original query and one or more validated derived signals recall the same canonical memory or validated source lineage
- **THEN** the service retains one stable canonical result identity and does not add incomparable raw scores across query signals

#### Scenario: Equivalent fused candidates share source evidence
- **WHEN** otherwise distinct fused candidates resolve to one validated source event or parent-memory lineage in the exact resolved scope
- **THEN** the service selects one stable representative before handing evidence to context diversity selection and retains bounded authorized citations for the equivalent sources

#### Scenario: One optional recall path is unavailable
- **WHEN** semantic, relation, or authorized chunk recall is unavailable while at least one validated required recall path remains available
- **THEN** the service returns a fused result from the remaining validated paths without using incomparable raw scores or widening scope

#### Scenario: One derived query signal is rejected
- **WHEN** a derived query signal is malformed, unavailable, duplicate, unsafe, or over budget
- **THEN** the service excludes that signal and continues the immutable original-query path within the existing candidate and result budgets

#### Scenario: Derived signal would widen retrieval boundaries
- **WHEN** a derived query signal implies a broader tenant, project, namespace, session, user, lifecycle, memory-class, or time boundary than the resolved request
- **THEN** the service rejects the broader effect and neither ranks nor discloses evidence obtained outside the original resolved boundaries

#### Scenario: Every derived signal is unavailable
- **WHEN** no validated derived query signal remains eligible for recall
- **THEN** the service returns the original-query baseline behavior without changing the ordinary public result identity or shape

#### Scenario: No fusion rollout is approved for the scope
- **WHEN** no approved fusion strategy rollout applies to the resolved scope
- **THEN** the service uses the current safe baseline strategy and preserves the ordinary public result shape
