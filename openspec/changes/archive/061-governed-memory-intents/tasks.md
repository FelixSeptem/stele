## 1. Contract And Domain Model

- [x] 1.1 Define the common intent envelope, supported types, lifecycle states, bounded failure categories, and JSON serialization rules; verify invalid types, missing attribution, oversized payloads, and foreign scope fail closed.
- [x] 1.2 Implement canonical request normalization and scoped fingerprinting for type, payload, target, evidence, actor, reason, and request identity; verify equivalent retries produce identical fingerprints while material differences do not.
- [x] 1.3 Add type-specific validation for remember, update, forget, contradiction, and feedback targets/evidence; verify stale targets, incomplete evidence, hidden lifecycle sources, and cross-scope references are rejected.
- [x] 1.4 Define append-only intent transition and outcome contracts with deterministic ordering and redacted diagnostic categories; verify replay, suppression, failure, and rollback transitions preserve attribution.

## 2. PostgreSQL Persistence And Idempotency

- [x] 2.1 Add a versioned migration for scoped intent requests, transition history, outcome references, unique idempotency keys, bounded indexes, and append-only mutation guards; verify manifest, checksums, up/down SQL, and dirty-schema checks.
- [x] 2.2 Implement repository create/read/list methods for exact-scope intent status and history; verify foreign scope returns no record and default listing is paginated and lifecycle-safe.
- [x] 2.3 Implement transactional idempotent submission with fingerprint conflict detection; verify concurrent identical submissions create one request and conflicting retries return a bounded conflict.
- [x] 2.4 Persist processing outcomes, target/evidence lineage, durable work references, and rollback metadata without storing raw sensitive diagnostics; verify append-only audit rows remain inspectable after each transition.

## 3. Governance And Durable Processing

- [x] 3.1 Add a service boundary that validates authorization, policy enablement, actor/reason, and exact scope before persistence; verify disabled or rolled-back policies reject or hold new intents without canonical writes.
- [x] 3.2 Hand accepted intents to the existing durable work queue with stable scope and idempotency identity; verify lease claim, renewal, retry, exhaustion, and restart recovery do not duplicate work.
- [x] 3.3 Route remember/update/forget through existing candidate and lifecycle governance with explicit target-version checks; verify canonical updates remain append-only and stale targets become bounded outcomes.
- [x] 3.4 Route contradiction and feedback intents through reserved insight/review and feedback policies; verify contradiction activation remains independently disabled and feedback cannot directly mutate insight lifecycle.
- [x] 3.5 Implement operator disablement, rollback, and compatible re-enable behavior; verify pending records remain inspectable and only eligible work resumes under a new policy version.

## 4. API, Adapter, And Inspection Surfaces

- [x] 4.1 Add OpenAPI-first intent submission, status, and history schemas/routes with stable response categories and idempotency conflict mapping; verify generated contract tests and exact-scope authorization.
- [x] 4.2 Map existing MCP remember/forget calls to the shared intent service while preserving preview/apply and read-only boundaries; verify adapter retries replay the original result and cannot bypass governance.
- [x] 4.3 Add authorized operator inspection for intent lineage, processing outcomes, and rollback state; verify hidden content, foreign scope, raw errors, and unbounded payloads are excluded.

## 5. Telemetry And Verification

- [x] 5.1 Add low-cardinality intent metrics, lifecycle logs, and bounded diagnostics for submission, replay, queue, outcome, retry, and rollback; verify payloads, claims, scopes, IDs, prompts, credentials, and raw errors never appear.
- [x] 5.2 Add focused Go tests for normalization, type validation, idempotency, append-only transitions, target versioning, contradiction/feedback policy handoff, MCP mapping, rollback, and context exclusion.
- [x] 5.3 Add owned PostgreSQL integration tests for concurrent idempotency, exact-scope isolation, migration integrity, durable restart recovery, retry exhaustion, lineage, and redacted inspection; verify reproducible reports.

## 6. Documentation And Release Gates

- [x] 6.1 Update OpenAPI/admin and operator documentation for intent types, lifecycle states, idempotency conflicts, policy handoff, rollback, and inspection; verify docs consistency checks.
- [x] 6.2 Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`; verify direct canonical mutation remains impossible from submission paths.
- [x] 6.3 Record owned PostgreSQL shadow/retry/rollback evidence and bounded diagnostic output; verify no enablement claim is made without an exact-scope compatible policy and fresh integration evidence.
