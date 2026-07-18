## ADDED Requirements

### Requirement: Self-hosting docs cover operational assurance and conformance
The service SHALL document a production-readiness workflow that follows smoke checks with assurance evaluation, capacity/load proof, backup/restore proof, integration conformance, incident inspection, alert candidate review, and recovery verification.

#### Scenario: Operator validates scope readiness
- **WHEN** an operator follows the self-hosting guide after completing smoke checks
- **THEN** the guide shows how to run health evaluation, conformance checks, and scope readiness inspection for the same scope

#### Scenario: Operator handles degraded readiness
- **WHEN** a readiness report identifies degraded runtime, capacity/load, backup/restore, integration, repair, ranking, feedback, task, or session health
- **THEN** the guide shows the relevant admin surfaces and runbook hints to investigate and remediate the condition

#### Scenario: Operator configures alert delivery
- **WHEN** an operator wants assurance alerts in a self-hosted deployment
- **THEN** the guide documents `disabled`, `stdout`, and generic `webhook` adapters, redaction expectations, retry behavior, HTTPS-by-default behavior, unsafe target rejection, bounded payloads/timeouts, and high-cardinality safety boundaries

#### Scenario: Operator validates operational proof
- **WHEN** an operator follows the production-readiness workflow
- **THEN** the guide explains how capacity/load proof and backup/restore proof participate in readiness without making Stele own load generation, backup scheduling, or restore execution

#### Scenario: Guide states product boundaries
- **WHEN** an operator reads the assurance and conformance documentation
- **THEN** the documentation states that SDK/UI, external agent runtime execution, model invocation, prompt orchestration, and final answer generation remain outside Stele's service boundary
