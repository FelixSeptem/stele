# bounded-graph-distance-retrieval Specification

## Purpose
Define safe, deterministic request-time graph-distance retrieval over derived
PostgreSQL relation projections without creating a second persistence model or
weakening Stele's existing evidence, isolation, and lifecycle guarantees.

## Requirements

### Requirement: Graph traversal is scope-governed and deployment-bounded
The service SHALL permit request-time graph-distance expansion only for an
approved `entity_relation` or `multi_hop` retrieval plan with an exact-scope
traversal policy. Deployment configuration MUST impose an absolute traversal
hard envelope whose `graph_max_hops` default is `3`; an exact-scope policy MUST
only lower that envelope. In the absence of a valid policy, the service MUST
execute its existing approved baseline without graph expansion. The default
traversal disposition for an enabled policy SHALL be one hop, and a valid policy
MAY select zero, one, two, or three hops without exceeding the deployment cap.

#### Scenario: Default graph policy executes one hop
- **WHEN** an approved entity-relation or multi-hop plan has an enabled
  exact-scope traversal policy that does not override its hop disposition
- **THEN** the service expands no more than one relation hop within the
  deployment resource envelope

#### Scenario: Scope policy requests an excessive hop count
- **WHEN** an exact-scope traversal policy requests more hops than the
  deployment `graph_max_hops` hard limit
- **THEN** the service rejects that traversal disposition and returns the
  approved baseline without increasing the deployment limit

#### Scenario: No exact-scope policy applies
- **WHEN** a query is classified as entity-relation or multi-hop but no valid
  exact-scope traversal policy is active
- **THEN** the service does not traverse relation projections and preserves the
  existing baseline result and public response shape

### Requirement: Every graph hop preserves evidence eligibility
The service MUST apply resolved exact scope, authorization, lifecycle,
valid-time, and source-version eligibility before admitting every graph seed or
edge at every hop. It MUST exclude hidden, suppressed, forgotten, deleted,
expired, future-invalid, out-of-scope, unauthorized, and source-version-stale
derived relation evidence before that evidence can contribute to a candidate or
path proof.

#### Scenario: An intermediate edge is no longer valid
- **WHEN** a candidate graph path contains an intermediate relation projection
  whose source version is expired, superseded for the resolved valid-time
  constraint, or otherwise ineligible
- **THEN** the service excludes that edge and does not return any candidate that
  relies on the path

#### Scenario: Traversal encounters foreign-scope evidence
- **WHEN** a relation projection would lead outside the resolved tenant,
  project, namespace, or authorized caller constraint
- **THEN** the service stops that path without disclosing foreign content,
  identifiers, or topology

### Requirement: Traversal work and path ranking are deterministic and finite
The service MUST impose independent configured limits for work per hop, per
seed, and per request, including explored edges, retained paths, output
candidates, and elapsed traversal time. It MUST truncate a path immediately on
repeated node or edge identity. Among otherwise eligible paths, it MUST order
results deterministically by shorter path length, relation confidence, source
reliability, temporal/projection freshness, and stable identity tie-breaker.

#### Scenario: A cyclic relation path is encountered
- **WHEN** traversal would revisit a node or edge already present in the same
  candidate path
- **THEN** the service terminates that path and does not emit a repeated or
  unbounded path proof

#### Scenario: A request traversal budget is exhausted
- **WHEN** the configured per-hop, per-seed, or request-wide traversal budget
  is exhausted
- **THEN** the service stops further expansion, retains only already eligible
  candidates within the result budget, and does not broaden scope or retry
  without bound

#### Scenario: Equivalent traversal is replayed
- **WHEN** fixed source projections, request constraints, clock, limits, and
  policy version are evaluated repeatedly
- **THEN** the service produces the same eligible path order, candidate order,
  truncation disposition, and bounded proof identity

### Requirement: Graph-derived candidates preserve canonical evidence identity
The service SHALL treat graph paths as transient derived proofs and SHALL return
only eligible canonical-memory candidates with their source-version citations
and provenance. A path proof MUST NOT be persisted as a canonical memory or
fact, become an independent public citation, or replace the canonical source of
record.

#### Scenario: A multi-hop path supplies a candidate
- **WHEN** a bounded eligible path identifies a canonical-memory candidate not
  present in the baseline relation result
- **THEN** the service returns that candidate only with its qualified canonical
  source-version citation and retains any path proof solely for authorized
  internal validation

### Requirement: Graph traversal failure is baseline-safe and reversible
The service MUST fail closed to the approved RQ1/RQ2 relation-retrieval
baseline when graph traversal is unavailable, policy validation fails,
authorization cannot be confirmed, a path cannot be proven, or traversal work
is rejected. Rollout SHALL progress only through diagnostics, shadow, and
active-for-exact-scope states; diagnostics and shadow MUST preserve ordinary
baseline results. Disabling or rolling back an active exact-scope policy MUST
restore baseline retrieval without rewriting canonical or derived history.

#### Scenario: Relation projection lookup fails
- **WHEN** PostgreSQL traversal cannot read required relation projection data
- **THEN** the service returns the existing approved baseline result and does
  not expose partial graph state or fail the ordinary retrieval request solely
  for that optional enhancement

#### Scenario: Active graph rollout is disabled
- **WHEN** an operator disables or rolls back the exact-scope traversal policy
- **THEN** subsequent matching requests use the RQ1/RQ2 baseline without
  deleting relation projections, fact versions, provenance, or audit history

### Requirement: Graph diagnostics and release evidence are redacted and fail closed
The service SHALL expose graph-traversal diagnostics only to authorized
evaluation or administrative paths and only as low-cardinality redacted
aggregates such as hop layer, relation category, policy version, budget or
truncation category, exclusion category, and count. Evaluation and release
evidence MUST treat cross-scope traversal, ineligible source evidence,
untruncated cycles, resource-limit overflow, citation/provenance mismatch,
ordinary-API leakage, and nondeterministic replay as hard failures that quality
improvement cannot offset.

#### Scenario: An ordinary caller receives graph-enhanced retrieval
- **WHEN** an ordinary search or context caller receives a graph-enhanced
  candidate
- **THEN** the response omits raw path text, node or edge identifiers, hop
  count, internal distance, raw score, policy values, diagnostics, and hidden
  topology

#### Scenario: A release evaluation detects a safety violation
- **WHEN** graph traversal evaluation finds a cross-scope path, ineligible
  source, untruncated cycle, budget overflow, citation/provenance mismatch,
  ordinary-API leakage, or nondeterministic replay
- **THEN** the evaluation records only an authorized redacted failure category
  and marks the traversal policy ineligible for activation
