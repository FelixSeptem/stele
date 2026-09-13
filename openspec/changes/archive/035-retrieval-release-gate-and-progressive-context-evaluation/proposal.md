## Why

Stele now has versioned retrieval, chunking, fusion, query-understanding,
reranking, benchmark, and context-projection primitives, but no single release
contract proves that a new retrieval representation or provider configuration is
safe to ship. P5 Task 6.7 is needed now to turn those primitives into repeatable
real-provider evidence, progressive-context comparisons, bounded diagnostics,
and reversible release decisions without changing default retrieval behavior.

## What Changes

- Add an opt-in retrieval release-gate evaluation that requires an explicitly
  owned PostgreSQL + pgvector evaluation DSN and compares every candidate with
  an immutable compatible baseline.
- Record logical embedding and reranker identities, representation/fusion/
  ranking/policy versions, bounded quality metrics, latency, fallback classes,
  and safety outcomes without persisting secrets or raw provider material.
- Evaluate short retrieval projections, medium session/context overviews, and
  canonical/chunk evidence as progressive context levels with freshness,
  watermark, budget, citation, and deterministic rebuild evidence.
- Evaluate parent-first and bounded hierarchical retrieval in offline or shadow
  mode, with exact-scope expansion and the same quality, isolation, latency, and
  rollback gates as other ranking changes.
- Add an authorized, redacted retrieval-trajectory report and bounded retention
  and deletion behavior for trajectory, diagnostics, reports, and fixtures.
- Add a memory-organization integrity report that separates action success from
  information integrity for consolidation, merge, reclassification, reflection,
  and projection changes.
- Publish release checklists and rebuild/re-index/rollback runbooks; keep all
  experiments disabled for ordinary production retrieval until the release
  policy explicitly approves them.

### Non-goals

- No new canonical memory store, filesystem-backed memory protocol, AGFS/RAGFS,
  or polyglot persistence layer.
- No change to public search/context response shapes or default ranking behavior.
- No mandatory remote model service, benchmark download, LLM judge, or task-level
  quality score in the deterministic release gate.
- No implementation of the broader P6 maintenance/observability closure beyond
  retention and evidence needed by this release gate.

## Capabilities

### New Capabilities

- `retrieval-release-gate-and-progressive-context-evaluation`: Defines the
  release-gate evidence contract, progressive-context and parent-first
  comparisons, redacted retrieval trajectories, memory-organization integrity
  reports, and reversible publication policy.

### Modified Capabilities

- None. Existing retrieval-evaluation, context-projection, benchmark, rollout,
  and observability requirements remain the source contracts; this change adds a
  release-evidence capability that composes them.

## Impact

- Affected areas include `internal/retrieval` evaluation/replay/reporting,
  context projection comparison, benchmark runners, admin/evaluation
  diagnostics, retention jobs, documentation, and CI/release scripts.
- The change may add report schemas and bounded metadata but does not require a
  canonical-memory migration or a new runtime dependency.
- Operators gain explicit local configuration for an owned evaluation DSN and
  provider profile; ordinary tests remain offline and provider-neutral.
- Implementation must follow the repository's proposal branch lifecycle and
  use the existing OpenSpec commands for apply, validation, archive, merge, and
  push.
