# PC5 verification and reuse record

This record covers `provider-durable-context-reference-contract`. Final Go,
documentation, OpenSpec and fresh-stack public verification passed on
2026-10-15. The initial and restart PC5 reports prove durable completion against
the current source image. HTTP 202 acceptance and skipped ordinary live tests
are separate from that integration evidence.

## Existing contracts and library decisions

The reviewed service boundaries are Provider runtime binding and operation
metadata, idempotent event ingestion, governed extraction/consolidation, normalized
memory paths, canonical/version persistence, promotion provenance, current
disclosure grants, and lifecycle/history. PC5 reuses those boundaries. PostgreSQL
remains the system of record; no separate reference table, queue, local consumer
store, migration, or new dependency is needed. Existing scope/path indexes support
the bounded exact selector. Ranked retrieval and context packing stay separate.

The repository's existing dependency versions were checked against their
pkg.go.dev package documentation on 2026-10-15 (all four pages returned HTTP 200):

| Existing library | Reviewed use | Decision |
| --- | --- | --- |
| [pgx/v5 v5.9.2](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.9.2) | PostgreSQL access, transactions, row scanning | Reuse existing repository and transaction APIs; do not introduce a new persistence framework |
| [google/uuid v1.6.0](https://pkg.go.dev/github.com/google/uuid@v1.6.0) | UUID parsing and canonical storage-compatible identifiers | Reuse parser with explicit canonical/nonzero checks at service boundaries |
| [kin-openapi/openapi3 v0.139.0](https://pkg.go.dev/github.com/getkin/kin-openapi/openapi3@v0.139.0) | Document loading/validation and `Schema.VisitJSON` | Reuse served-schema validation plus independent strict consumer semantics |
| [golang-migrate/migrate/v4 v4.19.1](https://pkg.go.dev/github.com/golang-migrate/migrate/v4@v4.19.1) | Existing migration version and upgrade execution | Keep existing migration mechanism; this implementation adds no migration |

Routing, configuration, logging, worker jobs, validation and OpenAPI are already
provided by the service's existing infrastructure. PC5 adds three routes and
adapts that infrastructure rather than replacing it. `go.mod`/`go.sum` require
no PC5 dependency edits.

## Requirement-to-code/test matrix

| Tasks / requirement | Implementation | Evidence boundary |
| --- | --- | --- |
| 1.2, 4.2: closed capability/versioned write/status/lookup | `internal/provider/reference.go`, `capability.go`; `openapi/provider_reference.go` | Provider decoder tests; OpenAPI dedicated-route/schema tests; assurance capability and malformed-response tests |
| 1.3: public acceptance and all status categories, strict serialized JSON | `internal/app/provider_reference.go` | `TestProviderReferencePublicStatesAndStrictBoundary`, `TestProviderReferenceWriteAcceptanceConflictAndLimits`; responses validated through independent assurance/OpenAPI consumer |
| 2.1: four allowlisted event types, ordinary governance gates | `internal/memory/reference.go`; `internal/governance/extractor.go` | `TestReferenceEventsUseGovernedDurableEpisodicCandidates`, `TestUnsupportedOrUnmarkedReferenceEventsRemainArchival` |
| 2.2: pending/retryable/terminal/completed/unavailable and bounded dependency errors | `internal/storage/postgres/provider_reference.go`; public handler | `TestRepositoryReferenceStatusGovernanceAndVisibility`, `TestRepositoryReferenceMissingAndDependencyFailure`; HTTP dependency error redaction |
| 2.3: scoped/principal/binding/path/value/attribution replay fingerprint | `internal/provider/reference.go`, `operation.go`; repository idempotent ingestion | Provider bound-attribution tests, repository fingerprint tests, public replay/conflict fixtures; live identical/conflicting replay |
| 2.4: interrupted promotion, retry and committed UUID lineage | `internal/storage/postgres/repository.go`, `governance_path.go`; governance extraction/consolidation | `TestPromotionCandidateLockFailureRollsBackWithoutCanonicalWrite`, `TestPromotionRetryReturnsCommittedLineageWithoutReactivatingLifecycle`, extraction retry tests; live bounded durable join |
| 3.1: non-root path at raw/candidate/canonical/version boundaries | Provider normalized input; governance extraction/summary/consolidation; PostgreSQL path persistence | Governance `path_retry_test.go`, PostgreSQL `governance_path_test.go`; live path equality joins |
| 3.2: bounded event/candidate/memory/version/provenance UUID lineage | Reference record repository projection and DTO | Malformed durable identity rejection; independent consumer UUID checks; public completed fixture; live PostgreSQL UUID join |
| 3.3, 3.4: exact selector, newest source before visibility, lifecycle exclusion | PostgreSQL exact reference selector/current version resolver | `TestRepositoryReferenceLookupUsesExactSelectorAndOmittedContent`, `TestReferenceLookupSelectsNewestBeforeDisclosureAndLifecycle`, public exact/descendant fixture; live replacement/lifecycle/history checks |
| 4.1: scope/current grants/disclosure before reads | Runtime binding middleware, `currentDisclosure`, repository visibility predicates | Public foreign-scope no-dispatch fixture; reference-only deliberately strips backend content; authorized content fixture; live foreign header denial |
| 4.3: UTF-8/encoded response/metadata limits and bounded polling | Provider request validation and response serialization; dedicated OpenAPI schema | Provider escaping-heavy response-budget tests, public allocation test, strict consumer limit/status tests; maintenance docs |
| 5.1–5.4: fresh PostgreSQL/pgvector, bootstrap, governance, restart, lifecycle | `internal/assurance/provider_reference_live_test.go`; `scripts/stele-provider-context-verify.ps1` | Separate bounded PC5 report and `.initial.json`/`.restart.json` copies; explicit Provider schema, fixture version and database migration checksum verification |
| 6.1–6.2: maintenance and consumer compatibility | README; Provider/architecture/components/self-hosting/best-practices/MCP docs; `skills/stele-memory/SKILL.md`; proposal delta specs | Documentation tests, consistency gates, strict OpenSpec validation; `docs/provider-durable-references.md` records defaults, errors, rollout/rollback and PC6/business-state boundaries |

The live replacement fixture uses a new independently governed episodic memory
for the same reference ID/path. It does not assume a new canonical version on
the same memory. Exact lookup must select the newer source; hiding it must not
fall back to the earlier source. A bounded privileged SQL assertion checks the
older canonical/version/provenance rows remain recorded.

## Focused evidence recorded so far

The following commands returned exit code 0 during this implementation:

| Command | Recorded result / limitation |
| --- | --- |
| `go test ./internal/assurance -run '^TestReference' -count=1` | Strict reference consumer and capability tests passed |
| `go test ./internal/assurance -run '^TestReference|^TestProviderReferenceFreshPostgresLive$' -count=1` | Focused suite passed; live test skipped because live environment was absent |
| `go test ./openapi -run '^TestDurableReference' -count=1` | Dedicated reference OpenAPI contract checks passed |
| `go test ./docs -count=1` | Documentation tests passed after the metadata/encoded-budget and replacement/history additions |
| `go test ./internal/app -run '^TestProviderReference' -count=1` | Strengthened serialized public-handler fixtures passed, including completed lineage/content/exact selector/lifecycle/replay |
| `go test ./internal/assurance ./internal/app -run '^TestReference|^TestProviderReference' -count=1` | Final focused consumer/handler tests passed after nil-UUID rejection and replacement/history live fixture additions; live test skipped without live environment |
| PowerShell parser check of `scripts/stele-provider-context-verify.ps1` | Parsed without errors after replacement/history report gates were added |
| `git diff --check` | Passed after final fixture, consumer and maintenance documentation edits |

The public fixture was first tightened to use the strict consumer and failed on
the former `completed` response without a reference record. After replacing that
fixture with a complete canonical UUID lineage, the focused public suite passed.
The strict consumer's nil-UUID regression also failed before the explicit
nonzero check was added, then passed in the focused suite. Repository projection
already rejected nil UUIDs; the consumer now independently enforces the same
durable-identity rule.
Initial Go test attempts blocked by the external build cache were rerun with the
required approval; those cache failures are not test passes.

## Final implementation verification

The following final-source commands passed on 2026-10-15:

| Command | Result |
| --- | --- |
| `go test ./... -count=1 -timeout 5m` | All 25 packages passed |
| `go test -race ./internal/provider ./internal/app ./internal/assurance ./internal/governance ./internal/storage/postgres -count=1 -timeout 5m` | All five changed packages passed |
| `go vet ./...` | Exit 0 |
| `pwsh -NoProfile -File scripts/check-documentation.ps1` | Documentation and docscheck passed |
| `pwsh -NoProfile -File scripts/check-self-hosting-smoke-docs.ps1` | Self-hosting documentation gate passed |
| PowerShell parser check of `scripts/stele-provider-context-verify.ps1` | No parser errors |
| `openspec validate provider-durable-context-reference-contract --strict` | Valid |
| Post-archive `openspec validate --all --strict` | 89 passed, 0 failed; all 17 added requirements synchronized |
| Post-archive documentation / self-hosting checks and `git diff --check` | All passed |
| `git diff --check` | Exit 0 |

Final review also reproduced two failures before the fixes: sequence 1/key A
replay after sequence 2/key B was incorrectly rejected as stale, and invalid
bound identities reached durable admission. Reference writes now use durable
idempotency independently of the compatibility-only `event_seq`, preserve the
original sequence in the response, and validate bounded scope before admission.
Tests cover replay through a new adapter, equal sequence with distinct keys,
invalid scope on all three HTTP routes without dispatch, and the matching
OpenAPI token constraints. Generic ingestion sequence behavior is unchanged.

Early live attempts passed initial governance and restart identity checks,
then exposed fixture errors: lifecycle calls lacked the required audit actor,
and the forgotten-state fixture used `:forget` instead of the existing
`:expire` action. The fixture now uses the documented action and actor and
checks the resulting durable lifecycle state directly. Those incomplete runs
are not acceptance evidence. While adding explicit migration provenance, a test-package import
cycle was caught and removed; the final fixture directly compares the database
migration ledger with the repository's ordered SQL asset checksums.

## Fresh public evidence

The following command completed with exit 0 on 2026-10-15:

```powershell
pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 `
  -Image stele-pc5-current:local `
  -ReportPath docs/evidence/provider-context-pc5-regression-live.json `
  -ReferenceReportPath docs/evidence/provider-durable-reference-live.json
```

The verifier created a fresh owned PostgreSQL 18 + pgvector volume, performed
official administrator/runtime bootstrap with a current content disclosure
grant, and exercised public HTTP writes/reads. The context regression passed
initial/restart in 42.571s/3.970s; PC5 passed initial/restart in 18.300s/3.975s.
The owned containers, volume and temporary credentials were removed afterward.

- [PC5 initial report](../../../../docs/evidence/provider-durable-reference-live.json.initial.json)
- [PC5 restart report](../../../../docs/evidence/provider-durable-reference-live.json.restart.json)
- [PC1–PC4 context regression initial report](../../../../docs/evidence/provider-context-pc5-regression-live.json.initial.json)
- [PC1–PC4 context regression restart report](../../../../docs/evidence/provider-context-pc5-regression-live.json.restart.json)

Both PC5 stages contain four completed task/run/handoff/compilation lineages
with identical identity hashes. Checks include normalized non-root exact paths,
all event/candidate/canonical/version/provenance UUIDs joined under exact scope,
identical/conflicting replay and replay of sequence 1 after sequences 2–4,
content/reference-only modes and foreign-scope denial. Restart additionally
proves a new independently governed handoff source replaces the selector,
the old event remains independently addressable, the latest hidden source
never falls back to the older source, suppressed/forgotten/deleted reads are
unavailable, and earlier canonical/version/provenance rows remain recorded.

| Provenance | Pinned value |
| --- | --- |
| Image identity reported by Docker | `sha256:bb55ed10d6eda7d41fbf7835bfa67e5f9a26c4ddface3956acc0b3ad008f289b` |
| Baseline Git revision | `009c40dca0c96ab2b6f46f5d409b018b592b891f` |
| Current production-source digest (includes uncommitted implementation) | `323b1125c5932ffb84947af97c438efe19e7f9b69ea36e42e84479c9d6f61a46` |
| Served OpenAPI digest | `4f1f13a60b212e6a157e500d4da9a7c55048929364b9f12d6c6ff6cc4def8068` |
| Provider / reference / fixture versions | `provider-v1` / `durable-reference-v1` / `provider-durable-reference-live-v1` |
| Applied migration version | `36`, clean; every ledger checksum matches source SQL and ledger cardinality matches manifest |
| Migration manifest digest | `f5ee9a3e4bc8954d131fa990ce27b65834f3a6ad7ec73c13aea2c0651ecceca0` |

The acceptance image used the existing local runtime base
`stele-pc5-verify:local` with its service binary replaced by a fresh
`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'
./cmd/stele` build (host Go 1.26.8). This verifies the current application in
the owned Compose runtime; it does not claim a fresh download/build of all
standard Dockerfile base layers. No PC5 dependency or migration was added.
The baseline revision alone does not identify these uncommitted changes;
the source digest and pinned image identify the tested implementation.

All acceptance gates passed. The roadmap records PC5 readiness. The repository
sequencing script synchronized all 17 added requirements across six capabilities,
normalized the existing PC4 archive to `045-provider-context-digest-and-continuity`
(filling the existing sequence gap), and archived this change as
`072-provider-durable-context-reference-contract`. All 22 tasks are checked.
`openspec list --json` reports no active changes after archiving.
PC6 and Danny's business-state ownership remain outside this acceptance.
