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
	root := completeDocumentationFixture(t)
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

func TestCheckRequiresPortalNavigation(t *testing.T) {
	root := completeDocumentationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err == nil || !strings.Contains(err.Error(), "portal-navigation") {
		t.Fatalf("Check() error = %v, want portal-navigation", err)
	}
}

func TestCheckRequiresArchitectureNavigation(t *testing.T) {
	root := completeDocumentationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "docs", "architecture.md"), []byte("# Architecture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err == nil || !strings.Contains(err.Error(), "architecture-navigation") {
		t.Fatalf("Check() error = %v, want architecture-navigation", err)
	}
}

func TestCheckRequiresPublicEntryPointNavigation(t *testing.T) {
	root := completeDocumentationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Stele\n## Documentation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err == nil || !strings.Contains(err.Error(), "entry-point-navigation") {
		t.Fatalf("Check() error = %v, want entry-point-navigation", err)
	}
}

func completeDocumentationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	contents := map[string]string{
		"README.md": "# Stele\n## Documentation\n[portal](docs/README.md)\n## Choose an integration boundary\n",
		"docs/README.md": "# Docs\n## Understand Stele\n## Run Stele\n## Integrate Stele\n## Extend and maintain Stele\nOpenAPI Provider MCP Self-hosting OpenSpec authoritative\n",
		"docs/getting-started.md": "**Role:** tutorial\nPostgreSQL is the only system of record\n## Choose your next integration path\n[self-hosting](self-hosting.md)\n",
		"docs/architecture.md": "## Authorization and disclosure flow\n## Event-to-memory lifecycle\n## Choosing an integration boundary\nPostgreSQL is the only system of record\n",
		"docs/components.md": "components\n",
		"docs/best-practices.md": "practices\n",
		"docs/self-hosting.md": "**Role:** operations reference\n[Getting started](getting-started.md)\n",
		"docs/agent-runtime-memory-provider.md": "**Role:** reference\n[portal](README.md) does not authorize fallback\n",
		"docs/integrations/mcp-agent-skill.md": "**Role:** reference\n[portal](../README.md)\ntenant project namespace idempotency who_am_i memory_remember memory_forget_preview memory_forget_apply\nwho_am_i memory_search memory_context memory_browse memory_remember memory_forget memory_forget_preview memory_forget_apply\n",
		"skills/stele-memory/SKILL.md": "tenant project namespace idempotency who_am_i memory_remember memory_forget_preview memory_forget_apply\nwho_am_i memory_search memory_context memory_browse memory_remember memory_forget memory_forget_preview memory_forget_apply\n",
	}
	for path, content := range contents {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
