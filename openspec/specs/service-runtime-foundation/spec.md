# service-runtime-foundation Specification

## Purpose
Define the baseline runtime contract for the Stele service, including mode-based startup, environment-backed configuration validation, and unauthenticated health/readiness endpoints.
## Requirements
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

### Requirement: Assurance and conformance runtime settings are validated
The service MUST validate runtime configuration for assurance cadence, conformance cadence, operational proof checks, retention windows, and alert delivery before serving API traffic or starting worker or scheduler execution.

#### Scenario: Assurance cadence uses default fallback
- **WHEN** assurance or conformance cadence settings are omitted
- **THEN** the service uses the existing maintenance interval as the default cadence without changing the supported `api`, `worker`, and `scheduler` runtime modes

#### Scenario: Retention settings are invalid
- **WHEN** assurance or conformance retention windows are negative, unparsable, or shorter than minimum safe bounds
- **THEN** startup fails with an actionable configuration error before cleanup jobs can run

#### Scenario: Operational proof settings are invalid
- **WHEN** capacity/load thresholds or backup/restore proof freshness windows are invalid, unbounded, or internally inconsistent
- **THEN** startup fails with an actionable configuration error rather than treating missing proof as healthy

#### Scenario: Webhook settings are unsafe
- **WHEN** alert delivery is configured for `webhook` with an unsupported scheme, unsafe local or metadata network target, missing explicit local override for insecure endpoints, rejected header, invalid timeout, or oversized payload limit
- **THEN** startup fails or the adapter remains disabled with an actionable configuration error before outbound delivery is attempted

### Requirement: Principal bootstrap configuration is fail-safe
The service MUST validate bootstrap-operator and deprecated legacy key settings
before serving protected traffic or starting background execution.

#### Scenario: Bootstrap configuration is incomplete
- **WHEN** a bootstrap credential is configured without a valid default tenant, project, and namespace
- **THEN** startup fails with an actionable configuration error

#### Scenario: Deprecated unrestricted key configuration is present
- **WHEN** legacy public or admin key allow-list settings are configured without the explicit constrained bootstrap migration setting
- **THEN** startup fails before the service accepts protected requests

#### Scenario: Principal-backed runtime starts
- **WHEN** PostgreSQL principal storage and valid principal-access configuration are available
- **THEN** API mode starts protected routes and worker or scheduler modes use the same principal-access configuration without exposing credentials

