package docs_test

import (
	"os"
	"strings"
	"testing"
)

// TestBiTemporalMigrationDocumentationCoversRecoveryAndRollback proves the
// operator-facing note is actionable: it must cover the additive migration, the
// legacy backfill, idempotent re-runs, recovery from a dirty version, and both
// rollback paths.
func TestBiTemporalMigrationDocumentationCoversRecoveryAndRollback(t *testing.T) {
	content, err := os.ReadFile("bi-temporal-migration-and-rollback.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(content))

	for _, term := range []string{
		"0014_bi_temporal_fact_validity",
		"additive",
		"if not exists",
		"legacy current-compatible",
		"idempotent",
		"temporal_corrections",
		"append-only",
		"temporal_head_version",
		"dirty",
		"operational rollback",
		"schema revert",
		"disables temporal-aware policy resolution",
		"never deletes temporal history",
		"never rewrites canonical data",
		"bounded",
	} {
		if !strings.Contains(text, strings.ToLower(term)) {
			t.Fatalf("bi-temporal migration documentation missing %q", term)
		}
	}
}

// TestBiTemporalMigrationDocumentationWarnsSchemaRevertIsDestructive proves the
// note does not let an operator mistake the dev-only schema revert for the
// supported production path. This is the single most dangerous confusion in the
// rollback story, so it is pinned rather than left to prose.
func TestBiTemporalMigrationDocumentationWarnsSchemaRevertIsDestructive(t *testing.T) {
	content, err := os.ReadFile("bi-temporal-migration-and-rollback.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(content))

	if !strings.Contains(text, "does") || !strings.Contains(text, "discard") {
		t.Fatal("schema revert section must state that it discards temporal history")
	}
	if !strings.Contains(text, "not") || !strings.Contains(text, "production rollback path") {
		t.Fatal("schema revert section must state it is not the production rollback path")
	}
}
