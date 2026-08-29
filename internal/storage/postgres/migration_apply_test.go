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
	if len(migrations) != 2 {
		t.Fatalf("migration assets = %v, want initial up and down assets", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" {
		t.Fatalf("migration assets = %v, want stable initial migration names", migrations)
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
