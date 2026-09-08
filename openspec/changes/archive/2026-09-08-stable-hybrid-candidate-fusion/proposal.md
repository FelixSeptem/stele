## Why

Stele currently merges lexical, semantic, relation, and opt-in chunk-derived
candidates by adding raw channel scores. Those values have incompatible scales,
so an embedding-provider change or lexical-score distribution shift can silently
change channel dominance even when the underlying evidence quality is unchanged.
The retrieval evaluation baseline and versioned chunk representation are now in
place, making it possible to replace that unstable merge with a measured,
reversible fusion contract before adding deduplication, diversity, query analysis,
or reranking.

## What Changes

- Introduce a versioned, deterministic candidate-fusion contract that accepts
  bounded ranked candidates from lexical, semantic, relation, and authorized
  chunk-derived recall paths before final truncation.
- Make Reciprocal Rank Fusion (RRF) the default fusion strategy; retain a
  normalized weighted strategy as an explicit offline-comparison experiment,
  never as implicit score arithmetic.
- Preserve exact resolved scope, lifecycle visibility, memory-class filters,
  canonical parent identity, and bounded citations before, during, and after
  fusion.
- Define deterministic tie breaking and bounded internal channel-rank/fusion
  diagnostics for authorized evaluation or admin paths only.
- Add scope-selectable, default-safe fusion rollout and rollback behavior, with
  graceful degradation when an optional recall channel is unavailable.
- Extend retrieval evaluation fixtures, reports, and regression checks to compare
  named fusion versions for recall, MRR, latency, candidate-pool, duplicate, and
  zero-leakage outcomes.

## Non-goals

- Do not implement identity or semantic deduplication, MMR, or diversity-aware
  context packing.
- Do not add query decomposition, query rewriting, temporal/entity analysis,
  feedback-aware quality signals, or model-based rerankers.
- Do not change canonical memory, raw events, chunk lineage, public search result
  identity, or ordinary public diagnostic exposure.
- Do not add an external search engine, graph database, SDK, UI, MCP adapter, or
  a second system of record.

## Capabilities

### New Capabilities

- `stable-hybrid-candidate-fusion`: Versioned, deterministic, bounded candidate
  fusion strategies, rollout selection, fallback behavior, and authorized
  fusion diagnostics for governed retrieval.

### Modified Capabilities

- `hybrid-memory-retrieval`: Replace raw-score addition with stable ranked
  channel fusion while preserving canonical fallback, lifecycle-safe defaults,
  scope isolation, and public result shape.
- `retrieval-evaluation-baseline`: Record and compare named fusion strategy
  versions and channel-level aggregate dispositions without weakening existing
  zero-leakage or lifecycle assertions.

## Impact

- Affected code: `internal/retrieval`, retrieval rollout policy/domain types,
  PostgreSQL rollout-policy storage where a durable selection is required,
  evaluation/reporting packages, telemetry, and focused tests.
- Affected APIs: ordinary public search and context responses remain compatible;
  fusion metadata is restricted to authorized evaluation/admin diagnostics.
- Affected operations: fusion is default-safe, scope-selectable, measured through
  the existing evaluation workflow, and can revert to the prior approved
  canonical merge path without data rewrite or schema rollback.
- Dependencies: archived `retrieval-evaluation-baseline`,
  `hierarchical-memory-representation-bounded-chunking`, lifecycle/scope
  contracts, and `openspec validate stable-hybrid-candidate-fusion --strict`
  before implementation.
