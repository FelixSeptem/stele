## MODIFIED Requirements

### Requirement: Active rollout changes ranking without changing visibility
The service SHALL apply an active scoped rollout policy as a bounded ranking hint layer for configured search or context surfaces, including optional quality-feature and reranker identities, while preserving lifecycle and scope isolation as primary visibility controls.

#### Scenario: Active policy applies to search
- **WHEN** a caller searches memory in a scope with an active matching rollout policy
- **THEN** the service can adjust ranking using approved usefulness, task-success, verification, quality-feature, and optional reranker signals while excluding hidden or out-of-scope memory from results

#### Scenario: Active policy references provider credentials
- **WHEN** a rollout policy is created or activated
- **THEN** the durable policy stores only logical provider/model and strategy identities, never endpoint URLs, API keys, DSNs, or raw provider payloads

#### Scenario: No matching active policy exists
- **WHEN** a caller searches or assembles context without an active matching policy
- **THEN** the service preserves baseline ranking behavior

#### Scenario: Active policy applies to context assembly
- **WHEN** a caller assembles context in a scope with an active matching rollout policy
- **THEN** the service can adjust candidate priority within budget and section rules while preserving citations, lifecycle visibility, and context safety

#### Scenario: Request asks for diagnostics while policy is active
- **WHEN** a caller requests ranking diagnostics or dry-run comparison while an active policy applies to the resolved scope
- **THEN** the service may include bounded policy impact diagnostics while ordinary result ordering follows the active policy unless the request is explicitly diagnostics-only

### Requirement: Ranking signals are rebuildable and bounded
The service SHALL derive ranking signals from durable active feedback, task evaluations, session verification, quality findings, and versioned quality-feature summaries using bounded categories and rebuildable summaries.

#### Scenario: Signal summary is rebuilt
- **WHEN** ranking signal aggregation is rerun or repaired
- **THEN** the service recomputes summaries from durable source evidence and records the feature/policy version used

#### Scenario: Single negative event exists
- **WHEN** a single low-confidence negative event exists below the policy threshold
- **THEN** the service records diagnostics but does not apply that signal as a default ranking adjustment

#### Scenario: Superseded feedback exists
- **WHEN** feedback or task evidence has been superseded or corrected
- **THEN** active rollout signals exclude superseded evidence by default while preserving history for admin inspection

#### Scenario: Quality provider fails during aggregation
- **WHEN** an optional provider is unavailable while signals are being evaluated
- **THEN** the service marks the provider signal unavailable and retains the deterministic baseline
