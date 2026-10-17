package memory

import (
	"strings"
	"testing"
	"time"
)

func TestBuildMemoryRepositoryPreviewBuildsDeterministicExplicitPlan(t *testing.T) {
	snapshot := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files:         []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- shipping preference [added: 2026-10-16]\n"}},
	}
	parsed, err := ParseMemoryRepositorySnapshot(snapshot)
	if err != nil || len(parsed.Entries) != 1 {
		t.Fatalf("ParseMemoryRepositorySnapshot() result=%+v err=%v", parsed, err)
	}
	request := MemoryRepositoryPreviewRequest{
		RepositoryID:       "repository",
		Scope:              Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		Actor:              "agent",
		Reason:             "review external snapshot",
		ExpectedGeneration: 4,
		Snapshot:           snapshot,
		Selections:         []MemoryRepositorySelection{{Path: "MEMORY.md", EntryKey: parsed.Entries[0].Key, Class: MemoryClassProfile, MemoryPath: "/preferences"}},
	}

	preview, err := BuildMemoryRepositoryPreview(request, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildMemoryRepositoryPreview() error = %v", err)
	}
	if preview.Generation != 4 || preview.ParserVersion != MemoryRepositoryParserVersion {
		t.Fatalf("preview=%+v", preview)
	}
	if len(preview.Entries) != 1 || preview.Entries[0].MemoryPath != "preferences" {
		t.Fatalf("entries=%+v", preview.Entries)
	}
	if preview.EffectiveSource != (SourceMetadata{}).Effective() {
		t.Fatalf("effective source=%+v", preview.EffectiveSource)
	}
	if !memoryRepositoryDigest(preview.PlanDigest) || !memoryRepositoryDigest(preview.WarningDigest) {
		t.Fatalf("digests plan=%q warning=%q", preview.PlanDigest, preview.WarningDigest)
	}
}

func TestBuildMemoryRepositoryPreviewPreservesEntryPointNavigationOrder(t *testing.T) {
	snapshot := MemoryRepositorySnapshot{
		FormatVersion: MemoryRepositoryFormatVersion,
		Revision:      "r-navigation",
		Files: []MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- [[zeta]]\n- [[alpha]]\n"},
			{Path: "zeta.md", Content: "- zeta fact\n"},
			{Path: "alpha.md", Content: "- alpha fact\n"},
		},
	}
	parsed, err := ParseMemoryRepositorySnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	selections := make([]MemoryRepositorySelection, 0, len(parsed.Entries))
	for _, entry := range parsed.Entries {
		selections = append(selections, MemoryRepositorySelection{Path: entry.Path, EntryKey: entry.Key, Class: MemoryClassEpisodic, MemoryPath: "/" + strings.TrimSuffix(entry.Path, ".md")})
	}
	preview, err := BuildMemoryRepositoryPreview(MemoryRepositoryPreviewRequest{
		RepositoryID: "repository", Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}, Actor: "agent", Reason: "navigation", Snapshot: snapshot, Selections: selections,
	}, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Navigation) != 2 || preview.Navigation[0].Target != "zeta.md" || preview.Navigation[1].Target != "alpha.md" {
		t.Fatalf("preview navigation = %+v", preview.Navigation)
	}
}

func TestBuildMemoryRepositoryPreviewRejectsImplicitOrUnknownSelection(t *testing.T) {
	base := MemoryRepositoryPreviewRequest{
		RepositoryID: "repository", Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		Actor: "agent", Reason: "review", Snapshot: MemoryRepositorySnapshot{
			FormatVersion: MemoryRepositoryFormatVersion, Revision: "r1", Files: []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact\n"}},
		},
	}
	for _, selection := range []MemoryRepositorySelection{
		{Path: "MEMORY.md", Class: MemoryClassProfile, MemoryPath: "/"},
		{Path: "MEMORY.md", EntryKey: "not-an-entry", Class: MemoryClassProfile, MemoryPath: "/"},
		{Path: "MEMORY.md", EntryKey: "not-an-entry", Class: "unknown", MemoryPath: "/"},
	} {
		request := base
		request.Selections = []MemoryRepositorySelection{selection}
		if _, err := BuildMemoryRepositoryPreview(request, time.Now().UTC()); err == nil {
			t.Fatalf("BuildMemoryRepositoryPreview() accepted selection %+v", selection)
		}
	}
}
