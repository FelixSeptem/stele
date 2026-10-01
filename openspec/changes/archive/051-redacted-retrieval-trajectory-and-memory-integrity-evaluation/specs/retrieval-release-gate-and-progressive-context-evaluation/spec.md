## ADDED Requirements

### Requirement: Trajectory and integrity evidence participate in release review

The release evaluator SHALL accept trajectory and memory-integrity artifacts
only when they are redacted, exact-scope, compatible with the release policy,
and fresh for the evaluated source watermark. These artifacts MAY explain or
block a release decision but MUST NOT authorize a rollout without all existing
quality, resource, lifecycle, isolation, replay, and rollback gates.

#### Scenario: Compatible safety evidence is supplied

- **WHEN** an evaluation includes fresh compatible trajectory and integrity reports
- **THEN** the release report includes their bounded categories and evaluates them with the existing required gates

#### Scenario: Integrity evidence is missing or stale

- **WHEN** a candidate lacks required integrity evidence or the evidence watermark is stale
- **THEN** the candidate remains non-pass or shadow-only and the baseline remains active
