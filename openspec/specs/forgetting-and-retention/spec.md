# forgetting-and-retention Specification

## Purpose
TBD - created by archiving change governance-pipeline-and-memory-consolidation. Update Purpose after archive.
## Requirements
### Requirement: Distinct forgetting semantics
The service SHALL model forgetting as at least three distinct actions: `suppress`, `expire`, and `delete`.

#### Scenario: Suppress action hides memory
- **WHEN** a suppress action is applied to a candidate or canonical memory
- **THEN** the memory is excluded from default retrieval while remaining available for audit or controlled inspection

#### Scenario: Expiry action follows retention policy
- **WHEN** a memory reaches its retention boundary
- **THEN** the service applies expiry behavior according to retention policy without requiring destructive deletion by default

#### Scenario: Delete action removes payload
- **WHEN** an explicit delete action is applied for compliance or operator reasons
- **THEN** the service removes the memory payload and associated read projections according to deletion policy

### Requirement: Default visibility excludes hidden lifecycle states
Non-admin reads MUST exclude suppressed, forgotten, and expired memories by default.

#### Scenario: Hidden memory is not returned by default reads
- **WHEN** a standard internal read path loads memory for future retrieval or context assembly
- **THEN** suppressed, forgotten, and expired records are excluded unless an explicit administrative or debug path opts in

