## ADDED Requirements

### Requirement: Chunk candidates use controlled canonical fallback
The service SHALL permit authorized derived chunk candidates to participate in a
versioned, exact-scope retrieval rollout or shadow evaluation. The canonical-memory
retrieval path SHALL remain available as the default fallback. A chunk candidate
MUST retain parent/source citations and pass the same scope and lifecycle filters as
canonical candidates before it can affect a result.

#### Scenario: Chunk rollout is disabled
- **WHEN** no approved chunk-candidate rollout applies to the resolved scope
- **THEN** hybrid retrieval preserves canonical-memory candidate behavior and does
  not return chunk-derived results

#### Scenario: Chunk candidate is enabled for an exact scope
- **WHEN** an approved chunk-candidate rollout applies and a lifecycle-visible chunk
  matches the query in that exact scope
- **THEN** retrieval can use the chunk as derived evidence while preserving bounded
  parent/source citations and canonical fallback behavior

### Requirement: Chunk diagnostics remain bounded and authorized
The service SHALL expose chunk candidate selection, source validation, parent
expansion, and rollout disposition only through authorized evaluation or admin
diagnostics. Ordinary retrieval responses MUST NOT disclose raw source ranges,
hidden chunk content, foreign identifiers, internal policy details, or shadow-only
candidate information.

#### Scenario: Authorized evaluation observes chunk behavior
- **WHEN** an authorized evaluation runs with chunk candidates enabled or shadowed
- **THEN** it receives bounded category-level chunk diagnostics sufficient to compare
  source coverage and exclusions

#### Scenario: Ordinary public retrieval runs
- **WHEN** a client invokes ordinary hybrid retrieval
- **THEN** the response preserves the public result contract and does not expose
  internal chunk diagnostics
