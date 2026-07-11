## 1. Data Model And Contracts

- [x] 1.1 Add migrations for usefulness feedback records, feedback subject links, feedback supersession records, expected-recall target records, usefulness summaries, session idempotency keys, outcome attribution, and session verification history.
- [x] 1.2 Add Go domain types for feedback type, subject kind, expected-recall target kind, opaque expected-recall token, source surface, active/superseded feedback state, effective usefulness quality, feedback summary, idempotency key, and verification history entry.
- [x] 1.3 Add repository methods for creating, deduplicating, superseding, listing, reading, and summarizing feedback with tenant/project/namespace filters.
- [x] 1.4 Add repository methods for session turn idempotency, outcome event attribution, bounded outcome payload persistence through existing ingestion, and verification history reads.
- [x] 1.5 Add repository methods or query helpers that compute summaries from active feedback only while preserving superseded feedback for admin inspection and audit.
- [x] 1.6 Add repository tests for scope isolation, feedback idempotency, append-only evidence preservation, supersession audit behavior, active-only summary rebuildability, expected-recall target resolution, opaque expected-recall token handling, and high-cardinality evidence storage.

## 2. Session Contract Hardening

- [x] 2.1 Extend memory session turn creation with optional idempotency keys and duplicate-safe turn creation semantics.
- [x] 2.2 Extend session outcome recording to accept existing outcome event ids and bounded event payloads routed through the existing event ingestion path.
- [x] 2.3 Persist session/turn attribution on outcome events without bypassing event validation, admission, governance, provenance, or lifecycle rules.
- [x] 2.4 Preserve multiple verification attempts per session or turn and expose latest verdict plus verification history in session reports.
- [x] 2.5 Add service and HTTP tests for duplicate turn/outcome requests, bounded outcome payload ingestion, invalid payload rejection, verification history preservation, and no model/prompt behavior.

## 3. Usefulness Feedback API

- [x] 3.1 Implement scoped public or service-side APIs to record feedback for memory hits, raw events, context citations, derived insights, sessions, session turns, verifications, and expected-recall misses.
- [x] 3.2 Validate feedback type, subject references, source surface, idempotency key, actor/reason attribution, metadata bounds, and resolved scope.
- [x] 3.3 Validate expected-recall feedback targets as bounded known evidence target kind/id pairs or opaque caller tokens that are not treated as internal identifiers.
- [x] 3.4 Implement append-only feedback correction or supersession APIs with actor/reason attribution, scope checks, and duplicate-safe behavior.
- [x] 3.5 Implement admin APIs to list feedback records, include superseded feedback when requested, read feedback detail, and read subject-level usefulness summaries.
- [x] 3.6 Enforce out-of-scope rejection without exposing target memory, insight, event, session, turn, verification, expected-recall target, or feedback content.
- [x] 3.7 Enforce public summary visibility so scoped callers can see bounded summaries only through their own authorized session reports while cross-subject summaries remain admin-only.
- [x] 3.8 Add API tests for feedback creation, duplicate idempotency behavior, invalid feedback type rejection, typed expected-recall validation, opaque expected-recall handling, feedback supersession, public/admin summary boundaries, admin inspection, and out-of-scope denial.

## 4. Summary Aggregation And Retrieval Diagnostics

- [x] 4.1 Implement usefulness summary aggregation from active feedback records with rebuildable source-of-truth semantics and deterministic exclusion of superseded feedback.
- [x] 4.2 Expose usefulness summaries in authorized session reports and admin memory, event, citation, insight, session, turn, verification, or expected-recall inspection where authorized.
- [x] 4.3 Add feedback-aware retrieval diagnostics for returned hits, missing expected recall, hidden-memory safety signals, and repeated noisy or stale subjects.
- [x] 4.4 Add feedback-aware context assembly diagnostics and per-request optional ranking hints while preserving default ranking behavior unless explicitly enabled for the individual request.
- [x] 4.5 Reject or ignore scope-wide/global feedback-aware ranking policy inputs in this proposal and document the boundary in service validation where applicable.
- [x] 4.6 Add tests proving default search/context ranking remains stable, per-request feedback-aware ranking is explicit, scope-wide ranking policy is rejected or ignored, diagnostics include bounded feedback signals, superseded feedback does not affect summaries or ranking hints, and hidden memory content is not exposed.

## 5. Quality, Repair, And Loop Closure

- [x] 5.1 Map repeated active noisy, stale, irrelevant, missing expected, unsafe, and needs-review feedback into bounded quality finding codes or finding summaries.
- [x] 5.2 Link feedback-derived findings to scoped quality evaluations and session reports without exposing hidden or out-of-scope evidence.
- [x] 5.3 Generate approval-gated repair recommendations from feedback findings for manual review, suppression review, embedding retry, governance inspection, or derived insight replay.
- [x] 5.4 Add post-feedback verification flow that can rerun session verification or scope proof after repair recommendation or operator remediation.
- [x] 5.5 Ensure superseded feedback is excluded from new quality finding generation unless an admin explicitly inspects historical evidence.
- [x] 5.6 Add tests proving feedback can drive quality/repair recommendations without auto-approving, auto-suppressing, executing repairs inline, using superseded feedback as active evidence, or leaking hidden evidence.

## 6. Observability, OpenAPI, And Docs

- [x] 6.1 Add low-cardinality metrics for feedback creation, deduplication, rejection, supersession, summary aggregation, feedback-derived findings, and session feedback outcomes.
- [x] 6.2 Add structured logs for feedback lifecycle transitions and summary aggregation outcomes without high-cardinality metric labels.
- [x] 6.3 Update OpenAPI documentation and OpenAPI tests for feedback endpoints, feedback supersession, expected-recall target shapes, strengthened session request/response shapes, verification history, summaries, and diagnostics.
- [x] 6.4 Update self-hosting docs to show the external-agent memory feedback loop from session context through outcome, verification, feedback, correction/supersession, quality finding, repair recommendation, and rerun.
- [x] 6.5 Document remaining product gaps after this proposal: SDK/UI collection surfaces, external agent runtime integration, default feedback-aware ranking rollout, task-success evaluation harnesses, and alert routing.

## 7. Verification

- [x] 7.1 Run targeted tests for feedback domain services, repositories, HTTP handlers, session services, retrieval/context diagnostics, quality bridge, telemetry, docs, and OpenAPI.
- [x] 7.2 Run `go test ./... -count=1`.
- [x] 7.3 Run `openspec validate memory-usefulness-feedback-and-agent-session-contract --strict`.
