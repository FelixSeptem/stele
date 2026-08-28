## ADDED Requirements

### Requirement: Compose uses the constrained bootstrap-admin access path
The documented Compose deployment SHALL configure all Stele modes with the
principal-backed bootstrap-admin settings accepted by the current runtime. It
MUST NOT configure deprecated unrestricted public or admin API key allow-list
settings.

#### Scenario: Operator starts documented local Compose stack
- **WHEN** an operator follows the documented Compose startup flow with the
  required bootstrap-admin secret and default scope values
- **THEN** PostgreSQL, API, worker, and scheduler pass configuration validation
  and start using the same accepted authorization model

#### Scenario: Obsolete credential settings are introduced
- **WHEN** a Compose file, environment example, smoke command, or operational
  documentation references deprecated unrestricted key allow-list settings as a
  live configuration path
- **THEN** automated deployment documentation checks fail and point to the
  bootstrap-admin-first replacement flow

### Requirement: First deployment establishes durable least-privilege principals
The self-hosting guide SHALL document a repeatable first-deployment sequence
that uses the constrained bootstrap credential only to create a durable admin,
then creates a separately scoped runtime principal for normal memory traffic.

#### Scenario: Operator establishes first trust chain
- **WHEN** an operator follows the documented first-deployment sequence against
  a fresh database
- **THEN** the guide shows how to authenticate as the bootstrap operator in its
  default scope, create the first durable administrator, store the one-time
  credential securely, create an exact grant for a non-admin runtime principal,
  and use that runtime credential for memory APIs

#### Scenario: Durable administrator is established
- **WHEN** the first durable admin exists and no documented emergency override
  is configured
- **THEN** the guide states that bootstrap access is rejected and does not treat
  the bootstrap credential as a routine application credential

### Requirement: Deployment profiles state local and production secret boundaries
The repository SHALL distinguish its bundled local evaluation profile from the
production self-hosted profile and SHALL document all required configuration,
secret, database, TLS/reverse-proxy, migration, and runtime-mode responsibilities.

#### Scenario: Operator evaluates locally
- **WHEN** an operator uses the bundled local Compose profile
- **THEN** the documentation identifies its generated/example credentials and
  bundled PostgreSQL as local-only and provides the exact startup, bootstrap,
  smoke, and cleanup commands

#### Scenario: Operator prepares production deployment
- **WHEN** an operator prepares the supported production self-hosted profile
- **THEN** the documentation requires externally supplied non-default secrets,
  an operator-managed PostgreSQL DSN, explicit migration policy, and documented
  TLS/reverse-proxy request-boundary responsibilities without requiring a
  vendor-specific secret manager or platform

### Requirement: Fresh-install smoke proves protected lifecycle behavior
The documented initial smoke loop SHALL validate a real protected lifecycle,
not only liveness and readiness endpoints.

#### Scenario: Operator runs fresh-install smoke
- **WHEN** a newly bootstrapped operator follows the canonical smoke command
- **THEN** it verifies migration status, API contract discovery, durable
  principal and grant authorization, idempotent event ingest, worker/scheduler
  processing, scoped retrieval, context assembly, and the relevant readiness or
  assurance evidence for the same exact scope

#### Scenario: Fresh-install smoke is interrupted
- **WHEN** any bootstrap or lifecycle smoke step fails
- **THEN** the guide identifies the next migration status, auth/grant,
  health/readiness, job, retrieval, context, or runtime log surface to inspect
  without exposing credentials in copied commands or diagnostic output

