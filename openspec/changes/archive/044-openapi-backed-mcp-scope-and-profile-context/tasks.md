## 1. Adapter boundary and configuration

- [x] 1.1 Review maintained Go MCP SDK/transport options and record the selected adapter dependency and compatibility envelope in design notes; verify the choice does not add a second auth or storage boundary.
- [x] 1.2 Add disabled-by-default MCP configuration, bounded limits, and startup validation; verify `go test ./internal/config ./internal/app -count=1` proves absent, invalid, and enabled configurations fail closed or expose only the intended API-mode adapter.
- [x] 1.3 Define MCP request/response schemas, tool annotations, machine-readable error categories, and normalized dispatch context; verify schema tests reject unbounded queries, payloads, budgets, IDs, and unknown operations.

## 2. Server-resolved identity and scope

- [x] 2.1 Reuse principal authentication, exact tenant/project/namespace grants, and provider runtime/session bindings in the MCP request boundary; verify unit tests reject missing, foreign, malformed, or widened scope before service/repository access.
- [x] 2.2 Implement explicit-scope precedence over server-owned active scope with no client-controlled grant expansion; verify tests cover explicit authorized scope, active-scope fallback, missing active binding, revoked grant, and read-only grant behavior.
- [x] 2.3 Implement bounded `who_am_i` output for principal role, access mode, request-verified scope summary, and active-scope status without credentials, false grant counts, or tenancy leakage; verify redaction and unauthorized-scope tests.

## 3. Read and context tools

- [x] 3.1 Implement query-relevant memory search delegation using the existing search contract, including bounded query/filters, temporal selectors, lifecycle defaults, and citations; verify `go test ./internal/retrieval ./internal/app -count=1` covers current and historical searches and foreign/hidden exclusion.
- [x] 3.2 Implement assembled context/profile delegation using existing context assembly and versioned projections; verify fixed-budget tests preserve section names, projection freshness, temporal validity, citations, and baseline fallback.
- [x] 3.3 Implement lifecycle-safe memory browsing with bounded pagination and existing public resource filters; verify hidden, expired, deleted, and out-of-scope records never appear in ordinary MCP results.
- [x] 3.4 Keep search, profile/context, and browse result shapes separate and redact raw scores, candidate pools, feedback, calibration, query/scope values, hidden IDs, and trajectories; verify response-schema and golden redaction tests.

## 4. Governed mutations and forgetting

- [x] 4.1 Map remember/create and update operations to governed memory intents with actor, scope, provenance, version/concurrency, and idempotency metadata; verify no direct canonical write path is reachable from an MCP tool.
- [x] 4.2 Map suppress, expire, and delete requests to the existing privileged lifecycle surface; verify authorization, audit attribution, duplicate retry, and lifecycle-safe default-read tests.
- [x] 4.3 Implement semantic forget preview with exact-scope candidate bounds and a preview identity; verify dry-run tests produce no mutation and exclude foreign/hidden candidates.
- [x] 4.4 Implement preview apply using only the caller-reviewed, validated, bounded memory-ID set; verify drift, duplicate, stale, foreign, and oversized ID sets fail closed without rerunning unconstrained semantic deletion.

## 5. Replay safety, errors, and conformance

- [x] 5.1 Add bounded request/idempotency handling and stable MCP error mapping for auth, scope, validation, lifecycle, compatibility, dependency, and retryable failures; verify equivalent retries return the original outcome and conflicting reuse returns a bounded conflict.
- [x] 5.2 Add MCP contract tests against the published OpenAPI document and service interfaces; verify tool delegation cannot diverge from exact scope, lifecycle, temporal, citation, or budget behavior.
- [x] 5.3 Add owned PostgreSQL + pgvector conformance fixtures for scope isolation, read-only grants, active/explicit scope precedence, search/context safety, governed writes, preview-bound forgetting, idempotency, and citations; verify reports are bounded and redacted.
- [x] 5.4 Add adapter disablement and rollback tests; verify MCP becomes unavailable while ordinary OpenAPI/provider behavior and PostgreSQL canonical state remain unchanged.

## 6. Documentation and release evidence

- [x] 6.1 Document MCP enablement, endpoint/transport configuration, tool contracts, scope resolution, safe forgetting, redaction, and disablement in self-hosting and API contract docs; verify links and configuration names with `pwsh -File scripts/check-self-hosting-smoke-docs.ps1`.
- [x] 6.2 Publish the MCP adapter contract through the authoritative OpenAPI/runtime compatibility documentation without exposing secrets or internal diagnostics; verify `go test ./openapi ./internal/app -count=1` and route coverage.
- [x] 6.3 Run focused adapter tests, `go test ./... -count=1 -timeout 15m`, `go test -race ./... -timeout 20m`, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`; record outcomes and any controlled non-pass before requesting integration review.

Verification evidence (2026-09-26): focused `go test ./internal/mcp ./internal/auth ./internal/storage/postgres ./internal/app ./openapi -count=1`, full `go test ./... -count=1 -timeout 15m`, and uncached `go test -race ./... -count=1 -timeout 20m` passed. `go vet ./...`, `pwsh -File scripts/check-self-hosting-smoke-docs.ps1`, `openspec validate --all --strict` (68 passed, 0 failed; existing long-requirement informational notices only), and `git diff --check` passed. The real PostgreSQL + pgvector MCP conformance test also passed against disposable `docker.1ms.run/pgvector/pgvector:0.8.6-pg18` (PostgreSQL 18.6, pgvector 0.8.6) using `STELE_TEST_POSTGRES_MCP_DSN`; the test exercised migrations, durable preview/replay across repositories, exact-scope isolation, governed fixture setup, Streamable HTTP MCP search, and redacted response assertions. The explicitly named container `stele-mcp-conformance-20260926` was removed after the run. The 2026-09-26 integration review findings were addressed with bounded context budget mapping, semantic forget threshold validation/filtering, browse offset bounds, runtime-configured OpenAPI MCP path, durable `read_only`/`read_write` scope grants, and release of provider lifecycle claims after mutation/completion failures.
