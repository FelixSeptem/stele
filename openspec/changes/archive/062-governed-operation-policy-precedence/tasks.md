## 1. Contract and normalization

- [x] 1.1 Define the versioned precedence decision envelope, fixed stage/outcome enums, normalized scope proof, policy result, and replay classification; verify focused unit tests cover every stage and reject unsupported versions.
- [x] 1.2 Define bounded redaction and compatibility rules for decision metadata and public errors; verify tests reject scope values, payloads, hidden identifiers, credentials, and raw errors from non-admin outputs.
- [x] 1.3 Add shared conformance fixtures for scope, lifecycle visibility, principal grant, policy approval, replay/conflict, handoff, and mutation ordering; verify fixtures fail at the first violated gate.

## 2. Persistence and worker handoff

- [x] 2.1 Extend existing PostgreSQL operation or audit records to retain precedence version, normalized fingerprint, stage, and bounded outcome without introducing a second policy store; verify migration tests preserve append-only history and exact-scope uniqueness.
- [x] 2.2 Carry the decision envelope through the existing durable work queue and worker recovery path; verify restart and lease-reclaim tests do not duplicate transitions and classify stale/incompatible decisions deterministically.
- [x] 2.3 Add rollback and compatible re-enable handling that stops at the approval gate and resumes only eligible exact-scope pending work; verify integration tests preserve prior history and do not affect other scopes.

## 3. Governed memory and insight paths

- [x] 3.1 Integrate precedence evaluation into governed memory intent submission and processing before idempotency lookup or queue handoff; verify intent tests cover foreign scope, hidden targets, disabled policy, replay, conflict, and asynchronous submission.
- [x] 3.2 Integrate precedence evaluation into reserved insight admission, replay, shadow, and rollback; verify tests keep provider output non-authoritative and prevent activation when evidence, policy, lifecycle, or compatibility gates fail.
- [x] 3.3 Preserve existing append-only provenance and lifecycle records while recording shared decision metadata; verify canonical memory and derived insight history remains inspectable after rejection, suppression, rollback, and replay.

## 4. Provider, MCP, and admin boundaries

- [x] 4.1 Route provider and MCP operations through resolved principal scope and the shared precedence contract; verify adapter conformance covers widened scope, lifecycle filtering, policy denial, idempotent replay, and direct-mutation bypass attempts.
- [x] 4.2 Apply the contract to manual lifecycle actions and canonical mutations before concurrency or durable writes; verify admin tests cover grant/approval denial, stale expected version, conflicting replay, and stable repeated outcomes.
- [x] 4.3 Ensure all public OpenAPI and adapter responses expose only bounded machine-readable stage/outcome categories and compatible metadata; verify contract tests contain no hidden identifiers or raw errors.

## 5. Observability and conformance delivery

- [x] 5.1 Add low-cardinality metrics, structured logs, and authorized diagnostics for precedence stage, policy state, replay/conflict, rollback, and conformance outcomes; verify telemetry tests reject scope values, payloads, identifiers, credentials, and reason text.
- [x] 5.2 Add end-to-end conformance runs for one exact scope across intent, insight, provider, lifecycle, and manual mutation adapters; verify passing runs are bounded and diagnostic only, with no default retrieval/context change.
- [x] 5.3 Document the precedence contract, failure categories, rollback behavior, and self-hosted verification commands in `docs/`; verify `openspec validate --all --strict`, documentation consistency checks, and `git diff --check` pass.

## 6. Verification and rollout

- [x] 6.1 Run focused unit and integration tests for every adapted subsystem using the existing PostgreSQL/pgvector environment; verify exact isolation, append-only history, replay safety, and lifecycle-safe retrieval behavior.
- [x] 6.2 Run product conformance and degraded-dependency scenarios before enabling enforcement per adapter; verify stale or incompatible evidence produces bounded incomplete/degraded results without readiness claims.
- [x] 6.3 Record rollout and rollback procedures for each adapter and verify a disabled enforcement flag leaves submitted records intact and restores the prior owner-policy behavior without destructive migration.
