## ADDED Requirements

### Requirement: Chunks preserve validity snapshots
Every derived chunk MUST retain the immutable source canonical version and
validity identity used at materialization. Ordinary and historical chunk reads
MUST apply the same temporal predicate as canonical retrieval.

#### Scenario: Chunk source expires
- **WHEN** a chunk's source version is no longer valid for the current query
- **THEN** ordinary chunk retrieval excludes the chunk and does not expose its
  content or parent through fallback

#### Scenario: Historical chunk query is authorized
- **WHEN** an explicit temporal plan requests an interval containing the chunk's
  source validity
- **THEN** the chunk may participate with its source-version citation and exact
  scope preserved
