## ADDED Requirements

### Requirement: Derived insight derivation runs asynchronously
The service SHALL derive governed experience insights through the worker or scheduler runtime rather than foreground API request paths.

#### Scenario: Scheduler runs insight derivation
- **WHEN** the configured maintenance cadence reaches a scope eligible for derived insight processing
- **THEN** the scheduler can run a bounded derivation job that evaluates repeated failure evidence for that scope

#### Scenario: Ingest path remains lightweight
- **WHEN** a client submits a raw event or mutates memory
- **THEN** the request does not synchronously derive failure patterns or lessons

### Requirement: Derived insight derivation is idempotent
The service MUST keep repeated insight derivation safe across retries, restarts, and duplicate scheduler triggers.

#### Scenario: Duplicate derivation sees same evidence window
- **WHEN** the same derivation job runs again for the same scope and evidence window
- **THEN** the service updates the same derived insight fingerprint instead of creating duplicate active failure patterns

#### Scenario: Derivation job fails
- **WHEN** derived insight processing fails before completion
- **THEN** the service records job failure through the existing durable job execution path without partially activating unsupported insights
