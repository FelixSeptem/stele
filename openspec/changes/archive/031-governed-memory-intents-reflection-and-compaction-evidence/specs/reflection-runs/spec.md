# Durable Reflection Runs and Review

## ADDED Requirements

### Requirement: Persist restart-safe reflection runs
Every reflection run MUST persist a stable identity, trigger, input watermark, transcript schema version, processed offset, lease, attempt/retry budget, status, bounded failure category, outputs, and evidence references.

#### Scenario: Worker restart
- **WHEN** a worker stops after a checkpoint is committed
- **THEN** another worker can claim the expired lease and resume from that checkpoint without duplicating durable outputs

#### Scenario: Replay
- **WHEN** an operator requests replay for a run and the same input watermark/schema version
- **THEN** the service creates or returns a deduplicated replay identity and preserves the original run history

### Requirement: Govern reflection candidates
Reflection outputs MUST enter candidate state and MUST be reviewable before canonical activation when policy requires review.

#### Scenario: Accept candidate
- **WHEN** an authorized reviewer accepts a candidate
- **THEN** the existing versioned canonical-memory path creates a new version with reviewer provenance

#### Scenario: Suppress candidate
- **WHEN** a reviewer suppresses or rejects a candidate
- **THEN** it remains auditable but is excluded from default retrieval and projection
