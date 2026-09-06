package docs_test

import (
	"os"
	"strings"
	"testing"
)

func TestContextProjectionDocumentationCoversSafetyContract(t *testing.T) {
	data, err := os.ReadFile("context-projections.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, term := range []string{"append-only", "tenant", "project", "namespace", "source watermark", "disable projection consumption", "canonical memory"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(term)) {
			t.Fatalf("documentation missing %q", term)
		}
	}
}
