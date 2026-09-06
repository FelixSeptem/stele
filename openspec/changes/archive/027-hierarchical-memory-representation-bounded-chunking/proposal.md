## Why

Stele now has a reproducible retrieval baseline and governed context projections,
but retrieval still operates primarily on canonical-memory granularity. Long or
heterogeneous source records can therefore hide independently useful evidence,
while later ranking work would have no stable, provenance-preserving representation
to compare. This change introduces a bounded, hierarchical derived representation
before hybrid fusion or reranking is changed.

## What Changes

- Add a versioned, PostgreSQL-backed source-chunk model derived from immutable raw
  events and authorized canonical-memory versions.
- Implement deterministic boundary-aware chunking with configurable character and
  token bounds, source ordinals, parent references, and extraction/renderer
  versions.
- Preserve exact tenant/project/namespace/session/user isolation for chunk creation,
  reads, rebuilds, and parent-child expansion.
- Add repository and service contracts for chunk creation, scoped lookup, source
  lineage, idempotent rebuilds, and bounded adjacent/parent context lookup.
- Apply explicit memory-class granularity policies for profile, episodic,
  procedural, summary, and relation memory.
- Keep chunk candidates behind a versioned, default-off or shadow-only rollout
  switch so the existing canonical retrieval path remains the safe fallback.
- Extend retrieval evaluation and context diagnostics to identify chunk-derived
  evidence and bounded parent-context inclusion without exposing internal source
  payloads or hidden records.
- Add regression and PostgreSQL integration coverage for provenance, lifecycle
  filtering, rebuild determinism, duplicate behavior, and scope isolation.

### Non-goals

- Do not change the default lexical/semantic/relation fusion strategy in this
  change; RRF is a subsequent proposal.
- Do not add semantic deduplication, MMR, diversity-aware packing, query
  decomposition, or model-based reranking.
- Do not mutate canonical memory or raw events in place, and do not make chunks a
  second system of record.
- Do not add an external search engine, graph database, MCP adapter, SDK, UI, or
  provider-specific reasoning dependency.
- Do not widen retrieval scope through namespace trees, parent traversal, or
  session adjacency.

## Capabilities

### New Capabilities

- `hierarchical-memory-chunking`: Versioned, bounded, deterministic derived source
  chunks with parent lineage, class-aware granularity, rebuildability, and exact
  scope/lifecycle enforcement.

### Modified Capabilities

- `hybrid-memory-retrieval`: Permit the controlled retrieval path to use authorized
  chunk candidates and bounded parent evidence while preserving lifecycle-safe
  defaults, source citations, and an explicit canonical fallback.
- `context-assembly`: Permit bounded parent/adjacent source context and chunk
  citations to participate in existing sections without changing public section
  names, budgets, or hidden-memory filtering.

## Impact

- Affected code: `internal/memory`, `internal/retrieval`,
  `internal/storage/postgres`, `internal/app`, evaluation/diagnostic packages,
  and related tests.
- Affected storage: one forward PostgreSQL migration for chunk metadata,
  versioned source identity, scoped indexes, and rebuild/idempotency constraints.
- Affected APIs: internal/admin or evaluation contracts gain bounded chunk
  diagnostics; ordinary public search and context response shapes remain stable.
- Affected operations: chunk materialization and rebuild require explicit scope,
  policy/renderer versions, bounded work, and rollback to canonical retrieval by
  disabling the rollout switch.
- Dependencies: existing canonical-memory lifecycle, provenance, versioned context
  projection, retrieval-evaluation baseline, and PostgreSQL migration contracts.
- Related roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`, Phase 6 Task
  6.2 / Stage 5 first slice.
- Related workflow: after approval, use `/opsx:apply`; validate with
  `openspec validate hierarchical-memory-representation-bounded-chunking --strict`.
