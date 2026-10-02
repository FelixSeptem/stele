package postgres

import (
	"strings"
	"testing"
)

func TestMigrationManifestIncludesDerivedWorkQueue(t *testing.T) {
	manifest, err := MigrationManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 24 || manifest[23].Version != 24 || manifest[23].Name != "0024_derived_work_queue.up.sql" {
		t.Fatalf("manifest tail = %+v, want version 24 derived work queue", manifest)
	}
	contents, err := migrationFS.ReadFile("migrations/0024_derived_work_queue.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{"derived_work_items", "derived_work_checkpoints", "derived_work_attempts", "tenant", "project", "namespace", "work_key"} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}
