## Context

Stele's retrieval service already executes lexical, semantic, relation, and
controlled chunk-candidate searches under one resolved scope. It currently
deduplicates by canonical memory ID and adds the channel scores, which couples
the final ranking to the implementation and score scale of each provider. The
archived retrieval-evaluation baseline can measure channel/rank behavior, and
the archived chunking change supplies another bounded candidate source, so this
is the right point to establish a stable merge boundary.

The design must preserve the repository's non-negotiable contracts: PostgreSQL
remains the system of record, raw events and canonical memory remain immutable,
every candidate is scope- and lifecycle-checked, public hits remain canonical
and citation-backed, and a ranking experiment must be reversible without a data
migration.

## Goals / Non-Goals

**Goals:**

- Define one versioned fusion interface for bounded ranked candidates from each
  enabled recall channel.
- Make deterministic Reciprocal Rank Fusion (RRF) the default strategy while
  retaining an explicit normalized-weighted strategy for offline comparison.
- Keep channel failures and empty optional channels from widening scope or
  making the safe canonical path unavailable.
- Reuse the existing scoped ranking rollout controls for diagnostics, dry runs,
  activation, disablement, and rollback rather than creating a second approval
  system.
- Produce bounded, redacted channel and fusion diagnostics for evaluation/admin
  consumers and stable metrics for comparing named strategy versions.

**Non-Goals:**

- Semantic similarity clustering, identity deduplication beyond the existing
  canonical merge, MMR, or diversity-aware packing.
- Query rewriting/decomposition, temporal or entity analysis, feedback-aware
  features, or model-based reranking.
- Changes to public response fields, canonical memory state, chunk storage, or
  source provenance.

## Decisions

### 1. Fuse ranked channel lists, not raw scores

Each channel returns a bounded list with a stable candidate identity, channel
name, channel rank, canonical parent, and source/citation metadata. The fusion
layer assigns ranks after per-channel validation and does not use raw lexical,
vector, or relation scores to determine cross-channel dominance.

**Rationale:** lexical and vector scores are not comparable across providers or
index revisions. Rank is the common, deterministic signal that survives those
changes.

**Alternative rejected:** continue adding raw scores. It is simple but silently
changes behavior when a provider or score normalization changes.

### 2. Use weighted RRF with explicit versioned parameters

The default strategy is a named RRF version with a positive rank constant and
explicit per-channel weights. For a candidate `c`, the fused score is the sum
of `weight(channel)/(rank_constant + rank(channel))` over the channels that
returned `c`. The default weights and rank constant are part of the strategy
version, validated as bounded values, and never inferred from provider scores.

**Rationale:** RRF provides a stable baseline while allowing relation or chunk
channels to be intentionally enabled without making their raw scores dominant.

**Alternative rejected:** unweighted RRF as the only behavior. It hides an
important product decision when optional channels are less reliable or sparse.

### 3. Keep normalized weighted fusion as an experiment

The implementation may normalize each channel's scores into a bounded range and
apply explicit weights, but this strategy is selected only by an evaluation or
scoped rollout configuration carrying its complete version and parameters. It
cannot become the implicit production fallback.

**Rationale:** preserving the experiment lets the evaluation baseline compare
  RRF with score-aware alternatives without coupling default retrieval to them.

**Alternative rejected:** remove score-aware fusion entirely. It would prevent
future evidence-based comparison and make provider calibration work invisible.

### 4. Validate before fusion and retain canonical identity

The service validates resolved scope, lifecycle state, class filters, parent
identity, and bounded citations before a candidate enters a channel list. A
candidate from multiple channels is represented once by its canonical memory ID;
the selected public hit remains the canonical memory, while channel ranks and
chunk/source citations stay internal or authorized diagnostics.

**Rationale:** fusion must not become a side door around existing isolation or
  lifecycle controls, and public clients should not need to understand physical
  candidate representations.

**Alternative rejected:** fuse first and filter later. That can allow hidden or
  foreign candidates to affect rank and can leak their existence through
  diagnostics or score changes.

### 5. Reuse ranking rollout policy with an explicit fusion version

Fusion selection is represented as a versioned strategy reference on the existing
scope-aware ranking rollout surface. `diagnostics_only` and `dry_run` compare the
candidate strategy without changing ordinary results; `active_for_scope` uses it
only after the existing activation gate is satisfied; disabled or rolled-back
policies return to the canonical baseline strategy. If no fusion selection is
configured, the service uses the safe baseline behavior defined by the current
deployment and records the effective strategy version internally.

**Rationale:** the repository already has durable scope grants, activation gates,
  audit reasons, and rollback records for ranking changes.

**Alternative rejected:** add a separate fusion policy table and lifecycle. That
  would duplicate authorization and create conflicting rollout state.

### 6. Degrade per channel, fail closed on invariant violations

An unavailable optional channel contributes no candidates and is recorded as a
bounded channel status. A failure in the required canonical retrieval path keeps
its current error behavior. Invalid scope, lifecycle, lineage, or malformed
strategy data is a hard failure or omission according to the existing retrieval
contract; it never widens the candidate pool or falls back to hidden data.

**Rationale:** optional relation/chunk retrieval should not make basic retrieval
  unavailable, while security and data-integrity checks must remain fail-closed.

### 7. Deterministic final ordering

After fusion, candidates are ordered by fused score descending, then explicit
memory-class policy priority, source timestamp descending, and stable canonical
memory ID. The same strategy version, candidate lists, and visible source state
must yield the same order across replay runs.

**Rationale:** deterministic output is required for reproducible evaluation,
  cache behavior, and rollback comparison.

## Risks / Trade-offs

- [RRF can underuse a high-quality channel with a sparse rank list] → Keep
  channel weights explicit, bound candidate-pool sizes, and compare protected
  evidence metrics before activation.
- [A rollout policy may accidentally select an incomplete strategy definition]
  → Validate strategy name, version, rank constant, weights, and channel set
  before persistence or evaluation; reject incomplete policies.
- [Channel failure categories could expose provider or scope details] → Emit
  low-cardinality channel status categories only; keep raw errors in bounded
  internal logs under existing redaction rules.
- [Fusion changes can alter result order even when recall is unchanged] → Keep
  the baseline strategy selectable, record per-channel ranks in authorized
  diagnostics, and require dry-run evidence before scope activation.
- [Additional candidate pools increase work] → Enforce per-channel and total
  candidate bounds before fusion and preserve existing top-k and context
  budgets.

## Migration Plan

1. Add strategy/domain validation and the fusion interface without changing the
   default result path.
2. Implement RRF and normalized-weighted fusion with deterministic unit tests.
3. Adapt lexical, semantic, relation, and chunk candidate collection to emit
   bounded channel lists and preserve existing lifecycle/scope checks.
4. Wire the existing ranking rollout and evaluation replay to named fusion
   versions; keep diagnostics-only and dry-run modes available first.
5. Run the retrieval baseline and protected-category regression suite. Activate
   RRF only for explicitly approved scopes after the existing rollout gate.
6. If metrics regress or an invariant fails, disable/rollback the fusion policy;
   no schema downgrade or canonical-data rewrite is required.

## Open Questions

- Should the first RRF version use equal channel weights, or a slightly lower
  default weight for optional relation/chunk channels until real-stack evidence
  is available?
- Should the effective fusion version be persisted in every evaluation report
  only, or also in bounded retrieval operation telemetry?
- Do existing ranking rollout APIs need additive fields for strategy reference,
  or can the first implementation keep the selection in an internal evaluator
  configuration until a durable scope activation is requested?
