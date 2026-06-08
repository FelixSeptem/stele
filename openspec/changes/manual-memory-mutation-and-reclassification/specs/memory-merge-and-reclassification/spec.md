## ADDED Requirements

### Requirement: Duplicate canonical memories can be merged onto a surviving target
The service SHALL support explicit merge of duplicate canonical memories onto one surviving target identity.

#### Scenario: Operator merges duplicate canonical memories
- **WHEN** a privileged caller merges a source memory into a target memory within the same authorized scope and class
- **THEN** the service appends a new version to the target memory, preserves the target `memory_id`, suppresses the source memory from default public reads, and preserves audit lineage for both records

#### Scenario: Merge violates governance guardrails
- **WHEN** a merge request targets memories from different scopes or different classes
- **THEN** the service rejects the merge instead of coercing the records into a partial or ambiguous result

### Requirement: Canonical memory reclassification remains bounded
The service MUST support bounded manual reclassification for eligible canonical memory classes without reopening unrestricted mutation semantics.

#### Scenario: Operator reclassifies an eligible memory
- **WHEN** a privileged caller reclassifies an eligible canonical memory into an allowed target class
- **THEN** the service preserves the stable `memory_id`, appends a new version, updates the current canonical class, and records a manual reclassification provenance operation

#### Scenario: Reclassification targets an excluded class
- **WHEN** a privileged caller attempts to reclassify a memory into an excluded class such as `summary`
- **THEN** the service rejects the request and preserves the current canonical classification

#### Scenario: Reclassification targets excluded projection-specific relation class
- **WHEN** a privileged caller attempts to reclassify a memory into `relation` during this phase
- **THEN** the service rejects the request and requires relation-specific authoring to stay on dedicated manual create or update flows

### Requirement: Merge and reclassification preserve public lifecycle safety
Manual merge and reclassification MUST not weaken the lifecycle-safe behavior of public memory reads, search, or context assembly.

#### Scenario: Standard caller reads merged memory
- **WHEN** a standard caller lists or searches memory after a duplicate merge
- **THEN** the caller sees the surviving target memory while the suppressed source remains hidden from default public read paths
