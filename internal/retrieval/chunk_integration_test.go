package retrieval

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type chunkSearcherStub struct {
	candidates []ChunkCandidate
}

func (s chunkSearcherStub) SearchChunks(context.Context, ChunkSearchInput) ([]ChunkCandidate, error) {
	return s.candidates, nil
}

func testChunk(scope memory.Scope, id, content string, state memory.MemoryState) ChunkCandidate {
	source := memory.ChunkSourceReference{Kind: memory.ChunkSourceKindCanonicalVersion, ID: id + "-v1", Version: 1, MemoryID: id, Scope: scope}
	chunk := memory.MemoryChunk{ID: "chunk-" + id, Scope: scope, Source: source, Class: memory.MemoryClassProfile, Ordinal: 0, Content: content, SourceRange: memory.ChunkRange{Start: 0, End: len(content)}, CharacterCount: len([]rune(content)), TokenCount: len(strings.Fields(content)), LifecycleState: memory.MemoryStateActive, PolicyVersion: "p", RendererVersion: "r"}
	parent := memory.CanonicalMemory{ID: id, Scope: scope, Class: memory.MemoryClassProfile, State: state, Content: "canonical", ModifiedAt: now()}
	return ChunkCandidate{Chunk: chunk, Parent: parent, Score: ScoreBreakdown{Lexical: 1}, Citations: []Citation{{MemoryID: id, Operation: "chunk_source"}}}
}

func now() (t time.Time) { return time.Unix(1, 0).UTC() }

func TestChunkRolloutDefaultOffPreservesCanonicalFallback(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	s := NewService(ServiceDependencies{Lexical: &stubLexicalSource{}, Semantic: &stubSemanticSource{}, Chunks: chunkSearcherStub{candidates: []ChunkCandidate{testChunk(scope, "chunk-parent", "derived", memory.MemoryStateActive)}}, ChunkRollout: memory.ChunkRolloutModeDefaultOff})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "q", IncludeSummaries: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range result.Hits {
		if hit.Chunk != nil {
			t.Fatalf("default-off returned chunk hit: %+v", hit)
		}
	}
}

func TestChunkRolloutActiveReturnsParentAndCitation(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	s := NewService(ServiceDependencies{Chunks: chunkSearcherStub{candidates: []ChunkCandidate{testChunk(scope, "parent", "derived", memory.MemoryStateActive)}}, ChunkRollout: memory.ChunkRolloutModeActive})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "q", IncludeSummaries: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "parent" || result.Hits[0].Chunk == nil {
		t.Fatalf("hits=%+v", result.Hits)
	}
	if len(result.Hits[0].Citations) != 1 || result.Hits[0].Citations[0].MemoryID != "parent" {
		t.Fatalf("citations=%+v", result.Hits[0].Citations)
	}
}

func TestChunkRolloutRejectsForeignAndHiddenParents(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	foreign := testChunk(memory.Scope{Tenant: "other", Project: "p", Namespace: "n"}, "foreign", "x", memory.MemoryStateActive)
	hidden := testChunk(scope, "hidden", "x", memory.MemoryStateSuppressed)
	s := NewService(ServiceDependencies{Chunks: chunkSearcherStub{candidates: []ChunkCandidate{foreign, hidden}}, ChunkRollout: memory.ChunkRolloutModeActive})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "q", IncludeSummaries: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 0 {
		t.Fatalf("hits=%+v, want hidden/foreign omitted", result.Hits)
	}
}

func TestChunkContextUsesExistingSectionAndBoundsCharacterBudget(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	candidate := testChunk(scope, "parent", "derived evidence", memory.MemoryStateActive)
	s := NewService(ServiceDependencies{Chunks: chunkSearcherStub{candidates: []ChunkCandidate{candidate}}, ChunkRollout: memory.ChunkRolloutModeActive})
	result, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1, CharacterBudget: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Profile) != 1 || result.Profile[0].Memory.ID != "parent" || result.Profile[0].Memory.Content != "derived evidence" {
		t.Fatalf("profile=%+v, want chunk evidence in existing profile section", result.Profile)
	}

	bounded, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1, CharacterBudget: 1, IncludeDiagnostics: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded.Profile) != 0 {
		t.Fatalf("bounded profile=%+v, want chunk omitted", bounded.Profile)
	}
	foundChunkDiagnostic := false
	for _, diagnostic := range bounded.Diagnostics {
		if diagnostic.Section == "chunk_context" {
			foundChunkDiagnostic = true
			break
		}
	}
	if !foundChunkDiagnostic {
		t.Fatalf("diagnostics=%+v, want redacted chunk diagnostic", bounded.Diagnostics)
	}
}

func TestChunkShadowLeavesPublicResultUnchanged(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	s := NewService(ServiceDependencies{Chunks: chunkSearcherStub{candidates: []ChunkCandidate{testChunk(scope, "parent", "derived", memory.MemoryStateActive)}}, ChunkRollout: memory.ChunkRolloutModeShadow})
	result, err := s.Search(context.Background(), SearchInput{Scope: scope, Query: "q", IncludeSummaries: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("ordinary shadow response=%+v, want no chunk disclosure", result)
	}
}
