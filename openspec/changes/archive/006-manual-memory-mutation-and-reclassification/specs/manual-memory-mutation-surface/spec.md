## ADDED Requirements

### Requirement: Privileged manual canonical memory creation
The service SHALL expose a privileged API for direct canonical memory creation when an operator needs to seed governed memory without going through raw event ingestion.

#### Scenario: Operator creates manual canonical memory
- **WHEN** a privileged caller submits a valid manual memory creation request within an authorized scope
- **THEN** the service creates an active canonical memory, writes version `1`, and records a provenance operation for the manual creation

#### Scenario: Operator attempts to create excluded derived memory
- **WHEN** a privileged caller attempts to create a derived-only class such as `summary` through the manual creation endpoint
- **THEN** the service rejects the request and preserves the boundary between operator-authored memory and compaction-derived memory

### Requirement: Bounded manual canonical memory update
The service MUST expose a bounded update surface for canonical memory corrections without allowing destructive in-place overwrite of governed history.

#### Scenario: Operator corrects canonical content
- **WHEN** a privileged caller updates the content of an eligible canonical memory
- **THEN** the service preserves the stable `memory_id`, appends a new memory version, updates the current canonical projection, and records a manual update provenance operation

#### Scenario: Operator attempts to mutate excluded fields
- **WHEN** a privileged caller attempts to change scope, lifecycle state, or class through the bounded update endpoint
- **THEN** the service rejects the request and requires the caller to use the dedicated governance surface for that concern

### Requirement: Manual mutation remains on a privileged governance surface
Manual canonical memory mutation MUST remain separated from ordinary public memory reads and raw event admission.

#### Scenario: Standard product caller writes memory
- **WHEN** a standard caller needs to contribute new memory content
- **THEN** the caller continues to use raw event ingestion instead of direct canonical mutation unless privileged governance authorization is explicitly granted
