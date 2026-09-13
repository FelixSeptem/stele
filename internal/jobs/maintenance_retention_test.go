package jobs

import "testing"

func TestMaintenanceRetentionAllowlistExcludesCanonicalAndIncidentRecords(t *testing.T) {
	for _, category := range []string{"job_execution", "projection_evidence", "conformance_evidence", "redacted_trajectory"} {
		if !IsDerivedMaintenanceRetentionCategory(category) {
			t.Fatalf("category %q should be allowed", category)
		}
	}
	for _, category := range []string{"canonical_memory", "raw_event", "memory_version", "incident_audit"} {
		if IsDerivedMaintenanceRetentionCategory(category) {
			t.Fatalf("category %q must not be allowed", category)
		}
	}
}
