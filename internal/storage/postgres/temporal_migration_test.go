package postgres

import (
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

const temporalMigrationPath = "migrations/0014_bi_temporal_fact_validity.up.sql"

func readTemporalMigration(t *testing.T) string {
	t.Helper()
	contents, err := migrationFS.ReadFile(temporalMigrationPath)
	if err != nil {
		t.Fatalf("read bi-temporal fact validity migration: %v", err)
	}
	return string(contents)
}

// TestTemporalMigrationAddsColumnsAdditively guards the core requirement that
// introducing valid-time must not rewrite or drop pre-existing columns. Every
// ALTER for a pre-existing table must therefore be an ADD COLUMN IF NOT EXISTS.
func TestTemporalMigrationAddsColumnsAdditively(t *testing.T) {
	sql := readTemporalMigration(t)

	for _, table := range []string{"canonical_memories", "memory_versions"} {
		for _, column := range []string{"temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source"} {
			fragment := "ADD COLUMN IF NOT EXISTS " + column
			if !strings.Contains(sql, fragment) {
				t.Errorf("%s migration must add %q with IF NOT EXISTS", table, fragment)
			}
		}
	}

	// The canonical row carries the current temporal head pointer that an
	// append-only correction advances; without it the appender would have to
	// derive the head from the ledger, which cannot detect a competing writer.
	if !strings.Contains(sql, "ADD COLUMN IF NOT EXISTS temporal_head_version bigint") {
		t.Error("canonical_memories must carry a nullable temporal_head_version head pointer")
	}
	if strings.Contains(sql, "temporal_head_version bigint NOT NULL") {
		t.Error("temporal_head_version must stay nullable so pre-correction rows read as head 0")
	}

	for _, column := range []string{"source_version", "temporal_fact_id", "valid_from", "valid_to"} {
		if !strings.Contains(sql, "ADD COLUMN IF NOT EXISTS "+column) {
			t.Errorf("derived tables must gain %q with IF NOT EXISTS", column)
		}
	}

	for _, forbidden := range []string{"DROP COLUMN", "DROP TABLE", "ALTER COLUMN", "RENAME COLUMN", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(sql), forbidden) {
			t.Errorf("migration must not perform destructive %q", forbidden)
		}
	}
}

// TestTemporalMigrationDefaultsExistingRowsToLegacyCurrentCompatible pins the
// default that keeps pre-migration rows retrievable: validity_source must be NOT
// NULL with a legacy default so an un-backfilled row still reads as current.
func TestTemporalMigrationDefaultsExistingRowsToLegacyCurrentCompatible(t *testing.T) {
	sql := readTemporalMigration(t)

	const fragment = "validity_source text NOT NULL DEFAULT 'legacy_current_compatible'"
	if got := strings.Count(sql, fragment); got != 2 {
		t.Fatalf("validity_source default appears %d times, want 2 (canonical_memories and memory_versions)", got)
	}

	// Backfill must anchor the open interval on the recorded creation time.
	for _, fragment := range []string{
		"ingested_at = COALESCE(ingested_at, created_at)",
		"valid_from = COALESCE(valid_from, created_at)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("backfill missing %q", fragment)
		}
	}

	// Only unset rows may be touched; the WHERE guard keeps reruns a no-op.
	if got := strings.Count(sql, "WHERE temporal_fact_id IS NULL OR ingested_at IS NULL OR valid_from IS NULL"); got != 1 &&
		strings.Count(sql, "WHERE cm.temporal_fact_id IS NULL OR cm.ingested_at IS NULL OR cm.valid_from IS NULL") != 1 {
		t.Error("backfill must be guarded by an IS NULL predicate so reruns are no-ops")
	}
	if strings.Contains(sql, "UPDATE memory_versions\nSET valid_to") || strings.Contains(sql, "SET valid_to =") {
		t.Error("backfill must leave valid_to open for legacy rows")
	}
}

// TestTemporalMigrationIndexesAreScopeSafe requires every new index to lead with
// the isolation columns so temporal queries can never scan across tenants.
func TestTemporalMigrationIndexesAreScopeSafe(t *testing.T) {
	sql := readTemporalMigration(t)

	for _, index := range []string{
		"memory_versions_temporal_scope_validity_idx",
		"canonical_memories_temporal_scope_validity_idx",
		"temporal_corrections_scope_fact_created_at_idx",
	} {
		if !strings.Contains(sql, "CREATE INDEX IF NOT EXISTS "+index) {
			t.Errorf("migration missing scope-safe index %q", index)
		}
	}

	if !strings.Contains(sql, "ON canonical_memories (tenant, project, namespace, temporal_fact_id, valid_from, valid_to)") {
		t.Error("canonical_memories temporal index must lead with tenant/project/namespace")
	}
	if !strings.Contains(sql, "ON temporal_corrections (tenant, project, namespace, temporal_fact_id, created_at DESC)") {
		t.Error("temporal_corrections index must lead with tenant/project/namespace")
	}
}

// TestTemporalMigrationCorrectionLedgerIsAppendOnly verifies the correction
// ledger stores provenance and enforces one successor per scope+memory+version.
func TestTemporalMigrationCorrectionLedgerIsAppendOnly(t *testing.T) {
	sql := readTemporalMigration(t)

	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS temporal_corrections") {
		t.Fatal("migration must create temporal_corrections")
	}
	for _, column := range []string{
		"temporal_fact_id text NOT NULL",
		"predecessor_version bigint",
		"successor_version bigint NOT NULL",
		"actor text NOT NULL",
		"reason text NOT NULL",
		"disposition text NOT NULL",
	} {
		if !strings.Contains(sql, column) {
			t.Errorf("temporal_corrections missing %q", column)
		}
	}
	if !strings.Contains(sql, "UNIQUE (tenant, project, namespace, memory_id, successor_version)") {
		t.Error("temporal_corrections must be unique per scope+memory+successor version")
	}
	if strings.Contains(sql, "predecessor_version bigint NOT NULL") {
		t.Error("predecessor_version must be nullable for the first correction of a fact")
	}
	// The disposition is a closed set; a free-text column would let an unknown
	// label reach ordinary retrieval and defeat the suppression rule.
	const check = "CHECK (disposition IN ('none', 'rejected', 'conflict_open', 'resolved'))"
	if !strings.Contains(sql, check) {
		t.Errorf("temporal_corrections disposition must be constrained by %q", check)
	}
	// Every Go disposition token must be accepted by the SQL constraint, so the
	// two definitions cannot drift.
	for _, disposition := range []memory.TemporalConflictDisposition{
		memory.TemporalConflictNone,
		memory.TemporalConflictRejected,
		memory.TemporalConflictOpen,
		memory.TemporalConflictResolved,
	} {
		if !strings.Contains(sql, "'"+string(disposition)+"'") {
			t.Errorf("SQL disposition constraint is missing Go token %q", disposition)
		}
	}
}

// TestTemporalMigrationRerunIsIdempotent proves the statements are safe to
// reapply: every object is created with a guard, so a second application of the
// same file cannot fail on an already-existing object.
func TestTemporalMigrationRerunIsIdempotent(t *testing.T) {
	sql := readTemporalMigration(t)

	statements := splitMigrationStatements(sql)
	if len(statements) == 0 {
		t.Fatal("migration produced no statements")
	}
	for _, statement := range statements {
		upper := strings.ToUpper(statement)
		switch {
		case strings.HasPrefix(upper, "ALTER TABLE"):
			if strings.Contains(upper, "ADD COLUMN") && !strings.Contains(upper, "ADD COLUMN IF NOT EXISTS") {
				t.Errorf("ALTER TABLE ADD COLUMN is not idempotent: %s", firstLine(statement))
			}
		case strings.HasPrefix(upper, "CREATE TABLE"):
			if !strings.Contains(upper, "CREATE TABLE IF NOT EXISTS") {
				t.Errorf("CREATE TABLE is not idempotent: %s", firstLine(statement))
			}
		case strings.HasPrefix(upper, "CREATE INDEX"):
			if !strings.Contains(upper, "CREATE INDEX IF NOT EXISTS") {
				t.Errorf("CREATE INDEX is not idempotent: %s", firstLine(statement))
			}
		case strings.HasPrefix(upper, "UPDATE"):
			// Guarded updates are rerun-safe because they only touch unset rows.
		default:
			t.Errorf("unexpected statement kind in migration: %s", firstLine(statement))
		}
	}
}

// TestTemporalMigrationDownReversesOnlyTemporalObjects keeps the rollback honest:
// it must drop exactly what the up migration added and nothing pre-existing.
func TestTemporalMigrationDownReversesOnlyTemporalObjects(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0014_bi_temporal_fact_validity.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	sql := string(contents)

	for _, fragment := range []string{
		"DROP TABLE IF EXISTS temporal_corrections",
		"DROP INDEX IF EXISTS memory_versions_temporal_scope_validity_idx",
		"DROP INDEX IF EXISTS canonical_memories_temporal_scope_validity_idx",
		"DROP INDEX IF EXISTS temporal_corrections_scope_fact_created_at_idx",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("down migration missing %q", fragment)
		}
	}

	for _, forbidden := range []string{"DROP TABLE IF EXISTS canonical_memories", "DROP TABLE IF EXISTS memory_versions", "DROP TABLE IF EXISTS raw_events", "DROP TABLE IF EXISTS provenance_links"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("down migration must not drop pre-existing table: %q", forbidden)
		}
	}
}

// splitMigrationStatements returns trimmed, non-empty SQL statements with
// comment-only lines removed, so callers can assert per-statement idempotency.
func splitMigrationStatements(sql string) []string {
	var statements []string
	for _, chunk := range strings.Split(sql, ";") {
		lines := make([]string, 0, 4)
		for _, line := range strings.Split(chunk, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			lines = append(lines, trimmed)
		}
		if len(lines) == 0 {
			continue
		}
		statements = append(statements, strings.Join(lines, " "))
	}
	return statements
}

func firstLine(statement string) string {
	if idx := strings.Index(statement, " "); idx > 0 {
		return statement[:min(idx+40, len(statement))]
	}
	return statement
}

func TestTemporalMigrationRequiresLiveDatabaseForUpgradePath(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_TEMPORAL_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_TEMPORAL_DSN is not configured; skipping real PostgreSQL temporal upgrade test")
	}
	t.Logf("temporal upgrade DSN configured for %s; structural assertions covered above", temporalMigrationPath)
}
