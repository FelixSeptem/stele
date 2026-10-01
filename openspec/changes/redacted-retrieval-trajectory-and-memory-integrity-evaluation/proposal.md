## Why

Stele already has retrieval release-gate and progressive-context evidence
contracts, but its remaining diagnostic surface does not provide one bounded,
redacted view of retrieval paths together with separate evidence that memory
organization actions preserved information. The next proposal should make
those safety and effectiveness signals reviewable before any future retrieval
strategy or insight policy is expanded.

## What Changes

- Add a new evaluation capability for bounded, redacted retrieval trajectory
  aggregates and memory-organization integrity reports.
- Record only low-cardinality channel, candidate-count, expansion,
  disposition, fallback, freshness, budget, and latency categories; never
  persist raw queries, scope values, memory/event identifiers, hidden
  candidates, raw scores, provider payloads, credentials, or prompts.
- Separate action-success outcomes from information-integrity outcomes for
  consolidation, merge, reclassification, reflection, and context-projection
  changes.
- Treat scope leakage, lifecycle leakage, missing or altered protected
  evidence, nondeterministic replay, stale inputs, and retention violations as
  hard integrity failures independent of retrieval quality gains.
- Expose reports only through authorized evaluation/admin surfaces with exact
  scope resolution, bounded pagination, deterministic retention cleanup, and
  append-only report history.
- Support deterministic replay and rollback evidence without changing default
  retrieval, context assembly, canonical memory, or reserved-insight
  activation behavior.
- Extend observability and release-evidence contracts so the new reports can
  be compared only when fixture, policy, strategy, and source-watermark
  identities are compatible.
- Update the roadmap so archived change 050 is no longer listed as active and
  this proposal is recorded as the immediate next proposal.

## Non-goals

- No change to public search or context response payloads for ordinary callers.
- No activation of `goal`, `contradiction`, or `causal_link` insights and no
  widening of the existing reviewed-`hypothesis` policy.
- No raw trajectory, prompt, chain-of-thought, provider response, credential,
  or cross-scope identifier storage.
- No new canonical store, graph database, SDK, UI, hosted product, or provider
  dependency.
- No automatic rollout of progressive context, parent-first retrieval,
  reranking, or other experimental strategy based on these reports.

## Capabilities

### New Capabilities

- `redacted-retrieval-trajectory-and-memory-integrity-evaluation`: bounded,
  redacted trajectory aggregates, separate action-success and
  information-integrity reports, deterministic replay, retention, and
  rollback evidence.

### Modified Capabilities

- `retrieval-release-gate-and-progressive-context-evaluation`: require the new
  trajectory and integrity evidence as compatible, redacted, safety-gated
  inputs without allowing them to authorize rollout by themselves.
- `retrieval-release-evidence-run`: include bounded trajectory and
  memory-integrity results in owned evaluation reports and fail closed on
  missing, stale, incompatible, or unsafe evidence.
- `admin-inspection-surface`: expose authorized, exact-scope inspection of the
  new reports with redacted output and bounded retention/deletion outcomes.
- `service-observability`: add low-cardinality metrics for trajectory,
  integrity, replay, retention, and hard safety failures.

## Impact

- Affected areas include evaluation/release evidence services, retrieval and
  context diagnostics, memory organization verification, PostgreSQL migrations
  and repositories, admin/OpenAPI inspection, metrics, replay, and retention
  tests.
- PostgreSQL remains the only system of record; all reports are derived,
  append-only, exact-scope, and rebuildable from source records.
- Default API retrieval, context assembly, canonical memory lifecycle, and
  reserved-insight activation remain unchanged.
- Related workflow commands include `openspec status`,
  `openspec instructions`, `openspec validate --all --strict`, and the normal
  focused/full verification suite before implementation and archive.
