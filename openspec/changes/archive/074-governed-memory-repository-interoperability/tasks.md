## 1. Contract and Dependencies

- [x] 1.1 (≤3h) Record a `pkg.go.dev` evaluation of mature Markdown parsers and select bounded parsing support; verify the decision against fixtures for fences, top-level bullets, metadata, and wiki links.
- [x] 1.2 (≤4h) Define OpenAPI schemas and examples for source registration and preview, including exact scope, selections, limits, diagnostics, and effective trust; verify schema and request-validation contract tests.
- [x] 1.3 (≤4h) Define OpenAPI submission, status, and manifest contracts with replay/conflict/error semantics and entitlement mapping; verify response examples and public contract tests before implementing handlers.

## 2. Snapshot Interpretation

- [x] 2.1 (≤3h) Implement bounded snapshot/path admission and canonical snapshot digests; verify UTF-8, NFC, duplicate paths, traversal, encoded separators, body limits, and file-order determinism with table tests.
- [x] 2.2 (≤4h) Implement the Markdown subset and bounded metadata/diagnostics; verify fact bullets, fences, unsupported prose, malformed metadata, date handling, and unknown-key preservation with reference fixtures.
- [x] 2.3 (≤3h) Implement bounded wiki-link interpretation and entry fingerprints; verify cycles, missing/non-Markdown targets, duplicate occurrences, reorder stability, and changed-content identities.
- [x] 2.4 (≤3h) Add adversarial and fuzz coverage for parser and path boundaries; verify bounded processing and no execution/network/filesystem side effects for hostile input.

## 3. Durable Scoped Import State

- [x] 3.1 (≤4h) Add additive PostgreSQL migrations and scoped repository/preview storage; verify fresh migration, upgrade compatibility, exact-scope constraints, revision binding, and preview expiry with integration tests.
- [x] 3.2 (≤4h) Add durable import plans, per-entry dispatch attempts, identity tombstones, and provenance mappings; verify transactional persistence and uniqueness under duplicate insertion.
- [x] 3.3 (≤4h) Implement generation comparison and idempotent import acceptance; verify concurrent distinct submissions, equivalent replay after generation advance, changed-key conflicts, and zero partial dispatch on rejection.

## 4. Public Admission and Worker Dispatch

- [x] 4.1 (≤3h) Implement repository registration with feature gating and current exact-scope authorization; verify disabled defaults, denied registration, and non-disclosing foreign-scope behavior.
- [x] 4.2 (≤4h) Implement immutable preview generation with explicit class/path mapping and warning acknowledgement contract; verify no intent/canonical mutation and deterministic reviewed plans.
- [x] 4.3 (≤4h) Implement preview-bound submission and authorization/policy revalidation; verify digest mismatch, expiry, stale generation, trust entitlement, and replay precedence through HTTP contract tests.
- [x] 4.4 (≤4h) Connect durable dispatch to existing governed remember intents with deterministic child idempotency; verify event/candidate provenance and default untrusted/restricted metadata.
- [x] 4.5 (≤4h) Implement worker recovery and bounded terminal outcomes; verify restart after child submission, partial failure, exhausted retries, grant revocation, and no implicit new submission of accepted identities.

## 5. Inspection and Navigation

- [x] 5.1 (≤4h) Implement bounded import inspection separating dispatch completion from downstream activation; verify response limits, current disclosure checks, terminal failure visibility, and absence of historical payload leakage.
- [x] 5.2 (≤4h) Implement generation-watermarked manifest navigation with at most 100 items; verify entry-point ordering, cycles, partial activation, current eligibility, and no automatic context/ranking promotion.
- [x] 5.3 (≤4h) Add cross-surface lifecycle/isolation regression coverage; verify consolidation, retrieval, suppressed/forgotten/deleted targets, source-label filtering, missing-file semantics, and no resurrection on identical reimport.

## 6. Operations and Maintenance Documentation

- [x] 6.1 (≤3h) Integrate preview expiry/payload retention with existing cleanup policy and preserve permitted audit/deduplication records; verify expired payload handling and fail-closed disclosure after lineage changes.
- [x] 6.2 (≤3h) Implement admission flag and governed draining on disable; verify default-off upgrade and rollback without deleting accepted work or canonical history.
- [x] 6.3 (≤4h) Add the import operations guide and update README, integration guide, and agent skill; verify documented shapes, errors, authorization, limits/defaults, warning acknowledgement, compatibility subset, migrations, rollback, and HTTP-only/no-execution boundaries against OpenAPI.

## 7. End-to-End Acceptance

- [x] 7.1 (≤4h) Add a public HTTP plus PostgreSQL/worker acceptance fixture covering preview → submit → inspect → consolidation → manifest/retrieval → forgetting; verify replay, concurrent conflict, and cross-scope denial in the same reproducible fixture.
- [x] 7.2 (≤3h) Run relevant Go package tests, the full Go suite, OpenAPI checks, documentation checks, and strict OpenSpec validation; record commands/results and resolve failures before declaring implementation complete.
