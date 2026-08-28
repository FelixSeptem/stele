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

const CurrentMigrationVersion int64 = 1

type MigrationStatus string

const (
	MigrationStatusCurrent       MigrationStatus = "current"
	MigrationStatusPending       MigrationStatus = "pending"
	MigrationStatusDirty         MigrationStatus = "dirty"
	MigrationStatusUninitialized MigrationStatus = "uninitialized"
	MigrationStatusIncompatible  MigrationStatus = "incompatible"
)

type MigrationState struct {
	CurrentVersion int64
	LatestVersion  int64
	Dirty          bool
	Pending        bool
	Status         MigrationStatus
}

type migrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type MigrationRunner struct {
	OpenDB func(dsn string) (*sql.DB, error)
}

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
			return fmt.Errorf("migration validation failed: status=%s current_version=%d latest_version=%d dirty=%t", state.Status, state.CurrentVersion, state.LatestVersion, state.Dirty)
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
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
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
	state := MigrationState{LatestVersion: CurrentMigrationVersion}
	var version uint
	var dirty bool
	err = db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "schema_migrations") {
			state.Status = MigrationStatusUninitialized
			state.Pending = true
			return state, nil
		}
		return MigrationState{}, fmt.Errorf("read migration ledger: %w", err)
	}
	state.CurrentVersion = int64(version)
	state.Dirty = dirty
	switch {
	case dirty:
		state.Status = MigrationStatusDirty
	case state.CurrentVersion > CurrentMigrationVersion:
		state.Status = MigrationStatusIncompatible
	case state.CurrentVersion < CurrentMigrationVersion:
		state.Pending = true
		state.Status = MigrationStatusPending
	default:
		state.Status = MigrationStatusCurrent
	}
	return state, nil
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

	state := MigrationState{LatestVersion: CurrentMigrationVersion}
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

	state.CurrentVersion = version
	state.Dirty = dirty
	switch {
	case dirty:
		state.Status = MigrationStatusDirty
	case version > CurrentMigrationVersion:
		state.Status = MigrationStatusIncompatible
	case version < CurrentMigrationVersion:
		state.Pending = true
		state.Status = MigrationStatusPending
	default:
		state.Status = MigrationStatusCurrent
	}
	return state, nil
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01" && strings.Contains(strings.ToLower(pgErr.Message), "schema_migrations")
}
