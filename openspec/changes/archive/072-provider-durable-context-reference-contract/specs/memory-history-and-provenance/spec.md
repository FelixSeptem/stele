## ADDED Requirements

### Requirement: Durable references expose bounded lineage

Reference status and exact-path reads SHALL expose bounded lineage connecting the
source event, candidate when applicable, canonical memory, selected version and
provenance operation within the exact scope.

#### Scenario: Authorized caller inspects reference lineage
- **WHEN** a completed reference is read in its authorized scope
- **THEN** the response contains stable lineage identities and does not include
  unrelated scope content or raw hidden payloads

#### Scenario: Promotion changes the canonical version
- **WHEN** consolidation or deduplication creates a new canonical version
- **THEN** the reference retains source lineage and identifies the current
  version without erasing prior provenance

### Requirement: Lifecycle removal leaves auditable reference history

Reference suppression, forgetting and deletion SHALL preserve authorized audit
lineage while removing the reference from ordinary reads according to lifecycle
policy.

#### Scenario: Reference becomes forgotten
- **WHEN** lifecycle processing marks a referenced memory forgotten
- **THEN** ordinary exact-path lookup returns a bounded unavailable/lifecycle
  outcome and privileged history can retain the transition and provenance
