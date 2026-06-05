## ADDED Requirements

### Requirement: Summary memory compaction
The service SHALL support summary memory creation as a governed compaction path for dense or stale episodic material.

#### Scenario: Episodic cluster is summarized
- **WHEN** a worker identifies a session or topic cluster eligible for compaction
- **THEN** the service creates a summary memory record that references the underlying evidence through provenance links

### Requirement: Summary does not destroy evidence
Summary generation MUST preserve access to the underlying raw events or episodic memories used as evidence.

#### Scenario: Underlying evidence remains auditable
- **WHEN** a summary memory is created
- **THEN** the service keeps the underlying evidence available for audit and later reprocessing even if lifecycle visibility changes
