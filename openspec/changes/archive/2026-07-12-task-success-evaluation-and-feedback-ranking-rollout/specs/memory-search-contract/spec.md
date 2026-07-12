## MODIFIED Requirements

### Requirement: Search ranking remains stable unless feedback-aware ranking is explicit
The service MUST keep default search ranking behavior stable unless feedback-aware ranking is explicitly requested for that search request or an authorized scoped rollout policy is active for the resolved scope.

#### Scenario: Default search runs without active policy
- **WHEN** a caller searches memory without feedback-aware ranking enabled and no active matching rollout policy exists
- **THEN** usefulness feedback and task-success signals do not silently alter ranking or filtering

#### Scenario: Feedback-aware search runs
- **WHEN** feedback-aware ranking is explicitly enabled for one search request
- **THEN** retrieval may use usefulness summaries as bounded ranking hints while preserving lifecycle and scope isolation guarantees

#### Scenario: Governed scope policy is active
- **WHEN** an authorized ranking rollout policy is active for the search surface and resolved scope
- **THEN** retrieval may use active usefulness feedback, task-success summaries, verification outcomes, and quality signals as bounded ranking hints according to the policy

#### Scenario: Scope-wide ranking policy is requested without governance
- **WHEN** an operator attempts to enable feedback-aware search ranking as a default scope-wide setting outside the ranking rollout governance contract
- **THEN** the service rejects or ignores that setting and preserves baseline ranking

#### Scenario: Ranking policy references hidden evidence
- **WHEN** active ranking signals reference suppressed, forgotten, expired, deleted, or out-of-scope memory
- **THEN** search excludes hidden memory from default results and exposes only lifecycle-safe aggregate diagnostics where authorized

## ADDED Requirements

### Requirement: Search exposes ranking rollout diagnostics
The service SHALL expose bounded diagnostics explaining how an active or dry-run ranking rollout policy affected search results.

#### Scenario: Search runs with rollout diagnostics
- **WHEN** an authorized request asks for ranking rollout diagnostics or dry-run comparison
- **THEN** the response can include baseline rank, adjusted rank, bounded reason codes, signal categories, and evidence counts for returned hits

#### Scenario: Search diagnostics include omitted hidden memory
- **WHEN** hidden or out-of-scope memory would otherwise have ranking evidence
- **THEN** diagnostics do not expose that memory content or identifier through public search results
