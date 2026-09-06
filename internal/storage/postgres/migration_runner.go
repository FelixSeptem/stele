package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

type MigrationStatus string

const (
	MigrationStatusCurrent       MigrationStatus = "current"
	MigrationStatusPending       MigrationStatus = "pending"
	MigrationStatusDirty         MigrationStatus = "dirty"
	MigrationStatusDivergent     MigrationStatus = "divergent"
	MigrationStatusUninitialized MigrationStatus = "uninitialized"
	MigrationStatusIncompatible  MigrationStatus = "incompatible"
)

type MigrationIntegrityStatus string

const (
	MigrationIntegrityVerified MigrationIntegrityStatus = "verified"
	MigrationIntegrityLegacy   MigrationIntegrityStatus = "legacy"
	MigrationIntegrityUnknown  MigrationIntegrityStatus = "unknown"
)

type MigrationState struct {
	CurrentVersion  int64                    `json:"current_version"`
	LatestVersion   int64                    `json:"latest_version"`
	Dirty           bool                     `json:"dirty"`
	Pending         bool                     `json:"pending"`
	Status          MigrationStatus          `json:"status"`
	IntegrityStatus MigrationIntegrityStatus `json:"integrity_status"`
	IntegrityRows   int                      `json:"integrity_rows"`
}

type migrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type migrationIntegrityRecord struct {
	Version        int64
	Name           string
	ChecksumSHA256 string
}

type MigrationRunner struct {
	OpenDB func(dsn string) (*sql.DB, error)
}

const migrationIntegrityLockID int64 = 0x5354454C45 // "STELE"

func MigrateDatabase(ctx context.Context, dsn string, policy string) error {
	runner := NewMigrationRunner()
	switch strings.TrimSpace(policy) {
	case "", "auto":
		return runner.Apply(ctx, dsn)
	case "validate", "off":
		state, err := runner.Status(ctx, dsn)
		if err != nil {
			return err
		}
		if state.Status != MigrationStatusCurrent {
			return migrationStateError(state)
		}
		return nil
	default:
		return fmt.Errorf("unsupported migration policy %q", policy)
	}
}

func NewMigrationRunner() MigrationRunner {
	return MigrationRunner{OpenDB: func(dsn string) (*sql.DB, error) {
		config, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		return stdlib.OpenDB(*config), nil
	}}
}

func (r MigrationRunner) Apply(ctx context.Context, dsn string) error {
	db, err := r.openDB(dsn)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()
	// A migration runner is short-lived and serial by design. One connection
	// keeps the session advisory lock and every reconciliation statement on the
	// same PostgreSQL session.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationIntegrityLockID); err != nil {
		return fmt.Errorf("acquire migration integrity lock: %w", err)
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationIntegrityLockID)
	}()

	state, err := r.statusWithDB(ctx, db)
	if err != nil {
		return err
	}
	switch state.Status {
	case MigrationStatusDirty, MigrationStatusDivergent, MigrationStatusIncompatible:
		return migrationStateError(state)
	}

	source, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{
		MigrationsTable:       "schema_migrations",
		MultiStatementEnabled: true,
	})
	if err != nil {
		return fmt.Errorf("configure postgres migration driver: %w", err)
	}
	migrator, err := migrate.NewWithInstance("stele-embedded", source, "stele", driver)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if err := reconcileMigrationIntegrity(ctx, db); err != nil {
		return err
	}
	state, err = r.statusWithDB(ctx, db)
	if err != nil {
		return err
	}
	if state.Status != MigrationStatusCurrent || state.IntegrityStatus != MigrationIntegrityVerified {
		return migrationStateError(state)
	}
	return nil
}

func (r MigrationRunner) Status(ctx context.Context, dsn string) (MigrationState, error) {
	db, err := r.openDB(dsn)
	if err != nil {
		return MigrationState{}, fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return MigrationState{}, fmt.Errorf("ping migration database: %w", err)
	}
	return r.statusWithDB(ctx, db)
}

func (r MigrationRunner) statusWithDB(ctx context.Context, db *sql.DB) (MigrationState, error) {
	state := MigrationState{LatestVersion: CurrentMigrationVersion, IntegrityStatus: MigrationIntegrityUnknown}
	var version uint
	var dirty bool
	err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "schema_migrations") {
			state.Status = MigrationStatusUninitialized
			state.Pending = true
			return state, nil
		}
		return MigrationState{}, fmt.Errorf("read migration ledger: %w", err)
	}
	return inspectMigrationIntegritySQL(ctx, db, int64(version), dirty)
}

func reconcileMigrationIntegrity(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS stele_schema_migration_ledger (
		version bigint PRIMARY KEY,
		migration_name text NOT NULL,
		checksum_sha256 char(64) NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migration integrity ledger: %w", err)
	}
	state, err := inspectMigrationDriverSQL(ctx, db)
	if err != nil {
		return err
	}
	if state.Status == MigrationStatusUninitialized || state.Status == MigrationStatusDirty || state.Status == MigrationStatusIncompatible {
		return migrationStateError(state)
	}
	manifest, err := MigrationManifest()
	if err != nil {
		return err
	}
	if state.CurrentVersion < 1 || state.CurrentVersion > int64(len(manifest)) {
		return migrationStateError(MigrationState{CurrentVersion: state.CurrentVersion, LatestVersion: CurrentMigrationVersion, Status: MigrationStatusDivergent})
	}
	for _, asset := range manifest[:state.CurrentVersion] {
		if _, err := db.ExecContext(ctx, `INSERT INTO stele_schema_migration_ledger (version, migration_name, checksum_sha256) VALUES ($1, $2, $3) ON CONFLICT (version) DO NOTHING`, asset.Version, asset.Name, asset.ChecksumSHA256); err != nil {
			return fmt.Errorf("record migration integrity version %d: %w", asset.Version, err)
		}
	}
	verified, err := rStatusWithIntegrity(ctx, db)
	if err != nil {
		return err
	}
	if verified.Status != MigrationStatusCurrent && verified.Status != MigrationStatusPending {
		return migrationStateError(verified)
	}
	if verified.IntegrityStatus != MigrationIntegrityVerified {
		return migrationStateError(MigrationState{CurrentVersion: verified.CurrentVersion, LatestVersion: verified.LatestVersion, Status: MigrationStatusDivergent, IntegrityStatus: verified.IntegrityStatus})
	}
	return nil
}

func inspectMigrationDriverSQL(ctx context.Context, db *sql.DB) (MigrationState, error) {
	state := MigrationState{LatestVersion: CurrentMigrationVersion, IntegrityStatus: MigrationIntegrityUnknown}
	var version uint
	var dirty bool
	err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "schema_migrations") {
			state.Status = MigrationStatusUninitialized
			state.Pending = true
			return state, nil
		}
		return MigrationState{}, fmt.Errorf("read migration ledger: %w", err)
	}
	if terminal, terminalState := terminalMigrationState(int64(version), dirty); terminalState {
		return terminal, nil
	}
	state.CurrentVersion = int64(version)
	if state.CurrentVersion < CurrentMigrationVersion {
		state.Status = MigrationStatusPending
		state.Pending = true
		return state, nil
	}
	state.Status = MigrationStatusCurrent
	return state, nil
}

func rStatusWithIntegrity(ctx context.Context, db *sql.DB) (MigrationState, error) {
	state, err := inspectMigrationDriverSQL(ctx, db)
	if err != nil || state.Status == MigrationStatusUninitialized || state.Status == MigrationStatusDirty || state.Status == MigrationStatusIncompatible {
		return state, err
	}
	return inspectMigrationIntegritySQL(ctx, db, state.CurrentVersion, false)
}

func migrationStateError(state MigrationState) error {
	return fmt.Errorf("migration validation failed: status=%s integrity_status=%s current_version=%d latest_version=%d dirty=%t", state.Status, state.IntegrityStatus, state.CurrentVersion, state.LatestVersion, state.Dirty)
}

func (r MigrationRunner) openDB(dsn string) (*sql.DB, error) {
	if r.OpenDB != nil {
		return r.OpenDB(dsn)
	}
	return NewMigrationRunner().OpenDB(dsn)
}

func MigrationFS() fs.FS { return migrationFS }

// InspectMigrationState reads the version ledger without mutating the database.
// A missing ledger is reported as uninitialized so callers can choose whether
// to bootstrap or require an explicit migration command.
func InspectMigrationState(ctx context.Context, db migrationDB) (MigrationState, error) {
	if db == nil {
		return MigrationState{}, fmt.Errorf("migration database is required")
	}

	state := MigrationState{LatestVersion: CurrentMigrationVersion, IntegrityStatus: MigrationIntegrityUnknown}
	var version int64
	var dirty bool
	err := db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			state.Status = MigrationStatusUninitialized
			state.Pending = true
			return state, nil
		}
		if isUndefinedTable(err) {
			state.Status = MigrationStatusUninitialized
			state.Pending = true
			return state, nil
		}
		return MigrationState{}, fmt.Errorf("read migration ledger: %w", err)
	}

	return inspectMigrationIntegrityPGX(ctx, db, version, dirty)
}

func inspectMigrationIntegritySQL(ctx context.Context, db *sql.DB, version int64, dirty bool) (MigrationState, error) {
	if state, terminal := terminalMigrationState(version, dirty); terminal {
		return state, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT version, migration_name, checksum_sha256 FROM stele_schema_migration_ledger ORDER BY version`)
	if err != nil {
		if isUndefinedIntegrityLedger(err) {
			return classifyMigrationState(version, dirty, nil, true)
		}
		return MigrationState{}, fmt.Errorf("read migration integrity ledger: %w", err)
	}
	defer rows.Close()
	records := make([]migrationIntegrityRecord, 0)
	for rows.Next() {
		var record migrationIntegrityRecord
		if err := rows.Scan(&record.Version, &record.Name, &record.ChecksumSHA256); err != nil {
			return MigrationState{}, fmt.Errorf("scan migration integrity ledger: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return MigrationState{}, fmt.Errorf("iterate migration integrity ledger: %w", err)
	}
	return classifyMigrationState(version, dirty, records, false)
}

func inspectMigrationIntegrityPGX(ctx context.Context, db migrationDB, version int64, dirty bool) (MigrationState, error) {
	if state, terminal := terminalMigrationState(version, dirty); terminal {
		return state, nil
	}
	rows, err := db.Query(ctx, `SELECT version, migration_name, checksum_sha256 FROM stele_schema_migration_ledger ORDER BY version`)
	if err != nil {
		if isUndefinedIntegrityLedger(err) {
			return classifyMigrationState(version, dirty, nil, true)
		}
		return MigrationState{}, fmt.Errorf("read migration integrity ledger: %w", err)
	}
	defer rows.Close()
	records := make([]migrationIntegrityRecord, 0)
	for rows.Next() {
		var record migrationIntegrityRecord
		if err := rows.Scan(&record.Version, &record.Name, &record.ChecksumSHA256); err != nil {
			return MigrationState{}, fmt.Errorf("scan migration integrity ledger: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return MigrationState{}, fmt.Errorf("iterate migration integrity ledger: %w", err)
	}
	return classifyMigrationState(version, dirty, records, false)
}

func classifyMigrationState(version int64, dirty bool, records []migrationIntegrityRecord, ledgerMissing bool) (MigrationState, error) {
	state := MigrationState{CurrentVersion: version, LatestVersion: CurrentMigrationVersion, Dirty: dirty, IntegrityStatus: MigrationIntegrityUnknown, IntegrityRows: len(records)}
	switch {
	case dirty:
		state.Status = MigrationStatusDirty
		return state, nil
	case version > CurrentMigrationVersion:
		state.Status = MigrationStatusIncompatible
		return state, nil
	}

	manifest, err := MigrationManifest()
	if err != nil {
		return MigrationState{}, err
	}
	if version < 1 || version > int64(len(manifest)) {
		state.Status = MigrationStatusDivergent
		return state, nil
	}
	if ledgerMissing {
		state.Status = MigrationStatusPending
		state.Pending = true
		state.IntegrityStatus = MigrationIntegrityLegacy
		return state, nil
	}
	if len(records) != int(version) {
		state.Status = MigrationStatusDivergent
		return state, nil
	}
	for index, record := range records {
		expected := manifest[index]
		if record.Version != expected.Version || record.Name != expected.Name || record.ChecksumSHA256 != expected.ChecksumSHA256 {
			state.Status = MigrationStatusDivergent
			return state, nil
		}
	}
	state.IntegrityStatus = MigrationIntegrityVerified
	if version < CurrentMigrationVersion {
		state.Pending = true
		state.Status = MigrationStatusPending
		return state, nil
	}
	state.Status = MigrationStatusCurrent
	return state, nil
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01" && strings.Contains(strings.ToLower(pgErr.Message), "schema_migrations")
}

func terminalMigrationState(version int64, dirty bool) (MigrationState, bool) {
	state := MigrationState{CurrentVersion: version, LatestVersion: CurrentMigrationVersion, Dirty: dirty, IntegrityStatus: MigrationIntegrityUnknown}
	if dirty {
		state.Status = MigrationStatusDirty
		return state, true
	}
	if version > CurrentMigrationVersion {
		state.Status = MigrationStatusIncompatible
		return state, true
	}
	return MigrationState{}, false
}

func isUndefinedIntegrityLedger(err error) bool {
	if isUndefinedTable(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "stele_schema_migration_ledger") && strings.Contains(strings.ToLower(err.Error()), "does not exist")
}
