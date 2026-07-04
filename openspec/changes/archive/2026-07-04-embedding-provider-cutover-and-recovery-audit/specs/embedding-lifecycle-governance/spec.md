## ADDED Requirements

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
