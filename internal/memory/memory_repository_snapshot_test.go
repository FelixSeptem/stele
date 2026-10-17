package memory

import (
	"strings"
	"testing"
)

func TestMemoryRepositorySnapshotValidatesAndDigestsIndependentlyOfFileOrder(t *testing.T) {
	first := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "git:8798cb2",
		Files: []MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- [[projects/payments]]\n"},
			{Path: "projects/payments.md", Content: "- Payments share a launch deadline\n"},
		},
	}
	second := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "git:8798cb2",
		Files: []MemoryRepositoryFile{
			{Path: "projects/payments.md", Content: "- Payments share a launch deadline\n"},
			{Path: "MEMORY.md", Content: "- [[projects/payments]]\n"},
		},
	}

	firstDigest, err := first.CanonicalDigest()
	if err != nil {
		t.Fatalf("first.CanonicalDigest() error = %v", err)
	}
	secondDigest, err := second.CanonicalDigest()
	if err != nil {
		t.Fatalf("second.CanonicalDigest() error = %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("file ordering changed snapshot digest: %q != %q", firstDigest, secondDigest)
	}
}

func TestMemoryRepositorySnapshotRejectsUnsafeOrAmbiguousFiles(t *testing.T) {
	valid := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files:         []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact\n"}},
	}
	for _, tt := range []struct {
		name     string
		mutate   func(*MemoryRepositorySnapshot)
		contains string
	}{
		{"unknown format", func(s *MemoryRepositorySnapshot) { s.FormatVersion = "memory-repo-v2" }, "format version"},
		{"missing manifest", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = "topic.md" }, "MEMORY.md"},
		{"traversal", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = "../MEMORY.md" }, "path"},
		{"backslash", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = "topics\\MEMORY.md" }, "path"},
		{"encoded separator", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = "topics%2fMEMORY.md" }, "path"},
		{"non NFC path", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = "topic/e\u0301.md" }, "path"},
		{"invalid UTF-8 path", func(s *MemoryRepositorySnapshot) { s.Files[0].Path = string([]byte{'m', 0xff, '.', 'm', 'd'}) }, "path"},
		{"invalid UTF-8 content", func(s *MemoryRepositorySnapshot) { s.Files[0].Content = string([]byte{'-', ' ', 0xff}) }, "UTF-8"},
		{"duplicate path", func(s *MemoryRepositorySnapshot) {
			s.Files = append(s.Files, MemoryRepositoryFile{Path: "MEMORY.md", Content: "- duplicate"})
		}, "duplicate"},
		{"oversized revision", func(s *MemoryRepositorySnapshot) {
			s.Revision = strings.Repeat("r", MemoryRepositoryMaxRevisionBytes+1)
		}, "revision"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := valid
			snapshot.Files = append([]MemoryRepositoryFile(nil), valid.Files...)
			tt.mutate(&snapshot)
			if _, err := snapshot.CanonicalDigest(); err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("CanonicalDigest() error = %v, want error containing %q", err, tt.contains)
			}
		})
	}
}

func TestMemoryRepositorySnapshotRejectsDuplicateNormalizedPathsBeforeParsing(t *testing.T) {
	snapshot := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files: []MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- fact\n"},
			{Path: "topic/e\u0301.md", Content: "- other\n"},
		},
	}
	if _, err := ParseMemoryRepositorySnapshot(snapshot); err == nil {
		t.Fatal("ParseMemoryRepositorySnapshot() accepted a non-normalized path")
	}
}
