## Why

Stele can already assemble scoped, deduplicated, diverse context and can collect
usefulness, task-success, verification, and quality signals, but it does not yet
measure context efficiency as a single deterministic release surface. Without
that evidence, feedback-aware ranking risks optimizing for popularity or a
small number of noisy observations instead of useful, fresh, independently
supported evidence. RQ1–RQ3 now provide the bounded planning, temporal, and
graph-evidence contracts needed to calibrate context cost safely.

## What Changes

- Add a versioned context-efficiency evidence model that reports bounded
  relevant-token ratio, evidence density, duplicate/stale token rates,
  quality-per-budget, pass-level candidate/context cost, and latency buckets.
- Add deterministic fixed-clock replay fixtures that compare the approved flat
  context baseline with optional feedback-calibrated packing under identical
  scope, temporal constraints, graph policy, and caller budget.
- Add a weak feedback-calibration layer that consumes only active, scoped,
  non-superseded aggregate feedback and task-quality signals; apply explicit
  caps, decay, minimum evidence thresholds, and asynchronous/rebuildable
  summaries.
- Keep feedback calibration opt-in through per-request or exact-scope governed
  rollout policies, with diagnostics-only, shadow, active, disable, and
  rollback states.
- Extend authorized release evidence with redacted efficiency deltas and hard
  gates for protected recall, lifecycle/scope isolation, citation coverage,
  duplicate/stale rates, budget overflow, deterministic replay, and rollback.
- Preserve existing public search/context response shapes and keep ordinary
  requests baseline-equivalent unless an explicit approved calibration policy
  applies.

## Capabilities

### New Capabilities

- `context-efficiency-feedback-calibration`: Defines versioned, bounded,
  deterministic context-efficiency metrics and weak feedback calibration with
  governed rollout and rollback.

### Modified Capabilities

- `context-assembly`: Add efficiency accounting, feedback-calibrated packing
  behavior, and baseline-equivalent fallback requirements without changing
  public section names or citation shape.
- `feedback-ranking-rollout-governance`: Add context-efficiency evidence,
  minimum-signal/decay/cap rules, and calibration-specific activation gates.
- `retrieval-release-gate-and-progressive-context-evaluation`: Add redacted
  efficiency evidence and hard release failures for protected recall loss,
  stale/duplicate budget waste, nondeterministic replay, and rollback failure.

## Impact

- Affected code: `internal/retrieval`, `internal/memory`, `internal/insights`,
  `internal/app`, `internal/storage/postgres`, and `internal/telemetry`.
- Affected contracts: context assembly planning/packing, feedback signal
  aggregation, evaluation reports, rollout policy identities, and admin-only
  diagnostics. Ordinary OpenAPI/MCP response models remain unchanged.
- Affected persistence: additive, rebuildable derived evidence and calibration
  summaries only; no canonical memory, raw event, provenance, or source-version
  overwrite.
- Dependencies: existing PostgreSQL system of record, pgvector-compatible
  retrieval, deterministic evaluation fixtures, and current rollout/release
  gate infrastructure. No new graph database, queue, provider, or external
  ranking service is required.
- Non-goals:
  - No global popularity or unbounded reinforcement ranking.
  - No automatic canonical-memory mutation from feedback.
  - No LLM judge as the sole release gate.
  - No new public diagnostic fields, raw feedback history, raw scores, or
    hidden candidate identifiers.
  - No replacement of existing deduplication, diversity, temporal, or graph
    safety contracts.

## References

- `openspec/specs/context-assembly/spec.md`
- `openspec/specs/feedback-ranking-rollout-governance/spec.md`
- `openspec/specs/retrieval-release-gate-and-progressive-context-evaluation/spec.md`
- `openspec/specs/evidence-deduplication-and-diversity-packing/spec.md`
- `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- `openspec validate --all --strict`
