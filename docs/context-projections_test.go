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
	for _, term := range []string{"仅追加", "tenant", "project", "namespace", "来源水位", "禁用投影消费", "规范记忆"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(term)) {
			t.Fatalf("documentation missing %q", term)
		}
	}
}
