## ADDED Requirements

### Requirement: Parent-first grouping uses validated chunk lineage

The chunking capability SHALL expose a bounded grouping identity that lets a
shadow plan select a parent chunk or projection before expanding children or
adjacent chunks. The grouping MUST retain source-version and exact-scope
validity snapshots and MUST not make parent summaries canonical memory.

#### Scenario: Parent grouping is valid
- **WHEN** visible chunks share a validated parent and source-version snapshot
- **THEN** the shadow plan can group them under that parent within configured
  candidate and token/character limits

#### Scenario: Child lineage is invalid
- **WHEN** a child has a missing, stale, hidden, or foreign parent snapshot
- **THEN** the child is omitted and no fallback lookup broadens the scope

### Requirement: Hierarchical expansion remains bounded and reversible

Parent or adjacent expansion MUST return a bounded disposition for included,
omitted, stale, hidden, foreign, duplicate, or over-budget candidates and MUST
leave the existing flat chunk retrieval order unchanged.

#### Scenario: Expansion exceeds the limit
- **WHEN** a parent-first plan reaches its configured child, adjacent, or latency
  limit
- **THEN** expansion stops, records a bounded limit category, and the baseline
  remains available for comparison
