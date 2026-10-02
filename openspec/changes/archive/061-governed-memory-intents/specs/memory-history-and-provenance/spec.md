## ADDED Requirements

### Requirement: Intent lineage is inspectable as provenance
The provenance surface MUST link each intent to its request fingerprint, actor, reason, source evidence, target memory/version, processing outcome, and resulting candidate or lifecycle transition within the exact scope.

#### Scenario: Operator inspects intent lineage
- **WHEN** an authorized operator requests provenance for a processed intent
- **THEN** the service returns bounded stable references and transition metadata without exposing unrelated scope content

#### Scenario: Intent is retried or suppressed
- **WHEN** an intent is replayed, suppressed, or fails after processing begins
- **THEN** the lineage retains the original request, all outcome transitions, and the bounded failure category
