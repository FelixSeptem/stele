package postgres

import (
	"strings"
	"testing"
)

func TestGovernedMemoryMigrationDefinesScopeAndReplayGuards(t *testing.T) {
	contents, err := migrationFS.ReadFile("migrations/0003_governed_memory_intents_reflection_compaction.up.sql")
	if err != nil {
		t.Fatalf("read governed memory migration: %v", err)
	}
	sql := string(contents)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS memory_intents",
		"CREATE TABLE IF NOT EXISTS reflection_runs",
		"CREATE TABLE IF NOT EXISTS reflection_run_checkpoints",
		"CREATE TABLE IF NOT EXISTS reflection_review_decisions",
		"CREATE TABLE IF NOT EXISTS compaction_evidence",
		"memory_intents_scope_idempotency_idx",
		"memory_intents_scope_operation_idx",
		"request_fingerprint text NOT NULL",
		"reflection_runs_scope_replay_key_idx",
		"reflection_runs_scope_input_dedup_idx",
		"compaction_evidence_scope_source_dedup_idx",
		"memory_intents_append_only",
		"reflection_run_checkpoints_append_only",
		"reflection_review_decisions_append_only",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
}
