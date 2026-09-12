## Why

Stable fusion, evidence deduplication, and diversity-aware packing now protect the
retrieval output, but the service still relies on one literal caller query. Stele
needs bounded query understanding to improve implicit, temporal, entity-centric,
mixed-language, and multi-hop recall while preserving the original query and the
existing scope/lifecycle-safe fallback path.

## What Changes

- Add a versioned, provider-independent query-analysis contract for deterministic
  normalization, bounded aliases and terms, optional entity/time/class/intent hints,
  and capped subquery decomposition.
- Keep the original query immutable and always eligible as a retrieval signal;
  normalized or decomposed queries are additions, never replacements.
- Run every validated signal through the same exact-scope, lifecycle-safe bounded
  recall, stable fusion, identity deduplication, diversity selection, citation, and
  budget pipeline used by the original query.
- Reuse existing exact-scope rollout governance for diagnostics-only, shadow,
  active, disabled, and rollback states, with deterministic original-query fallback
  for absent, malformed, adversarial, unavailable, or over-budget analysis.
- Add authorized bounded diagnostics and evaluation evidence for analysis version,
  signal/fallback categories, temporal and multi-hop coverage, protected simple-fact
  recall, candidate-pool bounds, and latency without exposing query plans or hidden
  evidence in ordinary responses.
- Require an explicitly owned PostgreSQL + pgvector evaluation result proving the
  preceding diversity duplicate-rate and budget gates before query decomposition may
  become active. When that DSN is absent, retain the documented
  `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` non-pass state.

## Non-goals

- Do not add feedback-aware feature ranking, task-outcome ranking, learned reranking,
  cross-encoders, online model query planners, or any Phase 6 Task 6.6 behavior.
- Do not replace, mutate, or persist caller query text as canonical memory or raw
  events, and do not expose internal normalized terms, subqueries, plans, or provider
  reasoning through ordinary APIs.
- Do not widen tenant, project, namespace, session, user, lifecycle, class, or time
  boundaries, and do not fetch broader source data to complete missing hints.
- Do not add a new public endpoint, request flag, response shape, SDK, UI, external
  search store, graph database, or second rollout system.
- Do not activate query decomposition using synthetic-only or skipped real-stack
  evidence.

## Capabilities

### New Capabilities

- `query-understanding-and-multi-signal-retrieval`: Versioned deterministic query
  normalization, bounded hint extraction and subquery decomposition, exact-scope
  rollout, safe original-query fallback, and redacted diagnostics.

### Modified Capabilities

- `hybrid-memory-retrieval`: Require original and derived query signals to share one
  bounded lifecycle-safe recall/fusion/deduplication/diversity path without changing
  ordinary public result identity or shape.
- `retrieval-evaluation-baseline`: Add versioned query-analysis replay, temporal,
  multi-hop, ambiguous, mixed-language, and adversarial fixtures, protected fallback
  metrics, and the real-stack diversity gate required before active decomposition.

## Impact

- Affected code: `internal/retrieval` query contracts and orchestration, existing
  ranking-rollout resolution in `internal/memory` and PostgreSQL storage if additional
  versioned parameters are required, retrieval evaluation fixtures/reports, bounded
  diagnostics/telemetry, and focused tests.
- Public search and context API shapes remain compatible; analysis details are
  restricted to existing authorized evaluation or admin diagnostics.
- PostgreSQL remains the sole system of record, and pgvector/PostgreSQL full-text
  search remain the semantic and lexical recall implementations.
- Depends on archived Phase 6 Tasks 6.2-6.4 and their stable chunking, fusion,
  deduplication, diversity, citation, and rollback contracts.
- References: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` Phase 6 Task 6.5;
  use `openspec validate bounded-query-understanding-and-multi-signal-retrieval
  --strict` before implementation and `/opsx:apply` to execute approved tasks.
