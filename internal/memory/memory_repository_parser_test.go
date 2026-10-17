package memory

import "testing"

func TestParseMemoryRepositorySnapshotExtractsTopLevelFactsAndNavigation(t *testing.T) {
	parsed, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files: []MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- Team preference: short updates [source: https://example.test/1; added: 2026-10-16]\n- [[projects/payments]]\n```sh\n- never import this\n```\n"},
			{Path: "projects/payments.md", Content: "- Payments share a deadline\n  - nested is unsupported\n"},
		},
	})
	if err != nil {
		t.Fatalf("ParseMemoryRepositorySnapshot() error = %v", err)
	}
	if len(parsed.Entries) != 2 {
		t.Fatalf("entries=%d, want 2", len(parsed.Entries))
	}
	if parsed.Entries[0].Text != "Team preference: short updates" || parsed.Entries[0].Metadata["source"] != "https://example.test/1" {
		t.Fatalf("first entry=%+v", parsed.Entries[0])
	}
	if len(parsed.Navigation) != 1 || parsed.Navigation[0].Target != "projects/payments.md" {
		t.Fatalf("navigation=%+v", parsed.Navigation)
	}
	if len(parsed.Diagnostics) == 0 {
		t.Fatal("nested list must be diagnosed")
	}
}

func TestParseMemoryRepositorySnapshotRejectsMalformedMetadata(t *testing.T) {
	_, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{FormatVersion: MemoryRepositoryFormatVersion, Revision: "r1", Files: []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact [source: one; source: two]\n"}}})
	if err == nil {
		t.Fatal("duplicate metadata keys accepted")
	}
}

func TestMemoryRepositoryEntryKeyIsIndependentOfMetadataOrder(t *testing.T) {
	first, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{FormatVersion: MemoryRepositoryFormatVersion, Revision: "r1", Files: []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact [source: https://example.test; added: 2026-10-16]\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{FormatVersion: MemoryRepositoryFormatVersion, Revision: "r2", Files: []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact [added: 2026-10-16; source: https://example.test]\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Entries[0].Key != second.Entries[0].Key {
		t.Fatalf("entry keys differ: %q != %q", first.Entries[0].Key, second.Entries[0].Key)
	}
}

func TestParseMemoryRepositorySnapshotReportsInvalidAndMissingWikiLinksWithoutFollowingThem(t *testing.T) {
	parsed, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files:         []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- [[missing]]\n- [[report.sql]]\n- [[../private]]\n"}},
	})
	if err != nil {
		t.Fatalf("ParseMemoryRepositorySnapshot() error = %v", err)
	}
	if len(parsed.Navigation) != 0 {
		t.Fatalf("navigation=%+v, want no resolvable targets", parsed.Navigation)
	}
	if len(parsed.Diagnostics) != 3 {
		t.Fatalf("diagnostics=%+v, want three invalid/missing-link diagnostics", parsed.Diagnostics)
	}
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Code != "unresolved_wiki_link" && diagnostic.Code != "invalid_wiki_link" {
			t.Fatalf("diagnostic=%+v, want wiki-link diagnostic", diagnostic)
		}
	}
}

func TestParseMemoryRepositorySnapshotCollapsesDuplicateBulletsAndReportsCycles(t *testing.T) {
	parsed, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files: []MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- [[topic]]\n- same fact\n- same fact\n"},
			{Path: "topic.md", Content: "- [[MEMORY]]\n"},
		},
	})
	if err != nil {
		t.Fatalf("ParseMemoryRepositorySnapshot() error = %v", err)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("entries=%+v, want one collapsed fact", parsed.Entries)
	}
	foundCycle := false
	for _, diagnostic := range parsed.Diagnostics {
		foundCycle = foundCycle || diagnostic.Code == "wiki_link_cycle"
	}
	if !foundCycle {
		t.Fatalf("diagnostics=%+v, want bounded cycle diagnostic", parsed.Diagnostics)
	}
}
