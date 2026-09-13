## MODIFIED Requirements

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
