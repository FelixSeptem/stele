# self-hosting-bootstrap Specification

## Purpose
Define the self-hosting bootstrap assets and operator guidance required to run the service in containerized environments.
## Requirements
### Requirement: Self-host startup assets
The service SHALL provide container-oriented startup assets for operators to run the `api`, `worker`, and `scheduler` modes in self-hosted environments.

#### Scenario: New operator bootstraps the service
- **WHEN** an operator follows the provided container startup assets
- **THEN** they can launch the required runtime modes without reverse-engineering the repository layout or command model

### Requirement: Docker image packaging
The service MUST provide a `Dockerfile` for building a deployable `stele` service image.

#### Scenario: Operator builds a service image
- **WHEN** an operator builds the repository container image
- **THEN** the repository provides a documented `Dockerfile` that can produce a runnable `stele` image for mode-based startup

### Requirement: Compose-based local self-hosting
The service MUST provide a `docker-compose.yml` for local or small-scale self-hosted startup.

#### Scenario: Operator starts the full runtime stack locally
- **WHEN** an operator runs the documented compose startup flow
- **THEN** the compose configuration brings up PostgreSQL plus the `api`, `worker`, and `scheduler` service modes with the required runtime wiring

### Requirement: Bootstrap documentation
The service MUST document runtime prerequisites and configuration clearly enough for repeatable self-host setup.

#### Scenario: Operator prepares PostgreSQL and environment configuration
- **WHEN** an operator reads the bootstrap documentation
- **THEN** the documentation specifies required PostgreSQL extensions, configuration variables, runtime modes, and initialization steps

### Requirement: Smoke-checkable installation
The service MUST provide a basic verification path for newly bootstrapped environments.

#### Scenario: Operator verifies a fresh deployment
- **WHEN** a new self-hosted environment starts for the first time
- **THEN** the operator can run a documented smoke check that confirms health, readiness, and the baseline runtime wiring

### Requirement: Bootstrap guidance covers semantic provider configuration
The service MUST document the configuration and startup expectations needed to run semantic rebuild execution with concrete embedding providers in self-hosted deployments.

#### Scenario: Operator prepares provider-backed deployment
- **WHEN** an operator reads the bootstrap documentation for a deployment that intends to use semantic rebuilds
- **THEN** the documentation specifies the required embedding route configuration, provider-specific settings, and failure modes for missing or invalid provider wiring

#### Scenario: Operator prepares lexical-only deployment
- **WHEN** an operator chooses to run without semantic providers
- **THEN** the documentation explains that lexical-plus-relation retrieval can still run while semantic rebuild execution remains intentionally inactive

### Requirement: Smoke checks distinguish semantic readiness from baseline startup
The service MUST provide a verification path that lets operators confirm whether embedding rebuild execution is truly wired, not merely whether the process is up.

#### Scenario: Provider-backed deployment verifies semantic readiness
- **WHEN** an operator runs the documented smoke check for a provider-backed deployment
- **THEN** the check confirms both baseline service readiness and the presence of actionable semantic rebuild wiring or embedding diagnostics

#### Scenario: Degraded deployment verifies expected semantic inactivity
- **WHEN** an operator runs the documented smoke check for a lexical-only deployment
- **THEN** the check can confirm that semantic rebuild execution is intentionally unavailable rather than silently misconfigured

### Requirement: Bootstrap guidance covers provider cutover operations
The service MUST document the operator workflow for creating, activating, pausing, cancelling, and rolling back embedding provider cutovers.

#### Scenario: Operator prepares a provider cutover rollout
- **WHEN** an operator reads the bootstrap documentation before migrating a scope to a new embedding target
- **THEN** the documentation describes the required admin routes, rollout sequencing, runtime validation expectations, and progress inspection workflow

#### Scenario: Operator plans rollback after a failed cutover
- **WHEN** an operator needs to reverse a provider migration
- **THEN** the documentation explains that rollback is modeled as a new forward cutover plan toward the prior target rather than as direct vector history mutation

### Requirement: Smoke checks cover cutover progress and recovery audit
The service MUST provide operator guidance for verifying cutover progress and recovery history during rollout incidents.

#### Scenario: Operator monitors an active cutover
- **WHEN** an operator runs the documented smoke check for an active provider cutover
- **THEN** the documented workflow confirms how to inspect plan progress, backlog pressure, and memory-level cutover context in addition to baseline readiness

#### Scenario: Operator investigates remediation during rollout
- **WHEN** an operator follows the documented incident workflow for a failed provider cutover
- **THEN** the documentation shows how to query embedding recovery history at both scope and memory level to explain retry or requeue activity

### Requirement: First-ten-minutes smoke loop covers ingest to context
The service MUST provide a documented first-ten-minutes smoke loop that proves a self-hosted deployment can start, ingest scoped events, process background work, retrieve memory, assemble context, inspect admin state, and observe runtime signals.

#### Scenario: Operator runs full smoke loop
- **WHEN** an operator follows the smoke loop for a fresh self-hosted deployment
- **THEN** the documented flow verifies readiness, event ingestion, worker processing, scheduler maintenance, memory search, context assembly, admin inspection, and metrics without requiring source-code inspection

#### Scenario: Smoke loop fails
- **WHEN** a smoke-loop step fails
- **THEN** the documentation identifies the next admin inspection, readiness diagnostic, job status, or metric check needed to distinguish configuration, database, worker, scheduler, retrieval, or replay failure

### Requirement: Smoke loop includes derived insight replay verification
The service SHALL include derived insight dry-run and bounded apply verification in the self-hosted smoke loop when derived insight processing is enabled.

#### Scenario: Operator verifies replay dry-run
- **WHEN** the smoke fixture has generated eligible insight evidence
- **THEN** the documented smoke loop can run an admin replay dry-run and show expected replay decisions without mutating insight state

#### Scenario: Operator verifies replay apply
- **WHEN** an operator executes the bounded smoke replay apply step
- **THEN** the documented flow shows how to inspect replay status, replay report counters, resulting derived insight visibility, and context assembly output

### Requirement: Smoke fixtures remain scope-safe
The service MUST keep smoke fixtures and examples scoped to explicit tenant, project, and namespace values and SHALL NOT rely on global or cross-scope retrieval behavior.

#### Scenario: Smoke data is ingested
- **WHEN** the smoke loop writes test events or evaluates replay
- **THEN** every request carries explicit scope and later verification reads only from that same authorized scope

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

