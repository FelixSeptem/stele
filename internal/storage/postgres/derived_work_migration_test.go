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
	if len(manifest) != 25 || manifest[23].Version != 24 || manifest[23].Name != "0024_derived_work_queue.up.sql" || manifest[24].Version != 25 || manifest[24].Name != "0025_progressive_experiment_evidence.up.sql" {
		t.Fatalf("manifest tail = %+v, want versions 24-25 derived evidence", manifest)
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
