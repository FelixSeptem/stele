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
	if len(migrations) != 16 {
		t.Fatalf("migration assets = %v, want eight up/down migration pairs", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" || migrations[2] != "0002_context_projections.down.sql" || migrations[3] != "0002_context_projections.up.sql" || migrations[4] != "0003_governed_memory_intents_reflection_compaction.down.sql" || migrations[5] != "0003_governed_memory_intents_reflection_compaction.up.sql" || migrations[6] != "0004_hierarchical_memory_chunks.down.sql" || migrations[7] != "0004_hierarchical_memory_chunks.up.sql" || migrations[8] != "0005_stable_hybrid_candidate_fusion.down.sql" || migrations[9] != "0005_stable_hybrid_candidate_fusion.up.sql" || migrations[10] != "0006_diversity_policy.down.sql" || migrations[11] != "0006_diversity_policy.up.sql" || migrations[12] != "0007_query_analysis_rollout.down.sql" || migrations[13] != "0007_query_analysis_rollout.up.sql" || migrations[14] != "0008_quality_aware_reranking.down.sql" || migrations[15] != "0008_quality_aware_reranking.up.sql" {
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
