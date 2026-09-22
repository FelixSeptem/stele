## ADDED Requirements

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
