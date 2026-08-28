package postgres

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestInspectMigrationStateReportsCurrentVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT version, dirty FROM schema_migrations`).
		WillReturnRows(pgxmock.NewRows([]string{"version", "dirty"}).AddRow(int64(1), false))

	state, err := InspectMigrationState(context.Background(), mock)
	if err != nil {
		t.Fatalf("InspectMigrationState() error = %v", err)
	}
	if state.CurrentVersion != 1 || state.Dirty || state.Pending {
		t.Fatalf("state = %+v, want clean version 1 with no pending migration", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInspectMigrationStateReportsDirtyDatabase(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT version, dirty FROM schema_migrations`).
		WillReturnRows(pgxmock.NewRows([]string{"version", "dirty"}).AddRow(int64(1), true))

	state, err := InspectMigrationState(context.Background(), mock)
	if err != nil {
		t.Fatalf("InspectMigrationState() error = %v", err)
	}
	if !state.Dirty || state.Status != MigrationStatusDirty {
		t.Fatalf("state = %+v, want dirty status", state)
	}
}

func TestInspectMigrationStateReportsIncompatibleFutureVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	mock.ExpectQuery(`SELECT version, dirty FROM schema_migrations`).
		WillReturnRows(pgxmock.NewRows([]string{"version", "dirty"}).AddRow(int64(CurrentMigrationVersion+1), false))

	state, err := InspectMigrationState(context.Background(), mock)
	if err != nil {
		t.Fatalf("InspectMigrationState() error = %v", err)
	}
	if state.Status != MigrationStatusIncompatible || state.Dirty || state.Pending {
		t.Fatalf("state = %+v, want incompatible future version", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
