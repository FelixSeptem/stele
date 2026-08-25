## MODIFIED Requirements

### Requirement: Feedback-aware ranking is explicit
The service MUST NOT silently change default context ranking based on usefulness feedback or task-success evidence unless feedback-aware ranking is explicitly requested for one context request or an authorized scoped rollout policy is active.

#### Scenario: Default context assembly runs without active policy
- **WHEN** a caller assembles context without an explicit feedback-aware ranking request and no active matching rollout policy exists
- **THEN** usefulness feedback and task-success signals can appear in diagnostics but do not alter the default context ranking behavior

#### Scenario: Feedback-aware ranking is requested
- **WHEN** a caller explicitly enables feedback-aware ranking for one context request
- **THEN** the service may use bounded usefulness summaries as ranking hints while preserving lifecycle, scope, budget, and citation safety rules

#### Scenario: Governed scope policy is active
- **WHEN** an authorized ranking rollout policy is active for the context surface and resolved scope
- **THEN** context assembly may use active usefulness feedback, task-success summaries, verification outcomes, and quality signals as bounded ranking hints while preserving lifecycle, budget, section, and citation safety rules

#### Scenario: Scope-wide ranking policy is requested without governance
- **WHEN** an operator attempts to enable feedback-aware context ranking as a default scope-wide setting outside the ranking rollout governance contract
- **THEN** the service rejects or ignores that setting and preserves baseline context ranking

#### Scenario: Ranking policy references hidden evidence
- **WHEN** active ranking signals reference suppressed, forgotten, expired, deleted, or out-of-scope memory
- **THEN** context assembly excludes hidden memory from ordinary sections and exposes only lifecycle-safe aggregate diagnostics where authorized

## ADDED Requirements

### Requirement: Context assembly exposes ranking rollout diagnostics
The service SHALL expose bounded diagnostics explaining how an active or dry-run ranking rollout policy affected context candidate priority and section packing.

#### Scenario: Context runs with rollout diagnostics
- **WHEN** an authorized request asks for ranking rollout diagnostics or dry-run comparison
- **THEN** the response can include baseline priority, adjusted priority, included or omitted status, bounded reason codes, signal categories, and evidence counts for lifecycle-visible context items

#### Scenario: Ranking changes budget outcome
- **WHEN** rollout ranking changes whether a lifecycle-visible item is included under the requested context budget
- **THEN** diagnostics can report the budget-safe inclusion or omission reason without exposing hidden memory content
