## ADDED Requirements

### Requirement: Projection derivation is auditable through provenance
The service SHALL preserve projection derivation metadata linking each visible
item to authorized canonical-memory version or raw-event evidence, source
watermark, policy version, renderer version, and materialization time.

#### Scenario: Operator inspects projection lineage
- **WHEN** an authorized operator inspects a projection item or rebuild result
- **THEN** the service returns bounded source references and version metadata in
  the item's scope without returning raw hidden content

#### Scenario: Projection source is superseded
- **WHEN** a canonical version or raw-event source is superseded or hidden
- **THEN** a subsequent read excludes the item from ordinary context while
  retaining its prior projection and provenance history for privileged audit
