# Compaction Evidence and Projection Integration

## ADDED Requirements

### Requirement: Record evidence-backed compaction
Each compaction MUST record trigger, source scope, source watermark, raw event range, canonical version references, derivation version, token estimates, evidence coverage, bounded recent-tail references, summary version, state, and follow-up reflection linkage.

#### Scenario: Explainable summary
- **WHEN** a compacted summary is inspected
- **THEN** an authorized operator can identify the source range, derivation version, and referenced evidence without reading unrelated scopes

#### Scenario: Stale source
- **WHEN** source evidence is stale, missing, forgotten, deleted, or foreign-scope
- **THEN** the compacted artifact is marked stale/failed and is excluded from default projection

### Requirement: Preserve projection policy
Projection integration MUST apply existing class-aware policy, lifecycle filtering, scope isolation, bounded context limits, and redaction to reflection and compaction outputs.

#### Scenario: Eligible derived memory
- **WHEN** a reviewed, active, same-scope derived memory has complete evidence
- **THEN** it may be included in the configured projection with bounded evidence references

#### Scenario: Rebuild
- **WHEN** a projection or compaction rebuild is requested
- **THEN** only derived records are recreated and canonical memory versions remain unchanged
