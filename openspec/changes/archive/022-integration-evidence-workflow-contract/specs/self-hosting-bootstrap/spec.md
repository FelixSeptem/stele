## ADDED Requirements

### Requirement: Self-hosting docs cover the golden integration workflow
The service SHALL document a golden external-agent integration workflow that connects memory sessions, context assembly, outcomes, verification, feedback, task evaluation, proof, conformance, readiness, incidents, and recovery verification through workflow contracts.

#### Scenario: Operator follows golden workflow
- **WHEN** an operator follows the self-hosting guide after smoke checks and scope proof
- **THEN** the guide shows how to create a workflow template, start a workflow run, record step evidence, inspect next actions, run conformance, inspect readiness, and verify recovery for the same scope

#### Scenario: Integration misses evidence
- **WHEN** a workflow run reports missing, stale, opaque-only, invalid, or out-of-scope evidence
- **THEN** the guide identifies the relevant Stele public or admin surface used to record or inspect the missing evidence

#### Scenario: Guide states service boundary
- **WHEN** an operator reads the workflow contract documentation
- **THEN** the documentation states that SDK/UI, external agent execution, model invocation, prompt orchestration, tool orchestration, and final-answer generation remain outside Stele's service boundary
