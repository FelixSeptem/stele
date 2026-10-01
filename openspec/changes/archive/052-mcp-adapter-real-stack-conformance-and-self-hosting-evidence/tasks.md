## 1. Real-stack harness foundation

- [x] 1.1 Refactor the existing PostgreSQL MCP integration setup into reusable, exact-scope fixture and cleanup helpers without changing its current coverage; verify `go test ./internal/storage/postgres -run '^TestMCPForgetLedgerPostgresPersistsReviewAndReplayAcrossRepositories$' -count=1` still skips cleanly without a DSN and passes with a disposable DSN.
- [x] 1.2 Add a table-driven Streamable HTTP conformance runner with bounded context, tool invocation, stable category names, and optional redacted JSON evidence output; verify unit tests reject credentials, DSNs, scopes, queries, raw payloads, and fixture content in emitted evidence.
- [x] 1.3 Add prerequisite and cleanup checks for migrations, pgvector extension, unique fixture scopes, and exact owned-record deletion; verify a forced test failure leaves no fixture records in a disposable database.

## 2. MCP contract matrix

- [x] 2.1 Exercise capability discovery, `who_am_i`, explicit authorized scope, runtime-binding fallback, explicit-scope precedence, and read-only grant denial over the MCP transport; verify authorized and denied cases produce bounded category results without foreign-scope existence leaks.
- [x] 2.2 Exercise search, context, browse, temporal selectors, lifecycle visibility, exact `path`, `path_prefix`, budgets, citations, and response redaction over PostgreSQL-backed fixtures; verify visible in-scope evidence is returned while hidden, foreign, and internal diagnostic fields are absent.
- [x] 2.3 Exercise `memory_remember` idempotency, `memory_forget_preview`, reviewed fixed-ID `memory_forget_apply`, repository/process recreation, equivalent replay, conflicting idempotency reuse, and out-of-scope IDs; verify canonical state and durable ledger outcomes match the existing governed contracts.
- [x] 2.4 Exercise MCP disablement and ordinary API non-regression in the self-hosting path; verify the disabled endpoint fails closed while health/OpenAPI probes and PostgreSQL state remain available and unchanged.

## 3. Self-hosting evidence and documentation

- [x] 3.1 Add an opt-in PowerShell conformance wrapper that requires `STELE_TEST_POSTGRES_MCP_DSN`, applies a bounded test timeout, supports an optional API base URL for disabled-state probes, and never prints secrets; verify missing prerequisites produce an explicit skip and configured runs return a nonzero status on category failure.
- [x] 3.2 Document Docker/PostgreSQL + pgvector prerequisites, the wrapper command, covered categories, cleanup ownership, redaction guarantees, and skip-versus-fail interpretation in `docs/self-hosting.md`; verify the documented command and environment variable names match the script and test selector.
- [x] 3.3 Add release/CI guidance that keeps the conformance run opt-in and does not make default unit or product verification depend on an external database; verify existing workflow commands remain unchanged and the new command is runnable when a disposable DSN is provisioned.

## 4. Verification and governance

- [x] 4.1 Run focused MCP unit, configuration, and PostgreSQL integration tests with and without a disposable DSN; verify all existing MCP contract tests remain green and the real-stack test reports skip rather than pass when prerequisites are absent.
- [x] 4.2 Run the conformance wrapper against an isolated Docker PostgreSQL + pgvector stack and inspect the redacted evidence artifact; verify all required categories pass, cleanup is exact, and no credential or fixture content appears in output.
- [x] 4.3 Run `openspec validate --all`, inspect the change status, and verify the proposal, delta spec, design, and task artifacts are complete and apply-ready.
