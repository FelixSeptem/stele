# Memory Management And History APIs Roadmap

## Purpose

This roadmap decomposes the active OpenSpec change `memory-management-and-history-apis` into execution phases and concrete tasks. It is intended to keep proposal scope, OpenSpec tasks, and implementation order aligned before code work continues.

Scope remains intentionally narrow:

- canonical memory list and detail APIs
- lifecycle-safe history and provenance reads
- privileged manual lifecycle actions for `suppress`, `expire`, and `delete`
- OpenAPI and self-hosting documentation for the above

Scope remains intentionally excluded:

- manual canonical memory `create`, `update`, `merge`, or `reclassify`
- raw event mutation APIs
- embedding pipeline redesign
- dashboard or hosted control plane work

## Milestone View

### M1: Memory Resource Surface

Goal: expose canonical memory as a stable public resource rather than only as retrieval output.

Exit signal:

- canonical memory has a stable resource contract
- public list and detail reads are scope-safe and lifecycle-safe
- OpenAPI reflects the base resource model

### M2: History And Provenance

Goal: make memory evolution and evidence lineage inspectable through formal APIs.

Exit signal:

- history responses return stable append-only versions
- provenance responses expose durable lineage references
- provenance audit fields are persisted and retrievable

### M3: Manual Lifecycle Governance

Goal: expose existing lifecycle semantics as privileged, auditable management actions.

Exit signal:

- admin callers can invoke `suppress`, `expire`, and `delete`
- actor, reason, request, and timestamp attribution are preserved
- repeated requests converge safely without contradictory durable state

### M4: Publication And Compatibility

Goal: publish the surface cleanly without weakening existing retrieval or context safety defaults.

Exit signal:

- OpenAPI includes all new routes and schemas
- self-hosting docs show read versus admin boundary usage
- regression coverage confirms hidden memory is still excluded from normal retrieval paths

## Phase Breakdown

## Phase 1: Resource Contract And Query Model

### Task 1.1: Stable canonical memory resource view

- Define a public resource model for canonical memory with `id`, `scope`, `class`, `state`, `content`, `created_at`, and `updated_at`.
- Keep the model retrieval-independent by excluding score, rank, and other search-only fields.
- Reuse existing domain enums and scope validation instead of creating parallel type systems.

Outputs:

- stable memory resource contract

Done when:

- handlers and OpenAPI can depend on one memory resource model without ad hoc shaping

### Task 1.2: Lifecycle-safe visibility rules

- Define how public reads treat `suppressed`, `forgotten`, `expired`, and `deleted` state.
- Ensure deleted payloads are not re-exposed through detail or history reads.
- Keep hidden-state deep inspection explicitly outside the public read surface.

Outputs:

- documented visibility and redaction rules

Done when:

- the public read layer can shape visible and deleted memory consistently

### Task 1.3: Query inputs and response containers

- Define list filters for scope, class, time window, and bounded pagination.
- Define response envelopes for list, detail, history, and provenance.
- Validate query inputs early so HTTP handlers do not duplicate domain rules.

Outputs:

- query service contracts for memory reads

Done when:

- repository-backed services can accept validated inputs instead of raw URL state

### Task 1.4: Schema-first route framing

- Reserve the final route set in OpenAPI for list, detail, history, provenance, and admin lifecycle actions.
- Keep resource naming aligned with existing `/v1/memories` conventions.

Outputs:

- route and schema framing for the full change

Done when:

- implementation work can proceed without route churn

## Phase 2: Public Memory Read APIs

### Task 2.1: Repository read methods for list and detail

- Add PostgreSQL read methods for scoped canonical memory list and detail loading.
- Reuse existing lifecycle filtering defaults instead of inventing a second visibility path.
- Preserve exact scope isolation across `tenant`, `project`, and `namespace`.

Outputs:

- repository support for list and detail reads

Done when:

- services can load visible canonical memory without going through retrieval ranking code

### Task 2.2: Public `GET /v1/memories`

- Implement list reads with class filters, time filters, and bounded pagination.
- Return only visible canonical memory by default.
- Keep handler behavior explicit for invalid filters and empty result pages.

Outputs:

- public list endpoint

Done when:

- SDK callers can enumerate canonical memory resources directly

### Task 2.3: Public `GET /v1/memories/{memory_id}`

- Implement scoped detail lookup by stable memory identifier.
- Return lifecycle-safe not-found behavior for hidden or out-of-scope records.
- Keep response shape consistent with list items so detail is a strict extension, not a parallel model.

Outputs:

- public detail endpoint

Done when:

- SDK callers can load one canonical memory without using search results as an index

### Task 2.4: Runtime wiring and auth boundary reuse

- Wire the query service into API mode startup.
- Reuse existing scoped auth middleware rather than adding a new auth branch.
- Keep public read dependencies separate from admin lifecycle dependencies.

Outputs:

- stable runtime wiring for public memory reads

Done when:

- API mode exposes the new read routes without weakening existing auth or scope controls

## Phase 3: History And Provenance APIs

### Task 3.1: Append-only history contract

- Shape history responses around version chronology rather than current-state projection only.
- Preserve version timestamps and stable identifiers.
- Make deleted memory history inspectable without implying payload resurrection.

Outputs:

- formal history response model

Done when:

- callers can inspect memory evolution through an append-only contract

### Task 3.2: Provenance lineage and audit persistence

- Shape provenance responses around stable lineage references across raw events, candidates, canonical memory, and lifecycle actions.
- Persist and retrieve `request_id`, `actor`, and `source_context` for provenance rows.
- Fix any current repository gaps where provenance fields exist in schema but are not written durably.

Outputs:

- complete provenance persistence and read model

Done when:

- lifecycle and promotion lineage can be inspected with actor and reason attribution intact

### Task 3.3: Public history endpoint

- Implement `GET /v1/memories/{memory_id}/history` using the lifecycle-safe boundary for visible memory.
- Return stable ordering and consistent redaction rules for hidden or deleted content.

Outputs:

- public history endpoint

Done when:

- callers can inspect version evolution without admin-only diagnostics access

### Task 3.4: Public provenance endpoint

- Implement `GET /v1/memories/{memory_id}/provenance` for visible memories.
- Keep internal-only storage details out of the public contract.
- Make lifecycle actions visible as lineage events where applicable.

Outputs:

- public provenance endpoint

Done when:

- callers can inspect evidence lineage and governance actions for visible memory

## Phase 4: Manual Lifecycle Actions

### Task 4.1: Admin lifecycle request contract

- Define request and validation rules for `suppress`, `expire`, and `delete`.
- Require actor and reason attribution and carry request correlation if available.
- Keep manual lifecycle actions separate from any future manual content mutation proposal.

Outputs:

- lifecycle action input contract

Done when:

- admin handlers can reject incomplete governance actions before touching storage

### Task 4.2: Lifecycle action service

- Add a small service layer that normalizes manual lifecycle actions before they hit governance or storage.
- Preserve idempotent semantics so retries are operationally safe.

Outputs:

- lifecycle action application service

Done when:

- handlers can invoke one service contract for all three manual actions

### Task 4.3: Lifecycle audit and projection updates

- Persist lifecycle action provenance with action type, actor, reason, request, and timestamp.
- Ensure visibility changes propagate to canonical reads, retrieval, and context assembly defaults consistently.

Outputs:

- auditable lifecycle mutation path

Done when:

- suppress, expire, and delete create consistent durable state and audit history

### Task 4.4: Admin lifecycle endpoints

- Implement:
  - `POST /v1/admin/memories/{memory_id}:suppress`
  - `POST /v1/admin/memories/{memory_id}:expire`
  - `POST /v1/admin/memories/{memory_id}:delete`
- Reuse the existing admin boundary instead of introducing mixed public/admin mutation paths.

Outputs:

- privileged lifecycle management surface

Done when:

- operators can invoke all three actions through stable HTTP APIs

## Phase 5: Publication, Docs, And Regression

### Task 5.1: OpenAPI publication

- Add paths, schemas, and examples for list, detail, history, provenance, and lifecycle action routes.
- Keep naming aligned with the memory resource contract introduced in Phase 1.

Outputs:

- published API contract for the full change

Done when:

- generated or embedded OpenAPI output contains the full route set

### Task 5.2: Self-hosting and README updates

- Document the auth boundary between public memory reads and admin lifecycle actions.
- Add one operator flow that shows list, inspect history, inspect provenance, and perform an admin lifecycle action.
- Keep README changes minimal and point deeper operational detail to `docs/self-hosting.md`.

Outputs:

- operator-facing usage documentation

Done when:

- a self-hosting user can understand how to use the new surface without reading source code

### Task 5.3: Compatibility verification

- Re-run repository, app, OpenAPI, and full regression coverage for the affected packages.
- Confirm search and context assembly remain lifecycle-safe by default after the new surface lands.

Outputs:

- regression evidence for safe publication

Done when:

- the change ships without weakening existing scope or lifecycle protections

## Execution Order

Recommended build order:

1. Phase 1
2. Phase 2
3. Phase 3
4. Phase 4
5. Phase 5

Reasoning:

- public route design is safer once the resource and visibility contracts are fixed first
- provenance persistence needs to be corrected before public provenance and admin lifecycle audit can be trusted
- admin lifecycle publication should follow the stable read surface rather than precede it

## Review Gates

Before moving between phases, verify:

- Phase 1 to 2: query contracts and lifecycle-safe shaping are stable enough that handlers will not invent their own response logic
- Phase 2 to 3: public list and detail reads are scope-safe and do not leak hidden records
- Phase 3 to 4: provenance persistence includes actor, request, and source context for both promotion and lifecycle events
- Phase 4 to 5: lifecycle actions are idempotent and visibility changes propagate to read surfaces consistently

## Immediate Next Step

If implementation resumes next, start with Phase 1 Task 1.1 and 1.2 together, then complete the rest of Phase 1 before wiring any new HTTP routes.
