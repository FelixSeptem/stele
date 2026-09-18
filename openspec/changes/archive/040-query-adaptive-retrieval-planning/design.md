## Context

See `proposal.md` for motivation. Stele already has deterministic query analysis,
lexical/semantic/relation/chunk recall, versioned fusion, quality features,
optional reranking, diversity selection, context assembly, exact-scope rollout,
and retrieval evaluation. `Service.Search` currently orchestrates these stages
with mostly static channel and candidate choices, while query analysis chiefly
adds bounded recall inputs.

The planner must compose these capabilities without becoming another search
provider, persistence system, or reasoning agent. PostgreSQL remains the only
system of record, and every channel continues to enforce the resolved tenant,
project, namespace, lifecycle, and optional lower-scope constraints.

## Goals / Non-Goals

**Goals:**

- Introduce a pure, versioned planner that produces one immutable, replayable
  retrieval plan from validated bounded inputs.
- Make query-family channel selection, candidate allocation, fusion parameters,
  reranker eligibility, memory-class quotas, and context priorities explicit.
- Bound all work inside one request-level candidate, latency, pass, and context
  envelope.
- Support one evidence-triggered follow-up pass without recursion.
- Reuse the current rollout, evaluation, release, telemetry, and rollback
  mechanisms with additive planner identities.
- Preserve public response compatibility and the current baseline when planning
  is disabled, incompatible, unavailable, or rejected.

**Non-Goals:**

- The planner will not call an LLM, embedding provider, database, or network
  service while classifying a query or constructing a plan.
- The planner will not add bi-temporal fact fields or recursive relation paths.
- The planner will not learn online, mutate its policy from request traffic, or
  use raw access popularity as a ranking feature.
- The planner will not produce answers, expose chain-of-thought, or run more
  than two total retrieval passes.
- This change will not replace existing searchers, fusion, reranking, diversity,
  context assembly, or release policy implementations.

## Decisions

### 1. Use a pure planner and a separate bounded executor

Add focused planner types under `internal/retrieval` rather than expanding the
query analyzer into an execution engine. The planner input contains the accepted
query identity, validated query-analysis result, already authorized narrowing
constraints, a versioned planner policy, and hard runtime limits. It contains no
repository, provider, scope-discovery, or network capability.

The planner output is an immutable `RetrievalPlan` with planner, policy,
analysis, fusion, ranking, and renderer identities; one query family; enabled
existing channels and ordered per-channel budgets; one total candidate budget;
memory-class quotas and context priority; reranker headroom; at most two passes;
latency/context envelopes; and an explicit baseline fallback.

`Service.Search` remains the executor. It validates the plan against hard limits
before any channel runs and accounts consumption through a request-local budget
ledger.

### 2. Use deterministic rule-based query families

The initial planner recognizes `exact_lookup`, `semantic`, `temporal`,
`entity_relation`, `multi_hop`, `procedural`, and `general`. Classification uses
only bounded analysis categories, signal kinds/counts, caller constraints, and a
versioned precedence table. It does not inspect repository state or call a model.

### 3. Treat budgets as a shared request envelope

The policy declares both soft allocations and hard ceilings. A budget ledger
tracks candidate requests and accepted candidates per channel/pass, elapsed
latency buckets, reserved reranker headroom, and context allowance. The planner
may redistribute only unused soft allocation; it cannot increase the hard
request envelope. The existing default total of 200 fusion candidates is the
baseline policy ceiling for the first implementation.

### 4. Make the follow-up pass evidence-triggered and non-recursive

After pass one, the executor builds a bounded `EvidenceAssessment` from visible
candidate counts, required fixture/evidence-group coverage where available,
post-filter attrition category, duplicate disposition, channel availability,
and remaining budget. The immutable plan contains the only permitted follow-up
action, and pass two is terminal.

### 5. Require intersecting authorization for optional reranking

`RetrievalPlan.RerankerEligible` is only an eligibility signal. Actual use also
requires existing runtime provider configuration, compatible exact-scope
rollout, evidence identity, candidate/token limits, timeout, and ledger headroom.

### 6. Extend the existing scoped ranking policy additively

Add planner identity, policy version, mode, and bounded policy payload to the
current scoped ranking rollout model using an additive PostgreSQL migration.
Reuse exact-scope lookup, expiry, activation, rollback, audit, and evidence
compatibility rather than creating a parallel rollout store.

### 7. Keep diagnostics aggregate and low-cardinality

Diagnostics contain only identities, query-family, channel availability,
budget/pass buckets, evidence/fallback categories, changed-rank counts, and
latency buckets. Raw text, scope values, IDs, content, scores, credentials, and
provider payloads are excluded.

### 8. Evaluate per family before active rollout

Activation requires deterministic offline evidence plus an explicitly owned
PostgreSQL and pgvector run. Aggregate improvement cannot offset regression in a
protected family or any scope, lifecycle, budget, latency, fallback, or rollback
failure.

## Risks / Trade-offs

- **Rule families may misclassify ambiguous queries** → Fall back to `general`
  and keep specialized templates diagnostics/shadow-first.
- **Adaptive budgets can increase p95 latency** → Enforce one aggregate ledger
  and make resource overflow a hard release failure.
- **Shadow mode can nearly double work** → Require explicit bounds, sampling,
  and exact-scope enablement.
- **A second pass may add duplicates** → Reuse canonical identity, lineage
  deduplication, fusion, and diversity; measure incremental evidence separately.
- **Planner fields can make rollout identities incompatible** → Validate the
  full dependency tuple and fail to baseline.
- **Search orchestration may grow** → Keep planner, ledger, evidence assessment,
  and executor helpers in focused files.

## Migration Plan

1. Add pure planner, budget-ledger, and evidence-assessment types with tests.
2. Add an idempotent additive migration and planner rollout persistence.
3. Integrate diagnostics-only planning, then shadow execution.
4. Extend evaluation and release gates.
5. Permit `active_for_scope` only after compatible evidence passes.
6. Roll back by disabling the exact-scope planner rollout; no canonical data is rewritten.

## Implementation Notes

- Dependency review: planner validation needs only fixed enum membership,
  deterministic slice/map copying and ordering, and bounded numeric/duration
  checks. The Go standard library already supplies these primitives. General
  validation packages would add reflection, tags, and another compatibility
  surface without reducing operational risk, so v1 adds no dependency.
