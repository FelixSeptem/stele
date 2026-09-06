## 1. Chunk Domain Contract And Deterministic Policy

- [x] 1.1 Add failing unit tests for chunk identity, exact scope, source lineage,
  bounded content/ranges, lifecycle snapshots, policy/renderer versions, and
  deterministic ordinal validation.
- [x] 1.2 Implement versioned chunk domain types, source-kind and source-version
  references, class-aware policy configuration, bounded omission reasons, and
  validated rollout/shadow modes.
- [x] 1.3 Add failing tests for deterministic message, paragraph, sentence, list,
  code, and hard-bound segmentation across profile, episodic, procedural, summary,
  and relation source fixtures.
- [x] 1.4 Implement the boundary-first deterministic chunker with original-source
  ranges, stable ordinals, configurable character/token bounds, and deterministic
  handling of oversized atomic units.

## 2. Durable Storage And Rebuildability

- [x] 2.1 Add a forward PostgreSQL migration for versioned chunk derivations,
  bounded chunk items, exact-scope indexes, immutable source identity, policy and
  renderer versions, and idempotency constraints; add the matching development/test
  down migration only if repository migration conventions require it.
- [x] 2.2 Add PostgreSQL repository contracts and implementations for scoped chunk
  create/read, source and parent lookup, lifecycle-safe visibility, source watermark
  inspection, and append-only derivation history.
- [x] 2.3 Add repository tests for exact tenant/project/namespace/session/user
  filtering, same-source idempotency, foreign/hidden source rejection, bounded
  parent/adjacent lookup, and no hidden-content disclosure.
- [x] 2.4 Implement materialization and rebuild services that read only authorized
  PostgreSQL sources, preserve previous derived history, record deterministic source
  identities/watermarks, and fail closed on stale or unverifiable source state.
- [x] 2.5 Add explicit-DSN PostgreSQL integration tests for concurrent same-scope
  materialization, source-version succession, rebuild determinism, lifecycle
  transitions, and scope/session isolation.

## 3. Controlled Retrieval And Context Integration

- [x] 3.1 Add failing retrieval tests for default canonical fallback, exact-scope
  chunk rollout, shadow-only diagnostics, parent citation preservation, and rejection
  of hidden or foreign chunk candidates.
- [x] 3.2 Extend retrieval orchestration with versioned default-off/shadow/active
  chunk-candidate rollout handling while preserving existing lexical, semantic, and
  relation fusion behavior.
- [x] 3.3 Implement bounded validated parent and adjacent expansion with count and
  character/token budgets, lifecycle revalidation, stable citation shaping, and
  aggregate authorized diagnostics.
- [x] 3.4 Add failing context assembly tests for chunk-derived evidence, parent or
  adjacent budget exhaustion, unchanged public section names, and lifecycle-safe
  diagnostic redaction.
- [x] 3.5 Integrate eligible chunk evidence into existing context assembly sections
  behind the same rollout controls and budget accounting, without exposing internal
  chunk diagnostics on ordinary public responses.

## 4. Evaluation, Operations, Documentation, And Verification

- [x] 4.1 Extend retrieval-evaluation fixtures, replay, metrics, and report
  compatibility checks to label chunk-derived coverage, candidate counts, duplicate
  behavior, bounded diagnostics, and zero-tolerance scope/lifecycle failures.
- [x] 4.2 Add operator/admin or maintenance wiring for bounded materialization,
  inspection, rebuild, and rollout status; ensure job admission, leases, retries,
  and diagnostics remain scoped and redacted.
- [x] 4.3 Update retrieval-quality, context, and self-hosting documentation with
  source lineage, class policy, rollout/shadow use, rebuild, retention/deletion,
  rollback, and PostgreSQL prerequisite guidance.
- [x] 4.4 Add documentation contract tests and focused Go tests covering domain,
  storage, retrieval, context, evaluation, app/runtime wiring, and redaction.
- [x] 4.5 Run `go test ./... -count=1 -timeout 15m`, execute opt-in owned
  PostgreSQL + pgvector chunk integration/replay tests when prerequisites are
  available, compare redacted chunk-shadow results with `canonical-v1` /
  `baseline-v1`, and validate with `openspec validate
  hierarchical-memory-representation-bounded-chunking --strict`.
