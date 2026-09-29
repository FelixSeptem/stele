package postgres

import (
	"strings"
	"testing"
)

func TestScopedMemoryPathMigrationIsRestartableAndBackfillsRoot(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0019_scoped_memory_paths.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS memory_path",
		"SET memory_path = '/'",
		"WHERE memory_path IS NULL",
		"CREATE INDEX IF NOT EXISTS canonical_memories_scope_path_updated_at_idx",
		"memory_path text_pattern_ops",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
}

func TestScopedMemoryPathDownMigrationPreservesPopulatedColumnsForOperationalRollback(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0019_scoped_memory_paths.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	if strings.Contains(sql, "DROP COLUMN") || strings.Contains(sql, "DROP INDEX") {
		t.Fatalf("down migration destroys path data or indexes needed by rollback: %s", sql)
	}
}
