package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/docscheck"
)

func TestPublicDocumentationConsistency(t *testing.T) {
	root, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := docscheck.Check(root); err != nil {
		t.Fatal(err)
	}
}

func TestPublicDocumentationPortalHasAuthorityMap(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OpenAPI", "Provider", "MCP", "Self-hosting", "OpenSpec", "authoritative"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("portal missing %q", want)
		}
	}
}
