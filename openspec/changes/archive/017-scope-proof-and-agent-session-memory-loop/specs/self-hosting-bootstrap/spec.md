## ADDED Requirements

### Requirement: Self-hosted scope proof workflow is documented
The service SHALL document a durable scope proof workflow for newly bootstrapped deployments.

#### Scenario: Operator proves a fresh scope
- **WHEN** an operator follows the self-hosting guide for a fresh deployment
- **THEN** the guide shows how to create a scope proof run, inspect its report, diagnose failed steps, and rerun the proof after remediation

#### Scenario: Proof replaces manual-only smoke checks
- **WHEN** a documented smoke step is covered by the proof workflow
- **THEN** the documentation points operators to the durable proof run result instead of requiring direct interpretation of unrelated endpoint responses

### Requirement: Self-hosted memory session workflow is documented
The service SHALL document a minimal memory session workflow for external agents.

#### Scenario: Operator tests session memory integration
- **WHEN** an operator or integrator follows the self-hosting guide
- **THEN** the guide shows how an external agent can create a session, request context, record turn outcome events, verify recall, and inspect the session report

#### Scenario: Session workflow fails
- **WHEN** the documented session workflow fails
- **THEN** the guide maps failure categories to the relevant admin inspection, quality evaluation, repair plan, job status, or context diagnostic surface
