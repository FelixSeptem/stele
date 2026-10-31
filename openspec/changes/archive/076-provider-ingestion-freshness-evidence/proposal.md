## Why

Stele currently exposes aggregate governance backlog and provider conformance
freshness, but it cannot produce a bounded, repeatable account of one event's
progress from durable admission through governance, canonical promotion and
public retrievability. This makes it difficult to distinguish a slow worker,
retryable governance failure, stale projection or disclosure exclusion during
self-hosted operations. MemOS's staged ingestion model is useful here as an
evaluation pattern, but Stele needs a PostgreSQL-backed, exact-scope and
redacted evidence contract rather than another scheduler or memory store.

## What Changes

- Add a versioned, admin/assurance-only ingestion freshness evidence contract
  with bounded stage timestamps, stage dispositions and latency buckets.
- Correlate stages using an opaque event/evidence reference resolved only inside
  the authorized exact tenant/project/namespace scope. Reports must not expose
  event IDs, memory IDs, paths, query text, content, scope values or ranking
  details.
- Distinguish durable admission, governance completion, canonical promotion and
  public retrievability. Public retrievability is an observed authorized read,
  not an assertion inferred from HTTP write success.
- Support deterministic replay of a bounded fixture/report and classify missing,
  stale, retrying, exhausted, lifecycle-hidden and disclosure-ineligible stages
  without turning them into successful retrieval evidence.
- Export only low-cardinality freshness/SLO metrics and retain bounded evidence
  under existing assurance and maintenance retention rules.
- Add focused unit/contract tests and a fresh PostgreSQL + pgvector assurance
  fixture covering worker delay, retry/restart, successful promotion, hidden or
  source-ineligible canonical memory, and exact-scope isolation.

## Capabilities

### New Capabilities

- `ingestion-freshness-evidence`: Bounded, exact-scope, redacted evidence for
  event-to-public-retrieval stage freshness and replay-safe SLO classification.

### Modified Capabilities


## Impact

Affected areas are `internal/assurance`, `internal/governance`,
`internal/storage/postgres`, `internal/jobs`, low-cardinality telemetry,
admin/assurance inspection contracts, OpenAPI capability publication if a
versioned inspection DTO is exposed, and the fresh-stack verification scripts.
Existing event ingestion, governance, retrieval, disclosure and lifecycle
semantics remain authoritative. No new external dependency, graph store,
client SDK, UI or Agent execution path is introduced.

## Non-goals

- Do not add a second queue, scheduler, canonical store or client-owned event
  ledger.
- Do not expose operational freshness details through public Provider context,
  retrieve, event acknowledgments or ordinary runtime responses.
- Do not claim public retrievability from candidate promotion alone; current
  scope, lifecycle, source and disclosure eligibility must still be observed.
- Do not execute an Agent, invoke a model, judge task success, rerank results or
  alter memory based on freshness evidence.
- Do not weaken fail-closed scope, lifecycle, source metadata or disclosure
  behavior to make a stage appear fresh.

Related workflow references: `openspec validate --all --strict`,
`scripts/stele-provider-context-verify.ps1`, and the repository's
`verification-before-completion` and `openspec-archive-seq.ps1` gates.
