## Why

The archived MCP adapter already has extensive unit and SDK-contract coverage,
and one PostgreSQL + pgvector integration test proves the forget ledger and a
basic Streamable HTTP search path. It does not yet provide one reproducible,
operator-facing real-stack conformance matrix for the complete adapter surface.
Closing that evidence gap now makes the optional MCP adapter trustworthy in a
self-hosted deployment without expanding MCP into a second storage or
authorization system.

## What Changes

- Add an opt-in real-stack MCP conformance matrix that runs against a disposable
  PostgreSQL + pgvector database and the actual Streamable HTTP adapter.
- Extend end-to-end coverage for capability discovery, `who_am_i`, search,
  context, browse, remember, forget preview, forget apply, retry/replay, and
  bounded/redacted responses.
- Exercise exact tenant/project/namespace isolation, explicit-scope versus
  runtime-binding precedence, read-only grants, lifecycle and temporal filters,
  exact `path` versus `path_prefix`, and mutation idempotency through the MCP
  transport rather than only through repository or unit boundaries.
- Verify process-restart/repository-recreation behavior, reviewed fixed-ID
  forgetting, conflicting idempotency reuse, adapter disablement, and the
  unchanged behavior of ordinary OpenAPI routes and PostgreSQL state.
- Provide a self-hosting-friendly command or script for running the matrix with
  an explicitly supplied test DSN, bounded timeouts, isolated fixture scopes,
  exact cleanup, and redacted pass/fail evidence suitable for CI or release
  review.
- Update self-hosting and assurance documentation with prerequisites, the
  conformance command, covered evidence categories, skip behavior when no DSN
  is supplied, and interpretation of failures.

### Non-goals

- No new MCP tools, storage tables, persistence system, or authorization
  boundary.
- No change to the canonical memory model, lifecycle semantics, retrieval
  defaults, or OpenAPI request/response contracts.
- No SDK, UI, external-agent execution, or product-specific integration.
- No automatic enabling of MCP; the adapter remains optional and default-off.
- No requirement for an external hosted service or a non-PostgreSQL system of
  record.

## Capabilities

### New Capabilities

- None. This change strengthens verification and self-hosting evidence for an
  existing capability rather than introducing a new runtime capability.

### Modified Capabilities

- `mcp-scope-profile-context`: require independently reproducible real-stack
  conformance evidence for the existing MCP scope, lifecycle, mutation,
  replay, path, response-redaction, and disablement contracts.

## Impact

- Tests: `internal/mcp`, `internal/storage/postgres`, and the Streamable HTTP
  adapter integration boundary.
- Tooling and fixtures: a bounded opt-in conformance runner using
  `STELE_TEST_POSTGRES_MCP_DSN`, disposable scoped fixtures, and redacted
  evidence output.
- Documentation: `docs/self-hosting.md` and related assurance/release evidence
  guidance.
- Runtime behavior: none when the conformance runner is not invoked; ordinary
  API, worker, scheduler, PostgreSQL, and MCP-disabled behavior remain
  unchanged.

Related workflow: `openspec-propose` -> `openspec-apply-change` ->
`openspec-archive-change`.
