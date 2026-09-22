# relation-enhanced-retrieval Specification

## Purpose
Define the optional relation-aware retrieval behavior that expands relevant entity neighborhoods without replacing the core hybrid retrieval path.

## Requirements

### Requirement: Optional relation-enhanced retrieval
The service SHALL support bounded relation-enhanced retrieval as an optional enrichment on top of baseline governed search.

#### Scenario: Entity-centric query enables relation expansion
- **WHEN** a query targets an entity or concept with related projection data and relation expansion is enabled
- **THEN** the service can enrich the candidate set with nearby related entities or relation facts

### Requirement: Relation expansion remains policy-safe
Relation-enhanced retrieval MUST respect the same scope and lifecycle visibility constraints as baseline retrieval.

#### Scenario: Related hidden memory exists behind an entity
- **WHEN** relation expansion reaches a memory or relation fact that is suppressed, forgotten, expired, deleted, or out of scope
- **THEN** the service excludes that result from the enriched response

### Requirement: Relation enhancement is optional and bounded
The service MUST keep relation expansion optional and bounded so baseline search does not depend on graph-style traversal.

#### Scenario: Relation projection data is unavailable
- **WHEN** relation projection data is absent or relation expansion is disabled
- **THEN** the service still returns baseline lexical and semantic retrieval results without failing the request

### Requirement: Relation evidence carries temporal source identity
Relation projections and relation-expanded candidates MUST retain source canonical
version and validity identity and MUST be eligible only when that source is valid
for the query's temporal constraint.

#### Scenario: Relation points to an expired source
- **WHEN** relation expansion reaches a source version that is expired for the
  current or historical query
- **THEN** the relation candidate is excluded without weakening scope or lifecycle
  filters

#### Scenario: Relation projection is rebuilt after correction
- **WHEN** a source fact receives a temporal successor
- **THEN** the service creates a new relation projection lineage for that source
  version and preserves the previous projection as derived history

### Requirement: Relation enrichment supports bounded graph distance
When an exact-scope graph-traversal policy authorizes an `entity_relation` or
`multi_hop` plan, relation-enhanced retrieval SHALL be able to enrich the
baseline candidate set through a finite relation-projection path of the
policy-selected depth. Every intermediate relation and final candidate MUST
pass the same exact scope, authorization, lifecycle, valid-time, and
source-version checks as baseline relation evidence.

#### Scenario: Eligible two-hop relation evidence is enabled for a scope
- **WHEN** a valid exact-scope policy selects two hops and a second-hop relation
  projection has an eligible canonical source version
- **THEN** relation-enhanced retrieval can enrich the candidate set with that
  canonical candidate while retaining its qualified source citation

#### Scenario: Intermediate relation evidence is hidden or invalid
- **WHEN** a path to an otherwise relevant candidate contains an out-of-scope,
  hidden, lifecycle-ineligible, valid-time-ineligible, or source-version-stale
  intermediate relation
- **THEN** relation-enhanced retrieval excludes the path and candidate without
  weakening baseline retrieval filters

### Requirement: Relation graph expansion remains finite and optional
Relation-enhanced retrieval MUST stop graph-distance expansion at its configured
hop and resource limits and on a repeated path node or edge. A missing,
disabled, invalid, or unavailable graph expansion path MUST preserve the
existing bounded relation and hybrid baseline behavior without failing the
ordinary request.

#### Scenario: Relation graph expansion reaches a cycle or limit
- **WHEN** an enabled relation graph traversal revisits a node or edge or
  exhausts its configured work budget
- **THEN** the service terminates further expansion and returns only eligible
  candidates already within the baseline/result budget

#### Scenario: Relation graph policy is absent
- **WHEN** no valid exact-scope graph policy enables relation graph expansion
- **THEN** the service does not depend on graph traversal and returns the
  existing relation-enhanced or hybrid baseline result
