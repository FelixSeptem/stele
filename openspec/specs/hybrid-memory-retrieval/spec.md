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

#### Scenario: Semantic retrieval uses only the active vector revision
- **WHEN** a canonical memory has multiple vector revisions from rebuild or provider rotation activity
- **THEN** the service only uses the active vector revision for the current canonical projection in default semantic retrieval

#### Scenario: Semantic projection is unavailable during rebuild
- **WHEN** a canonical memory has no active embedding because semantic projection is missing, stale, rebuilding, failed, or superseded
- **THEN** the retrieval pipeline excludes that memory from semantic recall without failing the overall hybrid retrieval request

### Requirement: Merged ranked retrieval output
The service MUST merge lexical, semantic, enabled relation, and authorized chunk-derived candidates into one lifecycle-safe ranked output using the selected versioned stable fusion strategy over bounded channel ranks, preserve one canonical memory identity per result, and optionally apply a bounded quality-aware reranking stage after fusion and before diversity/context packing.

#### Scenario: Query hits multiple recall paths
- **WHEN** a query produces candidates from two or more enabled recall paths
- **THEN** the service deduplicates overlapping canonical memories, fuses their bounded channel ranks through the selected strategy, optionally applies an approved quality-aware rerank, and returns one unified deterministic ranked result list

#### Scenario: No rerank rollout is approved for the scope
- **WHEN** no approved quality/rerank rollout applies to the resolved scope
- **THEN** the service uses the current safe RRF/fusion baseline and preserves the ordinary public result shape

#### Scenario: One optional recall path is unavailable
- **WHEN** semantic, relation, or authorized chunk recall is unavailable while at least one validated required recall path remains available
- **THEN** the service returns a fused result from the remaining validated paths without using incomparable raw scores or widening scope, and any optional reranker failure also falls back to the fused ordering

#### Scenario: No fusion rollout is approved for the scope
- **WHEN** no approved fusion strategy rollout applies to the resolved scope
- **THEN** the service uses the current safe baseline strategy and preserves the ordinary public result shape, with no quality-aware rerank applied unless separately approved

#### Scenario: Optional reranker is unavailable
- **WHEN** an enabled optional reranker is unavailable or returns invalid output
- **THEN** retrieval falls back to the validated fused ordering without widening scope or exposing provider details

### Requirement: Lifecycle-safe default retrieval
Non-admin hybrid retrieval MUST exclude hidden lifecycle states by default.

#### Scenario: Hidden memory exists in the recall corpus
- **WHEN** suppressed, forgotten, expired, or deleted memory would otherwise match a retrieval query
- **THEN** the service excludes it from the default retrieval result set

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
