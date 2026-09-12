## Context

See `proposal.md` for motivation. The repository already validates scope/lifecycle/lineage, performs bounded stable fusion, applies diversity-aware packing, and has durable scoped ranking rollout governance. Existing feedback hints are intentionally small and must remain compatible with the new stage.

## Goals / Non-Goals

**Goals:**

- Add a provider-independent rerank boundary and deterministic quality-feature adjustment.
- Keep RRF as the immutable fallback and make every non-baseline behavior exact-scope, measurable, and reversible.
- Support operator-supplied OpenAI-compatible services without committing secrets or environment-specific datasets.
- Preserve public result contracts and redacted, low-cardinality diagnostics.

**Non-Goals:**

- Training, online learning, provider-specific SDKs, mandatory model availability, or direct canonical-memory writes.
- Replacing existing feedback/task rollout governance with a second policy system.

## Decisions

### 1. Stage placement and baseline

The pipeline remains: query understanding → validated recall → stable fusion → quality feature extraction → optional provider rerank → bounded score adjustment → deduplication/diversity/context packing. The original fused score and order are retained as the baseline snapshot. If any new stage is disabled or fails, the snapshot is returned unchanged.

### 2. Versioned quality model

Introduce a compact `QualityFeatureVector` with normalized values and presence bits. Features are derived from existing repositories/summarizers, capped to a small bounded range, and identified by a version such as `quality-v1`. Missing evidence contributes neutral values; safety-related findings never become a positive boost.

### 3. Provider contract and adapter

Define a Go `Reranker` interface accepting a query plus already validated candidates and returning scores keyed only by candidate identity. Implement an HTTP adapter for an OpenAI-compatible `/v1/rerank`-style JSON contract with configurable path, model, timeout, max candidates, and max text bytes. The adapter validates status, schema, IDs, duplicates, finite scores, and response bounds. It never logs headers or raw bodies.

### 4. Rollout integration

Extend the existing ranking rollout policy with logical quality/reranker fields and a mode (`diagnostics_only`, `shadow`, `active_for_scope`). Durable policy data contains no secrets. Activation still requires matching dry-run evidence, thresholds, attribution, and no blockers. Diagnostics/shadow compute comparisons but do not alter ordinary results.

### 5. Configuration and secret handling

Add typed config with disabled defaults and environment-variable loading. Document `.env.example`-style placeholders only; recommend `.env.local`, Docker secrets, or process environment and ensure local files are ignored. Tests use fake in-process HTTP servers and deterministic providers, never real credentials or external datasets.

### 6. Evaluation and observability

Replay reports record feature/reranker identities, candidate-count buckets, changed-rank counts, fallback categories, and protected-metric deltas. Telemetry labels remain low-cardinality (provider logical name, mode, outcome, bounded count bucket); no scope, query, memory ID, score, or endpoint labels.

## Risks / Trade-offs

- [External provider latency] → strict timeout, candidate/text caps, shadow-first rollout, and immediate baseline fallback.
- [Score-scale mismatch] → normalize provider scores, clamp total delta, and never replace the fused score directly.
- [Sparse or biased feedback] → evidence minimums, neutral missing values, and no single-event activation.
- [Configuration leakage] → environment/secrets only, redacted logs, placeholder examples, and repository secret checks.
- [Policy schema drift] → additive migration, versioned policy identity, and backward-compatible zero-value defaults.

## Migration Plan

1. Deploy code with reranking disabled and existing policies mapped to baseline behavior.
2. Configure a fake/local provider and run diagnostics-only replay for owned scopes.
3. Compare protected recall, multi-hop coverage, duplicate rate, latency, and isolation metrics against the RRF baseline.
4. Activate only an exact scope after gates pass; rollback by disabling the policy, requiring no canonical-data rewrite.

## Open Questions

- The exact external provider JSON dialect may vary; keep the adapter path and response field mapping configurable while preserving the provider-independent contract.
