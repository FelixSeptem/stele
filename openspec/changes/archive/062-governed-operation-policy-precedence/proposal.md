## Why

Stele already protects memory intents, reserved insight activation, provider operations, and privileged lifecycle mutations with scope, lifecycle, authorization, policy, and idempotency checks. Those checks are currently specified and implemented in separate paths, so the same request can be rejected at different stages or expose different bounded outcomes as new adapters are added. A single precedence contract is needed now to make governance decisions predictable, fail closed, and conformance-testable across all governed operations.

## What Changes

- Introduce a versioned governed-operation precedence contract that evaluates, in order: exact scope, lifecycle visibility, principal grant, explicit approval or policy enablement, idempotency/replay, governance handoff, and canonical or derived mutation.
- Define a shared decision envelope and bounded denial categories for API, worker, scheduler, provider, MCP, intent, insight, ranking, and admin lifecycle paths. Decisions must preserve exact scope, policy/version compatibility, request identity, replay identity, and audit attribution without exposing payloads or hidden-record existence.
- Require every governed adapter to reuse the same precedence semantics and to fail before later-stage lookups or mutations when an earlier gate fails. Foreign-scope requests must not reach lifecycle, policy, idempotency, or governance state.
- Specify replay behavior for identical requests, conflicting idempotency reuse, stale or incompatible policy versions, disabled policies, and rollback/re-enable flows. Replays return the original bounded outcome; conflicts fail closed without a second transition.
- Add bounded conformance fixtures and diagnostics that prove ordering, exact tenant/project/namespace isolation, lifecycle-safe visibility, policy disablement, handoff safety, append-only history, and rollback behavior across representative operation surfaces.
- Keep PostgreSQL as the only system of record, reuse the existing durable work queue and policy stores, preserve append-only canonical and derived history, and leave default retrieval/context behavior unchanged.

### Non-goals

- No second policy store, queue, authorization service, SDK, UI, or hosted control plane.
- No new memory, insight, ranking, or provider feature solely as part of this contract.
- No direct canonical-memory writes from providers or models, and no bypass around scope, lifecycle, approval, provenance, or governance handoff.
- No change to ordinary retrieval/context visibility or to existing rollback and disablement meaning beyond making their evaluation order explicit.

## Capabilities

### New Capabilities

- `governed-operation-policy-precedence`: Defines the common evaluation order, decision envelope, bounded denial taxonomy, replay semantics, and cross-adapter conformance contract for governed operations.

### Modified Capabilities

- `governed-memory-intents`: Require intent submission and resumption to use the shared precedence contract before idempotency lookup, queue handoff, or lifecycle processing.
- `governed-reserved-insight-activation`: Require candidate admission, rollback, and replay to use the shared precedence contract and bounded decision categories.
- `agent-runtime-provider-adapter`: Require provider operations and provider errors to delegate precedence decisions without widening scope or bypassing approval and governance handoff.
- `manual-memory-lifecycle-actions`: Require privileged lifecycle actions to apply the shared ordering and stable replay/conflict semantics.
- `manual-mutation-governance-controls`: Require manual canonical mutations to apply precedence before concurrency checks and durable mutation, while retaining existing audit and projection guarantees.
- `service-observability`: Add low-cardinality precedence-stage, decision, replay, and denial diagnostics that remain redacted and exact-scope safe.

## Impact

- Affected code includes governed intent admission and workers, reserved insight activation, provider/MCP and admin handlers, manual lifecycle and canonical mutation services, policy/version validation, idempotency repositories, and conformance/diagnostic tooling.
- Public OpenAPI and adapter responses gain only bounded, machine-readable decision categories or compatible metadata needed to explain which governance stage stopped an operation; payloads, scope values, hidden identifiers, and raw errors remain excluded.
- Existing PostgreSQL schema, durable queue, audit/provenance history, rollback controls, and default retrieval/context behavior are retained and extended only where the shared contract requires durable decision evidence.
- Verification must cover unit and integration conformance for exact scope isolation, lifecycle filtering, grant and policy denial, replay/conflict handling, governance handoff, rollback/re-enable, redaction, and cross-adapter consistency.

## Related References

- `openspec/specs/governed-memory-intents/spec.md`
- `openspec/specs/governed-reserved-insight-activation/spec.md`
- `openspec/specs/agent-runtime-provider-adapter/spec.md`
- `openspec/specs/manual-memory-lifecycle-actions/spec.md`
- `openspec/specs/manual-mutation-governance-controls/spec.md`
- `openspec/specs/service-observability/spec.md`
- `openspec-propose`, `openspec validate --all --strict`
