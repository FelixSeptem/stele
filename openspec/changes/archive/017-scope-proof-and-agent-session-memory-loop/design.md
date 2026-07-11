## Context

Stele already exposes the service primitives needed for an agent memory product loop: scoped event ingestion, governed background processing, hybrid retrieval, context assembly, derived insight replay, quality evaluation, repair planning, and operator diagnostics. The remaining gap is orchestration and proof. Operators can manually follow a runbook, but the service does not persist a first-class answer to whether a scope is usable, and callers do not have a durable service-side contract for an agent-style memory session.

This design keeps the repository inside its service boundary. It adds proof/session records, worker-executed steps, report APIs, and evidence links. It does not add SDKs, UI, model invocation, prompt orchestration, or end-user agent behavior.

## Goals / Non-Goals

**Goals:**

- Provide a durable admin-only proof run that validates a tenant/project/namespace through ingest, governance, retrieval, context assembly, optional replay, quality evaluation, and repair recommendation.
- Provide a service-side memory session loop that lets external agents create a session, request context, record memory-relevant turn outcomes, and verify post-turn recall without Stele generating model output.
- Persist every run, step, verdict, fixture, attribution, and evidence link in PostgreSQL with strict scope filters.
- Execute proof/session verification asynchronously through existing worker or scheduler patterns.
- Produce operator reports that identify which existing admin surface or repair workflow should be used next.
- Export low-cardinality metrics and structured logs without leaking tenant, project, namespace, memory id, event id, session id, or proof id into labels.

**Non-Goals:**

- No SDK, UI, chat application, prompt runner, model call, or agent framework integration.
- No replacement of existing ingestion, retrieval, context assembly, quality repair, replay, embedding, or governance contracts.
- No automatic canonical memory rewrite or automatic repair approval.
- No cross-scope proof or session behavior.
- No load testing, backup/restore drill, SLO alert routing, billing, quota management, or tenant administration productization.

## Decisions

### Store proof and session orchestration as durable run/step records

Create `scope_proof_runs`, `scope_proof_steps`, `memory_session_runs`, `memory_session_turns`, and report/evidence metadata in PostgreSQL. Each record includes tenant, project, namespace, actor, reason, status, verdict, created/started/finished timestamps, and bounded failure reason codes.

Alternative considered: keep proof/session orchestration in docs and client scripts. That would avoid schema work but would not give operators auditable history, reruns, or a reliable API contract.

### Keep proof and session loops separate but linkable

Scope proof answers whether a scope is operationally usable. Memory session answers whether an external agent integration can use memory during a turn. A proof run can create an internal session fixture, and a session report can reference quality evaluations, but the domain records remain separate.

Alternative considered: one generic workflow table for everything. That is flexible but would hide important domain semantics and make OpenAPI/reporting harder to reason about.

### Model agent sessions as memory integration contracts, not agent runtime

The session API does not call a model or produce an assistant answer. It provides a bounded service contract:

- create session
- assemble context for a turn
- record external turn outcome as memory-relevant events
- wait for or inspect governed processing
- verify recall/context after the turn
- read a session report

Alternative considered: add full agent execution. That violates repo scope and would force SDK/UI/product choices into the memory service.

### Execute verification through worker/scheduler orchestration

Proof steps and asynchronous session verification use existing durable worker semantics: claim, lease, retry, failure summary, next eligibility, and idempotent completion. Admin requests create intent and inspect state; they do not synchronously run broad workflows.

Alternative considered: execute the entire proof inline in the HTTP handler. That would be simpler but brittle under worker latency, degraded dependencies, and retryable failures.

### Use existing subsystems for side effects

Proof/session execution calls existing ingestion, governance inspection, retrieval, context assembly, derived replay, quality evaluation, and repair planning APIs/services. It must not mutate canonical memory, vector revisions, derived insights, or provenance through special-purpose shortcuts.

Alternative considered: direct repository mutation for fixture setup and cleanup. That would be faster but would bypass the lifecycle/provenance rules this service is built to enforce.

### Reports contain detailed scoped evidence; metrics stay bounded

Reports can store event ids, memory ids, replay ids, evaluation ids, repair plan ids, and session ids as scoped durable evidence. Metrics only expose bounded labels such as step, status, verdict, component, and failure category.

Alternative considered: omit detailed evidence to simplify privacy concerns. That would reduce operator usefulness and force direct database access during failures.

## Risks / Trade-offs

- Broad scope can become a generic workflow engine -> Keep categories fixed to proof/session steps and call existing subsystem contracts.
- Session API can drift into agent runtime -> Explicitly exclude model calls, prompt construction, answer generation, and SDK behavior.
- Proof fixtures may pollute business memory -> Require explicit fixture metadata, actor/reason attribution, scoped isolation, and report-visible fixture ids.
- Asynchronous proof may be slower than a smoke script -> Prefer durable, retryable evidence over fast but non-auditable checks.
- Reruns may duplicate effects -> Use idempotency keys and new run ids linked to prior run templates; do not overwrite prior reports.
- Failure reports may expose hidden memory content -> Store identifiers and bounded summaries only; do not include hidden content outside authorized admin report paths.

## Migration Plan

1. Add schema and repository methods for proof runs, proof steps, session runs, session turns, reports, and evidence links.
2. Add domain services that create proof/session intent and reduce step results into verdicts.
3. Add admin proof APIs and scoped session APIs.
4. Add worker/scheduler execution for proof/session steps using existing lease and retry patterns.
5. Connect proof/session execution to event ingestion, governance status, retrieval, context assembly, derived replay, quality evaluation, and repair planning services.
6. Add metrics, structured logs, OpenAPI docs, and self-hosting runbook updates.

Rollback is schema-compatible by disabling new routes and worker dispatch. Existing ingestion, retrieval, context assembly, quality repair, replay, governance, and embedding flows continue independently.

## Open Questions

- Should memory session creation be a public scoped route (`/v1/memory-sessions`) or an admin-only diagnostic route at first? The preferred default is public scoped route with the same scope/auth rules as ingestion and context assembly.
- Should proof runs wait for governance completion or record a bounded pending/degraded verdict when worker latency exceeds the configured window? The preferred default is bounded waiting followed by `passed_degraded` or `failed` with diagnostics.
- Should proof fixture cleanup be explicit lifecycle suppression or safe retention with fixture metadata? The preferred default is safe retention plus optional future cleanup, because automatic cleanup could obscure audit history.
