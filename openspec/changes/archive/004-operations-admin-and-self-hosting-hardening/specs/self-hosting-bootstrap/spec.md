## ADDED Requirements

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
