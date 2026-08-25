## ADDED Requirements

### Requirement: Ranking rollout policies are durable and scoped
The service SHALL allow authorized administrators to create durable feedback and task-success-aware ranking rollout policies scoped by tenant, project, and namespace.

#### Scenario: Administrator creates rollout policy
- **WHEN** an authorized administrator creates a ranking rollout policy with scope, surfaces, signal sources, thresholds, mode, actor, and reason
- **THEN** the service persists the policy with status, audit attribution, creation time, and default inactive behavior until explicitly activated

#### Scenario: Policy targets unauthorized scope
- **WHEN** an administrator creates, reads, activates, or rolls back a policy outside the authorized scope
- **THEN** the service rejects the request without exposing policy or evidence content

#### Scenario: Policy uses unsupported signal
- **WHEN** a policy references an unsupported or unbounded ranking signal
- **THEN** the service rejects the policy rather than creating ungoverned ranking behavior

### Requirement: Ranking rollout requires dry-run impact evidence
The service MUST support dry-run reports that compare baseline ranking with feedback and task-success-aware ranking before activation.

#### Scenario: Administrator runs ranking dry-run
- **WHEN** an administrator runs a dry-run for a rollout policy over bounded queries or fixture requests
- **THEN** the service records baseline ranking, adjusted ranking, changed subjects, bounded reason codes, evidence counts, and lifecycle-safe impact summaries

#### Scenario: Dry-run references hidden evidence
- **WHEN** hidden, suppressed, forgotten, deleted, or out-of-scope evidence contributes to a dry-run decision
- **THEN** the dry-run report exposes only aggregate lifecycle-safe diagnostics and does not surface hidden content

#### Scenario: Dry-run lacks evidence threshold
- **WHEN** a policy dry-run has insufficient active feedback, task evaluation, verification, or quality evidence for the configured threshold
- **THEN** the service reports insufficient evidence and prevents activation unless the policy explicitly remains diagnostics-only

### Requirement: Ranking rollout activation is gated
The service MUST activate a ranking rollout policy only after bounded activation gates pass for the same resolved scope and configured surfaces.

#### Scenario: Activation follows successful dry-run
- **WHEN** an administrator activates a rollout policy after a successful dry-run for the same scope, surfaces, signal sources, and thresholds
- **THEN** the service records the activation with actor and reason attribution and makes the policy eligible for active ranking evaluation

#### Scenario: Activation lacks dry-run
- **WHEN** an administrator attempts to activate a rollout policy without a successful matching dry-run
- **THEN** the service rejects activation and preserves baseline ranking behavior

#### Scenario: Activation has blocker evidence
- **WHEN** a rollout policy has active blocker evidence such as unsafe hidden-memory feedback, failed verification, or insufficient evidence threshold status
- **THEN** the service rejects activation or keeps the policy diagnostics-only until the blocker is resolved

### Requirement: Active rollout changes ranking without changing visibility
The service SHALL apply an active scoped rollout policy as a ranking hint layer for configured search or context surfaces while preserving lifecycle and scope isolation as primary visibility controls.

#### Scenario: Active policy applies to search
- **WHEN** a caller searches memory in a scope with an active matching rollout policy
- **THEN** the service can adjust ranking using active usefulness feedback, task-success summaries, verification outcomes, and quality signals while excluding hidden or out-of-scope memory from results

#### Scenario: Active policy applies to context assembly
- **WHEN** a caller assembles context in a scope with an active matching rollout policy
- **THEN** the service can adjust candidate priority within budget and section rules while preserving citations, lifecycle visibility, and context safety

#### Scenario: No matching active policy exists
- **WHEN** a caller searches or assembles context without an active matching rollout policy and without per-request feedback-aware ranking
- **THEN** the service preserves baseline ranking behavior

#### Scenario: Request asks for diagnostics while policy is active
- **WHEN** a caller requests ranking diagnostics or dry-run comparison while an active policy applies to the resolved scope
- **THEN** the service may include policy impact diagnostics while ordinary result ordering follows the active policy unless the request is explicitly diagnostics-only

### Requirement: Rollout policies are reversible and auditable
The service MUST support pausing, disabling, and rolling back ranking rollout policies without mutating task evaluations, feedback records, or canonical memory.

#### Scenario: Administrator rolls back active policy
- **WHEN** an authorized administrator rolls back an active rollout policy with actor and reason attribution
- **THEN** the service records rollback history, disables the active ranking adjustment, and preserves policy, dry-run, impact, and evidence history

#### Scenario: Ranking behavior after rollback
- **WHEN** search or context assembly runs after rollback for that scope
- **THEN** the service returns to baseline ranking unless another active matching policy or explicit per-request ranking flag applies

### Requirement: Rollout impact reports are inspectable
The service SHALL expose scoped admin reports for rollout policy status, dry-run output, active impact, rollback history, and evidence summaries.

#### Scenario: Administrator reads rollout report
- **WHEN** an authorized administrator reads a rollout report within scope
- **THEN** the response includes policy status, configured surfaces, active signal categories, threshold status, dry-run ids, impact counters, rollback history, and bounded reason codes

#### Scenario: Report contains high-cardinality evidence
- **WHEN** rollout impact references specific task evaluations, feedback, sessions, memories, or quality findings
- **THEN** the service stores detailed ids in scoped durable evidence and excludes them from metric labels and public diagnostics

### Requirement: Ranking signals are rebuildable and bounded
The service SHALL derive rollout ranking signals from durable active feedback, task evaluations, session verification, and quality findings using bounded categories and rebuildable summaries.

#### Scenario: Signal summary is rebuilt
- **WHEN** ranking signal aggregation is rerun or repaired
- **THEN** the service recomputes signal summaries from durable source evidence rather than relying on non-auditable mutable scores

#### Scenario: Superseded feedback exists
- **WHEN** feedback or task evidence has been superseded or corrected
- **THEN** active rollout signals exclude superseded evidence by default while preserving history for admin inspection

#### Scenario: Single negative event exists
- **WHEN** a single negative feedback or task failure exists below the policy threshold
- **THEN** the service records diagnostics but does not apply that signal as a default ranking adjustment
