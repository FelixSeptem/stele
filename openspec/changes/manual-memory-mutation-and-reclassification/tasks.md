## 1. Mutation Contract And Guardrails

- [x] 1.1 Define the privileged route and schema surface for manual create, update, merge, and reclassify operations
- [x] 1.2 Define the bounded mutable fields for manual update and the excluded fields that must stay on other surfaces
- [x] 1.3 Define optimistic concurrency requirements for update, merge, and reclassification requests
- [x] 1.4 Define provenance operation names and audit attribution requirements for manual mutations

## 2. Manual Create And Update

- [x] 2.1 Add repository and service contracts for privileged manual canonical memory creation without synthetic raw events
- [x] 2.2 Implement append-only manual update semantics that preserve stable memory identity and version history
- [x] 2.3 Implement `POST /v1/admin/memories`
- [x] 2.4 Implement `PATCH /v1/admin/memories/{memory_id}`
- [x] 2.5 Ensure existing history and provenance APIs surface manual create and update operations correctly

## 3. Merge And Reclassification

- [x] 3.1 Define merge eligibility rules for same-scope and same-class canonical memories
- [x] 3.2 Implement `POST /v1/admin/memories/{memory_id}:merge` with target-version append and source suppression
- [x] 3.3 Define the bounded reclassification transition set and excluded classes
- [x] 3.4 Implement `POST /v1/admin/memories/{memory_id}:reclassify`
- [x] 3.5 Verify merge and reclassification preserve lifecycle-safe public read behavior and privileged auditability

## 4. Governance Controls And Retrieval Consistency

- [x] 4.1 Persist actor, reason, request attribution, and operation metadata for every manual mutation
- [x] 4.2 Update lexical search projections atomically when manual mutation changes canonical content
- [x] 4.3 Update or remove relation projections consistently when eligible manual mutations touch relation memories
- [x] 4.4 Clear or invalidate stale semantic embeddings on material content or class changes until a later reindex pipeline exists
- [x] 4.5 Verify search and context assembly remain lifecycle-safe after merge and reclassification

## 5. Publication And Documentation

- [x] 5.1 Publish OpenAPI paths and schemas for the manual mutation surface
- [x] 5.2 Document the governance boundary between raw event ingest and privileged canonical mutation
- [x] 5.3 Update self-hosting or operator docs with an end-to-end manual correction flow
- [x] 5.4 Add concise documentation for merge and reclassification semantics, including excluded cases
