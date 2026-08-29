## 1. Migration Foundation

- [x] 1.1 Inventory the current PostgreSQL bootstrap schema, startup call sites, and existing `golang-migrate` dependency; evaluate mature PostgreSQL migration libraries on `pkg.go.dev` and record the selected library, version, licensing, dirty-state, locking, and embedded-SQL rationale in implementation notes before adding production code.
- [x] 1.2 Define migration configuration, bounded error categories, migration status model, and startup policy (`auto`, `validate`, and documented externally managed mode); add configuration validation and focused unit tests.
- [x] 1.3 Convert the current supported base schema into an immutable initial numbered migration and implement a migration ledger/status reader that reports current, pending, dirty, divergent, and incompatible states.
- [ ] 1.4 Implement locked forward migration execution using PostgreSQL-owned serialization and tests that prove concurrent invocations cannot apply the same migration twice.
- [x] 1.5 Wire migration validation/execution into API, worker, and scheduler startup before protected traffic or job claims, with unit tests for policy, pending, and dirty outcomes.
- [x] 1.6 Add the standalone `stele migrate status` and forward-apply commands with machine-readable and human-readable output; verify that they use the same ledger and lock as runtime startup.
- [ ] 1.7 Create a prior-release populated database fixture and real PostgreSQL upgrade test that preserves authorized principal/grant, event/idempotency, canonical-memory, provenance, and history behavior.
- [x] 1.8 Add dirty-state and incompatible-version recovery tests plus operator documentation that prohibits automatic down migration and states the forward-remediation/restore path.

## 2. Runtime Resource and Lifecycle Safety

- [x] 2.1 Add bounded HTTP header, body, read, write, idle, and shutdown-drain settings to runtime configuration with defaults, startup validation, and unit tests for invalid values.
- [x] 2.2 Add shared request-body limiting and JSON decode error mapping to public and admin HTTP handlers; verify oversized, malformed, and valid requests cannot partially persist an event or expose request content.
- [x] 2.3 Configure `http.Server` limits and readiness drain state; add handler/server tests proving readiness goes non-ready before shutdown while health behavior remains documented.
- [x] 2.4 Replace the uncancelable main context with signal-aware `SIGINT`/`SIGTERM` propagation and implement bounded API server shutdown with exactly-once dependency cleanup tests.
- [x] 2.5 Make worker and scheduler loops observe cancellation, cease new claims, preserve durable retry/lease semantics, and close dependencies on cancellation or startup failure; add focused lifecycle tests.
- [x] 2.6 Add bounded lifecycle metrics/log events for startup, migration validation, readiness transition, signal, drain, timeout, and cleanup; verify metric labels and ordinary logs exclude scopes, principals, DSNs, credentials, and raw errors.

## 3. Runtime Contract Publication

- [x] 3.1 Define build/version/OpenAPI digest/schema compatibility metadata and reproducible build-time injection defaults; add unit tests for explicit and absent build metadata.
- [x] 3.2 Expose the authoritative embedded OpenAPI document from API mode with correct content type and cache validator, and add tests that parse the served document and verify conditional requests.
- [x] 3.3 Expose the bounded public version/compatibility endpoint and tests that confirm it reports contract/schema facts while excluding configuration, secrets, scope data, and operational detail.
- [x] 3.4 Add automated route-to-OpenAPI coverage checks for all public discovery and protected API routes, including authentication, scope, idempotency, and error contract assertions.

## 4. Self-hosted Bootstrap and Deployment Assets

- [x] 4.1 Refactor `docker-compose.yml` into documented local-evaluation wiring that uses only accepted bootstrap-admin authorization settings and one consistent configuration set across API, worker, and scheduler.
- [x] 4.2 Add a non-secret environment example and production configuration reference that distinguish local bundled PostgreSQL from external operator-managed PostgreSQL, secret injection, migration policy, and reverse-proxy/TLS responsibilities.
- [x] 4.3 Create a repeatable bootstrap smoke command/script that creates the first durable admin, least-privilege runtime principal and exact grant, securely captures one-time credentials, and verifies bootstrap deactivation semantics.
- [x] 4.4 Extend the smoke flow to prove migration status, OpenAPI/version discovery, idempotent ingest, async governance, retrieval, context assembly, and same-scope assurance evidence using the runtime principal.
- [x] 4.5 Update README and self-hosting documentation to make the canonical bootstrap and product smoke path authoritative, remove live obsolete allow-list examples, and document failure diagnostics by stage.
- [x] 4.6 Add documentation/deployment contract tests that reject obsolete auth settings and validate referenced environment variables, commands, routes, and smoke script sequence against the running configuration/OpenAPI contract.

## 5. PostgreSQL Backup and Recovery Operations

- [x] 5.1 Implement a constrained operator backup script that validates explicit source and destination inputs, invokes supported PostgreSQL tooling without echoing secrets, writes a checksum manifest with bounded compatibility metadata, and has command-level validation tests.
- [x] 5.2 Implement a constrained restore script that validates artifact/manifest integrity, requires a distinct explicit target and destructive confirmation, and refuses empty, broad, source-equal, or otherwise unsafe restore targets; add refusal-path tests.
- [x] 5.3 Implement a restore-verification command that validates current migrations and performs bounded authorized scope-safe read proof against the restored target; test success and stable failure categories.
- [x] 5.4 Bridge successful restore verification into existing backup/restore assurance proof with strict bounded metadata validation, freshness behavior, isolation tests, and lifecycle-safe readiness reporting.
- [x] 5.5 Document backup, restore, restore drill, failed-verification, forward-migration, and rollback runbooks, including PostgreSQL client prerequisites, RPO/RTO ownership boundaries, and a prohibition on implicit destructive restore.

## 6. Real-stack Product Verification

- [ ] 6.1 Build an isolated real PostgreSQL/pgvector integration harness with generated credentials, unique labelled resources, bounded deadlines, prerequisite detection, and ownership-safe cleanup behavior.
- [ ] 6.2 Add a fresh-stack black-box test that starts API/worker/scheduler, runs bootstrap-admin-first setup, creates exact grants, discovers the runtime API contract, and completes idempotent ingest through retrieval/context assembly.
- [ ] 6.3 Add black-box negative cases for ungranted-scope denial, unauthorized admin access, idempotency-key replay/conflict, and lifecycle-safe absence of cross-scope retrieval disclosure.
- [ ] 6.4 Add black-box signal/drain/restart tests for API, worker, and scheduler that prove readiness transition, bounded termination, no duplicate raw event, and durable continuation of eligible background work.
- [ ] 6.5 Add black-box migration-upgrade and disposable backup/restore verification cases using only harness-owned databases and compare restored scoped behavior with the source fixture.
- [ ] 6.6 Provide one documented local product-verification entrypoint with explicit non-pass skip behavior when container prerequisites are absent, plus CI wiring that treats the real-stack suite as mandatory for release builds.

## 7. Release Evidence and Completion Gate

- [ ] 7.1 Update observability and self-hosted assurance docs/fixtures for migration, startup/drain, product verification, and restore-proof evidence; verify all new telemetry remains low-cardinality and redacted.
- [ ] 7.2 Run focused unit tests, real PostgreSQL integration tests, Compose/product verification, migration upgrade/recovery checks, documentation contract checks, OpenAPI validation, and `openspec validate product-ready-self-hosting-foundation --strict`; fix every failure before marking tasks complete.
- [ ] 7.3 Execute the documented fresh-install, restart, and restore-drill runbooks from a clean harness environment; capture bounded reproducible release evidence and verify no user workspace volume, unlabelled database, or credential-bearing artifact was modified or committed.
