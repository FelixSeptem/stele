## 1. Principal Storage And Contracts

- [x] 1.1 Add additive PostgreSQL schema for principals, credentials, exact scope grants, principal/credential/grant audit records, and scoped event-idempotency claims/results, with lookup and isolation indexes.
- [x] 1.2 Add Go domain types and bounded validation for principal role/status, credential status, exact grants, lifecycle attribution, idempotency key, request fingerprint, claim lease, replay result, and safe public/admin response projections.
- [x] 1.3 Implement PostgreSQL repositories for principal lookup by credential identifier, constant-time digest verification support, grant authorization, principal lifecycle writes, bounded audit reads, and idempotency claim/result persistence.
- [x] 1.4 Add repository tests for scope isolation, disabled/expired credential rejection, revoke/rotate behavior, digest absence from reads, concurrent idempotency claims, exact replay, payload conflict, lease recovery, and transaction rollback.

## 2. Authentication And Runtime Configuration

- [x] 2.1 Replace static-key-only middleware with principal authentication and exact grant authorization contexts while retaining normalized scope context for existing handlers.
- [x] 2.2 Add a constrained bootstrap operator configuration path, legacy key deprecation validation, durable-admin detection, and an explicit emergency override audit path.
- [x] 2.3 Wire principal-backed authorization into API, worker, and scheduler runtime construction without logging raw credentials, digests, principal labels, or scope identifiers.
- [x] 2.4 Add unit and runtime wiring tests proving public/admin role separation, ungranted header denial before handler invocation, bootstrap default-scope restriction, durable-admin bootstrap lockout, and startup failure for unsafe configuration.

## 3. Principal Administration API

- [x] 3.1 Implement admin service methods for scoped principal create/read/list, one-time credential issuance and rotation, disable/expiry, exact grant create/list/revoke, and bounded audit inspection.
- [x] 3.2 Add admin HTTP routes and OpenAPI contracts for principal lifecycle, credentials, grants, and audit reads, including one-time secret response schemas and safe errors.
- [x] 3.3 Add HTTP tests for admin authentication, scope authorization, one-time raw credential redaction, credential rotation/revocation, grant revocation, non-disclosure of out-of-scope records, and route-role denial.

## 4. Idempotent Event Ingestion

- [x] 4.1 Extend `POST /v1/events` request parsing and OpenAPI with required bounded `Idempotency-Key` header, stable replay response metadata, conflict response, and safe retryable in-progress response.
- [x] 4.2 Implement normalized event request fingerprinting and transactional ingestion that atomically creates or resumes an idempotency claim, raw event, provenance, and completed replay result.
- [x] 4.3 Ensure admission rejection creates no completed event result, exact retry returns the original event/admission result, and incompatible key reuse returns conflict without another raw event.
- [x] 4.4 Add memory, repository, and HTTP tests for exact retry, conflicting payload, principal/scope isolation, interrupted-claim lease recovery, admission rejection retry, and no duplicate provenance/candidate work.

## 5. Observability, Documentation, And Verification

- [x] 5.1 Add low-cardinality metrics and structured lifecycle logs for principal authentication, grant authorization, credential lifecycle, bootstrap usage, and event idempotency outcomes without secrets or high-cardinality identifiers.
- [x] 5.2 Update self-hosting documentation with bootstrap-first-principal setup, key rotation and revocation, scoped grant administration, migration guidance for legacy key lists, and idempotent event client behavior.
- [x] 5.3 Add tests proving telemetry/redacted admin responses exclude raw credentials, digests, principal labels, event content, and raw scope values; verify docs and OpenAPI remain consistent.
- [x] 5.4 Run targeted auth, storage, memory, HTTP, config, telemetry, OpenAPI, and docs tests.
- [x] 5.5 Run `go test ./... -count=1`.
- [x] 5.6 Run `openspec validate scoped-principal-auth-and-ingest-idempotency --strict`.
