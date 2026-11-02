## 1. Closed correction contract and source resolution

- [x] 1.1 Define the closed `provider-correction-governed-version-v1` capability, request/response, verified correction-association domain types and stable bounded error categories; verify malformed, null, duplicate, unknown, first-revision and incompatible inputs fail validation in unit tests.
- [x] 1.2 Add a server-side correction resolver that requires the current exact runtime binding, a current corrected projection revision and eligible PC5 target lineage/version; verify foreign, missing, wrong-binding, hidden, superseded and mismatched target cases share non-disclosing outcomes before intent persistence.
- [x] 1.3 Extend the governed intent fingerprint, record/history and worker inputs with immutable server-verified correction attribution; verify generic caller evidence cannot impersonate this association and identical/colliding replay behaves deterministically.

## 2. PostgreSQL persistence and governed execution

- [x] 2.1 Pin Riverqueue v0.44.1 and its matching pgx v5 driver; add River's migrations in a dedicated schema and verify startup/upgrade/restart behavior without changing the Go 1.25 baseline.
- [x] 2.2 Add an additive PostgreSQL migration for immutable correction-to-intent attribution and a transactionally idempotent legacy memory-intent queue handoff; prevent the legacy worker from claiming that kind after cutover while preserving other kinds and historical status.
- [x] 2.3 Implement exact-scope intent admission and correction association persistence with River `InsertTx` in the same PostgreSQL transaction; verify replay creates no duplicate job and intent rollback also rolls back its River job.
- [x] 2.4 Implement the River memory-intent worker so source/target revalidation, governance candidate/promotion/version/provenance effects, append-only intent transition and `JobCompleteTx` share one transaction; verify rollback-and-retry, terminal stale-source completion, and that application interfaces do not expose `pgx.Tx`.
- [x] 2.5 Verify legacy pending/recoverable memory-intent jobs are handed off exactly once, terminal historical jobs are not replayed, and dual consumers cannot execute an intent twice.
- [x] 2.6 Route a valid correction-attributed update through candidate promotion and append-only versioning with immutable provenance linkage; verify the prior version is retained and the successor is current in a fresh PostgreSQL integration fixture.

## 3. Provider API, capability and conformance

- [x] 3.1 Add the dedicated bound Provider correction-update handler and adapter operation, with strict decoding, capability negotiation, disclosure/precedence enforcement, request/response limits and bounded error mapping; verify OpenAPI and HTTP contract tests.
- [x] 3.2 Publish capability discovery and strict consumer validation for the new operation; verify absent capability fails closed without generic-intent fallback and non-capable existing callers remain compatible.
- [x] 3.3 Extend Provider projection/feedback conformance so active summaries exclude superseded correction signals while retained history remains append-only; verify correction-to-update replay and API/worker restart behavior.

## 4. Fresh-stack evidence, documentation and release gates

- [x] 4.1 Extend the owned PostgreSQL + pgvector Provider verifier or dedicated fixture with initial and restart correction-update paths, denied attribution, successor lineage, replay/conflict, active-summary exclusion, River enqueue/complete rollback, retry and legacy handoff checks; retain only redacted bounded reports.
- [x] 4.2 Update OpenAPI integration documentation, provider projection maintenance, architecture, components, self-hosting, best practices and the Stele agent skill for capability negotiation, correction attribution, authorization, lifecycle/replay behavior and compatibility/rollback; verify documentation checks.
- [x] 4.3 Update roadmap item 10 only after the end-to-end acceptance evidence passes; run focused/full Go tests, race/vet where applicable, migration and fresh-stack verifier, strict OpenSpec validation and diff checks before marking all tasks complete.
