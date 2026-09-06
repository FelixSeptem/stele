package memory

import "testing"

func TestMemoryChunkValidateEnforcesIdentityScopeLineageAndBounds(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	chunk := MemoryChunk{
		ID: "chunk-1", Scope: scope,
		Source: ChunkSourceReference{Kind: ChunkSourceKindCanonicalVersion, ID: "memory-1", Version: 2, MemoryID: "memory-1", Scope: scope, SessionID: "session-1", UserID: "user-1"},
		Class:  MemoryClassEpisodic, Ordinal: 0, Content: "hello", SourceRange: ChunkRange{Start: 0, End: 5},
		CharacterCount: 5, TokenCount: 1, LifecycleState: MemoryStateActive,
		PolicyVersion: "policy-v1", RendererVersion: "renderer-v1",
	}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("valid chunk rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*MemoryChunk)
	}{
		{"foreign scope", func(c *MemoryChunk) { c.Scope.Project = "other" }},
		{"negative ordinal", func(c *MemoryChunk) { c.Ordinal = -1 }},
		{"invalid lifecycle", func(c *MemoryChunk) { c.LifecycleState = MemoryStateSuppressed }},
		{"invalid range", func(c *MemoryChunk) { c.SourceRange.End = 0 }},
		{"missing policy", func(c *MemoryChunk) { c.PolicyVersion = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := chunk
			tc.mutate(&copy)
			if err := copy.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestChunkPolicyResolvesEveryMemoryClassAndRolloutModes(t *testing.T) {
	policy := DefaultChunkPolicy("policy-v1", "renderer-v1")
	if err := policy.Validate(); err != nil {
		t.Fatalf("default policy invalid: %v", err)
	}
	for _, class := range []MemoryClass{MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural, MemoryClassSummary, MemoryClassRelation} {
		if _, ok := policy.ClassPolicy(class); !ok {
			t.Fatalf("missing class policy for %q", class)
		}
	}
	for _, mode := range []ChunkRolloutMode{ChunkRolloutModeDefaultOff, ChunkRolloutModeShadow, ChunkRolloutModeActive} {
		if !mode.Valid() {
			t.Fatalf("rollout mode %q should be valid", mode)
		}
	}
	if ChunkRolloutMode("unknown").Valid() {
		t.Fatal("unknown rollout mode accepted")
	}
}
