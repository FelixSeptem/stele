# memory-management-surface Specification

## Purpose
Publish a stable, resource-oriented API for governed canonical memory management.

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

### Requirement: Canonical memory resources expose path-aware filtering
The public canonical memory resource surface SHALL expose normalized memory path metadata and SHALL support exact `path` and explicit descendant `path_prefix` filters within the already resolved exact scope. Omitted paths SHALL preserve whole-scope behavior.

#### Scenario: Client browses one exact path
- **WHEN** a client lists canonical memory with `path=agents/research`
- **THEN** the service returns only lifecycle-visible resources at that exact path with stable pagination

#### Scenario: Client browses a path prefix
- **WHEN** a client lists canonical memory with `path_prefix=agents/research`
- **THEN** the service returns lifecycle-visible resources at the prefix and its segment descendants without including sibling prefixes

#### Scenario: Detail response reports path
- **WHEN** a client reads a visible canonical memory resource
- **THEN** its stable representation includes the normalized path or root-path value without exposing storage-specific columns
