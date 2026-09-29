package memory

import (
	"strings"
	"testing"
)

func TestNormalizeMemoryPathUsesStableRootAndCanonicalSegments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "omitted root", in: "", want: MemoryPathRoot},
		{name: "slash root", in: "/", want: MemoryPathRoot},
		{name: "trimmed segments", in: " /agents/research/ ", want: "agents/research"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeMemoryPath(tt.in)
			if err != nil {
				t.Fatalf("NormalizeMemoryPath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeMemoryPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeMemoryPathRejectsUnsafeOrOverlongValues(t *testing.T) {
	for _, value := range []string{
		"agents//research", "agents/../research", "agents/*", "agents/%2Fresearch",
		"agents/\t\nresearch", "agents/" + strings.Repeat("x", MemoryPathMaxSegmentLength+1),
		strings.Repeat("x", MemoryPathMaxLength+1),
	} {
		if _, err := NormalizeMemoryPath(value); err == nil {
			t.Errorf("NormalizeMemoryPath(%q) accepted unsafe value", value)
		}
	}
}

func TestMemoryPathSelectorUsesExactAndSegmentBoundaryPrefixMatching(t *testing.T) {
	exact, err := NewMemoryPathSelector("agents/research", "")
	if err != nil {
		t.Fatal(err)
	}
	if !exact.Matches("agents/research") || exact.Matches("agents/research/preferences") {
		t.Fatal("exact selector did not enforce exact matching")
	}
	prefix, err := NewMemoryPathSelector("", "agents/research")
	if err != nil {
		t.Fatal(err)
	}
	if !prefix.Matches("agents/research") || !prefix.Matches("agents/research/preferences") || prefix.Matches("agents/researcher") {
		t.Fatal("prefix selector did not enforce segment boundaries")
	}
}

func TestMemoryPathSelectorRejectsConflictingSelectors(t *testing.T) {
	if _, err := NewMemoryPathSelector("agents/research", "agents"); err == nil {
		t.Fatal("expected conflicting path selectors to be rejected")
	}
}

func TestMemoryPathFingerprintIncludesSelectorKindAndValue(t *testing.T) {
	exact, err := NewMemoryPathSelector("agents/research", "")
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := NewMemoryPathSelector("", "agents/research")
	if err != nil {
		t.Fatal(err)
	}
	if exact.Fingerprint() == prefix.Fingerprint() {
		t.Fatal("exact and prefix selectors must have different fingerprints")
	}
	if NewMemoryCursor("cursor", exact).Fingerprint() == NewMemoryCursor("cursor", prefix).Fingerprint() {
		t.Fatal("cursor fingerprint must include selector")
	}
}
