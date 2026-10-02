## 1. Contracts and data model

- [x] 1.1 Define immutable experiment policy, level/strategy identities, baseline identity, source watermark, freshness, exact-scope, and deterministic replay fields; verify migration and serialization tests preserve append-only history
- [x] 1.2 Add derived report and fallback/rollback dispositions using existing PostgreSQL evidence storage and retention conventions; verify failed and timed-out runs are non-consumable and cleaned up
- [x] 1.3 Document the redaction allowlist and reject/bucket query, scope, identifier, DSN, credential, raw score, and provider payload fields; verify report fixtures contain no forbidden values

## 2. Progressive projections and hierarchical planning

- [x] 2.1 Implement L0, L1, and L2 level materialization over existing versioned projections with source watermark, renderer/policy identity, citations, and budget accounting; verify deterministic rebuild tests
- [x] 2.2 Implement parent-first shadow grouping over validated chunk lineage and source-version snapshots; verify exact tenant/project/namespace/session/temporal isolation and bounded child/adjacent expansion
- [x] 2.3 Add fail-closed freshness, lifecycle, lineage, candidate, token/character, depth, and latency checks; verify stale, hidden, foreign, missing-lineage, and over-budget fixtures stop expansion without broader lookup

## 3. Replay and release evidence

- [x] 3.1 Add baseline-first replay orchestration that evaluates progressive levels and parent-first plans under the same fixture, scope, clock, and budget; verify baseline retrieval and public response behavior remain unchanged
- [x] 3.2 Emit redacted machine-readable and human-readable comparison artifacts with quality, citation, duplicate, freshness, latency, replay, fallback, and rollback categories; verify repeated identical replays produce stable identities and ordering
- [x] 3.3 Integrate protected recall, integrity, isolation, freshness, duplicate, latency, deterministic replay, and rollback checks with the existing retrieval release gate; verify safety failures override quality gains and block activation
- [x] 3.4 Add owned PostgreSQL + pgvector integration coverage for progressive and parent-first shadow runs, including DSN ownership, cleanup, stale evidence, and report attestation; verify no service DSN fallback

## 4. Context and observability integration

- [x] 4.1 Constrain context assembly to authorized diagnostic/shadow envelopes while preserving section names, citations, budget, diversity, and lifecycle rules; verify ordinary requests never expose experimental output
- [x] 4.2 Add low-cardinality metrics and bounded logs for level, strategy, mode, freshness, fallback, budget, rollback, and eligibility outcomes; verify sensitive labels are rejected, redacted, or bucketed
- [x] 4.3 Add authorized aggregate diagnostics and retention cleanup for experiment artifacts; verify hidden/foreign content and identifiers remain absent from diagnostics and public responses

## 5. Documentation and verification

- [x] 5.1 Update retrieval, projection, chunking, context, operations, and roadmap documentation with shadow-only activation, rebuild, rollback, and retention runbooks; verify docs consistency checks pass
- [x] 5.2 Run focused unit/integration tests for projections, chunks, replay, isolation, lifecycle, context budgets, and redaction; verify all change scenarios are covered
- [x] 5.3 Run `go test ./... -count=1`, focused race tests, `go vet ./...`, `git diff --check`, and `openspec validate --all --strict`; verify the change is ready for implementation/archive review
