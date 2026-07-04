# embedding-lifecycle-governance Specification

## Purpose
TBD - created by archiving change embedding-lifecycle-and-vector-governance. Update Purpose after archive.
## Requirements
### Requirement: Canonical memory has governed vector lifecycle state
The service SHALL persist durable embedding lifecycle state for the current canonical memory projection so semantic backfill, rebuild, and provider rotation can be coordinated without relying on implicit `NULL` embedding interpretation alone.

#### Scenario: Material mutation invalidates semantic projection
- **WHEN** a canonical memory mutation materially changes the retrievable content or semantic class of the current projection
- **THEN** the service marks that memory as requiring vector rebuild, prevents the stale semantic projection from remaining active, and preserves enough durable state for later asynchronous regeneration

#### Scenario: Missing semantic projection is eligible for backfill
- **WHEN** a visible canonical memory is eligible for semantic retrieval but has no active embedding for the current projection
- **THEN** the service records that the memory is eligible for asynchronous vector backfill rather than requiring inline embedding generation

### Requirement: Vector revisions are append-only and auditable
The service MUST record append-only vector revisions for the current canonical projection instead of overwriting semantic lineage in place.

#### Scenario: New active embedding supersedes an older revision
- **WHEN** a rebuilt or rotated embedding is successfully promoted for the current canonical memory projection
- **THEN** the service stores a new vector revision with provider, model, dimensions, source version, content hash, generation status, and supersession lineage while preserving the prior revision for audit

#### Scenario: Failed embedding generation is still auditable
- **WHEN** semantic generation fails for an eligible memory
- **THEN** the service records the failed vector revision attempt with failure status and attribution needed for later retry and diagnostics

### Requirement: Active vector promotion is compare-and-promote safe
The service MUST only activate a generated embedding when it still matches the current canonical memory projection that the rebuild targeted.

#### Scenario: Canonical content changes during rebuild
- **WHEN** an asynchronous embedding rebuild finishes after the canonical memory projection has already advanced to a newer version or content hash
- **THEN** the service records the completed attempt but does not promote that vector revision as the active semantic projection

#### Scenario: Promotion updates the active projection atomically
- **WHEN** a generated embedding still matches the targeted canonical source version and content hash
- **THEN** the service atomically marks that vector revision active for retrieval and supersedes the previously active revision for that memory

### Requirement: Embedding lifecycle state is operator-inspectable
The service MUST expose enough derived embedding lifecycle state for operators to diagnose missing semantic coverage, drift, and rebuild failure without weakening append-only lineage rules.

#### Scenario: Drift is visible before rebuild completion
- **WHEN** the active vector revision no longer matches the currently routed provider, model, or dimensions target for a memory's current canonical projection
- **THEN** operator inspection can report that drift state together with the requested replacement target and current active revision attribution

#### Scenario: Failed rebuild retains diagnostic attribution
- **WHEN** an embedding rebuild attempt fails for the current canonical projection
- **THEN** operator inspection can report the failed requested target, failure reason, attempt timing, and the still-active or missing semantic projection state for that memory

### Requirement: Embedding lifecycle inspection includes cutover attribution
The service MUST expose cutover context alongside derived embedding lifecycle state when a memory is participating in or affected by a provider cutover.

#### Scenario: Drifted memory is linked to an active cutover
- **WHEN** an operator inspects a memory whose current active vector no longer matches an active cutover plan target
- **THEN** the derived embedding inspection can report the linked cutover plan identity together with the requested replacement target and current active revision attribution

#### Scenario: Failed cutover item retains lifecycle context
- **WHEN** a memory fails rebuild during a provider cutover rollout
- **THEN** operator inspection can report both the failed embedding target and the cutover plan context without weakening append-only lineage rules

### Requirement: Recovery audit preserves rollout context
The service MUST preserve cutover attribution on embedding recovery actions when operators intervene during a provider rollout.

#### Scenario: Retry during cutover records plan attribution
- **WHEN** an operator retries or requeues a memory that belongs to an active or paused cutover plan
- **THEN** the recorded embedding recovery history can include the cutover plan identity together with the recovery before or after snapshots and operator attribution
