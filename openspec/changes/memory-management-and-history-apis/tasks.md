## 1. Resource Contract And Query Model

- [x] 1.1 Define a stable canonical memory resource model for public list and detail reads
- [x] 1.2 Define lifecycle-safe visibility and redaction rules for hidden and deleted memory
- [x] 1.3 Define validated query inputs for scope, class, time window, and pagination
- [x] 1.4 Reserve the full route and schema framing for list, detail, history, provenance, and admin lifecycle actions

## 2. Public Memory Read APIs

- [x] 2.1 Add repository read methods for scoped canonical memory list and detail loading
- [x] 2.2 Add query service contracts that shape repository results into stable memory resources
- [x] 2.3 Implement `GET /v1/memories` with scope isolation and class or time filters
- [x] 2.4 Implement `GET /v1/memories/{memory_id}` with lifecycle-safe not-found behavior for hidden or out-of-scope memory
- [x] 2.5 Wire the public memory read surface into API mode without changing the existing auth boundary

## 3. History And Provenance

- [x] 3.1 Define an append-only memory history response contract with stable ordering
- [x] 3.2 Define a provenance lineage response contract across raw events, candidates, canonical memory, and lifecycle actions
- [x] 3.3 Persist and retrieve provenance audit fields including `request_id`, `actor`, and `source_context`
- [x] 3.4 Implement lifecycle-safe `GET /v1/memories/{memory_id}/history`
- [x] 3.5 Implement lifecycle-safe `GET /v1/memories/{memory_id}/provenance`
- [x] 3.6 Verify hidden or deleted memory remains behind explicit privileged rules without unsafe payload leakage

## 4. Manual Lifecycle Actions

- [x] 4.1 Define request contracts for `suppress`, `expire`, and `delete` including reason, actor, and request attribution
- [x] 4.2 Add a dedicated lifecycle action service that normalizes and validates manual actions
- [x] 4.3 Persist lifecycle action audit records and provenance links durably
- [x] 4.4 Implement privileged `POST /v1/admin/memories/{memory_id}:suppress`
- [x] 4.5 Implement privileged `POST /v1/admin/memories/{memory_id}:expire`
- [x] 4.6 Implement privileged `POST /v1/admin/memories/{memory_id}:delete`
- [x] 4.7 Ensure lifecycle actions remain idempotent and update retrieval visibility and projections consistently

## 5. Publication, Documentation, And Compatibility

- [x] 5.1 Publish OpenAPI paths and schemas for the full memory management surface
- [x] 5.2 Document the auth boundary between public read APIs and privileged lifecycle actions
- [x] 5.3 Update self-hosting docs with an end-to-end memory management flow
- [x] 5.4 Add a concise README note for direct canonical memory APIs
- [x] 5.5 Verify existing search and context assembly behavior remains lifecycle-safe and backward compatible
