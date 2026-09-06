package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// TestMigrationRunnerSerializesConcurrentApply exercises the PostgreSQL-owned
// advisory lock against a disposable database supplied by the integration
// harness. It is intentionally skipped when no disposable DSN is configured;
// unit tests must never mutate a developer's or operator's database.
func TestMigrationRunnerSerializesConcurrentApply(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_DSN is not configured; skipping real PostgreSQL migration concurrency test")
	}

	runner := NewMigrationRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- runner.Apply(ctx, dsn)
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent migration apply failed: %v", err)
		}
	}

	state, err := runner.Status(ctx, dsn)
	if err != nil {
		t.Fatalf("migration status after concurrent apply: %v", err)
	}
	if state.Status != MigrationStatusCurrent || state.Dirty || state.Pending {
		t.Fatalf("state after concurrent apply = %+v, want current clean schema", state)
	}
	if state.IntegrityStatus != MigrationIntegrityVerified || state.IntegrityRows != int(CurrentMigrationVersion) {
		t.Fatalf("integrity state after concurrent apply = %+v, want verified complete ledger", state)
	}
	db, err := runner.openDB(dsn)
	if err != nil {
		t.Fatalf("open migration database after concurrent apply: %v", err)
	}
	defer db.Close()
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = $1 AND dirty = false`, CurrentMigrationVersion).Scan(&applied); err != nil {
		t.Fatalf("count applied migration ledger rows: %v", err)
	}
	if applied != 1 {
		t.Fatalf("applied migration ledger rows = %d, want exactly one", applied)
	}
}
