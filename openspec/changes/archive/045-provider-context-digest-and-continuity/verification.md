# PC4 verification record — Provider context digest and continuity

Status: implementation and acceptance evidence complete; archive is a separate
lifecycle step. This record distinguishes focused and live evidence and does not
treat archived PC1–PC3 reports as PC4 readiness.
No credential, Authorization header, scope value, raw continuity reference,
projection digest or context payload is retained here.

## Design and dependency review

Reviewed 2026-10-12 before adding PC4 infrastructure:

| Concern | Existing/reviewed component | Decision |
| --- | --- | --- |
| PostgreSQL transactions, locking and connection pool | [`github.com/jackc/pgx/v5`](https://pkg.go.dev/github.com/jackc/pgx/v5) | Reuse the existing pgx repository and PostgreSQL row locking; no new data-access dependency. |
| Additive migration execution | [`github.com/golang-migrate/migrate/v4`](https://pkg.go.dev/github.com/golang-migrate/migrate/v4) | Reuse the existing migration runner and append immutable migration `0036`; no new migration dependency. |
| OpenAPI schema parsing/validation | [`github.com/getkin/kin-openapi`](https://pkg.go.dev/github.com/getkin/kin-openapi) | Reuse the existing contract validator; OpenAPI remains the public source of truth. |
| Strict JSON/canonical digest | Go standard library `encoding/json`, `crypto/sha256`, `crypto/rand` | Existing standard-library primitives meet the closed DTO, canonical byte and opaque-token requirements; no additional dependency. |

## Requirement-to-evidence matrix

| PC4 concern | Implementation | Independent evidence |
| --- | --- | --- |
| Closed input/result and discovery | `openapi/provider_context.go`, `internal/provider/context.go`, `internal/provider/capability.go` | `internal/app/provider_context_digest_test.go`, `internal/provider/context_identity_test.go`, `openapi` tests, strict OpenSpec validation. |
| Typed canonical digest | `internal/provider/context_identity.go` | Four golden byte/response/hash vectors under `internal/provider/testdata/context-digest-v1`; `internal/provider/context_identity_test.go`. |
| Independent consumer validation | `internal/assurance/context_digest.go`, `provider_context_contract.go` | `internal/assurance/context_digest_test.go` rejects forged versions/hashes, invalid continuity shapes, duplicate keys and invalid Unicode without using Provider canonicalization. |
| PostgreSQL receipt authority | migration `0036_provider_context_continuity`, `internal/storage/postgres/context_continuity.go` | `TestContextContinuityPostgresTransactionsAndUpgrade`: migration/upgrade, provider `id_` principal identifier, transaction rollback, deduplication, caps, expiry and cleanup. |
| Route order and exact-body admission | `internal/app/provider_context_continuity.go`, `provider_http.go`, Provider admission/canonical types | app/provider/assurance focused tests, including malformed union rejection, final digest/body accounting and bounded errors. |
| Configuration and cleanup | `internal/config/config.go`, `internal/jobs/context_continuity.go`, `internal/app/app.go` | `internal/config/context_continuity_test.go`, `internal/jobs/context_continuity_test.go`, storage transaction test. |
| Current-source and lifecycle conformance | `internal/assurance/provider_context_live_test.go`, disclosure boundary and Provider context fixtures | Public-route fixtures cover source replacement, suppression/forgetting/deletion, restrictive trust/classification, foreign scope, reference-only text absence, downgrade/revocation and infeasible allocation. |
| Outage, race, TTL, capacity and incompatible receipts | `internal/storage/postgres/context_continuity_integration_test.go`, `internal/provider/context_continuity.go`, `internal/jobs/context_continuity_test.go` | PostgreSQL upgrade/transaction test covers rollback, concurrent deduplication, capacity, expiry/cleanup, foreign scope and unavailable dependencies; focused race suite passes. |
| Offline bounded conformance and redaction | `internal/assurance/provider_context_contract_test.go`, `internal/assurance/context_digest_test.go` | Independent consumer rejects forged hashes, malformed unions, incompatible contracts and accounting errors; report fixtures retain only bounded categories and hashes. |
| Fresh public/restart behavior | `internal/assurance/provider_context_live_test.go`, `scripts/stele-provider-context-verify.ps1` | Owned Docker/pgvector initial and restart run below. |
| External semantics and operating guidance | README, Provider guide, architecture/components, best practices, self-hosting, MCP guide and Agent Skill | `go test ./docs -count=1`, `scripts/check-documentation.ps1`. |

## Executed evidence

All commands used `GOPROXY=https://goproxy.cn,direct` and isolated PC4 Go
cache/module-cache paths.

| Command | Result |
| --- | --- |
| `go test ./internal/assurance ./internal/storage/postgres ./internal/app ./internal/provider ./openapi ./internal/retrieval ./internal/config ./internal/jobs -count=1` | Passed. |
| `go test ./... -count=1 -timeout 15m` | Passed in the final gate run. |
| `go test -race ./... -timeout 20m -p 1` | Passed in the final gate run, including changed concurrent paths. |
| `STELE_TEST_CONTEXT_RECEIPTS_DSN=… go test ./internal/storage/postgres -run '^TestContextContinuityPostgresTransactionsAndUpgrade$' -count=1 -v` | Passed against a newly created database in the dedicated disposable pgvector container. |
| `go vet ./...` | Passed in the final gate run. |
| `openspec validate provider-context-digest-and-continuity --strict` | Passed in the final gate run. |
| `openspec validate --all --strict` | Passed in the final gate run: 88 items, 0 failures (informational long-requirement notices only). |
| `pwsh -File scripts/check-documentation.ps1` | Passed in the final gate run. |
| `docker compose -f docker-compose.yml config --quiet` | Passed in the final gate run. |
| `git diff --check` | Passed in the final gate run. |
| `docker build --build-arg STELE_GO_IMAGE=docker.1ms.run/library/golang:1.25-bookworm --build-arg STELE_RUNTIME_IMAGE=docker.1ms.run/library/debian:bookworm-slim --build-arg STELE_GOPROXY=https://goproxy.cn,direct -t stele-pc4-verify:local .` | Passed. |
| `pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 -Image stele-pc4-verify:local -ReportPath .tmp/pc4-context-report.json` | Passed, initial and API/worker-restart stages against a newly owned `docker.1ms.run/pgvector/pgvector:pg18` stack. |

The live verifier bootstrapped an exact-scope public runtime with the official
script, wrote public governed fixtures, confirmed governance completion through
the owned PostgreSQL service, exercised non-root exact/prefix and genuine-empty
context, both response modes and full body accounting. It issued and compared a
metadata-only receipt, then restarted API and worker and compared the original
receipt again. The initial and restart reports both passed and recorded
`context-digest-v1`, `context-continuity-v1`, stable identity/citation/budget
hashes, and the bounded continuity categories `issued`, `unchanged`, `changed`,
`expired`, and `unavailable`. The public fixture also recorded source suppression,
forgotten/deleted and restrictive-source exclusion, current grant downgrade and
revocation denial, foreign-scope denial, reference-only text absence, and
infeasible-allocation refusal. Reports deliberately store only bounded
outcomes, low-cardinality counters, hashes, and build/schema provenance; they do
not contain credentials, scope values, raw references, projection digests, memory
identifiers, or context payloads. The verifier removed its generated Compose
project, volume and temporary credentials on completion.

`scripts/check-docs-consistency.ps1`,
`scripts/check-openspec-roadmap-status-consistency.ps1`, and
`scripts/check-openspec-example-impact-declaration.ps1` are not present in this
repository revision, so they were not represented as passing gates. The existing
documentation gate and docs package contract test are recorded above.

## Acceptance disposition

All implementation checkboxes are complete and have independent focused or live
evidence. The fresh owned PostgreSQL/pgvector initial and API/worker restart
reports cover the complete PC4 public contract, including current-source
comparison and bounded refusal outcomes. PC4 is ready for its separate archive
step; this verification record does not perform or imply that Git lifecycle
operation.
