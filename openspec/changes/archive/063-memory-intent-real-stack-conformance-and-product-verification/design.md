## Context

The preceding `governed-memory-intents` change added the intent envelope, PostgreSQL persistence, append-only transitions, durable `memory_intent` work, worker retry/recovery, OpenAPI/MCP mapping, and policy rollback. Focused tests and a repository-level PostgreSQL integration run cover those contracts, while `scripts/stele-product-verify.ps1` already owns a disposable self-hosted API/worker/scheduler stack for event ingest, restart, retrieval, context, and backup/restore verification.

This change connects those two layers. It adds an operator-facing real-stack evidence path without changing normal runtime behavior or creating a second conformance framework.

## Goals / Non-Goals

**Goals:**

- Extend the existing product-verification harness with a bounded memory-intent phase.
- Exercise API and optional MCP submission, PostgreSQL durable handoff, worker claim/retry/restart, scheduler coordination where applicable, and scoped status/history inspection.
- Use unique owned fixture scopes and idempotency identities so cleanup and replay are deterministic.
- Produce a stable, redacted report that can distinguish prerequisite skip, hard failure, degraded recovery, and consumable pass evidence.
- Verify policy disablement and compatible re-enable without enabling contradiction activation or changing default retrieval/context behavior.

**Non-Goals:**

- No new API routes, intent types, queue kinds, migrations, canonical memory classes, or runtime policy defaults.
- No replacement of existing unit, repository, MCP, or product-verification tests.
- No automatic rollout decision based only on this report; release activation remains an exact-scope policy decision.

## Decisions

### Reuse the existing product-verification harness

Add an intent phase to `scripts/stele-product-verify.ps1` and its existing compose-owned services instead of creating a second executable or test stack. This keeps startup, readiness, signal handling, backup/restore, ownership labels, and cleanup behavior in one place.

**Alternative considered:** a standalone intent conformance binary. This would isolate failures, but it would duplicate service startup and cleanup logic and could produce evidence against a stack configured differently from the documented product path.

### Verify through public boundaries first

Submit intents through the OpenAPI endpoint and, when MCP is enabled for the run, repeat the transport mapping through the MCP adapter. Inspect results through the authorized status/history surfaces. Repository queries remain limited to test setup, bounded assertions, and cleanup where public APIs cannot prove ownership safely.

**Alternative considered:** repository-only integration. It is faster but cannot prove authentication, scope header binding, API response mapping, MCP replay behavior, or runtime restart semantics.

### Keep the phase evidence-oriented and non-authoritative

The phase records lifecycle and recovery evidence; it does not activate a new policy, treat contradiction as authoritative, or compare retrieval quality. A passing report is consumable evidence for an operator or release review, not global enablement.

### Use a fixed phase and category vocabulary

Reports and telemetry use bounded phase names such as `submission`, `replay`, `scope_isolation`, `queue_recovery`, `worker_restart`, `rollback`, `inspection`, and `cleanup`, with fixed results such as `pass`, `skip`, `degraded`, and `fail`. Run identities, fixture scopes, request IDs, intent IDs, and raw diagnostics stay in local test artifacts only when needed for troubleshooting and are not emitted through ordinary metrics or logs.

### Validate restart and retry with durable identities

Each fixture uses a stable idempotency key and intent reference across API restart, worker lease loss, and retry. Assertions require one durable intent identity, ordered transitions, no duplicate effective canonical transition, and a bounded terminal or pending outcome consistent with the active policy.

## Risks / Trade-offs

- **[Risk]** A real-stack phase can be slow or unavailable on developer machines. → Make the DSN/container prerequisite explicit, use bounded timeouts, and report `skip` without claiming readiness when prerequisites are absent.
- **[Risk]** Failed processes may leave fixture rows or containers. → Generate unique owned labels and scopes, clean only resources created by the run, and retain bounded diagnostics on failure.
- **[Risk]** Restart timing can make queue assertions flaky. → Use polling with deadline categories, inspect durable status rather than sleep-only timing, and separate timeout from semantic failure in the report.
- **[Risk]** Conformance telemetry could leak sensitive identifiers. → Centralize report/metric shaping around fixed categories and add tests that reject scope, payload, IDs, DSNs, credentials, and raw errors.
- **[Risk]** A pass could be mistaken for activation authorization. → State in the report and documentation that evidence is scope-bound and activation remains separately governed and reversible.

## Migration Plan

No schema migration is required. Implement the verification phase, report schema, bounded telemetry, and documentation; run it against a disposable PostgreSQL + pgvector database and owned service containers. Rollback consists of omitting the new phase or disabling the optional conformance invocation; existing API, worker, scheduler, queue, and policy behavior remain unchanged.

## Open Questions

None. The first implementation can use the existing product-verification fixture scope and command-line DSN contract; any additional fixture cases can be added without changing the capability boundaries above.
