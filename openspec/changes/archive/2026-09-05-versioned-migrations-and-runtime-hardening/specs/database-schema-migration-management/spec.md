## ADDED Requirements

### Requirement: Applied migration assets have immutable integrity evidence
The service SHALL calculate a deterministic SHA-256 checksum for every ordered
embedded migration asset and SHALL persist the version, migration name,
checksum, and application timestamp in a PostgreSQL-resident Stele integrity
ledger. The runner SHALL validate every applied migration against this ledger
before reporting a schema current or admitting runtime work.

#### Scenario: Clean migrations match their embedded assets
- **WHEN** a database has a clean applied migration prefix and every persisted
  integrity record matches the binary's ordered embedded manifest
- **THEN** migration status reports the applicable current or pending state and
  includes bounded integrity facts without returning SQL, DSNs, or credentials

#### Scenario: Historical migration asset differs from ledger
- **WHEN** a persisted integrity record has a name or SHA-256 checksum that
  differs from the matching embedded version, or the records are missing,
  extra, duplicated, or non-contiguous outside supported legacy backfill
- **THEN** migration status is `divergent`, forward application is refused, and
  runtime modes fail before serving protected traffic or claiming jobs

### Requirement: Supported legacy migration ledger is reconciled safely
The service SHALL support a one-way integrity-ledger backfill only for a clean
database whose existing `schema_migrations` version is an exact prefix of the
binary's supported ordered manifest. Backfill SHALL be serialized with status
and application and SHALL NOT bless dirty, future, or otherwise divergent
history.

#### Scenario: Existing supported release is upgraded
- **WHEN** a clean prior supported database has no Stele integrity records and
  its driver version is an exact supported manifest prefix
- **THEN** the migration runner serializes integrity backfill, applies only
  pending forward migrations, and records the complete checksummed prefix

#### Scenario: Unsupported legacy history is encountered
- **WHEN** a database has a dirty driver ledger, a version newer than the
  binary, or a version that cannot be represented by the embedded manifest
- **THEN** the runner does not create integrity records and returns a bounded
  dirty, incompatible, or divergent diagnostic directing the operator to
  forward remediation or verified restore

### Requirement: Integrity reconciliation shares PostgreSQL coordination
The service SHALL serialize integrity-ledger creation, legacy reconciliation,
forward migration, and final integrity recording using PostgreSQL-owned
coordination shared by standalone commands and all runtime modes.

#### Scenario: Migration invocations overlap
- **WHEN** an operator command and one or more API, worker, or scheduler
  processes concurrently inspect or apply a clean pending database
- **THEN** at most one invocation reconciles or applies the transition at a
  time and every successful observer reports the same clean ledger prefix

#### Scenario: Process exits during migration transition
- **WHEN** a process exits after migration execution begins and before normal
  admission completes
- **THEN** later startup revalidates the driver and integrity ledgers and does
  not report current or admit work until a clean supported state is proven
