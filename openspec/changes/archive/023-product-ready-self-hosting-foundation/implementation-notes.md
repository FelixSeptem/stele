## Migration Library Selection

Task 1.1 selected `github.com/golang-migrate/migrate/v4` at the repository's
already resolved version `v4.19.1`.

- License: MIT.
- Compatibility: the module declares Go `1.24.0`; Stele builds with Go `1.25`.
- PostgreSQL support: the `database/pgx/v5` driver supports a dedicated
  migrations table, database-resident version/dirty state, PostgreSQL advisory
  locking, statement timeouts, and migration-size limits.
- Asset packaging: its `source/iofs` driver can consume migrations embedded in
  the service image, so the exact migration set is released with the binary.
- Connection model: the migration backend uses `database/sql`; Stele will use
  the pgx/v5 stdlib adapter only for migration execution while normal runtime
  repositories retain their existing `pgxpool` usage.

Alternatives rejected:

- Replaying `CREATE ... IF NOT EXISTS` base schema cannot identify database
  version, detect incomplete upgrades, or express data backfills safely.
- Hand-rolled migration tracking would duplicate established dirty-state,
  advisory-lock, source, and ordered-version behavior without reducing the
  service's operational risk.

