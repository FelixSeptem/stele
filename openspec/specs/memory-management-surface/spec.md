# memory-management-surface Specification

## Purpose
TBD - created by archiving change memory-management-and-history-apis. Update Purpose after archive.
## Requirements
### Requirement: Public canonical memory resource surface
The service SHALL expose a stable resource-oriented API for governed canonical memory reads.

#### Scenario: Client lists canonical memory within scope
- **WHEN** a client requests the memory list API
- **THEN** the service supports scope-bound filtering, class filtering, time-aware filtering, and stable pagination over canonical memory resources

#### Scenario: Client reads one canonical memory
- **WHEN** a client requests a specific canonical memory by identifier
- **THEN** the service returns a stable resource representation rather than a retrieval-specific ranked hit model

### Requirement: Lifecycle-safe default memory reads
Default canonical memory read APIs MUST preserve the same lifecycle safety guarantees as retrieval and context assembly.

#### Scenario: Hidden memory is not returned by default
- **WHEN** a canonical memory is suppressed, forgotten, expired, or deleted
- **THEN** the default public memory list and detail APIs exclude or redact that memory according to lifecycle-safe visibility rules

### Requirement: Stable memory metadata contract
The memory resource representation MUST expose enough governed metadata for SDK use without leaking internal storage mechanics.

#### Scenario: Client inspects a memory resource
- **WHEN** a client receives a canonical memory resource
- **THEN** the representation includes stable identifier, scope, class, lifecycle-safe state, timestamps, and content fields appropriate for that visibility level

