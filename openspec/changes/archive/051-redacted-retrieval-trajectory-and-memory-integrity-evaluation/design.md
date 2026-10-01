## Context

The proposal extends the existing retrieval release-gate, owned evidence-run,
admin inspection, and observability contracts. The current system already
requires PostgreSQL as the only system of record, exact tenant/project/namespace
isolation, append-only derived artifacts, redacted release reports, and
shadow-only experimental retrieval. The design therefore adds a common,
derived evidence envelope instead of introducing another diagnostic store.

## Goals / Non-Goals

**Goals:**

- Produce one versioned evidence envelope for redacted retrieval trajectories
  and memory-organization integrity checks.
- Make action success and information integrity independently reviewable.
- Enforce exact-scope authorization, compatibility, freshness, retention, and
  deterministic replay.
- Keep all experimental retrieval and reasoning behavior non-authoritative.
- Reuse existing admin, OpenAPI, release-policy, metrics, and PostgreSQL
  patterns.

**Non-Goals:**

- Changing public retrieval or context response payloads.
- Enabling new retrieval strategies or reserved insight types.
- Storing raw queries, content, scores, prompts, provider payloads, or secrets.
- Creating a second canonical or graph-backed persistence system.

## Decisions

### 1. Use a shared redacted evidence envelope

Trajectory and integrity outputs will share logical identities for exact scope,
fixture, policy, strategy, renderer, source watermark, and replay profile.
Payloads contain bounded enum categories, count buckets, boolean gate results,
and coarse latency/resource buckets. This keeps comparison deterministic while
avoiding high-cardinality leakage.

Alternative: persist raw traces and redact at read time. Rejected because raw
queries, identifiers, hidden candidates, and provider failures would remain a
retention and breach risk.

### 2. Keep trajectory collection separate from public request handling

The evaluator records trajectory aggregates as an optional evaluation/admin
side effect. Public search and context paths do not return trajectory details
and do not depend on trajectory persistence for success. If collection fails,
the request continues under the existing behavior while the evaluation result
is marked degraded.

Alternative: make every public request synchronously write a trace. Rejected
because it would add latency, coupling, retention pressure, and a new public
data-exposure path.

### 3. Evaluate information integrity as a hard independent gate

Each organization check produces action-success and integrity verdicts plus
bounded finding categories such as missing, altered, unexpected-duplicate,
misplaced, foreign-scope, hidden-lifecycle, stale-watermark, or nondeterministic.
Any hard integrity category makes the report non-pass even when ranking or
action metrics improve.

Alternative: fold integrity into a quality score. Rejected because a quality
gain cannot compensate for evidence loss or isolation failure.

### 4. Use append-only PostgreSQL artifacts with deterministic cleanup

Reports, trajectory aggregates, integrity findings, and retention outcomes are
append-only derived records keyed by compatible run/profile identities. Cleanup
marks or removes only expired derived artifacts according to an explicit
retention policy and never mutates canonical memory or source evidence.

Alternative: overwrite the latest report in place. Rejected because replay,
rollback, audit, and policy comparison require historical decisions.

### 5. Gate comparisons on compatibility and source freshness

The evaluator compares results only when fixture, policy, strategy, renderer,
provider capability, and source-watermark identities match. Missing or stale
inputs become stable skipped/degraded categories and cannot authorize rollout.

Alternative: compare across policy versions using best-effort normalization.
Rejected because it would make safety and quality deltas ambiguous.

## Risks / Trade-offs

- **[Risk]** Aggregation can hide a localized defect. → Keep hard categories,
  bounded per-scope evidence counts, and deterministic replay links; do not use
  aggregate quality to override integrity failures.
- **[Risk]** Evidence retention increases PostgreSQL storage. → Use bounded
  payloads, explicit retention ownership, cleanup telemetry, and no raw traces.
- **[Risk]** Collection failure could make evaluation appear healthy. → Mark
  missing collection as degraded/non-pass and expose the stable category.
- **[Risk]** Incompatible reports could be compared accidentally. → Require
  exact logical identity matching before producing deltas.

## Migration Plan

1. Add schemas, migrations, repositories, and redaction helpers with all
   collection disabled for ordinary public traffic.
2. Add offline fixtures and deterministic replay for trajectory and integrity
   categories; require exact scope and compatible identities.
3. Add authorized admin inspection and low-cardinality metrics.
4. Enable collection only for explicitly requested evaluation or shadow runs.
5. Verify retention cleanup and rollback; leave default retrieval, context,
   canonical memory, and reserved-insight activation unchanged.

Rollback disables collection and report comparison. Existing derived reports
   remain inspectable until retention expiry; canonical source records are not
   rewritten or deleted.

## Open Questions

None that change the contract or selected architecture. Exact bucket widths,
retention duration defaults, and endpoint naming can be selected during
implementation provided the redaction, compatibility, and scope guarantees
remain unchanged.
