# Retrieval Quality Baseline

The first repository-owned retrieval replay was executed on PostgreSQL 18 with
pgvector using the `retrieval-fixture-v1` fixture.

| Field | Value |
| --- | --- |
| Fixture version | `retrieval-fixture-v1` |
| Representation version | `canonical-v1` |
| Ranking version | `baseline-v1` |
| Compatible embedding revision | `deterministic-v1` (semantic channel inactive without vectors) |
| Policy version | `quality-policy-v1` |
| Replay cases | 13 |
| Safety failures | 0 |
| Database | PostgreSQL 18 / pgvector |

The replay completed all cases and verified append-only raw-event, canonical-version,
provenance, scope, and lifecycle behavior. The initial fixture queries intentionally
include semantic paraphrases; the current lexical-only replay therefore records a
zero candidate pool for this baseline. This is an explicit measurement result for
the pre-chunking, pre-fusion implementation and is the reference point for later
query understanding, chunk representation, and hybrid ranking changes.

Run the same owned-stack check with:

```powershell
$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = '<owned-pg18-test-dsn>'
$env:STELE_TEST_RETRIEVAL_EVALUATION_OWNED = 'true'
pwsh -File scripts/retrieval-evaluation.ps1
```

The command requires an explicitly owned disposable database and ownership
marker, and never falls back to
`STELE_POSTGRES_DSN` or another ambient operator database.

## Chunk representation shadowing

The hierarchical chunk representation is a derived PostgreSQL projection over raw
events and canonical-memory versions. It is identified by source kind/id/version,
chunk policy, renderer, and deterministic whitespace token-counter versions. Raw
events and canonical memory remain immutable; chunk rows are rebuildable and
append-only.

Chunk materialization and retrieval consumption are controlled independently:

- `default_off` keeps the canonical-memory retrieval path unchanged;
- `shadow` evaluates chunk candidates and emits only bounded diagnostics on an
  authorized evaluation path;
- `active` permits chunk-derived evidence to contribute while retaining the
  canonical parent identity and citations.

Every chunk read or parent/adjacent expansion re-checks exact
`tenant/project/namespace` scope and active lifecycle state. Hidden, stale, or
foreign sources fail closed. Parent and adjacent evidence is bounded by count and
character/token budgets and never broadens a session or user boundary.

Use an explicitly owned PostgreSQL + pgvector DSN for chunk integration checks:

```powershell
$env:STELE_TEST_POSTGRES_CHUNK_DSN = '<owned-test-dsn>'
go test ./internal/storage/postgres -run MemoryChunkPostgres -count=1
```

Disable chunk materialization or consumption to roll back; no destructive migration
or canonical-memory rewrite is required. Derived chunk retention and deletion must
follow the source lifecycle and the operator's PostgreSQL backup/restore policy.

## Stable candidate fusion

The stable hybrid fusion contract treats lexical, semantic, relation, and active
chunk retrieval as bounded ranked channels. The default strategy identity is
`rrf:rrf-v1`. It uses an explicit rank constant and channel weights; for a
visible candidate `c` the contract is:

```text
fused(c) = sum(weight(channel) / (rank_constant + rank))
```

`rank` is the channel's one-based rank, and every bounded validated contribution
from a channel is added once to the candidate's canonical memory identity. The default
parameters are deliberately part of `rrf:rrf-v1`:

- Rank constant: `60`
- Lexical channel weight: `1`
- Semantic channel weight: `1`
- Relation channel weight: `0.8`
- Chunk channel weight: `0.8`
- Per-channel candidate bound: `50`
- Total candidate bound: `200`

Candidates are scope-, lifecycle-, class-, and lineage-validated before fusion,
then represented by one canonical memory identity. Final output ordering is
deterministic: fused score descending, memory-class policy priority, source
timestamp descending, then canonical memory ID ascending. This ordering applies
after aggregation, so a duplicated parent from multiple recall paths cannot
produce two public hits.

Lexical retrieval remains the required canonical path. Semantic, relation, and
authorized chunk retrieval are optional: an empty or unavailable optional channel
contributes no candidates and the result falls back to the remaining validated
channels. Invalid strategy definitions and invalid scope, lifecycle, lineage, or
candidate-bound inputs fail closed; they never widen the candidate pool or make
hidden evidence influence ranking.

`normalized_weighted:normalized-weighted-v1` is an explicit offline comparison
experiment. It normalizes each channel's bounded scores before applying declared
weights and is never selected implicitly. Fusion strategy identity is included in
evaluation metadata so RRF and experimental results remain comparable and
rollback does not require a data migration.

Evaluation and replay diagnostics are available only on authorized local, CI, or
administrative paths. They retain bounded strategy identity, channel/rank state,
availability, and final-disposition categories for visible evaluated results.
They never include raw candidate pools, raw provider scores, hidden identifiers,
scope values, DSNs, credentials, or provider error text.

The Prometheus counter `stele_retrieval_fusion_total` emits only low-cardinality
`strategy`, `version`, `channel`, `availability`, `candidate_count`, and
`outcome` labels. Candidate-count buckets are `0`, `1_10`, `11_50`, and
`51_plus`; labels never contain query text, memory identifiers, policy IDs, or
scope values. The bounded `stele_ranking_rollout_total` counter records a
successful restoration as `reason_code=rollback_restored`.

Run focused contract coverage with:

```powershell
go test ./internal/retrieval ./internal/telemetry -count=1
go test ./internal/app -run RankingRollout -count=1
```

The replay command requires the explicitly owned disposable
`STELE_TEST_RETRIEVAL_EVALUATION_DSN` described above. It must not be supplied
from, or replaced by, `STELE_POSTGRES_DSN`.

## Identity deduplication and diversity-aware context packing

The retrieval pipeline performs identity and validated-lineage deduplication
after scope/lifecycle validation and stable fusion, and before any diversity or
budget selection:

```text
scope + lifecycle validation
  -> bounded recall and stable fusion
  -> canonical/source-event/parent-lineage deduplication
  -> optional semantic clustering and diversity selection
  -> existing section, citation, and token/character budget packing
```

Identity equivalence is established by canonical memory ID, source event ID, or
validated parent-memory lineage. The first candidate in the stable fused order
is the representative; citations are merged deterministically and remain
bounded per representative. An invalid, hidden, expired, suppressed,
forgotten, deleted, or foreign candidate is discarded before grouping and can
never suppress a visible candidate. With no approved diversity policy, this
identity-deduplicated order is the default behavior.

### Versioned diversity policy lifecycle

A diversity policy is versioned by name and version and is resolved only for
the exact `tenant/project/namespace` scope. Its bounded parameters include the
semantic threshold, MMR (or equivalent) relevance/diversity weight, candidate
limit, pairwise-comparison budget, citation limit, and coverage weights for
memory class, source session, entity, and time slice. Semantic clusters are
formed only from candidates with a compatible active embedding revision. If
embeddings, the revision, or the optional computation are unavailable, the
selector reports a bounded `semantic_unavailable` availability category and
falls back to identity-only selection without widening recall or querying
additional metadata.

Rollouts use the existing ranking-rollout governance and have explicit
`diagnostics_only`/`shadow`, `active_for_scope`, `disabled`, and rollback
states. Diagnostics-only and shadow policies never change ordinary search or
context output. Activation requires the normal dry-run, attribution, evidence
threshold, and no-blocker gates; an absent, malformed, disabled, hidden, or
foreign-scope policy is equivalent to no policy. Per-section selection runs
after eligibility and summary preference but before existing budgets, so public
section names, projections, citations, and caller budgets remain unchanged.

Authorized evaluation or admin diagnostics may expose only the policy identity,
aggregate dispositions (`selected`, `duplicate`, `omitted_by_diversity`,
`omitted_by_budget`, `invalid`, or `semantic_unavailable`), bounded counts, and
availability categories. Ordinary retrieval and context responses never
expose cluster membership, similarity values, candidate pools, hidden content,
foreign identifiers, scope values, or provider errors.

Evaluation reports include the diversity-policy name/version and aggregate
dispositions alongside duplicate rate, protected recall, evidence coverage,
candidate-pool size, and bounded latency. Scope or lifecycle leakage is always
a hard failure; protected-recall, multi-hop-coverage, budget, or latency
regressions block activation. Rollback disables the active selection and
restores the prior approved baseline without rewriting canonical memory, raw
events, chunks, provenance, or fusion state; telemetry records
`reason_code=rollback_restored`.

The real-stack replay is intentionally opt-in. It requires an explicitly owned,
disposable PostgreSQL + pgvector DSN in
`STELE_TEST_RETRIEVAL_EVALUATION_DSN`. When the variable is absent, the
evaluator prints `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` and exits with code
`2`; this is a controlled non-pass skip, not a successful evaluation. The
evaluator never falls back to `STELE_POSTGRES_DSN` or any ambient operator
database. Remove the fixture scope after the run and keep reports free of
credentials and raw candidate content.

### Bounded query understanding rollout

Query analysis retains the caller's original query as the first mandatory signal.
Normalization, aliases, hints, and decomposition are deterministic and bounded
by the versioned `query-analysis-v1` and `query-analysis-limits-v1` contracts;
derived signals copy the exact request scope, lifecycle visibility, memory-class,
and time-window constraints. Analyzer errors, malformed output, timeout, or an
ineligible policy fall back to original-only retrieval.

Query-analysis rollout policies use the existing exact-scope ranking governance.
`diagnostics_only` and `dry_run`/shadow may collect bounded aggregate evidence but
cannot alter ordinary ranking; only an exact-scope `active_for_scope` policy may
allow derived signals into the common fusion pipeline. Disabled, expired, foreign,
malformed, and rolled-back policies resolve to original-only behavior.

Authorized diagnostics expose only policy and limits versions, rollout disposition,
normalization/time status, bounded hint/signal/subquery/candidate counts, fallback
category, and elapsed budget. They never include query text, normalized text,
subqueries, scores, plans, identifiers, or scope values. Ordinary search and
context responses retain their existing schemas and do not include these fields
unless an explicitly diagnostic path requests them.

Active decomposition remains ineligible until the real-stack prerequisite and
original-versus-analyzed comparison pass with an explicitly owned disposable
PostgreSQL + pgvector DSN. Without that DSN the stable result is
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`; this is a non-pass skip and must not be
treated as release evidence.

## Query-adaptive retrieval planning

The query-adaptive planner is a provider-independent, deterministic policy
layer over the existing lexical, semantic, relation, and chunk channels. A
plan is identified by the schema, planner, and policy versions and records the
compatible analysis, fusion, ranking, and renderer identities. The supported
families are `exact_lookup`, `semantic`, `temporal`, `entity_relation`,
`multi_hop`, `procedural`, and `general`; classification precedence and all
channel ordering are canonicalized for replay.

Each request receives one shared hard envelope for total candidates,
per-channel candidates, elapsed latency, context items, reranker headroom, and
passes. Per-channel shares may be re-shared inside that envelope by a bounded
`simple`, `moderate`, or `complex` query-complexity category derived only from
validated analysis counts. A complexity allocation must cover every declared
channel, keep each channel within the per-channel hard limit, and preserve the
declared total candidate envelope, so it can never enlarge work or relax a
service hard limit. Class quotas and context priorities are applied after
scope/lifecycle validation. The ledger reserves reranker headroom and
redistributes unused channel capacity without allowing any channel or pass to
exceed the aggregate bound. Planner reranker eligibility is only a hint: the
separately governed reranker policy and the reserved ledger headroom are both
required.

Evidence assessment uses bounded aggregate counts and dispositions only. If the
first pass is insufficient, an eligible plan may run at most one follow-up pass
with the original query, exact scope, lifecycle filters, and remaining budget
preserved. Failed, malformed, unavailable, or over-limit planner inputs fall
back to the approved baseline; there is no recursive retrieval loop and no
planner-generated final answer. The ordinary search and context response
shapes remain unchanged.

Planner rollout is exact-scope and dependency-compatible. `diagnostics_only`
and `shadow` collect authorized, low-cardinality evidence without changing
results. Only an unexpired `active_for_scope` policy with matching dependency
identities can affect retrieval. Disabled, foreign, malformed, expired, or
rolled-back policies fail closed to baseline. Without an explicitly owned
PostgreSQL + pgvector evaluation DSN, the evaluator remains a stable
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` non-pass and active rollout is not
permitted.

The owned planner runner does not treat extension availability or a lexical
proxy as pgvector evidence. It creates and activates the fixture-owned
`planner-unit-x3-v1` vector revision, requires the production semantic query to
return the expected semantic memory with a positive score, and then requires
the planner replay candidate to retain a semantic-channel contribution. Any
failure in vector seeding, activation, direct semantic retrieval, or replay
attribution fails the run.
