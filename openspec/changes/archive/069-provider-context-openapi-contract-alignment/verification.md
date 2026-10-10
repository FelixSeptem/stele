# PC1 implementation verification

This change repairs the public `provider-v1` context wire contract using the
existing assembler and durable governance pipeline. The baseline revision is
`a556ac0ddacc4026a1bd7b02d7123d884185001a`; implementation was developed on
`feat/provider-context-openapi-contract-alignment` and archived as change 069.
All 20 tasks are complete and both delta specifications are synchronized into
the main specifications.

## Automated checks

The following checks passed against the implementation, including the final
selected-citation and dedicated nested DTO corrections:

- `go test ./... -count=1`: all 25 packages passed, including Provider, app,
  retrieval, assurance, OpenAPI, and documentation tests.
- `go vet ./...`: exit 0.
- `openspec validate provider-context-openapi-contract-alignment --strict`:
  valid change.
- `openspec validate --all --strict`: 85 passed, 0 failed. Existing long
  requirement informational messages remain.
- `pwsh -NoProfile -File scripts/check-documentation.ps1`: passed.
- `pwsh -NoProfile -File scripts/check-self-hosting-smoke-docs.ps1`: passed.
- `git diff --check`: passed.

After archive, the complete Go suite and documentation check passed again.
`openspec validate --all --strict` validated the synchronized main specifications
with 84 passed and 0 failed; the active change no longer contributes an extra
validation item. Roadmap status and archive-relative links were updated for 069.

The budget-one HTTP regression first failed because pre-budget candidate memory
and event identifiers appeared in public JSON. It passed after aggregate inner
citations were projected exclusively from returned items. An independent review
confirmed the correction, repeated the focused Provider/app regressions, and
found no remaining critical or important issues.

## Fresh PostgreSQL evidence

The [bounded live report](../../../../testdata/providercontext/pc1-live.evidence.json)
is separate from unit/schema results. The owned verifier uses
`docker.1ms.run/pgvector/pgvector:pg18`, a new database volume, official public
bootstrap, and normal governed `conversation.message` ingestion. Its final
passing stage must be `restart`, with two completed governed events. Checks
cover actual public HTTP/schema conformance, matching capability/OpenAPI digest,
event replay, non-root exact and prefix paths, genuine empty output, foreign
scope denial, lifecycle exclusion, citations, and stable identity/citation
hashes after API/worker restart with the original runtime binding.

The report pins the actual tested image digest and records the host source
revision/fingerprint and OpenAPI digest. The image is built from the worktree
with `STELE_GOPROXY=https://goproxy.cn,direct` before verification. These retained
fields do not independently prove source-to-image linkage through embedded
build metadata. The source revision names the baseline; the source fingerprint
includes the uncommitted repair. No credential, Authorization header, scope
value, raw request/response, memory identifier, or content is retained.

The missing-image negative verification exited nonzero and wrote only
`result=failed`, `category=context_verification_incomplete`, and
`consumable=false`. Unavailable dependencies and an unconfigured skipped live
test cannot substitute for passing live evidence. Owned containers, volumes,
and ephemeral credentials are removed after the run.

## Maintenance and scope

`AGENTS.md` now explicitly requires maintenance documentation updates in the
same change for externally visible semantics. The Provider guide includes
canonical JSON, categorized/empty response contracts, strict failed-contract
handling, selected-only citations, migration notes, and evidence limits.
README, best practices, MCP integration guidance, self-hosting, and roadmap
references were synchronized. The three copyable Provider JSON examples are
validated directly from Markdown against the authoritative schema.

`skills/stele-memory/SKILL.md` was reviewed; existing MCP tool signatures and
flows are unchanged. The MCP guide distinguishes Provider item budgets from
MCP `budget_bytes`.

PC2–PC6 remain pending. This repair does not implement disclosure profiles,
reference-only output, source-trust metadata, complete serialized-response byte
limits, Stele-issued selection digest/continuity, a durable reference contract,
or turn/outcome projection. Ordinary content retains its existing semantics and
the item budget retains its existing clamp behavior. No second selection engine
or memory store is introduced. Archive synchronizes the public contract and
retains this verification record; Git history records commit and integration.
