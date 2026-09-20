# Bounded Graph-Distance Retrieval and Evidence Paths Design

Date: 2026-09-20

## Purpose

RQ3 improves the completeness of entity-centric and multi-hop retrieval while
keeping PostgreSQL as Stele's only system of record and keeping graph memory an
optional, derived enhancement. It extends the approved RQ1 query families and
the RQ2 temporal/source-validity rules; it does not introduce a graph database,
a second fact store, or canonical multi-hop path records.

The proposed OpenSpec change name is `bounded-graph-distance-retrieval`.

## Scope and Non-Goals

The capability applies only to retrieval plans in the `entity_relation` and
`multi_hop` query families. A qualifying plan may start from existing approved
relation baseline hits and perform a bounded, request-time expansion over
PostgreSQL `relation_projections`.

The change does not:

- alter canonical memory in place or persist a derived path as canonical truth;
- create or require Neo4j, FalkorDB, or another graph system;
- add graph paths, edge IDs, hop counts, internal scores, or policy details to
  ordinary search or context APIs;
- enable expansion for other query families;
- allow a free-running retrieval loop, cross-scope traversal, or an unbounded
  retry after a resource limit or path failure.

## Architecture

The new layer has four bounded components:

1. `GraphTraversalLimits` is deployment configuration and the absolute hard
   envelope. `graph_max_hops` defaults to `3`. It also caps seed count, edges
   explored per hop, paths per seed, total request paths, output candidates,
   and traversal time.
2. `GraphTraversalPolicy` is a versioned, exact-scope rollout policy. It
   enables the applicable query family and can reduce hops or any budget from
   the deployment limit. It cannot increase any deployment maximum.
3. `GraphPathExpander` is a repository-facing component that receives already
   qualified seeds plus immutable retrieval constraints. It expands through
   `relation_projections` at request time and stops after the configured depth
   or a budget boundary.
4. `GraphPathProof` is an in-memory, derived proof for a selected path. It
   carries only stable seed, edge, and source-version identities, hop count,
   relation categories, and the policy version needed to check provenance. It
   is never persisted as a fact or exposed through ordinary APIs.

The data flow is:

```text
RQ1 entity_relation / multi_hop plan
        |
        +-- approved baseline relation hits become seeds
        |
        +-- exact-scope policy can only narrow deployment limits
        |
        v
request-time PostgreSQL relation_projections expansion
        |
        +-- scope, authorization, lifecycle, valid-time, and source-version
        |   checks at every hop
        +-- cycle truncation and per-hop/per-seed/request budgets
        |
        v
deterministic, in-memory GraphPathProof
        |
        v
existing canonical-memory candidates and safe citations
```

Default execution is one hop. A valid scoped policy may choose zero, one, two,
or three hops, never above the deployment hard cap of three. With no applicable
exact-scope policy, retrieval remains on the existing RQ1/RQ2 approved
baseline and the expander is not called.

## Eligibility, Paths, and Deterministic Ranking

The expander repeats every non-negotiable retrieval predicate before admitting
a seed or edge at every depth:

- exact `project`, `tenant`, and `namespace` scope;
- authorization and public-versus-admin visibility rules;
- active lifecycle state, excluding hidden, suppressed, forgotten, and deleted
  data unless an already-existing authorized path explicitly permits it;
- RQ2 valid-time eligibility for the request;
- source-version currency and provenance eligibility.

After those hard filters, candidate paths have this stable ordering:

1. shortest path length;
2. relation confidence;
3. source reliability;
4. temporal and projection freshness;
5. stable identity tie-breaker.

Revisiting a node or edge immediately terminates that path. The algorithm
separately enforces edge and path limits per hop, per seed, and per request so
that a high-degree seed cannot monopolize the request. Limits stop further
work while retaining already qualified candidates; they never loosen filters,
expand scope, or trigger an unbounded retry.

## Failure Closure and Public Contract

Traversal failure must fail closed to the approved baseline rather than expose
partial or unqualified graph state. Missing projections, PostgreSQL failures,
authorization failures, and proof-construction failures retain the existing
candidate and citation behavior. In diagnostics and shadow rollout, ordinary
responses always remain baseline responses.

Ordinary search and context contracts retain their current safe shape. They do
not disclose raw path text, node/edge or memory identifiers, hop counts,
internal distances, raw scores, scope information, policy values, DSNs, or
provider payloads.

An authorized evaluation or administrative diagnostic surface may provide only
redacted, low-cardinality aggregates: hop layer, relation category, policy
version, budget or truncation category, exclusion category, and counts. It
must not include query text, scope values, content, identifiers, credentials,
provider payloads, or raw scores.

Returned citations and provenance continue to name the actual eligible
canonical memory and source version. A `GraphPathProof` may validate an
internal derivation but must never become the public evidence source.

## Rollout, Migration, and Rollback

Rollout follows the existing governed sequence:

```text
index/configuration ready
  -> diagnostics
  -> shadow
  -> active-for-exact-scope
  -> policy disable or version rollback to baseline
```

PostgreSQL migrations may add only the indexes or constraints needed for
scope-, lifecycle-, valid-time-, source-currency-, and direction-aware bounded
relation lookup. They must not rewrite or delete relation projections, facts,
fact versions, provenance, or audit history.

During diagnostics and shadow, the normal response is precisely the baseline.
Only a named exact scope with an active valid policy receives expanded results.
Disabling that policy or rolling back its version restores the RQ1/RQ2 relation
retrieval behavior without deleting durable or derived historical records.

## Tests and Release Evidence

The implementation must provide deterministic tests and fixed-clock fixtures
for:

- default one-hop operation; permitted zero through three hops; and rejection
  or baseline fallback when a policy exceeds the deployment cap;
- policy narrowing, policy absence, invalid policy, and query-family gating;
- exact scope, authorization, lifecycle, valid-time, and source-version
  filtering for both seeds and each expanded edge;
- node and edge cycle truncation, duplicate paths, and deterministic replay;
- per-hop, per-seed, and request-wide edge/path/candidate/time exhaustion;
- missing projection, database error, authorization error, and proof error
  fallback to the baseline;
- ordinary API redaction and authorized diagnostic aggregation;
- citation/provenance alignment with qualified canonical evidence;
- PostgreSQL migration upgrade, relevant index use, rollback compatibility, and
  diagnostics/shadow/active replay.

Evaluation compares an exact-scope candidate run against the RQ1/RQ2 baseline
using redacted aggregate measures such as candidate count by hop layer,
truncation and fallback rates, budget hit rates, citation coverage, eligible
relation-hit rate, latency, and quality delta. Reports must not retain query
text, scope, IDs, content, DSNs, credentials, provider payloads, or raw
scores.

The following are hard release failures that no recall or quality improvement
can offset:

- a path that crosses scope or violates authorization;
- an admitted hidden, expired, future, lifecycle-ineligible, or
  source-version-ineligible seed or edge;
- an untruncated cycle or any depth, edge, path, candidate, or time-budget
  overflow;
- citation or provenance that does not match qualifying canonical evidence;
- internal graph information exposed by an ordinary API;
- nondeterministic fixture replay for fixed inputs, time, and policy version.

## Verification

Before implementation, the OpenSpec proposal must map each architecture,
eligibility, failure, public-contract, migration, rollout, test, and hard-gate
requirement in this document to an implementation task and a delta
specification. Implementation verification must include targeted unit and
PostgreSQL integration tests, repository-wide OpenSpec validation, formatting
and static checks, deterministic evaluation/replay evidence, and an explicit
review of API redaction and rollback behavior.
