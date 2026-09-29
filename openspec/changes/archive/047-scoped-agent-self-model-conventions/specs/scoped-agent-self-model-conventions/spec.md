## Purpose

Define an optional convention for organizing an agent's descriptive self-model as ordinary governed memories inside an already authorized exact scope. The convention preserves existing memory classes, authorization, lifecycle, provenance, and runtime capability authority.

## ADDED Requirements

### Requirement: Agent self-model uses ordinary scoped memories
The service documentation and supported examples SHALL define the optional path form `agents/{agent-id}/self/{category}` for agent self-model information inside a resolved exact tenant/project/namespace scope. The initial categories SHALL be `capabilities`, `preferences`, `limitations`, and `lessons`. Agent IDs and category segments MUST satisfy the shared normalized memory-path rules and MUST NOT act as a scope, grant, or authorization selector.

#### Scenario: Caller records an agent preference
- **WHEN** an authorized caller records a descriptive agent preference using the convention
- **THEN** it is represented as an ordinary governed `profile` memory at the selected exact scope and normalized path

#### Scenario: Caller records an operational lesson
- **WHEN** an authorized caller records a reusable operational procedure or lesson using the convention
- **THEN** it is represented using the existing `procedural` memory class at the selected exact scope and normalized path

#### Scenario: Invalid agent identifier is supplied
- **WHEN** an agent identifier or category makes the composed path violate shared path validation
- **THEN** the write is rejected by existing validation and no separate identity or path-based authorization behavior is inferred

### Requirement: Self-model writes preserve governed lifecycle
Self-model creation, correction, and forgetting SHALL use existing governed memory intents or authorized lifecycle APIs. The convention MUST NOT add direct canonical writes, in-place overwrites, a new memory class, or an alternate provenance or lifecycle path.

#### Scenario: Self-model statement is corrected
- **WHEN** an authorized caller updates a self-model statement
- **THEN** the change follows existing governed admission/versioning behavior and preserves prior version and provenance history

#### Scenario: Self-model information is forgotten
- **WHEN** an authorized caller forgets or deletes a self-model memory
- **THEN** existing preview, lifecycle, audit, and hidden-memory defaults remain in force

### Requirement: Self-model retrieval is explicit and authority-safe
Self-model retrieval SHALL use existing exact path or explicit bounded path-prefix search/context contracts and MUST remain within the already authorized exact scope and existing result/context budgets. Self-model memories MUST NOT be automatically injected into ordinary context by this convention. Remembered capabilities, limits, preferences, and lessons are descriptive evidence only and MUST NOT grant access, change runtime configuration, or override server-published capabilities, grants, or enforced limits.

#### Scenario: Agent requests one self-model category
- **WHEN** a caller explicitly requests `agents/{agent-id}/self/preferences` within an authorized exact scope
- **THEN** existing path-aware retrieval returns only lifecycle-visible eligible memories at that exact path subject to normal ranking and budget rules

#### Scenario: Agent requests a bounded self-model subtree
- **WHEN** a caller explicitly requests the self-model prefix for an authorized agent
- **THEN** existing segment-boundary prefix semantics apply without crossing tenant/project/namespace scope or increasing result/context limits

#### Scenario: Ordinary context omits a self-model selector
- **WHEN** a caller assembles context without explicitly requesting a self-model path
- **THEN** this convention alone does not add an always-on self-model section or alter default context eligibility

#### Scenario: Remembered capability conflicts with server authority
- **WHEN** a self-model memory claims a capability or limit that conflicts with server-published capabilities, grants, configuration, or runtime enforcement
- **THEN** the server authority remains effective and the remembered statement cannot enable or authorize the operation
