package postgres

import "testing"

func TestMigrationAssetsExposeImmutableInitialMigration(t *testing.T) {
	migrations, err := MigrationAssets()
	if err != nil {
		t.Fatalf("MigrationAssets() error = %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("migration assets = %v, want initial up and down assets", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" {
		t.Fatalf("migration assets = %v, want stable initial migration names", migrations)
	}
}
