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
	if len(migrations) != 58 {
		t.Fatalf("migration assets = %v, want twenty-nine up/down migration pairs", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" || migrations[36] != "0019_scoped_memory_paths.down.sql" || migrations[37] != "0019_scoped_memory_paths.up.sql" || migrations[38] != "0020_reserved_insight_activation.down.sql" || migrations[39] != "0020_reserved_insight_activation.up.sql" || migrations[40] != "0021_retrieval_integrity_evidence.down.sql" || migrations[41] != "0021_retrieval_integrity_evidence.up.sql" || migrations[42] != "0022_ranking_rollout_evidence_attestations.down.sql" || migrations[43] != "0022_ranking_rollout_evidence_attestations.up.sql" || migrations[44] != "0023_scheduler_run_history.down.sql" || migrations[45] != "0023_scheduler_run_history.up.sql" || migrations[46] != "0024_derived_work_queue.down.sql" || migrations[47] != "0024_derived_work_queue.up.sql" || migrations[48] != "0025_progressive_experiment_evidence.down.sql" || migrations[49] != "0025_progressive_experiment_evidence.up.sql" || migrations[50] != "0026_governed_reasoning_insight_candidates.down.sql" || migrations[51] != "0026_governed_reasoning_insight_candidates.up.sql" || migrations[52] != "0027_contradiction_review_metadata.down.sql" || migrations[53] != "0027_contradiction_review_metadata.up.sql" || migrations[54] != "0028_governed_memory_intent_transitions.down.sql" || migrations[55] != "0028_governed_memory_intent_transitions.up.sql" || migrations[56] != "0029_memory_intent_work_kind.down.sql" || migrations[57] != "0029_memory_intent_work_kind.up.sql" {
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
