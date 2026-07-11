## 1. Domain And Storage

- [x] 1.1 Add insight feedback domain types, bounded feedback enum validation, supersession state, and effective quality summary models.
- [x] 1.2 Add PostgreSQL migration for derived insight feedback records with scope, insight id, type, actor, reason, timestamps, supersession fields, and isolation indexes.
- [x] 1.3 Implement repository create, list, detail, supersede, and effective-summary methods with tenant, project, and namespace enforcement.
- [x] 1.4 Add repository tests for append-only feedback history, supersession behavior, unsupported feedback rejection, and out-of-scope access rejection.

## 2. Admin API Surface

- [x] 2.1 Extend OpenAPI with admin operations for creating, listing, reading, and superseding derived insight feedback.
- [x] 2.2 Implement admin HTTP handlers and request validation for insight feedback operations.
- [x] 2.3 Include effective feedback summary in derived insight admin detail responses.
- [x] 2.4 Add API tests for authorization, scope isolation, feedback validation, supersession, and hidden insight inspection.

## 3. Insight Governance Consumption

- [x] 3.1 Add feedback summary reads to the derived insight evaluator or maintenance path without changing foreground ingest behavior.
- [x] 3.2 Apply deterministic policy for useful, noisy, incorrect, stale, redundant, and needs-review signals during insight update, preservation, review, or suppression decisions.
- [x] 3.3 Record feedback-driven lifecycle transitions with policy attribution and without deleting evidence or feedback history.
- [x] 3.4 Add worker and evaluator tests for idempotent feedback consumption, conflicting feedback, retry safety, and feedback-driven suppression thresholds.

## 4. Context Assembly Integration

- [x] 4.1 Extend insight section ranking to consume effective feedback summaries when assembling optional `known_failures` and `experience_lessons` sections.
- [x] 4.2 Ensure hidden, suppressed, forgotten, deleted, or out-of-scope insights remain excluded even when they have positive feedback.
- [x] 4.3 Add optional admin or debug diagnostics for summarized quality state without exposing raw feedback in ordinary context assembly.
- [x] 4.4 Add context assembly tests for useful prioritization, noisy or stale deprioritization, budget-constrained omission, and lifecycle visibility precedence.

## 5. Observability And Diagnostics

- [x] 5.1 Add low-cardinality metrics for feedback create, supersede, operation result, feedback type, insight type, and feedback-driven lifecycle decisions.
- [x] 5.2 Add admin diagnostics for feedback coverage, noisy insight rate, needs-review count, and feedback-driven suppression count for an authorized scope.
- [x] 5.3 Add tests that metrics and diagnostics avoid high-cardinality identifiers such as tenant, project, namespace, actor, insight id, and reason text.

## 6. Verification And Documentation

- [x] 6.1 Update self-hosting or admin documentation for derived insight feedback operations and operational interpretation.
- [x] 6.2 Run `openspec validate governed-insight-quality-feedback --strict`.
- [x] 6.3 Run focused Go tests for memory, insights, jobs, retrieval or context assembly, app handlers, and OpenAPI changes.
- [x] 6.4 Run `go test ./... -count=1` before marking the proposal implementation complete.
