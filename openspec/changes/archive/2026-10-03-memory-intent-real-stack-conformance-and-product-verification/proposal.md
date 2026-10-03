## Why

The governed memory intent capability is now implemented and covered by focused Go and PostgreSQL repository tests, but the self-hosted product verification path does not yet exercise the complete intent lifecycle through the real API, durable queue, worker, scheduler, and operator inspection surfaces. A reproducible real-stack conformance run is needed before intent processing can be treated as operationally verified in a self-hosted deployment.

## What Changes

- Extend the owned product-verification workflow to submit `remember`, `update`, `forget`, `contradiction`, and `feedback` intents through the public API and configured MCP adapter where enabled.
- Verify exact tenant/project/namespace isolation, scoped idempotency replay, conflicting-key rejection, bounded validation failures, and redacted status/history inspection against a disposable PostgreSQL + pgvector stack.
- Exercise durable `memory_intent` queue claim, lease recovery, retry, retry exhaustion, worker restart, scheduler restart, and API drain/restart behavior without duplicating intent transitions or canonical versions.
- Verify remember/update/forget governance handoff, contradiction and feedback review-only behavior, policy disablement, pending-intent retention, compatible re-enable, and rollback evidence.
- Produce a bounded, redacted conformance report with stable run identity, scope-independent categories, prerequisite skip/fail states, replay results, recovery results, and rollback results suitable for operator and release review.
- Add a self-hosting command and documentation that use an explicitly supplied disposable DSN, owned fixture scopes, bounded timeouts, deterministic cleanup, and no claim of readiness when prerequisites or hard gates are missing.

## Non-goals

- Do not add new memory intent types, new canonical memory classes, or a second intent-processing policy.
- Do not change default retrieval, context assembly, lifecycle visibility, or contradiction activation behavior.
- Do not introduce a second queue, database, graph store, SDK, UI, hosted control plane, or provider-specific implementation.
- Do not expose payloads, prompts, claims, scope values, memory identifiers, credentials, raw provider/database errors, or raw ranking diagnostics in reports or telemetry.
- Do not automatically enable a policy or promote conformance evidence into global rollout; activation remains exact-scope, versioned, and separately governed.

## Capabilities

### New Capabilities

None. This change adds operational conformance and release evidence for existing governed intent and self-hosted runtime contracts.

### Modified Capabilities

- `self-hosted-product-delivery-verification`: require real-stack verification of governed intent submission, durable processing, restart/retry recovery, rollback, and redacted inspection.
- `runtime-memory-provider-contract`: require intent operations and their idempotency, scope, lifecycle, and replay guarantees in provider conformance coverage.
- `service-observability`: define bounded conformance, prerequisite, recovery, retry, rollback, and redaction categories without high-cardinality labels.

## Impact

- Affected areas include `scripts/stele-product-verify.ps1`, `internal/assurance`, `internal/app`, `internal/jobs`, `internal/storage/postgres`, MCP conformance fixtures, and self-hosting documentation.
- The verification run uses disposable PostgreSQL + pgvector resources and does not alter the runtime system of record outside its owned fixture scope.
- Existing unit, repository, and migration tests remain unchanged as baseline gates; this change adds an end-to-end evidence layer and operator-facing execution guidance.
- Related workflow references: `openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `verification-before-completion`, and `scripts/openspec-archive-seq.ps1`.
