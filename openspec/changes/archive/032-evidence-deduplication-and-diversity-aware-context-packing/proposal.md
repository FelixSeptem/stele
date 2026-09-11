## Why

Stable hybrid fusion now produces a deterministic, lifecycle-safe ranked
candidate set, but repeated versions, shared source evidence, and near-identical
memories can still consume the same context budget. Stele needs a measured,
reversible packing policy that preserves independent evidence and citations
before later query analysis or reranking changes the retrieval path.

## What Changes

- Add a versioned, deterministic evidence-deduplication policy that recognizes
  equivalent candidates by canonical memory, source event, and parent-memory
  lineage, with bounded configurable semantic-similarity clusters.
- Add diversity-aware selection after stable candidate fusion and before final
  context packing, balancing independent evidence across memory class, source
  session, entity, and time slice within existing character/token budgets.
- Use deterministic maximal marginal relevance (MMR), or an explicitly
  equivalent versioned selection policy, for approved scoped rollouts and
  authorized shadow evaluation.
- Preserve canonical identity and bounded citations for selected evidence; make
  duplicate, diversity, and budget omission reasons available only through
  bounded authorized diagnostics.
- Extend retrieval evaluation fixtures, reports, and release-policy comparison
  to measure duplicate rate, evidence coverage, protected recall, latency, and
  zero-leakage results for named diversity-policy versions.
- Keep rollback explicit: diversity selection can be disabled without rewriting
  canonical memory, raw events, chunks, provenance, or the approved fusion
  strategy.

## Non-goals

- Do not add query decomposition, query rewriting, temporal/entity query
  analysis, feedback-aware ranking, or model-based reranking.
- Do not change canonical memory or raw-event content, chunk lineage, ordinary
  public result identity, or public response shapes.
- Do not disclose similarity scores, cluster membership, hidden candidates,
  foreign-scope identifiers, or internal selection diagnostics on ordinary API
  paths.
- Do not introduce an external search engine, graph database, SDK, UI, MCP
  adapter, or a second system of record.

## Capabilities

### New Capabilities

- `evidence-deduplication-and-diversity-packing`: Versioned, deterministic,
  lifecycle-safe candidate identity deduplication and diversity-aware context
  selection with bounded diagnostics and reversible scoped rollout behavior.

### Modified Capabilities

- `hybrid-memory-retrieval`: Require post-fusion candidate identity handling and
  controlled handoff of only validated, canonicalized candidates to diversity
  selection without weakening scope, lifecycle, lineage, or fallback behavior.
- `context-assembly`: Require section-aware, deterministic diversity packing
  that respects the existing budget, citation, projection, and hidden-memory
  safety contracts.
- `retrieval-evaluation-baseline`: Require policy-version-aware duplicate,
  diversity, coverage, latency, and protected safety comparison for the new
  selection behavior.

## Impact

- Affected code: `internal/retrieval`, `internal/memory` context assembly and
  rollout policy code, PostgreSQL policy storage if durable selection is
  required, evaluation/reporting, diagnostics, telemetry, and focused tests.
- Affected APIs: ordinary search and context response shapes remain compatible;
  new diagnostics are restricted to existing authorized evaluation or admin
  paths.
- Dependencies: archived retrieval evaluation, hierarchical chunking, versioned
  context projections, governed intent/reflection/compaction evidence, and
  stable hybrid candidate fusion changes.
- References: roadmap Phase 6 Task 6.4; use `openspec validate
  evidence-deduplication-and-diversity-aware-context-packing --strict` before
  implementation and `/opsx:apply` to execute approved tasks.
