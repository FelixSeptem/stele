# PC6 implementation verification

Status: implemented and verified on 2026-10-16, with independent fresh public
PC6 pending/initial/restart evidence. The change remains active and unarchived;
no commit, merge, push or deployment was performed in this implementation turn.
PC1–PC5 evidence is not a substitute for PC6 evidence.

## Baseline and dependency decisions

- Baseline: `13c516fee461701ebc4b3a95efe09b1c8c6d7237`, verified identical to
  fetched `origin/main` on 2026-10-16.
- Isolated branch: `feat/provider-turn-outcome-and-feedback-projection-contract`;
  worktree: `.worktrees/pc6`. Existing PC2/PC5 worktrees are untouched.
- `go mod download` passed; baseline `go test ./...` passed all 25 packages.
- Consulted <https://pkg.go.dev/github.com/jackc/pgx/v5> and
  <https://pkg.go.dev/github.com/getkin/kin-openapi/openapi3> on 2026-10-16.
  Reuse pinned pgx/v5 5.9.2, kin-openapi 0.139.0, google/uuid 1.6.0,
  golang-migrate/v4 4.19.1, standard `net/http`, `encoding/json` and SHA-256.
  No new dependency, router, queue, validation framework or source of record.
- Reuse `CanonicalObject`, Unicode validation, operation metadata, exact binding,
  PC5 reference-only resolution and PC4 binding-owned receipt identity.
- Existing `MemorySessionService.RecordTurnOutcome` updates a turn's current
  outcome and admits arbitrary outcome payloads. Reuse scoped session/turn
  identity checks, but do not invoke that mutator for append-only corrections.
- Existing usefulness log, subject links, supersession audit and summary gates
  remain authoritative for feedback influence. Admission will not invoke ranking,
  canonical-memory lifecycle, insight activation or runtime business operations.
- A dedicated migration is necessary: existing raw events/session/feedback tables
  do not enforce a binding-owned logical effect plus append-only source revisions
  and durable normalized-payload conflict identity together.

## Requirement-to-code/test matrix

| Requirement | Implementation target | Verification target |
| --- | --- | --- |
| Closed bounded versioned contract | `internal/provider/projection.go`, dedicated OpenAPI schemas | `TestProjectionClosedContract`, authoritative schemas and strict consumer |
| Exact runtime-owned identity | projection repository + binding middleware | scoped repository and serialized HTTP fixtures |
| Equivalent replay / conflict / correction | logical identity lock and immutable revision ledger | concurrent PostgreSQL admission, delayed and restart replay |
| PC4 digest attribution | same-binding receipt lookup; unverified hash stays reported | foreign/mismatched/expired receipt cases |
| PC5 state snapshots and current lifecycle | reference-only resolution; no payload return | pending/completed/hidden/foreign fixtures |
| Feedback separate from influence | async versioned mapping to usefulness log | pending exclusion, supersession, no ranking/insight activation |
| Bounded history without existence leak | binding-owned history selector | current grant and lifecycle tests |
| Public durable readiness | official fresh-stack verifier | fresh PG/pgvector, serialized API, API/worker restart, redacted provenance |

## Concrete additive contract decisions

The implementation uses dedicated projection and history operations, never a
legacy fallback. Input occurrence timestamps are canonical UTC RFC3339 seconds;
revision range is 1–2147483647; effect tokens are ASCII and at most 128 bytes;
up to eight unique PC5 event selectors and eight provenance tokens are admitted.
Session/turn links use existing bounded opaque IDs (not UUID-only, because the
legacy service generates `session_...` and `turn_...`). PC5 event and ledger IDs
are canonical, nonnil UUIDs.

Request allocation is 32768 bytes, metadata 4096, history pages at most eight
records, response allocation 65536. No raw payload or free-form task state is
returned. Feedback uses existing `useful`, `irrelevant`, `needs_review` types and
a bounded reason code, not an unrestricted text/business-state object.

`context_digest` alone is caller-reported evidence. A supplied
`context_reference` must resolve to the same full PC4 binding and matching digest
before it may be labeled verified. It is never a completion/access receipt.
Historical reference snapshots and current reauthorized reference states are
separate fields; unavailable states omit event identities. History has bounded
pagination indication and must not claim completeness when truncated.

## Staged evidence (superseded by the final checks below)

- DTO tests first failed on missing decoder/contract implementation (RED), then
  passed with the bounded closed implementation (GREEN).
- Fresh disposable repository fixture DB8 applied all official migrations with
  a clean version-37 ledger. `TestProjectionPostgresTransactions` passed scoped
  round trips, 12 concurrent equivalent retries, conflicting keys, corrections,
  delayed revisions, immutable audit rows, read-only history and revocation.
  Its attribution fixture covers actual PC4 receipt issuance, matching/bare/
  missing/expired digests, scoped existing and foreign session/turn identities,
  and verifies no mutation of legacy outcome state or orphaned identity rows.
- Real database feedback regressions first failed in DB7: invalid PC5 source
  records still influenced active summaries and `IncludeSuperseded=true` lost
  audit rows. DB8 passed after the eligibility view was aligned with PC5 content,
  token, lineage and Unicode path validation and audit lists selected the source
  log explicitly. Unicode/path byte boundaries and revoke/correction behavior
  are covered in `provider_projection_feedback_integration_test.go`.
- Serialized HTTP tests validate the actual served whole capability document
  with `NegotiateDocument`, the served OpenAPI, all reference states and bounded
  history. Strict consumers reject aliases, missing capability, legacy fallback,
  malformed identity, correction chain and unavailable identity disclosure.
- Authorization JSON regression first reproduced text/plain denials on both
  PC6 routes, including outer principal middleware. Optional denial writers
  preserve legacy defaults while PC6 returns closed JSON error DTOs. App/auth
  tests pass for missing/invalid credentials, scope, binding, session, agent,
  conversation, principal and grant denial with zero business dispatch.
- Before the final parity/authorization edits, full `go test ./... -count=1
  -timeout 5m` passed all 25 packages, `go vet ./...` passed, and changed-package
  race checks passed for provider/app/postgres/jobs/assurance/OpenAPI. The
  documentation gate and strict change validation also passed. These checks
  will be repeated after the final edits; skipped live tests do not count as
  public durability evidence.
- At this stage public fresh-stack/restart verification and final integrated
  review were pending. The repository fixture alone did not establish readiness.

## Pre-review checks (2026-10-16, before final public verification)

- `go test ./internal/provider ./internal/app ./openapi -count=1` passed after
  the final metadata/optional-field/authorization fixes. Producer/schema/strict
  consumer parity regressions observed RED before the fix and GREEN afterward;
  canonical metadata and zero timestamps now fail closed without changing legacy
  metadata normalization. Typed int64 decoding enforces exact sequence overflow;
  aggregate metadata/path byte rules supplement OpenAPI field constraints.
- A new empty `pc6_final_1` database applied official migrations and passed
  `go test ./internal/storage/postgres ./internal/jobs -count=1` with the real
  PostgreSQL fixture enabled. No manual migration or governance state was used.
- `pwsh -NoProfile -File scripts/check-documentation.ps1` and strict validation
  of this change passed after the maintenance guide's final contract updates.
- Final production image built successfully as `stele-pc6-verify:local`, image
  ID `sha256:0a8b64e699735bbb06eb01b863d7dc69ee4f9d45fe1b6f90b140ea736f2c717e`.
  This build alone does not establish public PC6 readiness and was superseded
  by the final review fixes below.

## Final review regressions

- Dedicated PC6 error schemas close both envelope and nested error without
  narrowing legacy Provider errors. Authoritative route coverage and strict
  consumer tests first reproduced the shared-open-schema defect, then passed
  with exact status/category/code/retryability checks and bounded messages.
- A fresh official-migration database reproduced feedback scope starvation:
  projection-only scopes were absent from generic maintenance discovery. The
  dedicated ledger scope source now rotates using persisted last-service
  checkpoints. A fresh database passed three-scope, batch-one and 101-identity
  backlog tests, reconstructing the repository between dispatches.
- The first official public-stack run passed PC4 and PC5 initial checks but
  failed PC6 pending admission with HTTP 403. The route hardcoded the reference
  profile even though official bootstrap granted only the content profile;
  existing authorization and repository semantics permit reference-only mode
  with either default profile. The failed report is not readiness evidence.

- The second public run reached the final lifecycle check, which rejected an
  extra JSON `actor` field. The existing lifecycle API reads actor from
  `X-Stele-Actor` and accepts a reason-only JSON body. The verifier now follows
  that existing contract; no service change was needed. A third entirely fresh
  run passed every stage. Neither failed run is used as readiness evidence.
- The verifier's predictable temporary override cleanup could delete an unowned
  file after invalid project validation. An actual PowerShell sentinel test
  reproduced deletion before the fix and preservation afterward. The script
  allocates a GUID path only after preflight, tracks successful ownership and
  removes only the exact owned path. Parser and assurance tests pass.

## Final public evidence (2026-10-16)

The final official verifier command exited 0:

```powershell
pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 -Image stele-pc6-verify:local -ReportPath docs/evidence/provider-context-pc6-regression-live.json -ReferenceReportPath docs/evidence/provider-reference-pc6-regression-live.json -ProjectionReportPath docs/evidence/provider-evidence-projection-live.json
```

It used an entirely new owned PostgreSQL + pgvector volume, applied all 37
official migrations and used official administrator/runtime/bootstrap grants.
No governance, projection or feedback state was fabricated by SQL. SQL was
bounded read-only supplemental evidence. Ordinary worker/scheduler processing
completed governance and asynchronous `projection-feedback-v1` mapping.

- [PC6 final restart report](../../../docs/evidence/provider-evidence-projection-live.json)
- [PC6 pending snapshot](../../../docs/evidence/provider-evidence-projection-live.json.pending.json)
- [PC6 initial snapshot](../../../docs/evidence/provider-evidence-projection-live.json.initial.json)
- [PC6 restart snapshot](../../../docs/evidence/provider-evidence-projection-live.json.restart.json)
- [PC4 regression report](../../../docs/evidence/provider-context-pc6-regression-live.json)
- [PC5 regression report](../../../docs/evidence/provider-reference-pc6-regression-live.json)

Pending admission passed with consumers stopped and zero eligible feedback.
The API restarted while evidence was pending; ordinary consumers then recovered.
The settled initial and subsequent API/worker restart stages passed equivalent
and conflicting delivery, revision-two correction, stale revision rejection,
bounded history, exact binding denials, foreign/missing unavailable attribution,
PC4 receipt verification, strict closed errors and unchanged public context.
One logical identity, two immutable revisions and two distinct feedback records
remained stable across restart, with only the current revision eligible. Public
suppression removed eligible feedback and redacted ordinary attribution while
privileged SQL confirmed immutable projection/feedback/canonical provenance.
Every final PC6 stage flag is true; reports contain only bounded categories,
counts, booleans and hashes, with no credentials, scope/reference values or raw
payloads. The script cleaned its owned stack, volume and private checkpoint /
credential directory after success.

Pinned provenance shared by the final stage reports:

| Field | Value |
| --- | --- |
| Baseline source revision | `13c516fee461701ebc4b3a95efe09b1c8c6d7237` |
| Production source digest | `4b0377059d29b471b847d676a62e0a71aafdac9d90f16b48527aaa5203387d9e` |
| Image digest | `sha256:0d49d2f6476c9c71fd61e14fd5e7f3c79d7497355d0c907604f490d9a4099d14` |
| Served OpenAPI digest | `3502452eeee43a7364ae7e7f57516c1e636a59f3c2d118d10a0788e8691fda7e` |
| PC6 migration manifest digest | `91c0f2c19e903327ab76ea77d34f12aaee48c4c150520c2805ef15e5495585c9` |
| Migration version | `37`, clean ledger and each official checksum verified |
| Provider / projection / fixture | `provider-v1` / `turn-outcome-feedback-projection-v1` / `provider-turn-outcome-feedback-live-v1` |

The revision is the committed baseline; the separate source digest identifies
the uncommitted production changes used to build the image. Test/documentation
edits after the build do not change that production digest. PC5 retains its
existing manifest encoding, so its manifest hash differs from the PC6 encoding;
both independently check the same official migration checksums and clean ledger.

## Final gates and specification synchronization

- `go test ./... -count=1 -timeout 5m`: all 25 packages passed after production
  and cleanup fixes; the later lifecycle change affects only the live fixture
  and passed in the complete fresh public run.
- `go vet ./...`: passed after the final production and cleanup changes.
- `go test -race ./internal/auth ./internal/provider ./internal/app ./internal/storage/postgres ./internal/jobs ./internal/assurance ./openapi -count=1 -timeout 5m`:
  all seven affected packages passed on final source.
- Fresh empty `pc6_final_2` official-migration repository fixture:
  `go test ./internal/storage/postgres ./internal/jobs -count=1` passed,
  including scope fairness, replay/concurrency, correction, attribution,
  eligibility, revoke, lifecycle and audit-history checks.
- Authoritative OpenAPI and strict capability/request/response/error consumer
  checks passed in both package tests and the served-image public fixture.
- Thirteen added requirements across six capabilities were synchronized exactly
  into main specifications after the public verifier passed. The foreign/absent
  selector scenario explicitly uses identical unavailable attribution, while
  runtime binding/scope overrides fail before dispatch.
- Roadmap status is implemented and verified, active and unarchived. Final
  documentation, strict OpenSpec and whitespace gate results are recorded below.
- After the final roadmap/specification/verification updates:
  `go test ./... -count=1 -timeout 5m` passed all 25 packages;
  `go vet ./...` passed; `pwsh -NoProfile -File scripts/check-documentation.ps1`
  passed; `openspec validate --all --strict` passed 91 items with zero failures;
  and `git diff --check` passed. The change is 19/19 tasks complete.

Future archival must use the repository sequencing script. Since these deltas
are already synchronized, use `pwsh -File scripts/openspec-archive-seq.ps1
-ChangeName provider-turn-outcome-and-feedback-projection-contract -SkipSpecs`
after rechecking delta/main parity, avoiding duplicate ADDED requirements.
Archival was not performed by this implementation turn.
