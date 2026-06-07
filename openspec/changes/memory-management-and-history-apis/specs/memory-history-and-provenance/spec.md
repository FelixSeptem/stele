## ADDED Requirements

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
