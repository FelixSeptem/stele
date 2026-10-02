# memory-history-and-provenance Specification

## Purpose
Expose stable append-only memory history and evidence provenance for authorized audit.

## Requirements

### Requirement: Append-only memory history query
The service SHALL expose memory history as a versioned, append-only view of canonical memory evolution, including recorded time, temporal fact identity, valid-from, valid-to, and correction disposition when applicable.

#### Scenario: Client requests memory history
- **WHEN** a caller requests the history of a canonical memory
- **THEN** the service returns canonical version records in a stable recorded/version order without implying destructive in-place updates and preserves validity metadata

### Requirement: Provenance lineage query
The service MUST expose evidence lineage for canonical memory through a stable provenance API, including the source and correction lineage for temporal validity.

#### Scenario: Client inspects provenance for a memory
- **WHEN** a caller requests provenance for a canonical memory
- **THEN** the service returns stable references to relevant raw events, candidate records, lifecycle operations, temporal corrections, and source versions that contributed to that memory

### Requirement: Privileged inspection of hidden lifecycle history
The service MUST support privileged inspection of hidden or deleted memory history without weakening public read safety defaults.

#### Scenario: Operator investigates a deleted or forgotten memory
- **WHEN** a privileged caller inspects the history or provenance of a hidden memory
- **THEN** the service can expose lifecycle transitions and lineage diagnostics through a privileged inspection path while standard public reads remain lifecycle-safe

### Requirement: Projection derivation is auditable through provenance
The service SHALL preserve projection derivation metadata linking each visible item to authorized canonical-memory version or raw-event evidence, source watermark, policy version, renderer version, temporal validity identity, and materialization time.

#### Scenario: Operator inspects projection lineage
- **WHEN** an authorized operator inspects a projection item or rebuild result
- **THEN** the service returns bounded source references, validity metadata, and version information in the item's scope without returning raw hidden content

#### Scenario: Projection source is superseded
- **WHEN** a canonical version, validity interval, or raw-event source is superseded or hidden
- **THEN** a subsequent read excludes the item from ordinary context while retaining its prior projection and provenance history for privileged audit

### Requirement: Intent lineage is inspectable as provenance
The provenance surface MUST link each intent to its request fingerprint, actor, reason, source evidence, target memory/version, processing outcome, and resulting candidate or lifecycle transition within the exact scope.

#### Scenario: Operator inspects intent lineage
- **WHEN** an authorized operator requests provenance for a processed intent
- **THEN** the service returns bounded stable references and transition metadata without exposing unrelated scope content

#### Scenario: Intent is retried or suppressed
- **WHEN** an intent is replayed, suppressed, or fails after processing begins
- **THEN** the lineage retains the original request, all outcome transitions, and the bounded failure category
