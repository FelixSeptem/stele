## 1. Temporal domain contracts and validation

- [ ] 1.1 Add table-driven tests for half-open interval validation, open-ended current intervals, temporal identity, class eligibility, overlap conflicts, and deterministic error categories; verify the new tests fail before implementation.
- [ ] 1.2 Add immutable temporal value types for `temporal_fact_id`, `ingested_at`, `valid_from`, `valid_to`, evaluation instant/interval, validity source, and correction disposition; verify repeated validation is deterministic and does not mutate inputs.
- [ ] 1.3 Add tests for current-vs-historical selection at boundaries (`valid_from` inclusive, `valid_to` exclusive), malformed selectors, and recorded-time filter separation; verify all boundary cases pass.
- [ ] 1.4 Define the source-class policy (`profile`, `episodic`, `procedural` factual; `summary`/`relation` derived-only) and verify unsupported classes fail closed with stable categories.

## 2. PostgreSQL schema and legacy compatibility

- [ ] 2.1 Add migration tests covering additive columns/tables, scope-safe indexes, idempotent reruns, and unchanged pre-existing schema behavior; verify migration tests fail before the migration exists.
- [ ] 2.2 Add the versioned PostgreSQL migration for temporal identity, recorded time, valid bounds, validity source, correction ledger, and source-version indexes without destructive rewrites; verify manifest and upgrade tests pass.
- [ ] 2.3 Implement the legacy current-compatible backfill from existing version creation timestamps, preserving scope, lifecycle, provenance, and existing derived rows; verify rerunning the backfill is a no-op.
- [ ] 2.4 Add database tests proving malformed intervals and implicit mutually-exclusive overlaps are rejected or recorded as explicit conflicts; verify conflicting rows cannot enter ordinary current retrieval.
- [ ] 2.5 Add migration/recovery documentation and a rollback rehearsal proving policy disablement leaves temporal history intact; verify the rehearsal emits bounded audit evidence.

## 3. Append-only temporal corrections and provenance

- [ ] 3.1 Add failing repository/domain tests for successor versions, retroactive corrections, actor/reason provenance, conflict dispositions, and stable predecessor/successor lineage.
- [ ] 3.2 Implement append-only temporal correction persistence and current-head advancement without mutating prior version payloads; verify correction history remains readable in stable order.
- [ ] 3.3 Extend canonical consolidation and manual mutation validation to require temporal identity/interval rules for factual classes; verify invalid corrections leave canonical and provenance state unchanged.
- [ ] 3.4 Extend history and privileged provenance responses with bounded temporal metadata and inferred-vs-explicit validity source; verify hidden history remains unavailable to ordinary reads.

## 4. Current and historical repository retrieval

- [ ] 4.1 Add repository tests for current-valid, expired, as-of, valid-during, boundary, legacy-compatible, and scope-isolated selection across canonical versions.
- [ ] 4.2 Implement shared lifecycle-plus-validity predicates and request evaluation-clock capture; verify current retrieval excludes expired versions before ranking and uses one instant per request.
- [ ] 4.3 Preserve existing `time_from`/`time_to` recorded-time semantics while adding explicit valid-time selectors; verify combined recorded/valid filters have deterministic intersection behavior.
- [ ] 4.4 Add repository fallback behavior for missing historical derived indexes; verify it reports bounded omission categories and never widens scope or reveals hidden versions.

## 5. Derived evidence and rebuild propagation

- [ ] 5.1 Add tests requiring relation projections, chunks, embeddings/rebuild items, citations, and context projections to retain source version, temporal fact identity, and validity snapshot.
- [ ] 5.2 Extend relation projection persistence and reads with source temporal identity; verify expired or conflicting sources are excluded under current and historical predicates.
- [ ] 5.3 Extend chunk materialization/rebuild and parent/adjacent lookup with validity snapshots; verify successor versions create append-only derived history and stale chunks are excluded.
- [ ] 5.4 Extend embedding/rebuild and citation mappings with version-aware lineage; verify a missing historical vector falls back without substituting another scope or version.
- [ ] 5.5 Extend context projection/rebuild provenance and freshness checks with temporal source identity; verify deterministic rebuild order and preservation of prior derived versions.

## 6. Retrieval planner and public API

- [ ] 6.1 Add planner tests for explicit current, `as_of`, and `valid_during` constraints, missing-selector fallback, deterministic plan identity, and hard-bound rejection.
- [ ] 6.2 Extend RQ1 retrieval plan contracts and executor inputs with validated temporal constraints while retaining the original query, exact scope, lifecycle rules, and request envelope; verify planner failures return the approved baseline.
- [ ] 6.3 Add public request/response types and OpenAPI schemas for additive valid-time selectors, selected-version metadata, and stable validation errors; verify ordinary response shape remains compatible.
- [ ] 6.4 Add HTTP tests for current default behavior, authorized historical queries, malformed/conflicting selectors, and unauthorized history; verify no hidden content or identifiers leak.

## 7. Evaluation, diagnostics, and release policy

- [ ] 7.1 Add versioned fixtures for current-valid, expired, as-of, interval, retroactive correction, legacy compatibility, stale-similarity, and temporal isolation cases; verify fixture validation rejects unsafe or ambiguous cases before seeding.
- [ ] 7.2 Extend replay/report metadata with temporal policy, evaluation instant/interval, selected-version coverage, stale/conflict dispositions, and bounded fallback categories; verify repeated replay is deterministic under a fixed clock.
- [ ] 7.3 Add hard safety metrics and tests for stale-fact wins, validity ambiguity, provenance mismatch, hidden-version leakage, foreign-scope evidence, and resource overflow; verify quality gains cannot offset failures.
- [ ] 7.4 Extend authorized diagnostics and redaction tests with low-cardinality temporal categories only; verify query text, scopes, identifiers, hidden content, raw scores, DSNs, and provider payloads are excluded.
- [ ] 7.5 Extend release policy and progressive-context gates for compatible owned PostgreSQL + pgvector temporal evidence, deterministic rebuild, fallback, rollback, and active-scope eligibility; verify missing DSN remains a stable non-pass.

## 8. Documentation and operational verification

- [ ] 8.1 Update canonical lifecycle, history/provenance, search, retrieval, API, and operator docs with recorded-vs-valid time semantics and legacy compatibility; verify docs consistency checks pass.
- [ ] 8.2 Document migration/backfill ownership, correction conflict handling, temporal rollout stages, retention/audit behavior, and rollback; verify the operator checklist is actionable without exposing sensitive data.
- [ ] 8.3 Run focused uncached temporal, memory, retrieval, storage, API, benchmark, and migration tests; verify all new scenarios pass.
- [ ] 8.4 Run full Go tests, vet, and race tests with an isolated worktree cache; record exact environmental limits and any owned-DSN non-pass.
- [ ] 8.5 Run strict/all OpenSpec validation, `git diff --check`, OpenAPI contract tests, migration manifest checks, and sensitive/status scans; verify the change is apply-ready and no RQ3/RQ4 scope leaked into the artifacts.
