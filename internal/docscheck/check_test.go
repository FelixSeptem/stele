package docscheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryDocumentation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsFileAndRuleForUnsafeFixture(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"README.md",
		"docs/architecture.md",
		"docs/components.md",
		"docs/best-practices.md",
		"docs/integrations/mcp-agent-skill.md",
		"skills/stele-memory/SKILL.md",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "tenant project namespace idempotency who_am_i memory_remember memory_forget_preview memory_forget_apply\n"
		if path == "docs/integrations/mcp-agent-skill.md" || path == "skills/stele-memory/SKILL.md" {
			content += "who_am_i memory_search memory_context memory_browse memory_remember memory_forget memory_forget_preview memory_forget_apply\n"
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("[missing](docs/nope.md)\n```sql\nINSERT INTO memories VALUES (1);\n```\nsk-1234567890123456\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Check(root)
	if err == nil {
		t.Fatal("Check() error = nil for unsafe fixture")
	}
	message := err.Error()
	for _, want := range []string{"README.md:1", "local-link", "README.md:3", "direct-sql-example", "credential-literal"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Check() error = %q, missing %q", message, want)
		}
	}
}
