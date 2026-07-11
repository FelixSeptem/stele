## 1. Data Model And Contracts

- [x] 1.1 Add migrations for quality evaluation runs, evaluation findings, repair plans, repair actions, verification links, and admission pressure audit metadata.
- [x] 1.2 Add Go domain types for evaluation checks, bounded finding taxonomy, admission pressure decisions, repair action categories, and verification status.
- [x] 1.3 Add repository methods for creating, listing, updating, and inspecting evaluation runs, findings, repair plans, repair actions, and verification reports with tenant/project/namespace scope filters.
- [x] 1.4 Add unit tests for repository scope isolation, status transitions, idempotent inserts, and high-cardinality evidence storage.

## 2. Admission Pressure Evaluation

- [x] 2.1 Implement a reusable admission pressure evaluator for ingestion and repair work that returns `accept`, `accept_degraded`, `queue`, or `reject` with stable finding codes.
- [x] 2.2 Extend event ingestion to call the admission evaluator before writes and return admission metadata for accepted, degraded, queued, or rejected requests.
- [x] 2.3 Add tests for ingestion acceptance, degraded acceptance, queued processing metadata, rejection before write, and invalid scope rejection.
- [x] 2.4 Add tests proving admission findings use low-cardinality codes and do not leak raw memory, event, actor, or scope identifiers into metric labels.

## 3. Quality Evaluation Runs

- [x] 3.1 Implement admin APIs to create and inspect scoped quality evaluation runs.
- [x] 3.2 Implement retrieval/context evaluation checks for expected recall, lifecycle-safe exclusion, and degraded semantic projection findings.
- [x] 3.3 Implement ingestion and repair pressure checks as optional evaluation checks using the shared admission pressure evaluator.
- [x] 3.4 Add tests for authorized scope evaluation, unauthorized scope rejection, lifecycle-hidden memory exclusion findings, and semantic projection degradation findings.

## 4. Repair Planning

- [x] 4.1 Implement repair plan generation from evaluation findings with bounded action categories for embedding retry, governance requeue, derived insight replay, and manual review.
- [x] 4.2 Enforce repair planning limits for scope, action category, target count, and canonical memory rewrite prevention.
- [x] 4.3 Implement admin APIs to create dry-run repair plans, approve executable plans, and inspect proposed actions.
- [x] 4.4 Add tests for actionable finding mapping, unsupported finding manual-review fallback, canonical rewrite rejection, and out-of-scope report rejection.

## 5. Repair Execution And Verification

- [x] 5.1 Implement worker/scheduler repair action claiming with durable leases, retry state, exhaustion state, and idempotent completion.
- [x] 5.2 Wire repair actions into existing embedding rebuild retry, governance requeue, derived insight replay scheduling, and manual review recording paths.
- [x] 5.3 Implement post-repair verification runs linked to baseline evaluations and repair plans with before/after summary comparison.
- [x] 5.4 Add tests for lease-safe repair rejection, retryable repair failures, duplicate dispatch idempotency, successful verification, and residual finding verification failure.

## 6. Observability And Documentation

- [x] 6.1 Add metrics and structured logs for evaluation status, admission pressure decisions, repair action results, repair backlog, and verification outcomes.
- [x] 6.2 Add admin diagnostics that summarize recent evaluation health, dominant finding categories, admission pressure, repair progress, and residual verification issues for an authorized scope.
- [x] 6.3 Update OpenAPI documentation for new admin endpoints and event ingestion admission metadata.
- [x] 6.4 Update self-hosting docs with the operator runbook: evaluate quality, inspect pressure, generate repair plan, execute repair, and verify outcome.
- [x] 6.5 Run `openspec validate memory-quality-admission-repair-loop --strict`, targeted package tests, and `go test ./... -count=1`.
