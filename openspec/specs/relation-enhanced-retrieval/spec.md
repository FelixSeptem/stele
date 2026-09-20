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
