## Why

Stele already extracts bounded query hints and supports multiple recall, fusion,
reranking, diversity, and context strategies, but runtime selection is still
largely static. A versioned query-adaptive planner is needed to turn the
existing signals into deterministic, evidence-backed choices while keeping the
current original-query path as the safe baseline.

## What Changes

- Add a provider-independent retrieval-planning contract that classifies
  bounded query families and produces a complete, replayable plan.
- Let an approved plan select existing recall channels, per-channel and total
  candidate budgets, fusion parameters, memory-class quotas, reranker
  eligibility, and context-section priorities within hard service limits.
- Add adaptive candidate budgets based only on bounded query complexity,
  post-filter attrition categories, and reranker headroom; no plan may exceed
  the existing safety, latency, token, or candidate ceilings.
- Add one optional bounded second retrieval pass when first-pass evidence
  completeness is below a versioned threshold. The second pass reuses the exact
  resolved scope, lifecycle filters, original-query retention, and remaining
  resource budget and cannot recursively trigger another pass.
- Reuse exact-scope rollout with `diagnostics_only`, `shadow`, and
  `active_for_scope` stages. Shadow planning records comparison evidence but
  cannot change ordinary results.
- Extend deterministic evaluation and release evidence with per-query-family
  quality, plan identity, resource use, second-pass gain, fallback, and rollback
  results. Planning parameters must be justified by repository-owned replay or
  real-stack evidence rather than copied framework defaults.
- Keep the public search and context response shapes compatible; internal plans,
  derived query text, hidden candidates, and raw scores remain unavailable to
  ordinary callers.

## Capabilities

### New Capabilities

- `query-adaptive-retrieval-planning`: Defines deterministic query-family
  classification, bounded retrieval-plan construction, adaptive budgets,
  single-pass follow-up behavior, exact-scope rollout, and safe fallback.

### Modified Capabilities

- `query-understanding-and-multi-signal-retrieval`: Makes bounded query-analysis
  identities and hints eligible inputs to planning while preserving the
  immutable original query and fail-closed baseline.
- `stable-hybrid-candidate-fusion`: Allows an approved plan to select an
  explicit channel set, per-channel limits, total limit, and compatible
  query-family fusion parameters under existing hard bounds.
- `quality-aware-reranking`: Makes reranker eligibility a plan output without
  weakening the separate exact-scope reranker rollout and provider fallback
  controls.
- `context-assembly`: Allows an approved plan to prioritize existing context
  sections and quotas without changing response shape, citations, or caller
  budgets.
- `retrieval-evaluation-baseline`: Adds per-query-family plan replay, adaptive
  budget, first/second-pass, fallback, and resource measurements.
- `retrieval-release-gate-and-progressive-context-evaluation`: Requires
  compatible planner evidence, protected-family thresholds, bounded resource
  use, and tested fallback/rollback before scoped activation.

## Impact

- Retrieval orchestration under `internal/retrieval` gains focused planner,
  execution-budget, diagnostics, and evaluation types; existing searchers and
  fusion implementations remain reusable.
- Scoped ranking policy persistence and runtime configuration gain additive,
  versioned planner identities and hard limits. Any migration remains
  PostgreSQL-only and non-destructive.
- Evaluation fixtures and reports gain query-family expectations, plan
  identities, pass counts, candidate budgets, evidence-completeness categories,
  latency, and bounded fallback categories.
- Authorized admin/evaluation telemetry gains low-cardinality plan and pass
  outcomes; ordinary OpenAPI search/context payloads remain compatible.
- No new external runtime dependency or second system of record is introduced.

## Non-goals

- No bi-temporal validity model; that belongs to RQ2.
- No recursive graph traversal or graph-distance reranking; that belongs to RQ3.
- No online learning, autonomous policy mutation, or access-popularity ranking.
- No unbounded query rewrite, multi-pass, or agentic retrieval loop.
- No service-generated final answer, SDK, UI, or end-user product behavior.
- No replacement of PostgreSQL, pgvector, PostgreSQL full-text search, existing
  lifecycle filtering, or canonical-memory provenance.

## References

- Roadmap design:
  `docs/superpowers/specs/2026-09-16-retrieval-quality-roadmap-calibration-design.md`
- Authoritative roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Implementation workflow: `/opsx:apply` using the `openspec-apply-change`
  skill after proposal review.
- Archive workflow: `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName
  "query-adaptive-retrieval-planning"` only after implementation and
  verification are complete.
