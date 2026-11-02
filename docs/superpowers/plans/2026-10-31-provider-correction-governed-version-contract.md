# Provider Correction Governed Version Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Verify and retain a binding-owned Provider correction association through governed asynchronous update and append-only canonical successor promotion.

**Architecture:** Add an opt-in correction endpoint backed by an exact-scope PostgreSQL resolver. The endpoint only submits the existing governed update intent; the worker rechecks correction/reference/target eligibility before normal governance and version promotion. Existing generic Provider intents and projection admission remain unchanged.

**Tech Stack:** Go 1.25, net/http, OpenAPI embedded Go documents, PostgreSQL/pgx, Riverqueue v0.44.1, and existing governed-intent infrastructure.

---

### Task 1: Closed request and response contract

**Files:**
- Create: `internal/provider/correction.go`
- Create: `internal/provider/correction_test.go`
- Modify: `internal/provider/capability.go`
- Modify: `internal/app/provider_http.go`
- Modify: `internal/app/provider_http_test.go`
- Modify: `openapi/spec.go`
- Modify: `openapi/provider_correction.go`
- Test: `openapi/provider_context_test.go`

- [x] Define a request with canonical `metadata` and `input` containing only `contract_version`, `projection_id`, `target_memory_id`, `expected_version`, `content`, and `reason`.
- [x] Test valid parsing and rejection of null, duplicate, alias, unknown, trailing, oversized, zero UUID, invalid version, blank/oversized content, and blank/oversized reason.
- [x] Implement parsing by reusing the repository's `CanonicalObject` and metadata validation helpers.
- [x] Verify the parser tests fail before implementation and pass after implementation.
- [x] Advertise correction capability only when the correction resolver and policy are wired; add the route and OpenAPI response/error schemas.
- [x] Verify capability, strict body, route-gate, and OpenAPI document tests.

### Task 2: Verified association and intent attribution

**Files:**
- Modify: `internal/provider/operation.go`
- Modify: `internal/memory/governed_intent.go`
- Modify: `internal/memory/governed_intent_contract.go`
- Modify: `internal/memory/governed_intent_worker.go`
- Modify: `internal/storage/postgres/governed_memory_repository.go`
- Modify: `internal/app/app.go`
- Test: matching provider, memory, app, and PostgreSQL test files

- [x] Add a server-owned correction attribution value and include it in intent fingerprint/history without accepting it from generic evidence.
- [x] Add failing tests proving generic evidence cannot impersonate the verified association and that fingerprint/replay distinguishes correction source revisions.
- [x] Introduce resolver interfaces that carry the authenticated runtime binding and exact target/version, then implement them transactionally in PostgreSQL.
- [x] Verify source is a current later projection revision for the same full binding, with eligible current PC5 reference lineage matching active target/version; collapse foreign/missing/hidden/stale/mismatch outcomes.
- [x] Persist the immutable association atomically with intent/idempotency admission and test rollback/conflict/replay behavior.
- [x] Re-resolve before worker handoff; prove stale attribution appends a bounded nonactive transition without candidate/version creation.
- [x] Preserve association through candidate and promotion provenance; prove prior memory version remains intact and successor version is appended.

### Task 3: PostgreSQL migration and runtime wiring

**Files:**
- Create: `internal/storage/postgres/migrations/0042_provider_correction_intent_attribution.up.sql`
- Create: `internal/storage/postgres/migrations/0042_provider_correction_intent_attribution.down.sql`
- Modify: migration manifest/checksum tests and runtime dependency wiring
- Test: PostgreSQL repository and migration integration tests

- [x] Add an additive, scope-safe attribution schema with constraints preventing mutation of verified source identity.
- [x] Preserve attribution/history on rollback; the down migration disables/removes only optional indexes or uses the repository's non-destructive rollback convention.
- [x] Verify migration ordering, checksum, upgrade, constraints, and idempotent repository replay on a fresh PostgreSQL fixture.

### Task 4: End-to-end contract, docs, and release evidence

**Files:**
- Modify: `scripts/stele-provider-context-verify.ps1` and focused verifier fixtures
- Modify: `docs/provider-evidence-projections.md`, Provider integration docs, architecture/components/self-host/best-practices docs, `skills/stele-memory/SKILL.md`
- Modify: OpenSpec task checklist and roadmap only after acceptance passes

- [x] Add initial and restart fresh PostgreSQL + pgvector scenarios for success, denied attribution, replay/conflict, successor lineage, and superseded-signal exclusion.
- [x] Ensure evidence contains only bounded categories, counters, versions, migration provenance, and hashes; assert no credentials, payloads, IDs, paths, scopes, or ranking details.
- [x] Document capability negotiation, exact-binding authorization, errors, async lifecycle, replay, compatibility, and non-destructive rollback.
- [x] Run focused and full Go tests, race/vet as applicable, migration and owned fresh-stack verification, OpenSpec strict validation, documentation checks, and `git diff --check`.
- [x] Mark tasks complete and update roadmap item 10 only after all required evidence passes.
