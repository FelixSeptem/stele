## Context

The archived stable-hybrid-candidate-fusion change establishes bounded,
deterministic RRF fusion after exact scope, lifecycle, and source-lineage
validation. Hierarchical chunks can contribute derived evidence and context
assembly already has explicit sections, summary preference, and deterministic
character/token budgets. Those controls do not yet prevent several visible
memories that represent one source fact or near-identical evidence from
dominating an assembled context.

This change is a P3 quality step. It consumes fused candidates and context
assembly contracts; it neither changes recall channels nor adds query analysis,
feedback ranking, or a model reranker. PostgreSQL remains the sole system of
record, and all candidate handling retains exact tenant/project/namespace,
session, user, lifecycle, provenance, and citation boundaries.

## Goals / Non-Goals

**Goals:**

- Select more independent, relevant evidence per bounded context budget.
- Remove deterministic identity/lineage duplicates before diversity scoring.
- Make diversity policy versioned, scope-selectable, measurable, and reversible.
- Preserve deterministic output, public response compatibility, redacted
  authorized diagnostics, and zero-leakage invariants.

**Non-Goals:**

- Changing raw-event or canonical-memory persistence, provenance, chunk
  materialization, or stable fusion strategy.
- Adding query rewriting, query decomposition, entity/temporal query analysis,
  feedback-aware ranking, model reranking, or an external retrieval store.
- Exposing scores, cluster membership, candidate pools, or hidden evidence in
  ordinary search or context responses.

## Decisions

### 1. Apply a fixed safe pipeline after validated fusion

The retrieval path SHALL keep its existing validation and fusion order:

```text
scope/lifecycle/lineage validation
  -> bounded channel recall
  -> stable fusion
  -> deterministic identity/lineage deduplication
  -> section-aware diversity selection
  -> existing budget accounting and citation shaping
```

Identity/lineage deduplication groups only candidates whose canonical identity,
source event, or validated parent-memory lineage proves equivalence. The best
representative is selected by the pre-existing stable fused order, so no raw
score comparison or nondeterministic map iteration can influence the result.
Only representatives of already validated, bounded groups proceed to optional
diversity selection.

Alternatives considered:

- Deduplicating in recall channels was rejected because it duplicates policy
  logic and cannot see overlap between lexical, semantic, relation, and chunk
  channels.
- Deferring identity deduplication until after context packing was rejected
  because duplicates would consume ranking and budget work before removal.

### 2. Use a versioned, bounded diversity policy with safe degradation

A diversity policy SHALL have a name, version, bounded parameters, class-aware
coverage weights, and a bounded semantic-similarity threshold. It SHALL use the
existing scoped ranking-rollout governance surface for diagnostics-only, shadow,
active, disabled, and rollback states. A policy applies only to the exact
resolved scope; no policy may expand a tenant, project, namespace, session, or
user boundary.

The default approved behavior is identity/lineage deduplication plus the
existing deterministic order. Diversity selection is opt-in through an approved
scope rollout until evaluation evidence satisfies the release policy. Disabling
the policy returns to that deduplicated baseline without a data rewrite.

Alternatives considered:

- A global default MMR switch was rejected because it would make a quality
  change unmeasured and difficult to roll back by scope.
- A new standalone rollout system was rejected because the existing governed
  scoped rollout controls already record activation and rollback evidence.

### 3. Use deterministic MMR over visible, bounded candidates

For an enabled policy, selection SHALL use deterministic MMR or an explicitly
versioned equivalent. Relevance originates from the stable fused rank/order;
the diversity penalty uses policy-governed similarity and coverage attributes.
Ties SHALL use the same class priority, source timestamp, and canonical-memory
ID sequence used by stable fusion.

Semantic clustering and similarity are optional bounded inputs. They are
permitted only when both candidates have compatible active embedding revisions
and are already visible in the resolved scope. If embeddings, compatible
revisions, or a configured similarity computation are unavailable, the service
shall record a bounded authorized status and use identity/lineage deduplication
without failing or broadening retrieval.

Alternatives considered:

- Provider/model calls for on-demand semantic clustering were rejected because
  they add cost, privacy, availability, and nondeterminism to context assembly.
- Unbounded all-history clustering was rejected because it violates request
  bounds and could compare foreign or hidden evidence.

### 4. Select independently within existing context sections and budgets

Context assembly SHALL invoke diversity selection for each applicable existing
section after its normal eligibility and summary-preference checks. Candidate
attributes may include memory class, validated source session, already-known
entity attribution, and source time slice. An absent attribute is represented
as an explicit unknown category; the selector must not fetch broader source
data to fill it.

The existing section ordering, token/character accounting, citation limits, and
fail-closed packing behavior remain authoritative. Diversity selection can
choose among fitting candidates but cannot increase a requested budget, move an
item across an unauthorized section, or replace required citations.

### 5. Keep diagnostics and evaluation redacted, bounded, and policy-aware

Only authorized evaluation or admin diagnostics may record a policy identity,
aggregate cluster/omission category, selection disposition, and bounded
coverage counters. Ordinary responses retain current shapes and citations.
Evaluation fixtures and reports SHALL compare a named diversity-policy version
with the deduplicated baseline on duplicate rate, evidence coverage, protected
recall, candidate-pool size, latency, and hard isolation/lifecycle outcomes.

## Risks / Trade-offs

- [Semantic similarity incorrectly groups independent facts] -> Make semantic
  clustering policy-versioned, bounded to compatible active revisions, shadow it
  first, and protect required evidence-coverage fixtures before activation.
- [Diversity reduces simple-fact recall or a required section item] -> Preserve
  section and class policy, compare protected Recall@k and multi-hop coverage,
  and roll back to deterministic identity-deduplicated packing.
- [Selection becomes nondeterministic] -> Use bounded inputs, stable sorted
  candidates, explicit tie breaks, and repeat-replay tests.
- [Diagnostics expose sensitive candidate details] -> Emit only authorized,
  bounded reason categories and aggregate counts; never expose hidden content,
  identifiers, embeddings, or raw similarity values.
- [Packing latency grows with candidate count] -> Apply selection only after
  existing candidate bounds, cap per-section inputs and pairwise comparisons,
  and enforce the existing latency release budget.

## Migration Plan

1. Add policy/domain and deterministic pure-selection tests with diversity
   disabled by default.
2. Add any forward PostgreSQL migration needed to persist versioned scoped
   diversity rollout parameters and audit identity; no destructive migration or
   canonical-memory rewrite is permitted.
3. Wire identity deduplication into the post-fusion path and diversity selection
   into context assembly under diagnostics-only/shadow mode.
4. Extend owned retrieval fixtures and reports, then compare against the
   current approved fusion baseline using an explicitly owned PostgreSQL test
   DSN.
5. Activate only an exact-scope policy that passes quality, latency, budget,
   lifecycle, and isolation gates. Roll back by disabling that policy, which
   leaves durable source data and the stable fusion path intact.

## Open Questions

No blocking product decision remains for proposal approval. During implementation,
the concrete policy parameter defaults and any schema extension SHALL be chosen
by reusing existing rollout conventions and recorded as a versioned policy,
rather than becoming unversioned constants.
