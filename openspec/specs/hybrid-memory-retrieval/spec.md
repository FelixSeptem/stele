# hybrid-memory-retrieval Specification

## Purpose
Define the hybrid retrieval capability that combines lexical and semantic recall over governed canonical memory while preserving lifecycle-safe defaults.
## Requirements
### Requirement: Hybrid lexical and semantic retrieval
The service SHALL support governed retrieval through both lexical and semantic recall over canonical memory and summary memory.

#### Scenario: Lexical retrieval finds exact named memory
- **WHEN** a query depends on exact or near-exact text terms present in canonical memory
- **THEN** the service can return matching memory through PostgreSQL full-text lexical retrieval

#### Scenario: Semantic retrieval finds paraphrased memory
- **WHEN** a query is semantically similar to stored memory content without sharing the same exact wording
- **THEN** the service can return matching memory through vector-based semantic retrieval

### Requirement: Merged ranked retrieval output
The service MUST merge lexical and semantic candidates into a single ranked output rather than returning isolated recall streams.

#### Scenario: Query hits both recall paths
- **WHEN** a query produces both lexical and semantic candidates
- **THEN** the service deduplicates overlapping memories and returns one unified ranked result list

### Requirement: Lifecycle-safe default retrieval
Non-admin hybrid retrieval MUST exclude hidden lifecycle states by default.

#### Scenario: Hidden memory exists in the recall corpus
- **WHEN** suppressed, forgotten, expired, or deleted memory would otherwise match a retrieval query
- **THEN** the service excludes it from the default retrieval result set
