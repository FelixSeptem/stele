## 1. Data Model And Contracts

- [x] 1.1 Add migrations for scope proof runs, proof steps, proof report evidence, memory session runs, session turns, session verification records, and linked evidence metadata.
- [x] 1.2 Add Go domain types for proof status, proof step, session status, turn status, verdict, fixture mode, failure category, rerun link, and report evidence.
- [x] 1.3 Add repository methods for creating, claiming, updating, listing, and reading proof runs with tenant/project/namespace filters.
- [x] 1.4 Add repository methods for creating, updating, listing, and reading memory session runs, turns, verification records, and reports with tenant/project/namespace filters.
- [x] 1.5 Add repository tests for scope isolation, status transitions, idempotent rerun links, evidence persistence, and high-cardinality evidence storage.

## 2. Scope Proof Admin API

- [x] 2.1 Implement admin APIs to create, list, read, report, and rerun scoped proof runs.
- [x] 2.2 Validate actor, reason, fixture mode, requested checks, and resolved scope for proof requests.
- [x] 2.3 Return proof summaries with status, verdict, timestamps, step counts, and bounded failure categories.
- [x] 2.4 Return proof reports with step evidence links, linked quality evaluations, linked replay runs, linked repair recommendations, and next admin actions.
- [x] 2.5 Add API tests for authorized proof creation, out-of-scope rejection, invalid fixture input, proof report rendering, and rerun history preservation.

## 3. Memory Session API

- [x] 3.1 Implement scoped APIs to create memory sessions, start session turns, assemble turn context, record external turn outcomes, request verification, and read session reports.
- [x] 3.2 Keep session APIs service-side only: no model invocation, prompt orchestration, answer generation, SDK behavior, or UI assumptions.
- [x] 3.3 Persist context evidence summaries, citations, outcome event ids, verification expectations, and bounded failure categories for each turn.
- [x] 3.4 Enforce scope checks between sessions, turns, outcome events, context evidence, and verification records.
- [x] 3.5 Add API tests for session creation, context evidence capture, turn outcome ingestion, verification request, report rendering, and out-of-scope rejection.

## 4. Proof And Session Execution

- [x] 4.1 Implement durable proof step claiming with leases, retry state, failure summary, exhaustion or manual-review status, and idempotent completion.
- [x] 4.2 Implement proof steps for scope resolution, fixture planning, event ingestion, admission metadata capture, governance completion check, retrieval recall, context assembly, optional replay, quality evaluation, repair recommendation, and final verdict reduction.
- [x] 4.3 Implement asynchronous session verification worker execution with bounded wait, retry state, failure summary, and idempotent completion.
- [x] 4.4 Reuse existing ingestion, governance, retrieval, context assembly, replay, quality evaluation, and repair planning services instead of direct canonical memory or vector mutation.
- [x] 4.5 Add worker tests for successful proof execution, retryable proof step failure, duplicate dispatch idempotency, bounded wait degradation, and session verification failure.

## 5. Quality, Repair, And Diagnostics Bridge

- [x] 5.1 Link proof retrieval/context failures to scoped quality evaluations with bounded finding codes.
- [x] 5.2 Link session verification failures to scoped quality evaluations or finding summaries without exposing hidden memory content outside authorized reports.
- [x] 5.3 Generate repair plan recommendations from proof/session quality findings without auto-approving or executing repair actions.
- [x] 5.4 Map proof and session failures to stable next-action diagnostics that point to existing admin inspection, job status, replay report, context diagnostics, quality report, or repair plan surfaces.
- [x] 5.5 Add tests proving proof/session reports store detailed evidence durably while metrics and public diagnostics avoid high-cardinality labels.

## 6. Observability And Documentation

- [x] 6.1 Add low-cardinality metrics for proof runs, proof steps, session runs, session turns, verification results, verdicts, and failure categories.
- [x] 6.2 Add structured logs for proof/session lifecycle transitions and worker execution outcomes.
- [x] 6.3 Update OpenAPI documentation and OpenAPI tests for proof and memory session endpoints, report shapes, and attribution metadata.
- [x] 6.4 Update self-hosting docs to replace manual-only smoke checks with scope proof runs and to document the external-agent memory session integration loop.
- [x] 6.5 Document the remaining post-implementation product gaps: SDK/UI, external agent runtime integration, alert routing, capacity/load proof, backup/restore proof, and long-term memory usefulness scoring.

## 7. Verification

- [x] 7.1 Run targeted tests for proof/session domain services, repositories, HTTP handlers, workers, telemetry, docs, and OpenAPI.
- [x] 7.2 Run `go test ./... -count=1`.
- [x] 7.3 Run `openspec validate scope-proof-and-agent-session-memory-loop --strict`.
