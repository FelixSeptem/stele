## ADDED Requirements

### Requirement: Projection rebuilds use durable derived work

Context projection rebuilds SHALL be dispatched through the unified derived work
contract with exact scope, policy/renderer identity, source watermark, lease-safe
checkpointing, and idempotent duplicate handling.

#### Scenario: Rebuild is requested twice

- **WHEN** the same scope, kind, policy, renderer, and source watermark are submitted twice
- **THEN** one work item and one deterministic projection result are retained

#### Scenario: Rebuild is interrupted

- **WHEN** a projection worker loses its lease during rebuild
- **THEN** a later worker resumes from durable progress and cannot publish a stale or foreign projection

### Requirement: Projection freshness reports queue degradation

A projection SHALL be ineligible for ordinary retrieval when its rebuild work is
dropped in memory mode, flush-failed, exhausted, stale, divergent, or otherwise
missing required source evidence.

#### Scenario: Lossy queue drops rebuild

- **WHEN** an unflushed projection rebuild is lost
- **THEN** the affected projection remains or becomes ineligible and reports a bounded freshness/SLO category

### Requirement: Projection versions remain append-only

Queue retries and rebuilds SHALL create or resolve versioned projection results
without mutating canonical memory or replacing prior projection history.

#### Scenario: Rebuild produces a newer version

- **WHEN** a durable rebuild completes with a newer source watermark
- **THEN** PostgreSQL stores a new projection version and preserves the prior version as history
