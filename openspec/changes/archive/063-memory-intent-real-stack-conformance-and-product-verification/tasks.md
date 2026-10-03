## 1. Verification Contract And Fixture Ownership

- [x] 1.1 Define the intent product-verification phase contract, fixed phase/result/prerequisite categories, stable report envelope, and redaction rules; verify the report schema rejects unbounded or sensitive fields.
- [x] 1.2 Add owned fixture scope, idempotency-key, request-identity, and cleanup helpers to the existing product-verification harness; verify cleanup cannot target resources outside the generated ownership label and exact scope.
- [x] 1.3 Add prerequisite detection for Docker, PostgreSQL + pgvector, migration compatibility, API/worker/scheduler readiness, and optional MCP enablement; verify missing prerequisites produce explicit skip results without readiness claims.

## 2. Public Intent Submission And Inspection

- [x] 2.1 Extend the real-stack harness to submit bounded `remember`, `update`, and `forget` intents through OpenAPI with actor, reason, exact scope, and idempotency headers; verify accepted responses expose only stable intent metadata.
- [x] 2.2 Add OpenAPI replay and conflict scenarios for identical and materially different retries; verify identical retries preserve one intent identity while conflicting reuse returns the bounded conflict category.
- [x] 2.3 Add scoped status and history inspection assertions for accepted, candidate, suppressed, failed, and replayed outcomes; verify foreign-scope reads are non-disclosing and raw diagnostics/content are absent.
- [x] 2.4 Add optional MCP remember/forget conformance cases using the shared intent service; verify preview/apply and read-only boundaries remain intact and MCP retries replay the original result.

## 3. Durable Queue And Runtime Recovery

- [x] 3.1 Add assertions that accepted intents create one durable `memory_intent` queue item with the expected stable reference and exact scope; verify duplicate submission does not create a second work item.
- [x] 3.2 Exercise worker lease claim, checkpoint/complete, retry, and retry-exhaustion behavior for intent work; verify transition sequence, bounded failure category, and terminal queue disposition remain inspectable.
- [x] 3.3 Add API, worker, and scheduler restart/drain scenarios around an accepted or queued intent; verify the same durable identity resumes after restart without duplicate canonical transitions or lost history.
- [x] 3.4 Add assertions for scheduler/worker readiness and durable backlog diagnostics during recovery; verify timeout, dependency degradation, and recovery are reported as separate bounded categories.

## 4. Governance, Scope, And Rollback Gates

- [x] 4.1 Add exact tenant/project/namespace isolation fixtures for submission, queue claim, status, history, and canonical outcome reads; verify foreign scopes cannot observe or process the fixture intent.
- [x] 4.2 Add contradiction and feedback fixtures with evidence binding and review-only assertions; verify neither operation directly activates an insight or changes default retrieval/context output.
- [x] 4.3 Add policy disablement, pending retention, compatible re-enable, and rollback scenarios; verify new work is held or rejected according to policy and only eligible exact-scope work resumes.
- [x] 4.4 Add append-only canonical/version assertions for remember/update/forget processing; verify target-version checks and replay do not overwrite prior canonical memory or provenance rows.

## 5. Evidence, Telemetry, And Documentation

- [x] 5.1 Implement redacted conformance report persistence/output with stable run identity, phase results, recovery/rollback summaries, freshness, cleanup, and consumability verdict; verify payloads, scopes, IDs, credentials, DSNs, and raw errors are excluded.
- [x] 5.2 Add low-cardinality metrics and lifecycle logs for intent conformance phases, prerequisites, replay/conflict, queue recovery, rollback, cleanup, and redaction checks; verify sensitive label rejection tests pass.
- [x] 5.3 Add authorized operator diagnostics for aggregate intent conformance health and dominant failure categories; verify hidden content and foreign identifiers never appear in ordinary diagnostics.
- [x] 5.4 Update `docs/self-hosting.md`, provider conformance guidance, and release evidence documentation with the command, DSN ownership rules, covered gates, skip/fail semantics, and rollback interpretation; verify documentation links and examples are consistent.

## 6. End-To-End Verification And Release Evidence

- [x] 6.1 Run focused intent conformance tests against the Docker PostgreSQL + pgvector fixture and verify a reproducible redacted report is emitted.
- [x] 6.2 Run the full product-verification command with API, worker, and scheduler modes and verify restart, drain, backup/restore, and intent evidence all complete within bounded timeouts.
- [x] 6.3 Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`; verify no default retrieval, lifecycle, or scope regression.
- [x] 6.4 Record the owned PostgreSQL + pgvector conformance evidence and verify the final report makes no enablement claim when prerequisites, freshness, isolation, replay, recovery, or rollback gates are missing.
