package docs_test

import (
	"path/filepath"
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
