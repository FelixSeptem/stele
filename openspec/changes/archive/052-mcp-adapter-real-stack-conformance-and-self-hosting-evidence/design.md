## Context

See `proposal.md` for the motivation and scope. The existing
`internal/storage/postgres/mcp_forget_preview_integration_test.go` already
owns disposable scoped fixtures, migration and pgvector checks, governed
fixture setup, a repository-recreation replay check, and one real Streamable
HTTP search delegation. The MCP package has focused contract tests for the
remaining policy branches. The design therefore extends the existing
boundaries instead of creating a second conformance framework or production
evidence store.

The test database is supplied explicitly through
`STELE_TEST_POSTGRES_MCP_DSN`. PostgreSQL remains the only durable source of
truth; test fixtures are unique per run and are removed by exact identifiers.

## Goals / Non-Goals

**Goals:**

- Exercise the governed MCP contract through the actual Streamable HTTP
  transport on PostgreSQL + pgvector.
- Make each policy category independently identifiable in test output and in a
  bounded, redacted evidence artifact.
- Cover both enabled and disabled adapter states, including an ordinary API
  availability probe when the self-hosting command is run against a Compose
  stack.
- Keep local runs deterministic, time-bounded, safe to retry, and safe to skip
  when no dedicated test database is configured.

**Non-Goals:**

- Do not introduce runtime conformance endpoints, a new database schema, or a
  second authorization implementation.
- Do not replace existing unit, SDK, repository, product-verification, or
  retrieval-integrity tests.
- Do not require a hosted MCP client, model provider, or external agent.

## Decisions

### Reuse the existing PostgreSQL integration boundary

Extend and, where useful, split the existing PostgreSQL MCP integration test
into table-driven scenario helpers. It already knows how to apply migrations,
verify pgvector, seed through ingest/admission/promotion/lifecycle boundaries,
and clean exact fixture IDs. Keeping those helpers in the storage package avoids
an import cycle (`storage/postgres` already depends on `mcp`) and prevents test
fixtures from bypassing governance.

Alternative considered: create a new standalone harness package that writes
SQL fixtures directly. Rejected because it would duplicate cleanup and make a
passing conformance result weaker than the production storage path.

### Use the MCP SDK client over the real Streamable HTTP handler

Each transport scenario starts the existing adapter with the real authorizer,
retrieval service, lifecycle service, and PostgreSQL repositories, then connects
with the MCP SDK Streamable HTTP client. The matrix calls the public tool
contracts and inspects only bounded structured responses and stable error
categories. Repository calls remain limited to fixture setup, durable replay
assertions, and cleanup.

Alternative considered: test only handler functions or repository methods.
Rejected because that would miss protocol negotiation, API-key propagation,
schema validation, response shaping, and transport-level disablement behavior.

### Represent evidence as redacted category results

The Go runner will collect category outcomes such as `scope_isolation`,
`lifecycle_filtering`, `forget_replay`, and `adapter_disabled` and may emit a
JSON artifact when a report path is requested by the wrapper script. The
artifact contains schema version, overall status, category status/counts,
duration, and skip/failure reason codes only. Fixture IDs, credentials, DSNs,
queries, memory content, raw protocol payloads, and SQL are excluded.

The default `go test` output remains sufficient for local use; the optional
artifact makes CI and release review machine-readable without adding a
production persistence path.

### Keep orchestration opt-in and explicit

Add a small self-hosting wrapper that requires
`STELE_TEST_POSTGRES_MCP_DSN`, invokes the focused test with a bounded timeout,
and preserves the existing skip semantics when the variable is absent. When a
Compose API base URL is supplied, the wrapper also checks health/OpenAPI
availability with MCP disabled and verifies the MCP endpoint fails closed. It
never discovers a database or credential implicitly and never prints the DSN.

The normal CI path remains unchanged because external PostgreSQL availability
is not guaranteed. CI or release jobs that provision a disposable pgvector
database can call the wrapper explicitly.

### Keep the roadmap and self-hosting contract documentation aligned

Document the exact prerequisites, command, category coverage, disabled-state
probe, cleanup guarantees, redaction policy, and interpretation of skipped
versus failed results beside the existing MCP self-hosting section. The
roadmap's current P8.6 references are corrected when this proposal is adopted,
but implementation does not change runtime roadmap behavior.

## Risks / Trade-offs

- **[Risk]** A shared developer database could be polluted by a failed test. ->
  Require an explicit dedicated DSN, generate unique scopes, use bounded
  cleanup, and document disposable database ownership.
- **[Risk]** A test may pass while bypassing canonical governance. -> Seed
  every memory through ingest, candidate admission, promotion, and lifecycle
  repositories, and assert versions/provenance in the fixture setup.
- **[Risk]** Reports could leak secrets or high-cardinality data. -> Emit
  allow-listed category fields only and add redaction assertions for DSNs,
  credentials, scopes, queries, scores, and fixture content.
- **[Risk]** Streamable HTTP or Docker availability can make local runs slow or
  flaky. -> Keep the test opt-in, use explicit two-minute context bounds,
  avoid network discovery, and classify missing prerequisites as skipped rather
  than passed.
- **[Trade-off]** The matrix duplicates a small amount of setup across
  transport scenarios. -> Prefer independent category isolation and clear
  failure attribution over a single opaque end-to-end test.

## Migration Plan

1. Add the delta spec and implementation tests without changing runtime
   defaults.
2. Run the focused test against a disposable PostgreSQL + pgvector database;
   verify the redacted artifact and exact cleanup.
3. Add the opt-in wrapper and self-hosting documentation, then run it against
   the repository's Docker Compose stack when Docker is available.
4. Keep existing unit and product-verification commands unchanged; release
   operators may add the new wrapper as an explicit evidence step.
5. Rollback is deleting or disabling the opt-in test/script invocation. No
   migration rollback is required because this change adds no production
   tables, routes, or configuration defaults.
