## ADDED Requirements

### Requirement: Self-hosting docs cover task-success and ranking rollout loop
The service SHALL document a self-hosted task-success and feedback-ranking rollout workflow for external agent integrations.

#### Scenario: Operator validates task-success loop
- **WHEN** an operator follows the self-hosting guide after creating a memory session and recording usefulness feedback
- **THEN** the guide shows how to record a task evaluation, link it to session evidence, inspect task reports, and diagnose memory contribution categories

#### Scenario: Operator dry-runs ranking rollout
- **WHEN** an operator wants task and feedback signals to influence future recall
- **THEN** the guide shows how to create a ranking rollout policy, run a dry-run impact report, inspect changed ranking diagnostics, and verify evidence thresholds before activation

#### Scenario: Operator rolls back ranking rollout
- **WHEN** ranking rollout diagnostics show degraded or unsafe behavior
- **THEN** the guide shows how to disable or roll back the policy and verify that baseline search and context ranking are restored

#### Scenario: Guide states product boundaries
- **WHEN** an operator reads the task-success and ranking rollout documentation
- **THEN** the documentation states that SDK/UI, external agent runtime integration, model judgment, prompt orchestration, and final answer generation remain outside Stele's service boundary
