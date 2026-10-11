# PC3 implementation verification

Change: `provider-context-budget-and-selection-diagnostics`.
Date: 2026-10-11. Branch: `feat/provider-context-budget-and-selection-diagnostics`.
Implementation is verified and archived as change 071. Delta requirements are synchronized into the main specifications. PostgreSQL remains the system of
record; this change adds no migration, persistence store, tokenizer, or new
infrastructure dependency. Existing retrieval libraries and standard-library JSON
accounting cover the change without introducing a replacement infrastructure layer.

## Public behavior

The required positive `budget` counts item placements. Optional closed
`budget_limits` independently allocate UTF-8 content bytes, complete JSON body
bytes, and unique canonical citations. Omitted limits use discovered server
ceilings; explicit content/citation zero remains zero. Independent defaults are
100 items, content inherited from the response ceiling, 1 MiB body, and 100
citations. Startup rejects invalid settings. Token fields remain unsupported.

Retrieval owns ordering, eligibility, protected section preference and quotas.
Final whole-item admission uses the same closed disclosed DTO as serialization,
retains mandatory evidence, and continues only through existing bounded
candidates. A conservative accounting reserve is documented. Actual JSON body
bytes, including accounting itself, are finalized before the exact bytes are
emitted without a newline. Internal planner allocations never become public
budget reasons. Native calls without final admission retain their defaults.

Successful responses require `context-budget-v1` accounting and
`context-packing-v1`. Errors retain bounded 400 invalid input, 403 disclosure
denial, 413 infeasible allocation, and retryable 503 dependency failure. Invalid
metadata is normalized/validated before budget preflight. Consumer conformance
binds reported requested allocations and unsupported sections to the original
request, and verifies exact effective intersections when discovery is available.

## Fresh PostgreSQL and restart proof

Final invocation used a new owned project `stele-pc3-20261011-2226`, official
bootstrap and exact-scope grants, a fresh PostgreSQL/pgvector volume, and the
final repaired local image. Initial and API/worker restart runs both passed.
Seven labeled raw events completed governance. Identity, provenance and budget
evidence hashes agree across stages. Both content and reference modes passed
exact/prefix retrieval, replay, zero/omitted/oversized limits, safe empty,
unsupported section, lifecycle and scope checks.

- [Initial bounded report](evidence/provider-context-pc3.json.initial.json)
- [Restart bounded report](evidence/provider-context-pc3.json.restart.json)
- [Latest stage report](evidence/provider-context-pc3.json)

Pinned image manifest:
`sha256:c85846fbf802d876d91f475cc9f1095a967a88dfc0f065d09a9b6f6dff9fc091`.
Baseline source revision: `e53b8b7babee544a86f6893ec76acffb33d71920`.
Working source digest:
`f87bef6c1ceee3816cbe64146a281fd990e315f94b1e2fff153dfec8d81fae1a`.
OpenAPI digest:
`42a60b7bb0acb9506ea3adc78e5880cd97ce865380df66aa8694b859b6eaeab6`.
Disclosure/source versions: `context-disclosure-v1` / `source-metadata-v1`.
Budget/packing versions: `context-budget-v1` / `context-packing-v1`.
The revision is the baseline plus the pinned working-source digest, not a claim
that uncommitted PC3 code already exists at that Git revision.

The reports store bounded counters/outcomes and hashes only. Credentials,
authorization headers, content, raw payloads, scope values and memory IDs are not
retained. The owned script removed its stack, volume and temporary credentials;
existing projects were preserved. A previous fresh attempt exposed an overlong
fixture path segment before retrieval; the path was split into valid segments,
a normalization regression was added, and fresh verification was rerun. Failed
or skipped runs are not acceptance evidence.

## Task-to-evidence mapping

Paths below are relative to the repository. All named tests passed in the final
full run, except the separately invoked environment-dependent live test, whose
passing initial/restart outputs and reports are linked above.

| Task | Evidence |
| --- | --- |
| 1.1 | `openapi/provider_context_budget_test.go`: request, closed required accounting, discovery, bounded errors and exact canonical examples. |
| 1.2 | `TestContextBudgetStrictRequest`, `TestProviderContextRejectsNoncanonicalRequestsBeforeAssembly`: omissions, zero, null/duplicate/case/unknown/token/type/fraction/portable-overflow/trailing shapes use the public decoder/route. |
| 1.3 | `internal/provider/context.go`; strict-request, metadata-before-preflight and canonical HTTP tests. |
| 1.4 | `TestProviderContextBudgetConfiguration`, capability round-trip/bounds tests: defaults, independent overrides, inherited content, zero and startup rejection. |
| 1.5 | `TestProviderAdapterEnforcesConfiguredLimits`, strict-request and live oversized/intersection cases: clamp before assembly fanout; retrieve limit remains separate. |
| 2.1 | Closed OpenAPI accounting tests and `TestContextConsumerRejectsMalformedEmptySubstitutes`. |
| 2.2 | `TestContextBudgetExactUTF8RepeatedPlacementsAndCitations`, `TestContextBudgetZeroAndUnsafeReasons`, live multilingual/long-path and unique-evidence cases. |
| 2.3 | `internal/retrieval/context_admission.go`; full compilation and native/ample assembly comparisons. No retrieval-to-Provider dependency. |
| 2.4 | `TestContextBudgetSelfCountingDecimalTransitions`, exact UTF-8/repeated placement tests, captured HTTP-body/live accounting checks. |
| 2.5 | `TestContextBudgetReserveBoundaryAndMismatch`, `TestProviderContextSerializedCeilingRejectsWholeResponse`, live one-byte 413 cases: no partial projection. |
| 3.1 | Whole-item tests and disclosure/lifecycle isolation suites; unsafe evidence produces neither public costs nor reasons. |
| 3.2 | `TestContextAdmissionServicePreservesSectionsAndSummaryPreference`, active-planner quota/order comparison, existing diversity/calibration/protected retrieval regressions. |
| 3.3 | Zero-citation tests and `TestContextAdmissionServiceContinuesPastOversizedProfile`; bounded visits/TopK assertions, mandatory canonical evidence and citation deduplication. |
| 3.4 | Internal chunk/projection whole-evidence tests and unsupported-derived-section gate test; public live checks retain PC2 closed gates. |
| 3.5 | Unsafe/zero fixtures and live high/restricted/forgotten/deleted/foreign safe-empty checks; ordered fixed labels and bounded outcomes. |
| 3.6 | `TestProviderContextOldBindingReadsCurrentAuthorityAndFailsClosed`, dependency-503 regression and invalid-metadata-before-budget regression. |
| 4.1 | Canonical/nonempty actual HTTP contract tests, finalized-body ceiling tests, both-mode fresh evidence. |
| 4.2 | Consumer malformed substitutes, schema evidence, different-allocation HTTP rejection, omitted/zero/sections/discovery binding tests; adapter-only/missing evidence cannot pass. |
| 4.3 | UTF-8/escaping/long paths/repeated citation and reserve/digit-width tests, actual-body fresh usage validation, no partial 413 output. |
| 4.4 | Full retrieval scope/lifecycle/disclosure/diversity/protected baseline plus native/ample/tight admission comparisons; raw public diagnostics remain unsupported. |
| 5.1 | Official bootstrap, discovery versions, non-root governed ingestion, database raw-event/canonical completion; seven completed labeled events in both stages. |
| 5.2 | Initial report: exact/prefix, both modes, zero/omitted/oversized/minimum, safe omissions, lifecycle/scope and actual body counts. |
| 5.3 | Restart report: durable labels, governance, same identity/citation/budget hashes, allocations and grant checks after API/worker restart. |
| 5.4 | Linked stage reports pin source/image/OpenAPI/disclosure/budget identities; script requires passing stages, conformance/readiness tests reject missing or degraded evidence. Internal section proof is labeled below. |
| 6.1 | README, Provider integration guide, OpenAPI exact examples and documentation example tests. |
| 6.2 | Architecture, components, best practices, self-hosting and disclosure/source docs; `.env.local.example` and Compose settings; docs/config gates. |
| 6.3 | MCP integration guidance and `skills/stele-memory/SKILL.md`: extension discovery, closed sections, authority, request-bound accounting and no local ranking/packing/token estimates; documentation tests and review. |
| 6.4 | Updated roadmap keeps PC1/PC2 archived, PC3 implemented/verified and archived as 071, PC4–PC6 pending; this verification record links tasks, commands and fresh evidence. |
| 6.5 | Final gates below. |

## Internal section evidence scope

`TestContextAdmissionInternalChunkUsesWholeEvidence` and
`TestContextAdmissionInternalProjectionUsesWholeEvidence` use authorized internal
fixtures to prove whole multibyte content and mandatory citations reach final
admission without earlier `CharacterBudget` clipping. Oversized whole evidence
is refused. `TestContextAdmissionDoesNotBypassWithUnsupportedDerivedSections`
proves unsupported insights do not bypass canonical admission. These are focused
internal tests; public live verification does not claim those paths are enabled.
PC4 continuity and PC5 durable reference APIs remain pending.

## Review corrections

Independent review identified metadata error mapping/order, privileged planner
allocation leakage and request-unbound conformance evidence. Each issue was
reproduced by a failing test, corrected and independently re-reviewed. The
focused review run passed all four affected packages with no remaining critical
or important finding within that review scope.

## Final commands

Go commands used the existing dependency/build caches and
`GOPROXY=https://goproxy.cn,direct`.

| Command | Result |
| --- | --- |
| `go test ./... -count=1 -timeout 15m` | Passed all 25 packages; environment-dependent live skip is excluded from live acceptance. |
| `go test -race ./internal/provider ./internal/retrieval ./internal/app ./internal/assurance -count=1 -timeout 10m` | Passed all four affected packages. |
| `go vet ./...` | Passed. |
| Docker build with `docker.1ms.run` Go/Debian images and `STELE_GOPROXY=https://goproxy.cn,direct` | Passed; final image pinned above. |
| `pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 -ProjectName stele-pc3-20261011-2226 -Image stele-pc3-verify:local -ReportPath openspec/changes/provider-context-budget-and-selection-diagnostics/evidence/provider-context-pc3.json` | Passed initial and restart, fresh pgvector, owned cleanup. |
| `pwsh -NoProfile -File scripts/check-documentation.ps1` | Passed both documentation packages. |
| `openspec validate provider-context-budget-and-selection-diagnostics --strict` | Passed. |
| `openspec validate --all --strict` | Passed all 87 items, zero failures; requirement-length notices are informational. |
| `git diff --check` | Passed. |

## Archive verification

Archived with the repository sequencing script as change 071. Five main specifications were synchronized (10 added and 5 modified requirements). The live command above records its original pre-archive report path; the preserved evidence now lives beside this record under `evidence/`. Roadmap status and links were updated, and no active change remains.
