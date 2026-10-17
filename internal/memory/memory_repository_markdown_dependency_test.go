package memory

import (
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestMemoryRepositoryMarkdownDependencyDistinguishesFencedCodeFromTopLevelList(t *testing.T) {
	document := goldmark.DefaultParser().Parse(text.NewReader([]byte("- importable fact\n\n```sh\n- not an importable fact\n```\n")))
	var listCount, fencedCodeCount int
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.List:
			listCount++
		case *ast.FencedCodeBlock:
			fencedCodeCount++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatalf("walk parsed Markdown: %v", err)
	}
	if listCount != 1 || fencedCodeCount != 1 {
		t.Fatalf("parser structure list=%d fenced_code=%d, want 1 and 1", listCount, fencedCodeCount)
	}
}
