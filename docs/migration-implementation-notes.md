# Migration implementation notes

This note records the implementation decision required by the
`product-ready-self-hosting-foundation` change before migration production code
is introduced.

## Current bootstrap inventory

- `internal/storage/postgres/migrations.go` embeds `migrations/*.sql` and
  exposes `BaseSchemaSQL`, which currently reads the aggregate
  `0001_base_schema.up.sql` file.
- `internal/storage/postgres/bootstrap.go` exposes `BootstrapDatabase`; it
  opens one transaction, splits the aggregate SQL on semicolons, executes each
  statement, and commits the transaction.
- `internal/app/app.go` invokes `BootstrapDatabase` from the API, worker, and
  scheduler PostgreSQL startup paths.
- `internal/benchmark/longmemeval_postgres.go` also invokes the bootstrap path
  for its local benchmark database.
- `internal/storage/postgres/migrations/0001_base_schema.up.sql` is the
  supported schema snapshot. It creates the `vector` extension and the current
  raw-event, canonical-memory, provenance, workflow, assurance, and related
  tables and indexes.
- `internal/storage/postgres/bootstrap_test.go` currently verifies aggregate
  SQL execution with pgxmock, but does not verify ordered versions, pending
  migrations, dirty state, divergence, or concurrent startup.

## Library evaluation

### Selected library: `golang-migrate/migrate/v4` `v4.19.1`

The module already carries `github.com/golang-migrate/migrate/v4` at
`v4.19.1`. It will be promoted from an indirect to a direct dependency when
the migration runner is implemented.

Reasons for selection:

- mature PostgreSQL support and broad production adoption;
- explicit versioned migration model with ordered `up`/`down` assets;
- PostgreSQL source driver with database-resident migration metadata and dirty
  version reporting;
- PostgreSQL advisory-lock serialization for concurrent migrators;
- `io/fs` source support, allowing immutable migrations to remain embedded in
  the service binary without adding an external migration file dependency;
- clear forward migration, force/recovery, and no-change semantics that can be
  mapped to Stele's bounded status categories;
- permissive open-source licensing compatible with this repository.

### Alternatives considered

- `pressly/goose`: mature and supports embedded migrations, but would require a
  new migration abstraction and does not reuse the dependency already present
  in the module.
- `go-jet/jet`/`tern`: useful PostgreSQL tooling, but less aligned with the
  existing Go migration dependency and would add a different operational model.
- A repository-local runner over `CREATE TABLE IF NOT EXISTS`: rejected because
  it cannot represent ordered upgrades, dirty state, divergence, or safe
  concurrent startup.

## Constraints for the implementation

The runner must use immutable numbered SQL assets, a database-resident ledger,
PostgreSQL-owned serialization, and forward-only execution. Runtime modes and
the standalone CLI must share the same runner, lock, and status reader. A dirty,
divergent, incompatible, or partially applied state must fail closed; automatic
down migration is not permitted. The current aggregate schema is the source
for the immutable initial migration and must not remain a mutable startup
replay mechanism.
