## 1. Adapter contract and configuration

- [x] 1.1 Add bounded OpenAI-compatible reasoning adapter request/response types and local validation behind `reasoning.Provider`; verify unit tests reject malformed, oversized, out-of-scope, reserved-type, mutation, unknown-evidence, and tool-bearing responses.
- [x] 1.2 Add disabled-by-default endpoint/model/credential/timeout configuration and shared runtime registration for `api`, `worker`, and `scheduler`; verify enabled remote configuration fails honestly when incomplete and all modes expose the same capability view.
- [x] 1.3 Implement one-request standard-library HTTP execution with caller deadline/cancellation, bounded request/response bodies, fixed server-owned structured-output contract, and no automatic retries; verify a local `httptest.Server` sees only normalized bounded input and no credential leakage.

## 2. Failure, fallback, and conformance safety

- [x] 2.1 Map HTTP status, transport, timeout, cancellation, decode, and validation failures to existing stable reasoning categories with bounded redacted diagnostics; verify response bodies, prompts, credentials, and authorization headers never appear in errors or logs.
- [x] 2.2 Integrate adapter results with offline, shadow, live, and baseline fallback execution without canonical, lifecycle, grant, retrieval, or context side effects; verify offline mode makes zero HTTP calls and shadow disagreement remains non-authoritative.
- [x] 2.3 Add deterministic local adapter conformance fixtures for success, scope isolation, malformed/oversized output, timeout/cancellation, rate limiting, and idempotent retry identity; verify reports contain only bounded counters, versions, and safe categories.

## 3. Public contract, documentation, and rollout

- [x] 3.1 Publish bounded adapter capability/readiness information through the existing authorized reasoning diagnostics and OpenAPI contract; verify route/schema coverage and exact-scope authentication tests pass without exposing secrets or raw payloads.
- [x] 3.2 Update self-hosting configuration, rollback guidance, and provider support documentation; verify documentation contract and smoke tests identify disabled, offline, shadow, and live behavior correctly.
- [x] 3.3 Update the v1 roadmap to mark P8.3 archived as change 048 and `reasoning-openai-compatible-adapter` as the active P8.4 next proposal; verify roadmap/status contract tests pass.
- [x] 3.4 Run focused adapter tests, the full Go suite, race/vet checks, OpenSpec strict validation, documentation checks, and diff checks; verify no live endpoint or credential is required for release verification.
