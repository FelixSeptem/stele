package postgres

import (
	"context"
	"strings"
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
		WillReturnRows(pgxmock.NewRows([]string{"version", "dirty"}).AddRow(CurrentMigrationVersion, false))
	manifest, err := MigrationManifest()
	if err != nil {
		t.Fatalf("MigrationManifest() error = %v", err)
	}
	ledgerRows := pgxmock.NewRows([]string{"version", "migration_name", "checksum_sha256"})
	for _, asset := range manifest {
		ledgerRows.AddRow(asset.Version, asset.Name, asset.ChecksumSHA256)
	}
	mock.ExpectQuery(`SELECT version, migration_name, checksum_sha256 FROM stele_schema_migration_ledger`).WillReturnRows(ledgerRows)

	state, err := InspectMigrationState(context.Background(), mock)
	if err != nil {
		t.Fatalf("InspectMigrationState() error = %v", err)
	}
	if state.CurrentVersion != CurrentMigrationVersion || state.Dirty || state.Pending || state.IntegrityStatus != MigrationIntegrityVerified {
		t.Fatalf("state = %+v, want clean current migration version", state)
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

func TestInspectMigrationStateReportsDivergentIntegrityLedger(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery(`SELECT version, dirty FROM schema_migrations`).
		WillReturnRows(pgxmock.NewRows([]string{"version", "dirty"}).AddRow(int64(1), false))
	manifest, err := MigrationManifest()
	if err != nil {
		t.Fatalf("MigrationManifest() error = %v", err)
	}
	mock.ExpectQuery(`SELECT version, migration_name, checksum_sha256 FROM stele_schema_migration_ledger`).
		WillReturnRows(pgxmock.NewRows([]string{"version", "migration_name", "checksum_sha256"}).AddRow(manifest[0].Version, manifest[0].Name, "0000000000000000000000000000000000000000000000000000000000000000"))

	state, err := InspectMigrationState(context.Background(), mock)
	if err != nil {
		t.Fatalf("InspectMigrationState() error = %v", err)
	}
	if state.Status != MigrationStatusDivergent {
		t.Fatalf("state = %+v, want divergent integrity state", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestClassifyMigrationStateRejectsIncompleteOrExtraIntegrityRows(t *testing.T) {
	manifest, err := MigrationManifest()
	if err != nil {
		t.Fatalf("MigrationManifest() error = %v", err)
	}
	matching := migrationIntegrityRecord{Version: manifest[0].Version, Name: manifest[0].Name, ChecksumSHA256: manifest[0].ChecksumSHA256}
	for _, test := range []struct {
		name    string
		version int64
		records []migrationIntegrityRecord
	}{
		{name: "missing row", version: CurrentMigrationVersion, records: nil},
		{name: "extra row", version: CurrentMigrationVersion, records: append([]migrationIntegrityRecord{matching}, migrationIntegrityRecord{Version: CurrentMigrationVersion + 1, Name: "future.up.sql", ChecksumSHA256: matching.ChecksumSHA256})},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, err := classifyMigrationState(test.version, false, test.records, false)
			if err != nil {
				t.Fatalf("classifyMigrationState() error = %v", err)
			}
			if state.Status != MigrationStatusDivergent {
				t.Fatalf("state = %+v, want divergent", state)
			}
		})
	}
}

func TestMigrateDatabaseRejectsDivergentStateForValidatePolicy(t *testing.T) {
	state := MigrationState{Status: MigrationStatusDivergent, CurrentVersion: CurrentMigrationVersion, LatestVersion: CurrentMigrationVersion, IntegrityStatus: MigrationIntegrityUnknown}
	if err := migrationStateError(state); err == nil || !strings.Contains(err.Error(), "status=divergent") || !strings.Contains(err.Error(), "integrity_status=unknown") {
		t.Fatalf("migrationStateError() = %v, want bounded divergent integrity diagnostic", err)
	}
}
