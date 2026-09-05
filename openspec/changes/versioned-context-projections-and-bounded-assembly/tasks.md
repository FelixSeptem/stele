## 1. Projection Contract And Schema

- [ ] 1.1 Add failing unit tests for projection kinds, source references, scope validation, lifecycle visibility, bounded text, policy versions, and deterministic item ordering.
- [ ] 1.2 Implement versioned projection domain types and validation under the existing memory/retrieval conventions.
- [ ] 1.3 Add a forward PostgreSQL migration for projection headers/items, scoped indexes, source-version uniqueness, and append-only status transitions.
- [ ] 1.4 Add repository tests for create/read/rebuild, exact scope filtering, idempotency, and rejection of foreign or hidden source references.

## 2. Class-Aware Materialization And Rebuild

- [ ] 2.1 Add failing policy tests for profile always-visible gates, summary session eligibility, on-demand episodic/procedural/relation behavior, and archival raw-event evidence.
- [ ] 2.2 Implement one versioned projection policy resolver with stable omission reasons and lifecycle-safe filtering.
- [ ] 2.3 Add failing tests proving rebuilds preserve prior projection history, reuse source watermarks, and produce deterministic output for identical inputs.
- [ ] 2.4 Implement bounded materialization/rebuild services backed only by authorized PostgreSQL canonical versions and raw-event evidence.
- [ ] 2.5 Add PostgreSQL integration coverage for concurrent same-scope rebuilds, superseded source versions, and scope isolation.

## 3. Bounded Context Assembly Integration

- [ ] 3.1 Add failing retrieval/context tests for projection-backed always-visible/session sections, live retrieval fallback, deterministic ordering, and budget exhaustion.
- [ ] 3.2 Extend context assembly dependencies to read verified exact-scope projections without changing existing public section names.
- [ ] 3.3 Add failing redaction tests for projection citations and diagnostics covering hidden lifecycle, stale watermark, foreign scope, policy omission, and budget omission.
- [ ] 3.4 Implement bounded projection diagnostics and fail-closed assembly behavior.
- [ ] 3.5 Add API/session/proof regression coverage showing projection-backed context preserves existing citation, lifecycle, and scope contracts.

## 4. Runtime, Documentation, And Verification

- [ ] 4.1 Add explicit operator/admin or maintenance wiring for bounded projection materialization/rebuild while keeping projection consumption opt-in and rollback-safe.
- [ ] 4.2 Update self-hosting and context documentation with inspection, rebuild, omission diagnostics, source watermark, and rollback guidance.
- [ ] 4.3 Add documentation contract tests for projection terminology, exact-scope commands, no canonical mutation, and no destructive rollback.
- [ ] 4.4 Run focused memory/retrieval/storage/app/docs tests and opt-in PostgreSQL + pgvector projection integration tests.
- [ ] 4.5 Run `go test ./... -count=1 -timeout 15m` and `openspec validate versioned-context-projections-and-bounded-assembly --strict`; resolve all failures before archive.
