## ADDED Requirements

### Requirement: Progressive projection versions retain level provenance

Every L0, L1, or L2 projection version MUST retain its source canonical version,
level identity, renderer and policy identities, exact-scope identity, source
watermark, freshness result, and rebuild identity. New materialization MUST
append a version and preserve prior versions for audit and deterministic replay.

#### Scenario: A level is rebuilt after source change
- **WHEN** an authorized canonical version or projection policy changes
- **THEN** the service appends a new level version linked to the new source
  watermark and keeps the prior version as history

#### Scenario: Level freshness cannot be proven
- **WHEN** a projection read cannot verify its source watermark or lifecycle
  snapshot
- **THEN** the projection is ineligible for ordinary retrieval and shadow
  diagnostics report a bounded freshness or lifecycle category

### Requirement: Projection reads enforce level-specific scope and visibility

Projection reads used by progressive or parent-first experiments MUST apply the
same tenant, project, namespace, temporal, and lifecycle predicates as canonical
retrieval before returning an item or allowing expansion.

#### Scenario: Projection references a foreign record
- **WHEN** a level contains a source or child outside the resolved exact scope
- **THEN** the item is excluded and the experiment cannot broaden its lookup
