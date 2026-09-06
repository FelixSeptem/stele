package memory

import "testing"

func TestChunkTextUsesDeterministicBoundaryFirstSegmentation(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := DefaultChunkPolicy("policy-v1", "renderer-v1")
	policy.Classes[MemoryClassEpisodic] = ChunkClassPolicy{MaxCharacters: 64, MaxTokens: 16}
	input := ChunkingInput{
		Source: ChunkSourceReference{Kind: ChunkSourceKindRawEvent, ID: "event-1", Version: 1, Scope: scope, SessionID: "session-1", UserID: "user-1"},
		Scope:  scope, Class: MemoryClassEpisodic, Content: "First message.\n\nSecond paragraph with words.", Policy: policy,
	}
	first, err := ChunkText(input)
	if err != nil {
		t.Fatalf("ChunkText() error = %v", err)
	}
	second, err := ChunkText(input)
	if err != nil {
		t.Fatalf("second ChunkText() error = %v", err)
	}
	if len(first) != 2 || first[0].Ordinal != 0 || first[1].Ordinal != 1 {
		t.Fatalf("chunks = %+v, want two stable ordinals", first)
	}
	if first[0].Content != "First message." || first[1].Content != "Second paragraph with words." {
		t.Fatalf("chunks = %+v, want paragraph boundaries", first)
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].SourceRange != second[i].SourceRange {
			t.Fatalf("non-deterministic chunk %d: first=%+v second=%+v", i, first[i], second[i])
		}
	}
}

func TestChunkTextHardBoundsOversizedAtomicUnit(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := DefaultChunkPolicy("policy-v1", "renderer-v1")
	policy.Classes[MemoryClassProfile] = ChunkClassPolicy{MaxCharacters: 5, MaxTokens: 2}
	input := ChunkingInput{Source: ChunkSourceReference{Kind: ChunkSourceKindCanonicalVersion, ID: "m-1", Version: 3, MemoryID: "m-1", Scope: scope}, Scope: scope, Class: MemoryClassProfile, Content: "abcdefghijk", Policy: policy}
	chunks, err := ChunkText(input)
	if err != nil {
		t.Fatalf("ChunkText() error = %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	for i, c := range chunks {
		if len([]rune(c.Content)) > 5 || c.Ordinal != i {
			t.Fatalf("chunk %d = %+v exceeds bound or ordinal", i, c)
		}
	}
}

func TestChunkTextRejectsForeignOrHiddenSource(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := DefaultChunkPolicy("p", "r")
	input := ChunkingInput{Source: ChunkSourceReference{Kind: ChunkSourceKindRawEvent, ID: "e", Version: 1, Scope: Scope{Tenant: "other", Project: "p", Namespace: "n"}, LifecycleState: MemoryStateSuppressed}, Scope: scope, Class: MemoryClassEpisodic, Content: "secret", Policy: policy}
	if _, err := ChunkText(input); err == nil {
		t.Fatal("ChunkText() error = nil for foreign/hidden source")
	}
}

func TestChunkTextPreservesClassAwareNaturalBoundaries(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := DefaultChunkPolicy("policy-v1", "renderer-v1")
	cases := []struct {
		name    string
		class   MemoryClass
		content string
		want    int
	}{
		{"profile message facts", MemoryClassProfile, "name: Ada\nrole: engineer", 2},
		{"episodic sentences", MemoryClassEpisodic, "Arrived. Left.", 2},
		{"procedural list group", MemoryClassProcedural, "1. inspect\n2. repair", 1},
		{"summary paragraphs", MemoryClassSummary, "First coverage.\n\nSecond coverage.", 2},
		{"atomic relation", MemoryClassRelation, "Ada -> team", 1},
		{"fenced code", MemoryClassProcedural, "```go\nfmt.Println(\"x\")\n```", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := ChunkingInput{Source: ChunkSourceReference{Kind: ChunkSourceKindRawEvent, ID: tc.name, Version: 1, Scope: scope}, Scope: scope, Class: tc.class, Content: tc.content, Policy: policy}
			chunks, err := ChunkText(input)
			if err != nil {
				t.Fatalf("ChunkText() error = %v", err)
			}
			if len(chunks) != tc.want {
				t.Fatalf("got %d chunks (%+v), want %d", len(chunks), chunks, tc.want)
			}
			for i, chunk := range chunks {
				if chunk.Ordinal != i || chunk.SourceRange.Start < 0 || chunk.SourceRange.End > len(tc.content) {
					t.Fatalf("invalid source lineage: %+v", chunk)
				}
			}
		})
	}
}
