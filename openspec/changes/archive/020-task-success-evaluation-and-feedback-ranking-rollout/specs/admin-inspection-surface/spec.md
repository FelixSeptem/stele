## ADDED Requirements

### Requirement: Task evaluations are inspectable through admin surfaces
The service SHALL expose admin-only inspection for task evaluations, task summaries, task evidence links, and task evaluation correction history.

#### Scenario: Administrator lists task evaluations
- **WHEN** an authorized administrator lists task evaluations for a scope with optional verdict, contribution category, session, memory subject, time, or source filters
- **THEN** the admin surface returns matching task evaluation summaries with bounded verdicts, contribution categories, attribution, supersession state, and evidence link summaries

#### Scenario: Administrator reads task evaluation detail
- **WHEN** an authorized administrator reads one task evaluation within scope
- **THEN** the admin surface returns task objective summary, success criteria summary, external verdict, evidence links, linked session or feedback context, audit attribution, and active-summary participation state

#### Scenario: Administrator inspects superseded task evaluation
- **WHEN** an authorized administrator includes superseded task evaluations in an inspection request
- **THEN** the admin surface returns original and superseding task evaluation records with attribution and active-summary participation state

#### Scenario: Administrator requests out-of-scope task evaluation
- **WHEN** an administrator requests task evaluation evidence outside an authorized scope
- **THEN** the admin surface rejects the request without exposing task existence, verdict, evidence, or linked memory content

### Requirement: Ranking rollout policies are inspectable through admin surfaces
The service SHALL expose admin-only inspection and bounded controls for ranking rollout policies, dry-run reports, impact reports, active status, and rollback history.

#### Scenario: Administrator lists ranking rollout policies
- **WHEN** an authorized administrator lists ranking rollout policies for a scope
- **THEN** the admin surface returns policy status, configured surfaces, signal sources, threshold state, latest dry-run status, activation state, rollback state, and bounded impact counters

#### Scenario: Administrator reads rollout impact report
- **WHEN** an authorized administrator reads a rollout dry-run or active impact report within scope
- **THEN** the admin surface returns baseline versus adjusted ranking summaries, changed lifecycle-visible subjects, bounded reason codes, evidence counts, and hidden-evidence aggregate diagnostics

#### Scenario: Administrator controls rollout policy
- **WHEN** an authorized administrator activates, disables, or rolls back a rollout policy with actor and reason attribution
- **THEN** the admin surface records a bounded policy transition without mutating canonical memory, feedback records, task evaluations, or session history

#### Scenario: Administrator requests out-of-scope rollout policy
- **WHEN** an administrator requests ranking rollout policy or impact content outside an authorized scope
- **THEN** the admin surface rejects the request without exposing policy existence or ranking evidence
