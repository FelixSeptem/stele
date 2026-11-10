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

## Task-appropriate representation comparison

The EV3 evaluator compares the stable baseline with one compatible derived
representation under equal, declared limits. It has four repository-owned task
families:

| Task family | Candidate representation | Companion channel |
| --- | --- | --- |
| Exact terms and numbers | `exact-excerpt-with-search-v1` | Authorized source search |
| Cross-document evidence | `topic-projection-with-search-v1` or `hierarchical-search-v1` | Authorized source search |
| Profile preference | `topic-projection-with-search-v1` | Authorized source search |
| Evidence-backed procedural lesson | `hierarchical-search-v1` | Authorized source search |

Fixtures lock source and qrel checksums, exact scope, required and excluded
evidence, source watermark, renderer identity, candidate identity, and equal
candidate/character/token/latency limits. The comparison fails closed on a
foreign or lifecycle-hidden source, stale/incompatible identity, missing source
search, lost critical fact, broken citation, over-budget work, or
nondeterministic replay. Aggregate utility, repeated-error, or maintenance
observations are externally supplied and observational only: they cannot offset
a hard failure or establish that a representation caused an outcome.

Both baseline and candidate must satisfy these gates. Every returned citation
must resolve; a valid sibling citation cannot hide a broken target. Derived
evidence uses the same fixed-clock current-valid-time predicate as source
retrieval, including exclusive `valid_to` and future `valid_from` rejection.

Reports contain task-family and bounded aggregate metrics only: fact/source/
citation retention, count and size limits, latency, stale/noisy and
broken-reference counts, maintenance-observation coverage, and rollback state.
They exclude task/query/answer/source text, aliases, scope values, raw IDs,
raw scores, credentials, provider payloads, and per-stage costs. The PostgreSQL
report repository is append-only except scoped expiry cleanup; PostgreSQL stays
the only system of record.

The locked fixture includes source content, queries, exact critical values and
excluded lifecycle records. Replay validates both checksums even when the
caller bypasses the JSON loader. Each case executes the flat baseline, the
candidate twice, and the flat baseline again for rollback. Returned literal
excerpts, canonical versions and resolvable citations determine retention;
item counts cannot establish that a number survived. Candidate fact coverage
is measured on its representation, so companion search cannot hide a lost fact.
Search and representation sizes both count toward the candidate's limits.

Reports preserve baseline metrics and candidate-minus-baseline deltas. Explicit
task success/failure, repeated errors and maintenance cost remain separate from
coverage; absent observations and their deltas stay inconclusive. Token counts
use whitespace-separated fields for this fixture and are not model usage.
Measured latency is observational and excluded only from replay equivalence;
the report hash still binds the measured value. The owned corpus is synthetic
and narrow: executing it on real PostgreSQL proves service behavior, not a
general task benefit or a causal model improvement.

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

For the owned PostgreSQL + pgvector replay, run:

```powershell
pwsh -NoProfile -File scripts/stele-representation-evaluation-verify.ps1
```

The verifier starts and removes only its randomly named disposable container,
uses an explicit owned DSN rather than a runtime DSN, and emits a bounded JSON
report. It verifies all four task families, restart/replay persistence, source
search retention, rejection paths, expiry cleanup, and baseline rollback.
