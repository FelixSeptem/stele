# database-schema-migration-management Specification

## Purpose
TBD - created by archiving change product-ready-self-hosting-foundation. Update Purpose after archive.
## Requirements
### Requirement: PostgreSQL schema changes use an immutable ordered migration ledger
The service SHALL manage PostgreSQL schema evolution through immutable,
ordered, versioned migrations and a database-resident ledger that records the
applied version and dirty state. The released initial migration SHALL represent
the existing supported base schema, and service startup SHALL NOT replay a
mutable aggregate schema file as an upgrade mechanism.

#### Scenario: Fresh database is initialized
- **WHEN** a valid empty PostgreSQL database with required extensions is prepared
- **THEN** the migration runner applies every ordered migration, records the
  resulting current version in the ledger, and leaves the database clean

#### Scenario: Database is already current
- **WHEN** the migration runner is invoked against a clean database at the
  current version
- **THEN** it performs no schema mutation and reports the current version as
  applied

#### Scenario: Migration asset is changed after release
- **WHEN** the runner detects a migration history that is missing, divergent,
  out of order, or incompatible with the recorded ledger state
- **THEN** it fails before application modes serve traffic or claim jobs and
  reports an actionable bounded migration error

### Requirement: Migration execution is serialized and detects incomplete state
The service SHALL serialize migration execution across `api`, `worker`,
`scheduler`, and standalone migration invocations using PostgreSQL-owned
coordination. It MUST not treat a dirty or partially applied migration state as
ready.

#### Scenario: Multiple runtime modes start concurrently
- **WHEN** two or more service modes start against a database with pending
  migrations
- **THEN** exactly one invocation applies a migration at a time and every mode
  either observes the resulting clean version or waits/fails with a bounded
  migration-in-progress diagnostic

#### Scenario: Migration fails after it starts
- **WHEN** a migration cannot complete because of SQL, connectivity, or process
  interruption failure
- **THEN** the ledger records or preserves dirty/incomplete state and later
  startup refuses to report ready until an operator completes the documented
  recovery procedure

#### Scenario: Runtime sees a dirty migration ledger
- **WHEN** an application mode starts with a dirty or inconsistent migration
  ledger
- **THEN** it fails before opening protected HTTP traffic or background job
  claims and does not silently retry unrelated schema operations

### Requirement: Migration policy is explicit for every runtime mode
The service SHALL support documented migration policies for all runtime modes
and a standalone migration command. The default policy MUST be safe for the
supported self-hosted baseline and an externally managed policy MUST validate
rather than silently ignore pending schema work.

#### Scenario: Auto policy starts a supported deployment
- **WHEN** a supported single-node deployment uses the default auto-migration
  policy with pending clean forward migrations
- **THEN** the first runtime serializes migration execution before starting and
  subsequent modes validate the resulting current version

#### Scenario: Validate policy sees pending migration
- **WHEN** an operator configures validate-only migration policy and the
  database version is behind the image requirement
- **THEN** the runtime exits with a diagnostic identifying that an explicit
  migration command must be run before startup

#### Scenario: Operator inspects or applies migrations directly
- **WHEN** an operator runs the documented standalone migration status or
  forward-apply command with valid database configuration
- **THEN** the command uses the same ledger and coordination semantics as
  runtime startup and returns machine-readable success or failure status

### Requirement: Upgrades from a populated prior release are supported
The service SHALL verify forward migration from an already-populated database
created by the prior supported release without losing scoped records,
provenance, principal/grant state, idempotency state, or canonical memory
history.

#### Scenario: Prior database is upgraded
- **WHEN** a fixture representing the immediately preceding supported schema
  contains scoped memory and principal data and a new image applies pending
  migrations
- **THEN** the migration completes forward, existing scoped records remain
  readable through their authorized APIs, and a new scoped idempotent write can
  succeed without duplicating prior data

#### Scenario: Operator needs rollback after a forward migration
- **WHEN** a deployed image must be rolled back after a schema change
- **THEN** documentation and command output state whether the prior image is
  schema-compatible and otherwise direct the operator to a verified backup
  restore or a forward remediation migration rather than automatic downgrade

