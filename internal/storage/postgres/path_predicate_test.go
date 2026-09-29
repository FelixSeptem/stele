package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestMemoryPathPredicateSQLUsesExactOrSegmentBoundaryPrefix(t *testing.T) {
	exact, err := memory.NewMemoryPathSelector("agents/research", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := memoryPathPredicateSQL("cm", exact, 13); got != "AND cm.memory_path = $13" {
		t.Fatalf("exact predicate = %q", got)
	}
	prefix, err := memory.NewMemoryPathSelector("", "agents/research")
	if err != nil {
		t.Fatal(err)
	}
	got := memoryPathPredicateSQL("cm", prefix, 13)
	if !strings.Contains(got, "cm.memory_path = $13") || !strings.Contains(got, "LIKE replace(replace(replace($13") || !strings.Contains(got, "ESCAPE E'\\\\'") {
		t.Fatalf("prefix predicate = %q", got)
	}
}

func TestMemoryPathPredicateSQLOmitsUnselectedPath(t *testing.T) {
	if got := memoryPathPredicateSQL("", memory.MemoryPathSelector{}, 13); got != "" {
		t.Fatalf("none predicate = %q, want empty", got)
	}
}

func TestScanWithOptionalMemoryPathUsesPathDestinationsForSingleRows(t *testing.T) {
	scanner := &columnCountScanner{want: 3}
	var path string
	if err := scanWithOptionalMemoryPath(scanner, &path, []any{new(string), &path, new(string)}, []any{new(string), new(string)}); err != nil {
		t.Fatalf("scan with path columns: %v", err)
	}
	if scanner.calls != 1 {
		t.Fatalf("Scan calls = %d, want one path-aware scan", scanner.calls)
	}
}

type columnCountScanner struct {
	want  int
	calls int
}

func (s *columnCountScanner) Scan(dest ...any) error {
	s.calls++
	if len(dest) != s.want {
		return errors.New("number of field descriptions must equal number of destinations, got 3 and 2")
	}
	return nil
}
