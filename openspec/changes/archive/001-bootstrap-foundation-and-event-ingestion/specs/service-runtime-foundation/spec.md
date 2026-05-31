## ADDED Requirements

### Requirement: Service runtime modes
The service SHALL start from a single binary entrypoint and SHALL support `api`, `worker`, and `scheduler` runtime modes.

#### Scenario: API mode startup
- **WHEN** the service is started in `api` mode with valid configuration
- **THEN** the process starts the HTTP server and initializes shared dependencies required by API handlers

#### Scenario: Worker mode startup
- **WHEN** the service is started in `worker` mode with valid configuration
- **THEN** the process initializes shared dependencies and starts the background worker loop without starting the public HTTP server

#### Scenario: Scheduler mode startup
- **WHEN** the service is started in `scheduler` mode with valid configuration
- **THEN** the process initializes shared dependencies and starts scheduled maintenance execution without starting the public HTTP server

### Requirement: Configuration validation
The service MUST load configuration from environment-backed settings and MUST fail fast when required configuration is missing or invalid.

#### Scenario: Missing required database configuration
- **WHEN** the service starts without the required PostgreSQL connection configuration
- **THEN** startup fails before serving requests or starting background execution

#### Scenario: Invalid runtime mode
- **WHEN** the service is started with an unsupported runtime mode
- **THEN** startup fails with an actionable configuration error

### Requirement: Health and readiness endpoints
API mode SHALL expose health and readiness endpoints for operational checks.

#### Scenario: Liveness check
- **WHEN** an operator calls the health endpoint while the API process is running
- **THEN** the service returns a successful health response without requiring authentication

#### Scenario: Readiness check before database availability
- **WHEN** an operator calls the readiness endpoint and the database is not ready
- **THEN** the service returns a non-ready response indicating that dependencies are not available
