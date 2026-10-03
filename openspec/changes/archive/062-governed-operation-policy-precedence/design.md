## Context

The proposal addresses a cross-cutting behavior gap across the existing governed intent, reserved insight, provider, admin lifecycle, and manual mutation paths. Each path already persists scope, policy/version, provenance, and idempotency evidence in PostgreSQL, but the checks are distributed across handlers, services, adapters, and workers. The design must preserve those stores and the existing durable work queue. See `proposal.md` and the delta specs for the externally observable contract.

## Goals / Non-Goals

**Goals:**

- Make the precedence order executable and reviewable as one versioned decision contract.
- Reuse the current principal/grant resolution, lifecycle visibility checks, policy stores, idempotency repositories, governance handoffs, and append-only audit paths.
- Ensure every adapter can return the same bounded stage and outcome categories while retaining operation-specific details in authorized history.
- Make replay, rollback, re-enable, and conformance behavior deterministic across API, worker, scheduler, provider, MCP, insight, and admin paths.

**Non-Goals:**

- Introduce a new policy database, queue, authorization system, or public product surface.
- Move canonical memory or derived insight mutation into the precedence evaluator.
- Change default retrieval/context semantics, lifecycle meanings, or existing policy ownership boundaries.

## Decisions

### 1. Use a pure, versioned precedence decision contract

Define a small provider-neutral decision envelope containing operation kind, normalized exact scope proof, principal/grant result, lifecycle visibility result, policy/approval version, request and operation identity, idempotency fingerprint, and the final stage/outcome category. The evaluator returns a decision or a bounded denial and has no database writes or queue side effects. Callers persist the decision through their existing audit or operation record before invoking the existing handoff.

This keeps ordering testable and prevents a shared helper from becoming a second system of record. A new policy engine was considered, but it would duplicate policy ownership and make rollback semantics ambiguous. Copying checks into every adapter was rejected because it would preserve the current drift.

### 2. Resolve scope and lifecycle before any existence-sensitive lookup

The API authentication layer resolves the principal and exact grant first. The precedence evaluator then verifies the request scope and lifecycle visibility using the operation's authorized visibility mode. It must not query idempotency, provider state, policy detail, or target payload when an earlier gate fails. Services that cannot prove visibility return the same bounded lifecycle category used for hidden targets.

This ordering prevents foreign-scope probing and makes the contract safe for both public and admin surfaces. It also preserves the existing rule that hidden memories are excluded from ordinary retrieval and context.

### 3. Treat policy approval and compatibility as a gate, not a mutation

Intent, activation, ranking, lifecycle, and manual mutation paths continue to own their policy data and rollback state. The evaluator consumes a normalized policy decision: enabled/disabled, exact scope, compatible version, freshness, and approval requirement. It does not enable, disable, or rewrite policy records. A disabled or incompatible policy stops before idempotency and handoff, while a rollback record remains inspectable in the owning subsystem.

This reuses existing policy stores instead of adding a central registry. The trade-off is that each owner must provide a small adapter to normalize its current policy state; conformance fixtures will detect adapters that omit a gate.

### 4. Centralize replay classification while retaining operation-owned idempotency storage

The shared contract normalizes the fields that form an operation fingerprint and classifies `new`, `replayed`, or `conflict`. The existing PostgreSQL idempotency tables and unique indexes remain authoritative for durable claims. A replay returns the operation-owned original result; a conflict returns only the bounded category and never the original payload or target existence.

An in-memory cross-service cache was considered and rejected because it would not survive restart and would violate PostgreSQL-only system-of-record rules.

### 5. Keep handoff and mutation explicit

After a successful decision, the caller invokes the current durable queue, candidate governance, lifecycle, or derived-insight admission path. The decision envelope is carried as bounded metadata so workers can revalidate the same precedence version and exact scope after restart. The evaluator never marks canonical or derived records active and never substitutes for append-only history or provenance recording.

### 6. Expose fixed categories and conformance fixtures

Define a finite stage set (`scope`, `lifecycle`, `grant`, `approval`, `replay`, `handoff`, `mutation`) and outcome categories for denial, replay, conflict, retryable interruption, and dependency degradation. Metrics and non-admin logs contain only these fixed values and buckets. Authorized diagnostic responses may include counts, versions, and redacted in-scope references.

Fixtures run through existing public or adapter boundaries for one exact scope. They intentionally exercise foreign scope, hidden lifecycle, missing grant, disabled policy, identical replay, conflicting replay, rollback, worker recovery, and provider bypass attempts. Fixtures are diagnostic and must not activate data or change default retrieval/context behavior.

## Risks / Trade-offs

- **[Risk] Existing adapters perform side effects before invoking the evaluator.** → Add boundary tests and move the evaluator call to the first service boundary; reject any adapter that cannot prove the required stage order.
- **[Risk] Policy owners normalize different freshness or approval states.** → Define a small shared normalized policy result and require owner-specific adapters plus compatibility fixtures for disabled, expired, stale, and rolled-back states.
- **[Risk] Revalidating after queue restart changes an operation's result.** → Persist the precedence version and normalized fingerprint with the operation; classify a changed policy as a bounded stale/incompatible outcome and preserve the original history.
- **[Risk] Stage labels become high-cardinality or leak existence.** → Enforce fixed enums, redaction tests, and telemetry checks that reject scope values, identifiers, payloads, and raw errors.
- **[Risk] Broad cross-cutting rollout causes behavior drift.** → Introduce the contract in shadow/conformance mode first, compare decisions with current paths, then enable enforcement per adapter with rollback to the prior owner policy.

## Migration Plan

1. Define the versioned decision envelope, stage/outcome enums, normalization rules, and adapter interfaces without changing request behavior.
2. Add unit fixtures for ordering, redaction, replay/conflict, policy compatibility, and exact-scope isolation; run them against existing owner policies.
3. Integrate intent and reserved-insight paths first, persisting decision metadata alongside their existing append-only records.
4. Integrate provider/MCP and admin lifecycle/manual mutation boundaries, then enable enforcement after conformance passes for each adapter.
5. Add worker restart and rollback/re-enable fixtures using the existing durable queue and PostgreSQL integration environment.
6. Publish bounded observability and conformance summaries; retain the prior policy owner as the rollback target until all adapters use the shared version.

Rollback disables enforcement for the affected adapter at the approval/handoff boundary, leaves submitted records and decision history intact, and continues using the owner policy's existing rollback semantics. No destructive migration or canonical history rewrite is required.

## Open Questions

None. The remaining implementation choices are adapter-local and must preserve the contracts in the proposal and delta specs.
