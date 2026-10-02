# Progressive Context and Hierarchical Retrieval Experiments

Stele evaluates progressive context and parent-first retrieval as derived,
versioned experiments over the existing PostgreSQL and pgvector retrieval
path. The experiment vocabulary is:

- L0: a short retrieval projection for coarse recall.
- L1: a medium session or context overview for planning and reranking.
- L2: canonical or chunk evidence for detail and citations.

Every level is bound to a policy and renderer identity, an exact scope, a
source watermark, a freshness window, a budget, and a deterministic rebuild
identity. These artifacts are derived evidence. They never become canonical
memory and never replace the source lifecycle or provenance records.

## Execution Boundary

Progressive levels and parent-first expansion run in `offline` or `shadow` mode.
The stable `flat-fusion-v1` strategy remains the selected strategy for ordinary
retrieval and context assembly. Shadow output is available only to authorized
evaluation or diagnostic callers and is omitted from ordinary public responses.

Parent-first planning selects a validated parent projection or chunk, then
expands only children or adjacent chunks whose source version, lifecycle,
tenant, project, namespace, session, and temporal validity can be proven. The
plan stops when candidate, depth, token/character, or latency limits are
reached. A stale, hidden, foreign, missing-lineage, or over-budget item fails
closed and does not trigger a broader lookup.

## Evidence and Rollback

An experiment report contains only bounded identities, aggregate counts,
freshness, citation coverage, budget outcomes, fallback categories, and rollback
state. It excludes queries, prompts, source text, raw scores, provider payloads,
DSNs, credentials, scope values, and memory identifiers. Reports are stored as
append-only derived records with bounded expiration and exact-scope predicates.

Quality improvement cannot override an isolation, lifecycle, freshness,
integrity, deterministic replay, latency, or rollback failure. A non-pass run
falls back to the stable baseline and is not activation-consumable. Activation,
when considered in a later change, requires the existing retrieval release
evidence handoff and an independently authorized rollout policy.

## Verification

Focused checks cover deterministic level identities, hidden/foreign/stale
evidence, bounded parent-first expansion, redacted reports, exact-scope
repository reads, expiration cleanup, migration ordering, and low-cardinality
telemetry. The full verification gate is:

```powershell
go test ./... -count=1
go test -race ./internal/retrieval ./internal/storage/postgres ./internal/telemetry
go vet ./...
openspec validate --all --strict
```
