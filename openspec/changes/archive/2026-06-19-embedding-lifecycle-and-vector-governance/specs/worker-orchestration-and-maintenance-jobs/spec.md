## ADDED Requirements

### Requirement: Asynchronous embedding reindex execution
The service SHALL execute semantic backfill, rebuild, and provider-rotation work through the existing durable worker or scheduler runtime model rather than inline write paths.

#### Scenario: Background runtime claims eligible embedding work
- **WHEN** canonical memories are marked eligible for semantic backfill, rebuild, or provider-target drift correction
- **THEN** the service can claim and process that work asynchronously without requiring a foreground memory mutation or retrieval request

#### Scenario: Reindex execution retries safely after failure
- **WHEN** embedding generation or activation fails for a claimed reindex target
- **THEN** the service records durable failure state and can retry later without duplicating active-vector promotion or corrupting semantic lineage

#### Scenario: Provider rotation reuses the same durable execution path
- **WHEN** the desired embedding provider or model target changes for eligible canonical memory
- **THEN** the service schedules provider-drift rebuild work through the same durable asynchronous execution path used for missing or stale embeddings
