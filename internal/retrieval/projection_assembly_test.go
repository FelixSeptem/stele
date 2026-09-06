package retrieval

import (
	"context"
	"fmt"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type projectionReaderForTest struct{ projection memory.ContextProjection }

func (r projectionReaderForTest) ReadLatestContextProjection(_ context.Context, scope memory.Scope, kind memory.ContextProjectionKind) (memory.ContextProjection, error) {
	if kind != r.projection.Kind || scope != r.projection.Scope {
		return memory.ContextProjection{}, fmt.Errorf("not found")
	}
	return r.projection, nil
}

func TestProjectionAssemblyOmitsForeignHiddenAndOverBudgetItems(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	foreign := memory.Scope{Tenant: "other", Project: "p", Namespace: "n"}
	p := memory.ContextProjection{ID: "p", Scope: scope, Kind: memory.ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "s", PolicyVersion: "p", RendererVersion: "r", Status: memory.ContextProjectionStatusActive, Items: []memory.ContextProjectionItem{
		{ID: "visible", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "m1", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "ok", SortKey: "1", Citation: memory.ProjectionCitation{MemoryID: "m1", Operation: "context_projection"}},
		{ID: "hidden", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "m2", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateSuppressed, Text: "secret", SortKey: "2"},
		{ID: "foreign", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "m3", Version: 1, Scope: foreign}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "foreign", SortKey: "3"},
		{ID: "large", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "m4", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "too-large", SortKey: "4"},
	}}
	s := NewService(ServiceDependencies{Lexical: &stubLexicalSource{}, Semantic: &stubSemanticSource{}, Relations: &stubRelationSource{}, Projections: projectionReaderForTest{projection: p}})
	result, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 2, CharacterBudget: 2, IncludeDiagnostics: true, UseProjections: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Profile) != 1 || result.Profile[0].Memory.ID != "m1" {
		t.Fatalf("profile=%+v", result.Profile)
	}
	if len(result.Citations) != 1 || result.Citations[0].MemoryID != "m1" {
		t.Fatalf("citations=%+v", result.Citations)
	}
	foundBudget := false
	for _, d := range result.Diagnostics {
		if d.Status == "omitted_by_budget" {
			foundBudget = true
		}
	}
	if !foundBudget {
		t.Fatalf("diagnostics=%+v, want budget omission", result.Diagnostics)
	}
}

func TestProjectionAssemblyUsesRuntimeConsumptionSwitch(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	p := memory.ContextProjection{ID: "p", Scope: scope, Kind: memory.ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "s", PolicyVersion: "p", RendererVersion: "r", Status: memory.ContextProjectionStatusActive, Items: []memory.ContextProjectionItem{{ID: "visible", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "m", Version: 1, Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "visible", SortKey: "1"}}}
	s := NewService(ServiceDependencies{Lexical: &stubLexicalSource{}, Semantic: &stubSemanticSource{}, Relations: &stubRelationSource{}, Projections: projectionReaderForTest{projection: p}, ProjectionConsumptionEnabled: true})
	result, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Profile) != 1 || result.Profile[0].Memory.Content != "visible" {
		t.Fatalf("profile=%+v, want runtime-enabled projection", result.Profile)
	}
}
