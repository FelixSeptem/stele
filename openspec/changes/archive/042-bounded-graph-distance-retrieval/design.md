## Context

See [proposal.md](proposal.md) for motivation and the delta specifications for
the behavioral contract. RQ1 already produces immutable, replayable plans for
`entity_relation` and `multi_hop`; its rollout reader resolves a scoped planner
bundle. RQ2 already makes relation projections subject to source-version and
valid-time eligibility. The relation repository currently exposes bounded
one-step recall through `SearchRelations`, and PostgreSQL is Stele's sole
durable store.

The design must extend those paths without turning a derived relation projection
or an inferred route into canonical memory. It also must preserve the existing
ordinary search/context result contract and keep default retrieval available if
the optional expansion cannot run.

## Goals / Non-Goals

**Goals:**

- Implement policy-governed, request-time traversal over PostgreSQL
  `relation_projections` for approved entity-relation and multi-hop plans.
- Make default behavior one hop and deployment maximum three hops, with every
  exact-scope policy able only to narrow the deployment envelope.
- Reapply exact scope, authorization, lifecycle, valid-time, and source-version
  predicates at every hop; expose only eligible canonical candidates to the
  existing bounded fusion path.
- Make traversal deterministic, finite, observable through authorized redacted
  diagnostics, rollout-safe, and independently testable.

**Non-Goals:**

- A graph database, graph query language, materialized multi-hop path table, or
  a second source of truth.
- General graph search for arbitrary query families, recursive replanning, path
  summaries written as memories, or public graph introspection.
- A new public response field, client protocol, or a relaxation of existing
  candidate, latency, scope, lifecycle, temporal, and provenance contracts.

## Decisions

### 1. Use bounded PostgreSQL recursive expansion over relation projections

Add a repository operation dedicated to graph-path expansion rather than
overloading `SearchRelations`. It receives already approved seeds, immutable
resolved retrieval constraints, and effective graph limits, then issues a
parameterized PostgreSQL recursive CTE over `relation_projections`. The CTE
carries depth, path-local stable node/edge identities, and bounded ranking
attributes; it filters every recursive row using the same scope, lifecycle,
valid-time, and RQ2 source-currency predicates as relation recall.

The query will impose the effective depth and request-time bound in SQL. The
repository then applies deterministic per-seed/per-hop truncation and stable
ordering before returning only a bounded path record set. The retrieval service
performs the final aggregate budget check and turns qualified results into
canonical-memory candidates. The traversal receives the request context, so its
time limit participates in the existing request deadline and has no background
retry.

**Why:** One query keeps all relation visibility predicates adjacent to the
projection data, avoids N+1 edge lookups, and makes path cycle detection and
limits auditable in one PostgreSQL execution boundary.

**Alternatives considered:**

- Iterative one-hop repository calls are simpler to introduce but make
  per-request latency, repeated filtering, and cross-hop consistency harder to
  bound.
- Materialized one-to-three-hop paths reduce read work but introduce invalidation
  and new derived-path lifecycle state.
- Apache AGE is a credible PostgreSQL-community option for openCypher graph
  queries, but adopting it here would make an additional extension and image
  packaging prerequisite, add a second query-language execution surface, and
  make the exact scope/lifecycle/bi-temporal/source-currency predicates and
  hard traversal budgets harder to audit in one parameterized statement. The
  current requirement is a bounded maximum of three hops over an existing
  projection, so AGE would add operational cost without improving the safety
  envelope. Keep the repository interface isolated so an AGE-backed adapter can
  be evaluated later if real workloads demonstrate that recursive CTEs are the
  bottleneck.
- A graph database violates the PostgreSQL-only persistence architecture and
  creates operational/source-of-truth ambiguity.

### 2. Model limits and scope policy as two distinct, immutable layers

Define a deployment `GraphTraversalLimits` value with a default
`MaxHops: 3`, plus ceilings for seed count, edges per hop, paths per seed,
request paths, graph candidates, and elapsed traversal time. Normalize and
validate this configuration at service startup; invalid deployment configuration
must prevent graph traversal from being enabled.

Add a versioned optional graph-traversal policy to the existing exact-scope
rollout policy bundle rather than introducing a global policy resolver. The
policy contains enabled query families, requested hop disposition and narrowed
budgets, its version, and existing rollout status/mode. Resolution validates the
policy against `GraphTraversalLimits` before an immutable retrieval plan is
accepted. The policy is inapplicable unless its resolved tenant, project, and
namespace exactly match the request; existing optional session/user selectors
only narrow that match.

The retrieval-plan identity incorporates the effective graph policy version and
effective limits only for supported graph families. Unsupported families carry
no graph disposition. An absent, malformed, expired, or over-limit policy
returns a baseline traversal disposition rather than an improvised default.

**Why:** Deployment controls remain an operator-enforced absolute boundary,
while scope rollout can safely tune downward or disable the feature without
changing global runtime configuration.

**Alternative considered:** A scope policy that may raise global limits would
let application data bypass operator resource controls and is rejected.

### 3. Preserve graph routes as in-memory proofs and merge via the relation channel

Introduce an internal `GraphPathProof`/path-candidate representation containing
only the stable identities needed for validation: seed identity, ordered edge
and source-version identities, relation categories, hop count, traversal policy
version, and deterministic ranking tuple. It is request-scoped and not written
to PostgreSQL as a fact, relation, memory, citation, or audit event.

The service maps a qualified path endpoint back to its canonical memory and
source-version citation before candidate fusion. It feeds that candidate into
the existing relation recall channel with a stable bounded rank; it does not add
path scores to lexical, semantic, or other raw score spaces. Existing identity
and lineage deduplication remain responsible for returning one canonical result
identity.

**Why:** Existing fusion understands bounded channel rank and canonical
citations. Reusing it avoids a new public ranking model or a second evidence
format.

**Alternative considered:** Exposing a path as a standalone result would alter
the public contract, weakens canonical citation semantics, and risks topology
leakage.

### 4. Enforce eligibility before ranking and stop every path locally

The repository treats every seed and recursive edge as untrusted until it passes
the resolved constraint predicates. It identifies a cycle by repeated stable
node or edge identity in the same path and stops it before it can appear in a
result. It first applies hard eligibility, then ranks paths by hop count,
relation confidence, source reliability, temporal/projection freshness, and a
stable identity tie-breaker.

Budget exhaustion has a terminal category at the level that exhausted it. It
may retain earlier qualified work, but may neither explore additional edges nor
weaken a predicate. A graph-path error, source validation error, repository
error, or cancellation is recorded as a bounded category and causes graph work
to be omitted; it does not invalidate otherwise available baseline retrieval.

**Why:** Qualification before scoring prevents high-confidence stale/foreign
relations from becoming recoverable by rank. Local cycle and budget control
keeps cardinality finite even with high-degree projections.

### 5. Reuse governed rollout and add only redacted graph observability

Graph execution participates in the existing diagnostics-only, shadow, and
active-for-scope lifecycle. Diagnostics and shadow compute the traversal within
the effective envelope but return baseline candidates. Active graph candidates
are eligible only for their exact scope. Disabling the policy or choosing a
previous version skips graph execution on subsequent requests; it does not
delete projections or history.

Add a graph-specific authorized diagnostic record and allowlisted telemetry
labels for graph family, rollout stage, hop bucket, relation category,
exclusion/truncation category, budget category, and policy version. The record
must reject unallowlisted values and never carry text, scope, IDs, content, raw
scores, credentials, DSNs, or provider payloads. Ordinary HTTP/MCP response
types remain unchanged.

**Why:** Existing rollout semantics and telemetry redaction patterns are already
understood operational controls; adding a scoped record avoids leaking a raw
path through generic planner diagnostics.

## Risks / Trade-offs

- **[Recursive CTE latency or high-degree fanout]** → Bound seed/edge/path/time
  limits in both effective policy and SQL, add projection indexes, cancel with
  the request context, and release only after real PostgreSQL shadow evidence.
- **[Predicate divergence from baseline relation recall]** → Factor or reuse the
  existing scope/lifecycle/valid-time/source-currency predicate construction;
  add integration fixtures that prove each predicate applies at every hop.
- **[Cycle detection based on unstable projection fields]** → Use immutable
  stable relation/source identities in the CTE path state and replay fixtures.
- **[Policy payload compatibility or accidental privilege expansion]** → Version
  the optional payload, reject unknown/malformed/over-limit fields, require
  exact scope, and use baseline fallback on read/validation failure.
- **[Path-derived candidate changes established ordering]** → Feed only bounded
  relation-channel rank into the existing fusion/deduplication path and use a
  stable path tuple before fusion.
- **[Diagnostics reveal topology or query data]** → Keep diagnostic structures
  aggregate-only, validate labels against allowlists, and test ordinary API
  serialization separately from authorized evaluation output.

## Migration Plan

1. Add the configuration types/defaults and policy schema fields in a
   backward-compatible disabled state. Existing rollout payloads resolve as no
   graph policy and therefore keep baseline behavior.
2. Add additive PostgreSQL migration(s) for relation-projection traversal
   indexes only after reviewing the concrete predicate and join columns. The
   migration must be reversible and must not rewrite or delete projections,
   canonical memories, fact versions, provenance, or audit history.
3. Implement the repository traversal, proof validation, plan resolution,
   service integration, diagnostics, and fixtures behind diagnostics-only
   rollout. Run deterministic unit tests plus PostgreSQL integration/evaluation
   fixtures before enabling it in any scope.
4. Enable shadow for one exact scope and compare bounded redacted reports with
   baseline. Require all hard safety gates, deterministic replay, index/latency
   evidence, citation coverage, and rollback evidence before active rollout.
5. Activate only approved exact scopes. Roll back by disabling the graph policy
   or selecting its previous version; verify the following request returns the
   unchanged RQ1/RQ2 baseline. No data deletion is part of rollback.

## Open Questions

None. The scope, default/maximum hop model, policy hierarchy, path ranking,
diagnostic redaction, failure closure, and rollout criteria are intentionally
decided before implementation.
