## Why

Stable hybrid fusion and bounded query understanding now provide a reliable candidate set, but ranking still has limited awareness of evidence quality, freshness, verification outcomes, and task usefulness. Phase 6.6 adds a measured, reversible quality-aware reranking layer without making an external model or untracked operator configuration part of Stele's canonical behavior.

## What Changes

- Define a versioned, bounded quality-feature model for visible retrieval candidates, including evidence coverage, freshness, source reliability, conflict state, usefulness feedback, task-success summaries, and verification outcomes.
- Add deterministic score adjustments with explicit caps, missing-feature defaults, and stable tie-breaking after RRF fusion and before diversity/context packing.
- Add a provider-independent reranker contract and an optional OpenAI-compatible HTTP adapter for cross-encoder or scoring services.
- Extend scoped ranking rollout policy selection with logical reranker/quality-policy identities, while keeping endpoint URLs, API keys, DSNs, and evaluation data outside Git-tracked artifacts.
- Preserve diagnostics-only and shadow execution, exact tenant/project/namespace matching, activation gates, rollback, and the immutable original RRF baseline.
- Add bounded redacted diagnostics, low-cardinality telemetry, replay/evaluation metadata, configuration documentation, and focused tests.

## Capabilities

### New Capabilities

- `quality-aware-reranking`: Versioned quality signals, deterministic bounded adjustments, optional provider-independent reranking, fail-closed behavior, and redacted diagnostics.
- `reranker-provider-runtime`: Runtime configuration and OpenAI-compatible HTTP adapter for optional reranker services with secret-safe local injection.

### Modified Capabilities

- `hybrid-memory-retrieval`: Add the optional quality-aware reranking stage after stable fusion while preserving baseline ordering and visibility guarantees.
- `feedback-ranking-rollout-governance`: Allow scoped policies to select quality/reranker versions and require matching evidence and rollback gates without persisting provider secrets.

## Impact

- Affected code: `internal/retrieval`, `internal/memory` ranking rollout types, `internal/config`, runtime provider wiring, telemetry, evaluation/replay reporting, and retrieval/self-hosting documentation.
- No canonical-memory schema rewrite is required; any new durable rollout metadata uses additive PostgreSQL migrations and existing audit/repository patterns.
- Public search/context result shapes remain compatible. Detailed candidate features, raw model scores, credentials, DSNs, query text, and provider payloads remain restricted to authorized diagnostics or local evaluation.
- Model services are optional. Defaults are disabled and all endpoint, model, timeout, API key, and dataset settings are supplied through environment variables, Docker secrets, or ignored local files only.

## Non-goals

- No mandatory hosted model dependency, SDK, UI, or end-user product logic.
- No replacement of RRF, fusion, lifecycle filtering, scope isolation, or PostgreSQL as the system of record.
- No unbounded online learning, autonomous policy activation, or direct canonical-memory mutation based on model output.
