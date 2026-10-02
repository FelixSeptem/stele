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
	if len(manifest) != 27 || manifest[23].Version != 24 || manifest[23].Name != "0024_derived_work_queue.up.sql" || manifest[24].Version != 25 || manifest[24].Name != "0025_progressive_experiment_evidence.up.sql" || manifest[25].Version != 26 || manifest[25].Name != "0026_governed_reasoning_insight_candidates.up.sql" || manifest[26].Version != 27 || manifest[26].Name != "0027_contradiction_review_metadata.up.sql" {
		t.Fatalf("manifest tail = %+v, want versions 24-26 derived evidence", manifest)
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

func TestMigrationManifestIncludesContradictionReviewMetadata(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0027_contradiction_review_metadata.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"contradiction_review_state", "contradiction_temporal_disposition", "review_reason"} {
		if !strings.Contains(string(contents), required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestMigrationManifestIncludesGovernedReasoningCandidates(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0026_governed_reasoning_insight_candidates.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{"governed_reasoning_insight_candidates", "evidence_digest", "source_watermark", "replay_id", "prevent_governed_audit_mutation"} {
		if !strings.Contains(sql, required) {
			t.Errorf("reasoning migration missing %q", required)
		}
	}
}
