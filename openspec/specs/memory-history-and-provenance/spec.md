# memory-history-and-provenance Specification

## Purpose
TBD - created by archiving change memory-management-and-history-apis. Update Purpose after archive.
## Requirements
### Requirement: Append-only memory history query
The service SHALL expose memory history as a versioned, append-only view of canonical memory evolution.

#### Scenario: Client requests memory history
- **WHEN** a caller requests the history of a canonical memory
- **THEN** the service returns canonical version records in a stable order without implying destructive in-place updates

### Requirement: Provenance lineage query
The service MUST expose evidence lineage for canonical memory through a stable provenance API.

#### Scenario: Client inspects provenance for a memory
- **WHEN** a caller requests provenance for a canonical memory
- **THEN** the service returns stable references to the relevant raw events, candidate records, and lifecycle operations that contributed to that memory

### Requirement: Privileged inspection of hidden lifecycle history
The service MUST support privileged inspection of hidden or deleted memory history without weakening public read safety defaults.

#### Scenario: Operator investigates a deleted or forgotten memory
- **WHEN** a privileged caller inspects the history or provenance of a hidden memory
- **THEN** the service can expose lifecycle transitions and lineage diagnostics through a privileged inspection path while standard public reads remain lifecycle-safe

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

