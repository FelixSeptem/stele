## Context

See `proposal.md` for motivation. Retrieval already provides bounded lexical,
semantic, relation, and authorized chunk recall; versioned stable fusion;
identity/lineage deduplication; diversity-aware selection; citations; and
exact-scope rollout resolution. The missing architectural layer is a bounded
query analysis step that can add retrieval signals without replacing the
caller's query or creating a second ranking and safety path.

The public search and context contracts must remain compatible. PostgreSQL is
the sole system of record, pgvector and PostgreSQL full-text search remain the
recall stores, and every query path must preserve tenant, project, namespace,
optional session/user, lifecycle, class, and time constraints. There is
currently no explicitly owned evaluation DSN in the local environment, so real
PostgreSQL and pgvector activation evidence cannot be manufactured as part of
planning.

## Goals / Non-Goals

**Goals:**

- Introduce a pure, versioned analysis boundary whose results are deterministic,
  bounded, replayable, and safe to discard.
- Preserve one stable retrieval pipeline for original and derived signals, with
  explicit fan-out and candidate budgets.
- Reuse exact-scope rollout state and make original-only behavior the automatic
  fallback and rollback target.
- Provide enough redacted evidence to evaluate temporal and multi-hop gains,
  protected regressions, safety, and operational cost.

**Non-Goals:**

- Selecting or calling an online language model, embedding model, or external
  search service during query analysis.
- Introducing learned reranking, feedback-aware features, or any Phase 6 Task
  6.6 behavior.
- Defining a second persistence model, rollout mechanism, public request flag,
  or public response representation for query plans.
- Activating decomposition during implementation; activation remains an
  operator rollout decision after real-stack gates pass.

## Decisions

### 1. Use a pure provider-independent analyzer

The analyzer accepts the immutable query plus already-authorized request
constraints and a selected policy/limit version. It returns a value containing
the policy identity, normalized signal, bounded aliases or terms, optional
entity/time/class/intent hints, bounded subqueries, and an explicit disposition.
It performs no database reads, network calls, or canonical-memory writes.

This keeps replay deterministic, prevents analysis from discovering foreign
scope data, and makes timeout or malformed output equivalent to a discardable
optional result. The initial implementation will use deterministic rules only.
An interface boundary can permit a future provider, but this change adds no
provider implementation or dependency.

Alternatives considered:

- An online model analyzer could handle broader language but adds privacy,
  latency, cost, availability, and reproducibility risks before the bounded
  safety path is proven.
- Normalization without decomposition is simpler but does not address the
  roadmap's multi-hop retrieval objective.

### 2. Treat the original query as the mandatory first signal

The orchestrator constructs an ordered signal set with the original query
first. It validates, canonicalizes for equality only, removes duplicate derived
signals, and stable-sorts the remaining derived signals using documented policy
ordering. Neither normalization nor hints mutate the original value.

This makes baseline preservation structural instead of relying on every
analyzer implementation to remember a fallback. It also permits shadow
comparison from the same accepted input.

Alternatives considered:

- Replacing the query with its normalized form creates precision regressions
  that cannot reliably fall back.
- Allowing analyzer-specific ordering makes candidate truncation nondeterministic.

### 3. Fan out into the existing bounded recall and merge path

Each eligible signal uses the same resolved constraints and bounded lexical,
semantic, relation, and chunk recall adapters as the original query. Global
limits cap analysis work, eligible signals, subqueries, per-signal candidates,
aggregate candidates, and elapsed work. Signal provenance is carried only far
enough for authorized aggregate diagnostics.

Candidates then enter the existing versioned stable fusion, identity/lineage
deduplication, diversity selection, citation, and final budget stages. The
orchestrator does not sum lexical, semantic, or signal-local raw scores. Stable
ranks and existing fusion rules remain the comparison boundary, so adding a
signal cannot create a second ranking system.

Alternatives considered:

- Adding raw scores across subqueries would mix incomparable score scales and
  reward duplication.
- Giving each subquery a separate result quota would bypass global diversity
  and context budgets.

### 4. Constrain hints; never let them broaden the request

Request-resolved scope, lifecycle, class, and time constraints are authoritative.
An analyzer hint may narrow an unconstrained dimension or influence bounded
signal construction, but it cannot introduce a different scope identity,
expand an explicit time window, or authorize a hidden lifecycle state. A
conflicting or unrecognized hint is discarded with a stable category.

The analyzer receives no repository capable of looking up entities. Therefore
entity aliases are lexical aids, not authority-bearing identifiers, and cannot
trigger broader source fetches.

Alternatives considered:

- Treating extracted entities or dates as filters unconditionally risks false
  negatives and scope expansion from parser errors.
- Resolving entity hints against the database inside the analyzer couples
  understanding to persistence and makes isolation auditing harder.

### 5. Reuse exact-scope ranking rollout governance

Query-analysis parameters live in the existing exact-scope rollout lifecycle
and resolution rules: diagnostics-only, shadow, active, disabled, and rollback.
If the existing persisted policy payload can carry a typed, versioned
query-analysis section, implementation reuses it without schema expansion. A
forward-only migration is added only if current storage cannot represent the
version and bounds safely.

Diagnostics-only validates analysis without multi-signal recall. Shadow runs a
bounded candidate comparison but returns original-only results. Active permits
derived signals to affect the common pipeline. Any absent, malformed,
foreign-scope, expired, disabled, or rolled-back policy resolves to
original-only behavior.

Alternatives considered:

- A second rollout table and resolver duplicates isolation rules and creates
  conflicting policy precedence.
- A global feature switch cannot support exact-scope evidence and rollback.

### 6. Emit aggregate diagnostics, not query plans

Authorized diagnostics include analysis/policy version, original-retained and
normalization dispositions, entity/time/class/intent categories or counts,
subquery and signal counts, fallback category, rollout stage, candidate-pool
counts, and elapsed budget. They do not contain normalized strings, aliases,
subquery text, raw plans, provider reasoning, hidden/foreign identifiers, raw
candidates, or unbounded score lists. Ordinary responses receive no new fields.

This supplies reproducibility and release-gate inputs without turning logs or
API payloads into a second store of sensitive query or evidence content.

Alternatives considered:

- Logging full plans simplifies debugging but creates privacy, retention, and
  cross-scope disclosure risk.
- Exposing plans in public results makes an internal rollout contract permanent.

### 7. Gate activation on two compatible real-stack comparisons

Evaluation first requires compatible, explicitly owned PostgreSQL and pgvector
evidence that the preceding identity/lineage diversity policy meets duplicate
rate, protected coverage, candidate-pool, latency, isolation, and lifecycle
gates. It then compares the analyzed candidate with the immutable original-query
baseline under compatible fixture, representation, fusion, ranking, analysis,
and release-policy versions.

The query suite adds temporal, entity-centric, mixed-language, ambiguous,
multi-hop, malformed, and adversarial cases while retaining protected simple
facts. Reports record aggregate fallback and bound usage. Missing explicit
evaluation DSN produces `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`, which is
non-pass and never falls back to `STELE_POSTGRES_DSN` or another ambient DSN.

Synthetic replay remains useful for deterministic development tests, but it
cannot authorize active decomposition.

## Risks / Trade-offs

- [Expansion can reduce precision or crowd out strong original hits] → Keep the
  original signal mandatory, cap derived fan-out, compare protected per-category
  metrics, and retain immediate exact-scope rollback.
- [Fan-out increases latency and candidate volume] → Enforce analysis,
  per-signal, aggregate-candidate, and elapsed budgets and report their use in
  evaluation.
- [Entity or temporal parsing can falsely narrow or broaden retrieval] → Treat
  hints as non-authoritative, reject conflicts, never widen explicit constraints,
  and cover ambiguous/adversarial cases.
- [Diagnostics can leak caller text or evidence] → Emit only allowlisted
  aggregate categories, versions, counts, and durations through authorized paths.
- [Map iteration or alias generation can make replay unstable] → Use explicit
  stable ordering, canonical equality, deterministic truncation, and replay tests.
- [Reusing rollout storage couples query analysis to ranking policy evolution] →
  Use typed/versioned payload sections and fail closed on unknown versions;
  migrate only if current representation is insufficient.
- [Synthetic evidence can be mistaken for release evidence] → Encode the owned
  real-stack prerequisite and stable non-pass skip state in the release decision,
  not only in operator documentation.

## Migration Plan

1. Add analyzer domain contracts, bounds, deterministic rules, and pure tests
   while query analysis remains disabled by default.
2. Extend existing exact-scope policy validation and persistence. Add a
   forward-only PostgreSQL migration only if versioned query-analysis parameters
   cannot fit the current policy representation without ambiguity.
3. Wire diagnostics-only and shadow execution into retrieval with original-only
   public output, bounded telemetry, and fault-injection coverage.
4. Extend versioned fixtures, replay reports, comparisons, and release policy;
   keep synthetic evaluation distinct from real-stack activation evidence.
5. Run the explicit owned PostgreSQL and pgvector prerequisite and candidate
   gates. Only a compatible pass makes an exact scope eligible for active rollout.
6. Activate incrementally by exact scope. Roll back by disabling or reverting
   the policy to original-only behavior; no canonical-memory rewrite or data
   rollback is required.
