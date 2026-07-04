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
