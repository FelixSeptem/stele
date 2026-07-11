## Why

Derived experience insights can now surface failure patterns and lessons, but Stele does not yet have a durable way for operators to evaluate whether those insights are useful, noisy, wrong, stale, or worth preserving. Without a governed feedback loop, low-quality derived guidance can keep reappearing in admin inspection and context assembly even after an operator has diagnosed it.

This change adds an auditable quality feedback layer for derived insights so future derivation, suppression, ranking, and diagnostics can use explicit operator signals without rewriting canonical memory or deleting evidence history.

## What Changes

- Add durable insight quality feedback records scoped by tenant, project, and namespace.
- Support feedback types such as useful, noisy, incorrect, stale, redundant, and needs-review.
- Preserve feedback as append-only or superseding audit records rather than mutating the derived insight body in place.
- Extend admin inspection so operators can record, list, and supersede feedback for derived insights.
- Let insight derivation and scheduler jobs consume feedback signals when deciding whether to update, suppress, reactivate, or prioritize an insight.
- Let context assembly consider quality feedback when ranking optional `known_failures` and `experience_lessons` sections.
- Expose low-cardinality metrics and diagnostics for insight feedback coverage, noisy insight rates, and feedback-driven suppression.

## Non-goals

- No automatic LLM-based insight grading.
- No new autonomous insight types such as `hypothesis`, `goal`, `contradiction`, or `causal_link`.
- No direct deletion of derived insight evidence through feedback.
- No rewriting canonical memories, memory versions, vector revisions, or provenance records.
- No public end-user feedback API; feedback is admin-only in this change.
- No SDK, UI, hosted product, or MCP adapter work.

## Capabilities

### New Capabilities

- `insight-quality-feedback`: Durable, scoped, auditable quality feedback records for derived experience insights.

### Modified Capabilities

- `governed-experience-insights`: Derived insight lifecycle and derivation behavior must account for quality feedback without overwriting insight evidence.
- `admin-inspection-surface`: Admin inspection must support feedback write and read paths for derived insights.
- `context-assembly`: Optional insight context sections must rank or omit insights according to governed feedback signals.
- `worker-orchestration-and-maintenance-jobs`: Background derivation jobs must consume feedback idempotently and safely across retries.
- `service-observability`: Operational signals must include insight feedback quality and suppression metrics.

## Impact

- Affects derived insight domain models, repository contracts, PostgreSQL migrations, admin HTTP handlers, OpenAPI definitions, scheduler or derivation jobs, context assembly ranking, and metrics.
- Requires tests for scope isolation, append-only feedback history, admin authorization, feedback-driven suppression or ranking, and idempotent background consumption.
- Related commands and workflow references: `openspec validate governed-insight-quality-feedback --strict`, `openspec instructions apply --change governed-insight-quality-feedback --json`, `go test ./... -count=1`.
