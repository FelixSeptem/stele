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

### Requirement: API runtime enforces bounded HTTP resource limits
API mode SHALL configure bounded server timeout, header-size, and request-body
limits before accepting traffic. Every JSON endpoint that accepts an untrusted
body MUST apply the configured body limit before decoding.

#### Scenario: Client sends body within configured limit
- **WHEN** an authorized or public client sends a syntactically valid request
  whose body and headers are within configured limits
- **THEN** the API processes it with existing route, authentication, scope, and
  validation semantics unchanged

#### Scenario: Client exceeds body or header limit
- **WHEN** a request exceeds the configured maximum body or header size
- **THEN** the API returns a bounded client error, does not partially persist a
  request, and does not include request content or credentials in telemetry

#### Scenario: Timeout or size settings are invalid
- **WHEN** any configured server timeout, header limit, or body limit is zero
  where unsafe, negative, unparsable, or outside documented safe bounds
- **THEN** the affected runtime fails startup with an actionable configuration
  error before accepting traffic

### Requirement: All runtime modes perform signal-driven graceful shutdown
The service SHALL derive its runtime context from supported process termination
signals and SHALL stop API, worker, and scheduler modes through bounded graceful
shutdown rather than an uncancelable background context.

#### Scenario: API mode receives SIGTERM or SIGINT
- **WHEN** API mode receives a supported process termination signal
- **THEN** it marks readiness non-ready, stops accepting new work, attempts
  `http.Server` shutdown within the configured drain timeout, and closes
  allocated dependencies exactly once before exiting

#### Scenario: Worker or scheduler receives termination signal
- **WHEN** worker or scheduler mode receives a supported process termination
  signal
- **THEN** it stops claiming new work, propagates cancellation to active loops,
  preserves durable retry/lease semantics for unfinished jobs, closes allocated
  dependencies exactly once, and exits within the documented bounded drain
  behavior

#### Scenario: Startup fails after dependency allocation
- **WHEN** bootstrap, migration validation, server construction, or loop startup
  fails after one or more dependencies are allocated
- **THEN** the runtime releases those dependencies and returns the original
  actionable failure without leaving a serving listener or active loop

### Requirement: Runtime startup reports migration compatibility before work
Every runtime mode SHALL validate its database migration state according to the
configured migration policy before serving protected API traffic or claiming
background jobs.

#### Scenario: Database state is current
- **WHEN** runtime startup validates a clean schema compatible with its image
- **THEN** API mode can expose protected routes and worker/scheduler modes can
  begin normal execution

#### Scenario: Database state is incompatible
- **WHEN** startup finds pending, dirty, newer-incompatible, or divergent schema
  state under the selected migration policy
- **THEN** the mode fails before protected traffic or job execution and exposes
  only bounded migration/readiness diagnostics

