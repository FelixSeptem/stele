package docs_test

import (
	"os"
	"strings"
	"testing"
)

func TestChunkingDocumentationCoversDerivedSafetyAndRollback(t *testing.T) {
	content, err := os.ReadFile("chunking.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(content))
	for _, term := range []string{"derived postgresql", "raw events", "canonical-memory", "default_off", "shadow", "active", "tenant/project/namespace", "idempotent", "fail closed", "rollback", "stele_test_postgres_chunk_dsn"} {
		if !strings.Contains(text, strings.ToLower(term)) {
			t.Fatalf("chunking documentation missing %q", term)
		}
	}
}
