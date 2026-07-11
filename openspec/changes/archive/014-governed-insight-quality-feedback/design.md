## Context

Stele now persists governed derived experience insights and can surface active `failure_pattern` and evidence-backed `lesson` records through admin inspection and optional context assembly sections. The current model preserves evidence and lifecycle transitions, but it does not yet capture operator judgment about insight quality.

The missing loop matters because derived guidance can become noisy even when it is technically evidence-backed. A repeated failure pattern may be too broad, stale after remediation, redundant with a better insight, or incorrect because the evidence was misclassified. Operators need a governed way to record those judgments so derivation, context packing, and diagnostics can respond without deleting evidence or rewriting canonical memory.

## Goals / Non-Goals

**Goals:**

- Add a durable, scoped quality feedback model for derived insights.
- Keep feedback admin-only, auditable, and isolated by tenant, project, and namespace.
- Preserve insight evidence and lifecycle history when feedback is recorded or superseded.
- Let background derivation jobs and context assembly consume summarized feedback signals.
- Expose operational diagnostics for feedback coverage and feedback-driven suppression.

**Non-Goals:**

- Do not add automatic LLM grading or autonomous quality judgment.
- Do not introduce new active insight types beyond the existing failure pattern and lesson flow.
- Do not expose public end-user feedback APIs.
- Do not rewrite canonical memories, memory versions, vector revisions, or provenance records.
- Do not delete evidence through feedback actions.

## Decisions

### Store feedback as separate governed records

Feedback will be persisted separately from the derived insight body. Each feedback record carries scope, insight identity, feedback type, actor, reason, optional quality score, created time, supersession state, and request or audit metadata.

Rationale: separate records preserve the derived insight's audit trail and allow multiple operator judgments over time. Mutating insight content directly would make it harder to distinguish model-derived evidence from human quality assessment.

Alternative considered: store only aggregate quality fields on the insight row. This is simpler for reads, but loses history and makes it harder to reverse or supersede a bad operator action. Aggregates can still be computed or cached from the feedback records if needed.

### Use bounded feedback vocabulary

The initial feedback vocabulary will be intentionally small: `useful`, `noisy`, `incorrect`, `stale`, `redundant`, and `needs_review`. Implementations may include an optional score or weight, but the typed signal is the contract.

Rationale: typed values are testable, indexable, and safe for scheduler logic. Free-form feedback alone would not support deterministic ranking or suppression.

Alternative considered: use arbitrary tags. Tags are flexible, but they are too weak for reliable background behavior and metrics.

### Keep write paths admin-only

Feedback creation and supersession will live under the existing admin boundary. Public retrieval and context assembly can consume summarized quality state, but cannot write feedback.

Rationale: this keeps quality governance as an operator concern and avoids adding end-user product semantics to the service.

Alternative considered: expose feedback from public APIs so agents can rate insights. That may be useful later, but it needs stronger actor semantics and abuse controls than this change requires.

### Derivation consumes feedback through summaries

Background derivation jobs should not re-read raw feedback in ad hoc ways throughout the codebase. Repositories should expose a scoped quality summary for an insight or fingerprint, including active counts by type and effective suppression or review signals.

Rationale: summary reads make idempotent derivation easier and avoid hard-coding policy across multiple callers.

Alternative considered: have each caller query feedback records directly. That increases coupling and makes future policy changes harder.

### Context assembly uses quality as ranking and visibility input

Optional insight sections should deprioritize noisy, stale, incorrect, or needs-review insights and prefer useful insights when budget is constrained. Strong negative feedback can suppress an insight from optional context only when a governed lifecycle transition or policy explicitly marks it hidden.

Rationale: ranking can respond immediately to feedback while lifecycle suppression remains auditable and explicit.

Alternative considered: automatically hide any insight with negative feedback. That is operationally risky because a single mistaken feedback record could silently remove useful guidance.

## Risks / Trade-offs

- Feedback policy becomes another ranking input -> keep the first implementation deterministic and test-covered with small typed signals.
- Operators may record conflicting feedback -> preserve all records and compute effective quality with recency and supersession rules.
- Negative feedback may not hide bad insights fast enough -> support explicit admin lifecycle suppression in addition to feedback.
- Metrics could leak high-cardinality details -> expose only low-cardinality labels such as feedback type, insight type, lifecycle state, and outcome.
- Feedback summaries may become stale if cached -> either compute from source records or refresh cache transactionally when feedback changes.

## Migration Plan

1. Add PostgreSQL tables and indexes for insight feedback records and supersession history.
2. Add domain types, validation, repository contracts, and repository tests.
3. Add admin OpenAPI operations and HTTP handlers for feedback create, list, detail, and supersede flows.
4. Update insight derivation and context assembly to consume quality summaries.
5. Add metrics and diagnostics.
6. Preserve rollback safety by leaving existing insight records unchanged; disabling feedback consumption should return behavior to the current insight-only path.

## Open Questions

- Whether to persist aggregate feedback summaries in the first implementation or compute them from feedback records on demand should be decided during implementation based on query complexity.
- Exact admin route names should follow the existing admin inspection route style when implementation begins.
