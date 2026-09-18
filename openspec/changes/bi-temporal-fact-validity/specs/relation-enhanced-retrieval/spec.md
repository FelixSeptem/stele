## ADDED Requirements

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
