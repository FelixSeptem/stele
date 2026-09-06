## ADDED Requirements

### Requirement: Product verification uses real PostgreSQL and pgvector
The repository SHALL provide a bounded automated product-verification suite
that exercises the supported self-hosted stack against real PostgreSQL with
pgvector rather than only mocks or in-memory substitutions.

#### Scenario: Verification environment is created
- **WHEN** the documented product-verification command runs in an environment
  with its container prerequisite available
- **THEN** it starts isolated labelled PostgreSQL/pgvector and Stele runtime
  resources with generated test secrets and bounded timeouts

#### Scenario: Container prerequisite is unavailable locally
- **WHEN** an operator runs the product-verification command without its
  documented container prerequisite
- **THEN** it returns a clear prerequisite or explicit local-skip result without
  claiming that product verification passed

### Requirement: Product verification proves the protected memory lifecycle
The product-verification suite SHALL prove the documented bootstrap-admin-first
flow, durable principal creation, exact scope grant enforcement, idempotent
event ingestion, asynchronous governance, retrieval, and context assembly.

#### Scenario: Fresh stack completes golden memory flow
- **WHEN** the suite starts a fresh supported stack
- **THEN** it bootstraps the first durable administrator, creates a scoped
  runtime principal, performs an idempotent event retry, observes background
  processing, and verifies the resulting data through authorized retrieval and
  context APIs

#### Scenario: Caller crosses scope boundary
- **WHEN** the suite uses a credential against a valid-looking ungranted
  tenant, project, or namespace
- **THEN** the request is denied without revealing the target scope or allowing
  reads or writes

### Requirement: Product verification proves restart and drain safety
The product-verification suite SHALL prove bounded shutdown and restart behavior
for API, worker, and scheduler modes without duplicating idempotent writes or
leaking unavailable work into ready status.

#### Scenario: API receives termination during normal operation
- **WHEN** the suite sends the documented termination signal to API mode
- **THEN** readiness becomes non-ready before drain completes, in-flight work is
  bounded by the configured shutdown timeout, and the process exits cleanly

#### Scenario: Runtime restarts after durable work
- **WHEN** the suite restarts an affected runtime after an accepted idempotent
  event or pending background work
- **THEN** the system resumes through durable state, does not duplicate the raw
  event, and eventually produces lifecycle-safe retrieval/context results

### Requirement: Product verification proves migration and recovery paths
The product-verification suite SHALL test forward upgrade from a prior populated
schema fixture and a backup/restore verification target before the release gate
can pass.

#### Scenario: Prior schema fixture upgrades
- **WHEN** the suite applies the current service migration path to its prior
  populated schema fixture
- **THEN** it proves migration status is clean and existing authorized scoped
  data remains usable

#### Scenario: Backup is restored into verification target
- **WHEN** the suite creates a bounded test backup and restores it into its own
  distinct disposable target
- **THEN** restore verification proves schema currency and scoped read behavior
  without mutating the original source target

### Requirement: Verification cleanup is ownership-safe
The product-verification suite SHALL clean up only resources it owns and SHALL
preserve diagnostics on failure.

#### Scenario: Verification completes or fails
- **WHEN** the suite exits successfully or unsuccessfully
- **THEN** it removes only uniquely labelled containers, networks, volumes, and
  databases it created, retains or reports bounded diagnostic artifacts on
  failure, and never targets an operator's unlabelled PostgreSQL data

