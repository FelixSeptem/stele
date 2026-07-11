## MODIFIED Requirements

### Requirement: Hybrid lexical and semantic retrieval
The service SHALL support governed retrieval through both lexical and semantic recall over canonical memory and summary memory.

#### Scenario: Lexical retrieval finds exact named memory
- **WHEN** a query depends on exact or near-exact text terms present in canonical memory
- **THEN** the service can return matching memory through PostgreSQL full-text lexical retrieval

#### Scenario: Semantic retrieval finds paraphrased memory
- **WHEN** a query is semantically similar to stored memory content without sharing the same exact wording
- **THEN** the service can return matching memory through vector-based semantic retrieval

#### Scenario: Semantic retrieval uses only the active vector revision
- **WHEN** a canonical memory has multiple vector revisions from rebuild or provider rotation activity
- **THEN** the service only uses the active vector revision for the current canonical projection in default semantic retrieval

#### Scenario: Semantic projection is unavailable during rebuild
- **WHEN** a canonical memory has no active embedding because semantic projection is missing, stale, rebuilding, failed, or superseded
- **THEN** the retrieval pipeline excludes that memory from semantic recall without failing the overall hybrid retrieval request
