package postgres

import (
	"strings"
	"testing"
)

func TestMigrationAssetsExposeImmutableInitialMigration(t *testing.T) {
	migrations, err := MigrationAssets()
	if err != nil {
		t.Fatalf("MigrationAssets() error = %v", err)
	}
	if len(migrations) != 68 {
		t.Fatalf("migration assets = %v, want thirty-four up/down migration pairs", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" || migrations[36] != "0019_scoped_memory_paths.down.sql" || migrations[37] != "0019_scoped_memory_paths.up.sql" || migrations[38] != "0020_reserved_insight_activation.down.sql" || migrations[39] != "0020_reserved_insight_activation.up.sql" || migrations[60] != "0031_governed_goal_metadata.down.sql" || migrations[61] != "0031_governed_goal_metadata.up.sql" || migrations[62] != "0032_goal_visibility.down.sql" || migrations[63] != "0032_goal_visibility.up.sql" || migrations[64] != "0033_release_evidence_reconciliation.down.sql" || migrations[65] != "0033_release_evidence_reconciliation.up.sql" || migrations[66] != "0034_runtime_capability_event_sync.down.sql" || migrations[67] != "0034_runtime_capability_event_sync.up.sql" {
		t.Fatalf("migration assets = %v, want stable migration names", migrations)
	}
}

func TestInitialMigrationKeepsUsefulnessFeedbackTaskEvaluationReferenceOpaque(t *testing.T) {
	sql, err := BaseSchemaSQL()
	if err != nil {
		t.Fatalf("BaseSchemaSQL() error = %v", err)
	}
	feedbackStart := strings.Index(sql, "CREATE TABLE IF NOT EXISTS usefulness_feedback")
	if feedbackStart < 0 {
		t.Fatal("initial migration must define usefulness_feedback")
	}
	feedbackBlock := sql[feedbackStart:]
	if !strings.Contains(feedbackBlock, "task_evaluation_id text,") {
		t.Fatal("usefulness_feedback task evaluation reference must remain an opaque text identifier")
	}
	if strings.Contains(feedbackBlock, "usefulness_feedback_task_evaluation_id_fkey") {
		t.Fatal("usefulness_feedback task evaluation reference must not be constrained to task_evaluations")
	}
}
