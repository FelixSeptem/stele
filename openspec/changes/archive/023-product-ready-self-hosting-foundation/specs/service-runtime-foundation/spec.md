## ADDED Requirements

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

