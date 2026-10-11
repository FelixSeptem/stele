# PC2 verification record

## Result

PC2 (`provider-context-disclosure-and-source-trust`) is implemented and verified
on the isolated `feat/provider-context-disclosure-and-source-trust` worktree.
The public contract now advertises `context-disclosure-v1` and
`source-metadata-v1` alongside `provider-v1`. This is a breaking public semantic
change: public exact-scope grants require explicit administrator provisioning,
context defaults to `reference_only`, and `bounded_content` requires the content
profile. The implementation does not provide a compatibility fallback that
widens disclosure.

## Evidence and commands

The following checks were run with `GOPROXY=https://goproxy.cn,direct` and
workspace-local Go caches:

| Area | Command/evidence | Result |
| --- | --- | --- |
| Go regression suite | `go test ./... -count=1 -timeout 15m` | Passed; all packages reported `ok` |
| Static checks | `go vet ./...` | Passed with exit code 0 |
| Documentation | `pwsh -NoProfile -File scripts/check-documentation.ps1` | Passed (`internal/docscheck` and `docs`) |
| Agent skill format | `uv run --with pyyaml python <skill-creator>/scripts/quick_validate.py skills/stele-memory` | Passed (`Skill is valid!`); rerun outside the sandbox after the default uv cache was inaccessible |
| OpenSpec | `openspec validate provider-context-disclosure-and-source-trust --strict`; `openspec validate --all --strict` | Passed; all 85 items valid |
| Formatting hygiene | `git diff --check` | Passed |
| Focused disclosure tests | app boundary tests, Provider contract tests, auth/storage grant tests, MCP data-tool tests | Passed, including whole-response ceiling, current-grant recheck, alternate-route denial, and independent `who_am_i` denial |
| PostgreSQL derivation tests | `STELE_TEST_POSTGRES_DISCLOSURE_DSN=postgres://... go test ./internal/storage/postgres -run TestSourceMetadataPostgres -count=1 -v` | Passed: governance eligibility, mixed summary/insight joins, chunk/projection lineage, immutable history, promotion/mutation, hidden summary filtering, and PC1 upgrade defaults |
| Public fresh-stack evidence | `scripts/stele-provider-context-verify.ps1 -Image stele-pc2-verify:local -ProjectName stele-pc2-final4 -ReportPath docs/evidence/provider-context-disclosure-live.json` | Passed on fresh PostgreSQL + pgvector, with API/worker restart |

The bounded public report is
[`provider-context-disclosure-live.json`](../../../../docs/evidence/provider-context-disclosure-live.json).
It records a passing restart stage, six completed governance events, the served
OpenAPI digest, image/source digests, durable label verification, exact and
prefix reads, replay/conflict behavior, old-binding downgrade, revocation and
unconfigured denial, lifecycle/foreign/high/restricted exclusion, alternate
route denials, attestation reauthorization, and restart identity checks. The
report contains no credential, authorization header, raw payload, unbounded
content, or scope value.

The image digest is
`sha256:abc65fdb963e7b4d7f04053ec373dfd2208a71963ea432b9a06f90d1ef11ec3c`.
The source revision is the unchanged PC1 baseline
`6bebe3e24f29800fd2ac696f80e29e7cc6eb9c9b`, accompanied by the uncommitted PC2
source-tree digest
`b766138b372c0ed0dd404393858b12e9bb5bbb33d090e4daa457bd3051a6e75d`.
The baseline revision alone does not identify the implemented PC2 tree.

The live verifier used the official bootstrap/admin provisioning path and an
isolated Compose project. The image was built from the Linux binary produced
with `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, and a local verification
Dockerfile based on `docker.1ms.run/library/debian:bookworm-slim`. The repository's
full production Dockerfile build was not used for this evidence; dependency
resolution and the Go build used `goproxy.cn` with `direct` fallback.

The focused PostgreSQL cases intentionally supplement the public live fixture.
They cover derivation branches that the public API does not construct in one
bounded smoke run (all six insight evidence kinds, mixed/incomplete lineage,
version history and schema upgrade). They are not represented as public live
coverage. Optional graph, cache, calibration, and experimental visibility paths
remain omitted where an independent disclosure lineage could not be proven.

The direct MCP data-tool test initially exposed a public `who_am_i` identity
response that the shared HTTP gate had masked. An administrator-role check was
added inside the tool handler. All eight direct MCP data tools then passed the
public-principal denial test, and the live evidence above was refreshed using
the repaired binary. The native-route test also supplies valid exact-scope
headers without a runtime binding, so its denial proves the disclosure gate
rather than merely failing scope validation.

## Implementation-to-task map

The library review checked the existing `pkg.go.dev` documentation for
`github.com/jackc/pgx/v5`, `github.com/golang-migrate/migrate/v4`, and
`github.com/getkin/kin-openapi/openapi3` during this implementation cycle. Their
documentation was available and the existing repository integrations were
retained. This check was completed during implementation, rather than claimed
as a pre-implementation review. No new module requirement or parallel fact/
policy store was added.

All 38 OpenSpec tasks are checked in `tasks.md`. The mapping is:

- **1.1–1.4:** endpoint denial matrix and closed OpenAPI/source/profile contracts in
  `docs/context-disclosure-and-source-trust.md`, `openapi/`, and the focused app,
  Provider, auth, and MCP tests; existing auth, pgx, migration, and OpenAPI
  infrastructure was reused without a new policy store or dependency.
- **2.1–2.5:** migration `0035_context_disclosure_source_metadata`, durable exact
  scope grants, audit records, current-authority resolution, and explicit bootstrap
  profile provisioning in `internal/auth`, `internal/storage/postgres`, and
  `scripts/stele-bootstrap-smoke.ps1`.
- **3.1–3.7:** typed ingestion metadata, label-aware idempotency, candidate and
  canonical/version snapshots, restrictive promotion/mutation joins, and
  conservative legacy defaults in `internal/memory`, `internal/governance`, and
  PostgreSQL repositories/triggers.
- **4.1–4.5:** summary, reflection, insight, chunk, projection, retrieval
  eligibility, and canonical citation checks preserve the source join; focused
  PostgreSQL tests verify complete and incomplete lineage separately.
- **5.1–5.7:** closed Provider decoder and mode-specific DTOs, reference-only
  shaping, content metadata, capability/OpenAPI versions, serialized response
  ceiling, shared alternate-route gate, and safe event/intent acknowledgments in
  `internal/provider`, `internal/app`, `internal/mcp`, and `openapi`.
- **6.1–6.5:** offline conformance and boundary tests, the fresh public verifier
  report above, API/worker restart checks, and separately labeled focused
  PostgreSQL derivation/upgrade evidence.
- **7.1–7.4:** Provider, self-hosting, architecture, component, best-practice,
  governance, MCP, README, and `skills/stele-memory/SKILL.md` updates describe the
  breaking defaults, explicit grants, source trust, rollback, and Danny's
  business-authority boundary.
- **7.5:** this record, the final task reconciliation, full Go/vet/docs/OpenSpec/
  diff checks, and roadmap synchronization.

## Remaining limits

PC3–PC6 remain pending. Caller budget units and selection accounting, context
digests/continuity, durable reference receipts, and turn/outcome/feedback
projection are deliberately not claimed here. The complete serialized response
ceiling is enforced, while caller byte/token budgets remain PC3 work. Unsupported
public native/MCP content routes fail closed before lookup; no local retrieval,
ranking, repacking, or second memory store is introduced. A future route must
implement the same disclosure boundary before it can be opened.

## Archive closure

Archived as change `070-provider-context-disclosure-and-source-trust` on
2026-10-11 using `scripts/openspec-archive-seq.ps1`, with all 38 tasks complete.
The archive synchronized 14 added and 3 modified requirements across five main
specifications. Post-archive `openspec validate --all --strict` passed all
86 specifications. Roadmap and maintenance links point to the sequence archive;
PC3 is the next candidate, with PC3–PC6 remaining pending.
