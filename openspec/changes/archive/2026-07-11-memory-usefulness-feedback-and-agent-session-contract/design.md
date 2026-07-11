## Context

Stele now has a service-side loop for scope proof and memory sessions: a caller can create a session, request context, record turn outcomes, verify recall, and inspect reports. The remaining product gap is usefulness: Stele can prove that memory was recalled, but it cannot yet persist whether recalled memory helped, was noisy, was stale, missed expected evidence, or exposed a safety issue.

This design keeps Stele inside its service boundary. It adds feedback records, feedback summaries, stronger session turn attribution, and diagnostic bridges. It does not add SDKs, UI, model calls, prompt orchestration, or an external agent runtime.

## Goals / Non-Goals

**Goals:**

- Provide scoped APIs and repository contracts for recording usefulness feedback on memory hits, raw events, context citations, derived insights, sessions, session turns, verifications, and expected-recall misses.
- Preserve feedback as durable, append-only evidence with actor, reason, session, turn, context, and outcome attribution.
- Support append-only feedback correction or supersession without deleting the original feedback event.
- Strengthen memory session contracts with idempotent turn/outcome writes, bounded outcome payload ingestion through existing event ingestion, verification history, and report-level feedback summaries.
- Expose feedback-aware diagnostics for retrieval and context assembly without silently changing default ranking behavior.
- Bridge repeated negative feedback and recall misses into bounded quality findings and approval-gated repair recommendations.
- Add admin inspection, low-cardinality metrics, OpenAPI, and self-hosting documentation for the feedback loop.

**Non-Goals:**

- No SDK, UI, chat application, model invocation, prompt construction, answer generation, or agent framework integration.
- No direct canonical memory rewrite, derived insight rewrite, vector revision rewrite, or lifecycle mutation from public feedback.
- No automatic suppression, repair approval, or repair execution inline with feedback or session requests.
- No cross-scope aggregation or cross-tenant usefulness scoring.
- No public cross-subject usefulness summary reads except summaries embedded in the caller's own authorized session reports.
- No high-cardinality identifiers in metrics labels.
- No default feedback-aware reranking in the first slice; behavior changes must be explicit and opt-in.

## Decisions

### Store feedback as append-only scoped evidence

Feedback records are first-class durable records, scoped by tenant, project, and namespace. A record can target one or more bounded subject references: memory id, raw event id, context citation id, derived insight id, session id, turn id, verification id, or expected-recall target. Each record stores feedback type, optional severity, actor, reason, idempotency key, source surface, and metadata.

Alternative considered: store feedback counters directly on canonical memory rows. That would be simpler to read but would violate append-only audit expectations and make feedback provenance hard to inspect or correct.

### Use typed expected-recall targets

Expected-recall feedback distinguishes known scoped evidence from opaque caller expectations. Known targets use bounded target kinds such as event, memory, citation, insight, session, session turn, or verification with an identifier resolved inside the caller's scope. Opaque targets preserve caller-provided expectation tokens for later diagnostics but are not treated as event, memory, citation, insight, session, turn, or verification identifiers.

Alternative considered: store expected-recall misses as free-form text only. That would be flexible but would make recall diagnostics, quality findings, and repair recommendations harder to connect to governed evidence.

### Keep summaries derived and rebuildable

Usefulness summaries are derived from feedback records and can be recomputed. They expose aggregate counts, effective quality state, last feedback time, and bounded dominant categories. Summaries can be materialized for performance, but the feedback event log remains the source of truth.

Alternative considered: calculate summaries on every read only. That avoids summary drift but can make context/report reads expensive and makes diagnostics harder to paginate.

### Reuse existing ingestion for session outcome payloads

When a session outcome includes bounded event payloads, the service writes them through the existing event ingestion path and attaches session/turn attribution metadata. The session API does not mutate canonical memory directly and does not bypass admission, governance, provenance, or lifecycle rules.

Alternative considered: create outcome-specific memory rows directly from session results. That would be faster but would break the existing event -> candidate -> active lifecycle.

### Make feedback-aware ranking explicit and conservative

Default retrieval and context assembly continue to behave as before. Feedback signals appear in diagnostics and optional ranking hints. If an explicit request enables feedback-aware ranking for that request, ranking may downrank repeated noisy/stale subjects or highlight repeatedly useful subjects, while still respecting lifecycle and scope safety. This change does not introduce a scope-wide or global default ranking policy.

Alternative considered: immediately apply feedback to default ranking. That could improve relevance faster, but it risks surprising callers and requires broader ranking regression coverage.

### Allow feedback correction through append-only supersession

Feedback can be wrong, duplicated by clients, or later invalidated by operator review. A correction creates a new supersession record that marks the prior feedback as inactive for summaries while preserving the original record and attribution. "Active feedback" means a valid feedback record that has not been superseded and is eligible for summaries, diagnostics, and feedback-derived quality findings. "Superseded feedback" remains auditable and admin-inspectable but is excluded from default summaries and ranking hints.

Alternative considered: delete or update incorrect feedback in place. That would simplify summaries but would break auditability and make feedback-derived quality decisions harder to explain.

### Bridge feedback into quality and repair without automatic action

Repeated negative feedback, missing expected recall, and unsafe or hidden-memory feedback can create bounded quality findings or finding summaries. Repair plans can recommend manual review, suppression review, governance inspection, embedding retry, replay, or verification, but approval and execution remain under existing admin repair boundaries.

Alternative considered: allow public feedback to suppress memory automatically. That is too risky for a governed memory service and could let ordinary callers hide evidence without audit review.

### Preserve session verification and feedback history

Session verification requests create durable verification records. Session feedback records are linked to session, turn, context, outcome, and verification evidence. Session and turn summaries point to the latest verdict and active feedback summary, but reports preserve prior verification attempts, feedback history, outcome event ids, and expected recall history.

Alternative considered: overwrite turn verification state only. That keeps the model small but loses the ability to explain whether remediation improved recall over time.

## Risks / Trade-offs

- [Feedback can become noisy or adversarial] -> Require scope, attribution, idempotency, bounded feedback types, and admin inspection; do not let feedback directly mutate memory.
- [Ranking behavior can regress] -> Keep feedback-aware ranking opt-in and diagnostic-first in this change.
- [High-cardinality feedback evidence can leak into metrics] -> Store detailed identifiers only in scoped reports/evidence and test metrics for bounded labels.
- [Summary drift can occur] -> Treat feedback records as the source of truth and make summaries rebuildable.
- [Session API can drift toward agent runtime] -> Keep it limited to context, outcome ingestion, verification, feedback, and reports; explicitly exclude model calls and prompt behavior.
- [Large feedback volume can slow reads] -> Use summary aggregation workers or materialized summary records when synchronous reads become too expensive.
- [Public summary reads can leak aggregate behavior] -> Keep cross-subject feedback summaries admin-only; public callers see bounded summaries only through their own authorized session reports.

## Migration Plan

1. Add PostgreSQL schema for feedback records, feedback subject links, feedback supersession, expected-recall targets, feedback summaries, session verification history, and session idempotency fields.
2. Add domain types and repository methods with tenant/project/namespace filters.
3. Add service methods and scoped HTTP APIs for feedback creation, session outcome payload ingestion, verification history, and report reads.
4. Add feedback summary aggregation, active/superseded feedback handling, and feedback-derived quality finding generation.
5. Add context/retrieval diagnostics and opt-in feedback-aware ranking hints.
6. Add admin inspection, public session-report summaries, metrics, OpenAPI, docs, and tests.

Rollback is schema-compatible by disabling new routes and background aggregation. Existing ingestion, governance, retrieval, context assembly, quality repair, proof, and session flows continue independently.

## Open Questions

- Should the follow-up ranking rollout introduce a scope-level policy flag after per-request feedback-aware ranking has enough regression evidence?
- Should the follow-up SDK or MCP adapter expose a convenience wrapper for feedback subjects, while keeping this repository API-first?
