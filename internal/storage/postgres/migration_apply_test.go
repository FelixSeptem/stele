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
	if len(migrations) != 36 {
		t.Fatalf("migration assets = %v, want eighteen up/down migration pairs", migrations)
	}
	if migrations[0] != "0001_base_schema.down.sql" || migrations[1] != "0001_base_schema.up.sql" || migrations[2] != "0002_context_projections.down.sql" || migrations[3] != "0002_context_projections.up.sql" || migrations[4] != "0003_governed_memory_intents_reflection_compaction.down.sql" || migrations[5] != "0003_governed_memory_intents_reflection_compaction.up.sql" || migrations[6] != "0004_hierarchical_memory_chunks.down.sql" || migrations[7] != "0004_hierarchical_memory_chunks.up.sql" || migrations[8] != "0005_stable_hybrid_candidate_fusion.down.sql" || migrations[9] != "0005_stable_hybrid_candidate_fusion.up.sql" || migrations[10] != "0006_diversity_policy.down.sql" || migrations[11] != "0006_diversity_policy.up.sql" || migrations[12] != "0007_query_analysis_rollout.down.sql" || migrations[13] != "0007_query_analysis_rollout.up.sql" || migrations[14] != "0008_quality_aware_reranking.down.sql" || migrations[15] != "0008_quality_aware_reranking.up.sql" || migrations[16] != "0009_durable_maintenance.down.sql" || migrations[17] != "0009_durable_maintenance.up.sql" || migrations[18] != "0010_projection_freshness.down.sql" || migrations[19] != "0010_projection_freshness.up.sql" || migrations[20] != "0011_provider_runtime_bindings.down.sql" || migrations[21] != "0011_provider_runtime_bindings.up.sql" || migrations[22] != "0012_provider_lifecycle_idempotency.down.sql" || migrations[23] != "0012_provider_lifecycle_idempotency.up.sql" || migrations[24] != "0013_query_adaptive_retrieval_planning.down.sql" || migrations[25] != "0013_query_adaptive_retrieval_planning.up.sql" || migrations[26] != "0014_bi_temporal_fact_validity.down.sql" || migrations[27] != "0014_bi_temporal_fact_validity.up.sql" || migrations[28] != "0015_bounded_graph_distance_retrieval.down.sql" || migrations[29] != "0015_bounded_graph_distance_retrieval.up.sql" || migrations[30] != "0016_context_calibration_summaries.down.sql" || migrations[31] != "0016_context_calibration_summaries.up.sql" || migrations[32] != "0017_context_calibration_rollout.down.sql" || migrations[33] != "0017_context_calibration_rollout.up.sql" || migrations[34] != "0018_mcp_forget_preview_ledger.down.sql" || migrations[35] != "0018_mcp_forget_preview_ledger.up.sql" {
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
