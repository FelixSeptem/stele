## 1. Replay Data Model And Contracts

- [x] 1.1 Define derived insight replay request, mode, status, decision, skip reason, report, and counter domain types.
- [x] 1.2 Add PostgreSQL migration for replay run/report audit storage with scope, bounds, actor, reason, idempotency, status, counters, and failure summaries.
- [x] 1.3 Implement replay repository create, read, list, status update, report update, and idempotent lookup methods with tenant/project/namespace filtering.
- [x] 1.4 Add repository tests for scope isolation, idempotent replay identity, status transitions, report persistence, and unauthorized or out-of-scope reads.

## 2. Admin API And OpenAPI Surface

- [x] 2.1 Add OpenAPI schemas for replay dry-run requests, apply requests, replay run summaries, replay reports, decisions, counters, and validation errors.
- [x] 2.2 Add admin route for bounded replay dry-run planning that returns a plan without scheduling mutation work.
- [x] 2.3 Add admin route for bounded replay apply/backfill that records or returns a durable replay run identity.
- [x] 2.4 Add admin routes to list replay runs, read one replay run, and read the persisted replay report for an authorized scope.
- [x] 2.5 Add HTTP tests for auth boundary, missing bounds, scope mismatch, dry-run no-mutation behavior, apply enqueue behavior, and report reads.

## 3. Replay Planning And Execution

- [x] 3.1 Implement replay input validation for required scope, time window, evidence limit, insight type filters, actor, reason, and apply/dry-run mode.
- [x] 3.2 Implement replay dry-run planning over derived insight evaluator inputs with expected create, update, suppress, preserve, and skip decisions.
- [x] 3.3 Implement replay apply execution that reuses governed derived insight rules, feedback policy, lifecycle audit, and evidence preservation.
- [x] 3.4 Add idempotency protections so retrying the same replay run does not duplicate derived insights or lifecycle transitions.
- [x] 3.5 Add replay service tests for insufficient evidence, repeated failure evidence, unsupported reserved insight types, feedback-influenced decisions, and canonical memory non-mutation.

## 4. Worker And Scheduler Integration

- [x] 4.1 Register replay apply/backfill as durable background work using existing lease, retry, failure, and status conventions.
- [x] 4.2 Ensure admin apply requests enqueue replay work instead of executing broad mutations inline.
- [x] 4.3 Add bounded execution behavior for evidence limits, continuation-required status, and partial failure report updates.
- [x] 4.4 Add worker or scheduler tests for claim, retry, lease-safe execution, bounded processing, and failure report visibility.

## 5. Context Assembly And Observability

- [x] 5.1 Ensure replayed active insights can appear in optional `known_failures` and `experience_lessons` context sections only when scope, lifecycle, quality, and budget rules allow.
- [x] 5.2 Add authorized context diagnostics that explain whether replayed insight output was included, omitted by budget, omitted by quality, or hidden by lifecycle/scope rules.
- [x] 5.3 Add low-cardinality replay metrics for dry-run, apply, decision outcomes, skip reasons, failures, and bounded completion.
- [x] 5.4 Add tests for context visibility of replayed active insights, hidden replayed insights, and metric label cardinality safeguards.

## 6. Self-Hosting Smoke Loop And Documentation

- [x] 6.1 Add smoke fixture events and documented scope values that exercise ingest, worker processing, derived insight generation, replay dry-run, replay apply, search, and context assembly.
- [x] 6.2 Update self-hosting documentation with first-ten-minutes startup, readiness, ingest, worker, scheduler, replay, retrieval, admin inspection, metrics, and troubleshooting steps.
- [x] 6.3 Add or update smoke/check scripts where appropriate so operators can run the documented flow without reading implementation internals.
- [x] 6.4 Add docs or script tests that verify the smoke path references valid routes, required configuration, and expected diagnostics.

## 7. Verification

- [x] 7.1 Run focused tests for replay repository, admin HTTP routes, replay service, worker integration, context assembly, metrics, and docs checks.
- [x] 7.2 Run `go test ./... -count=1`.
- [x] 7.3 Run `openspec validate self-hosting-operational-replay-loop --strict`.
- [x] 7.4 Review `tasks.md`, `design.md`, and spec deltas for scope creep against non-goals before implementation begins.
