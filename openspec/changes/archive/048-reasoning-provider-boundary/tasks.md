## 1. Contract and provider boundary

- [x] 1.1 Define versioned reasoning capability and invocation envelopes with exact scope, operation/schema versions, deadlines, budgets, idempotency metadata, and redacted evidence; verify contract tests reject missing, widened, unsupported, or over-budget envelopes.
- [x] 1.2 Add provider registration and disabled-by-default capability discovery across `api`, `worker`, and `scheduler`; verify all modes expose the same bounded capability view and start without a configured reasoning provider.
- [x] 1.3 Validate candidate/intent output for scope, evidence citations, provenance, policy/version, confidence or uncertainty, size, and allowed insight types; verify direct canonical writes, lifecycle activation, grant changes, and reserved insight activation are rejected.

## 2. Replay, shadow, and failure governance

- [x] 2.1 Implement checksum-locked offline fixture replay with deterministic normalized inputs and redacted categorized outputs; verify repeated replay produces the same result without remote calls or canonical mutations.
- [x] 2.2 Add opt-in shadow execution and baseline comparison with bounded concurrency, deadline, retention, and disagreement diagnostics; verify shadow output cannot alter default retrieval, context, or canonical memory.
- [x] 2.3 Implement stable failure categories, bounded fallback, cancellation, timeout, stale-dependency, malformed-output, and idempotent retry handling; verify failures preserve the non-reasoning baseline and never broaden budgets or scope.

## 3. Conformance, documentation, and rollout safety

- [x] 3.1 Add scoped reasoning conformance profiles and append-only diagnostic evidence for capability compatibility, isolation, evidence completeness, replay determinism, fallback, and redaction; verify reports omit prompts, chain-of-thought, credentials, raw payloads, and foreign identifiers.
- [x] 3.2 Document operator configuration, disabled/offline/shadow modes, provider limits, rollback, and the boundary between reasoning candidates and governed insight activation; verify self-hosting and OpenAPI documentation remain consistent.
- [x] 3.3 Update the v1 roadmap to mark P8.2b archived and `reasoning-provider-boundary` as the P8.3 next proposal; verify the roadmap contract test and archive/status bookkeeping pass.
- [x] 3.4 Run focused reasoning tests, the full Go suite, race/vet checks as applicable, `openspec validate --all --strict`, and repository documentation checks; verify all required commands pass before enabling any live adapter.
