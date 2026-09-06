package memory

import (
	"strings"
	"testing"
)

func TestContextProjectionValidationRejectsInvalidContract(t *testing.T) {
	projection := ContextProjection{
		ID: "projection-1", Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"},
		Kind: ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "v1", PolicyVersion: "policy-v1", RendererVersion: "renderer-v1",
		Status: ContextProjectionStatusActive,
		Items:  []ContextProjectionItem{{ID: "item-1", Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "memory-1", Version: 1, Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}}, Class: MemoryClassProfile, LifecycleState: MemoryStateActive, Text: "hello", SortKey: "0001"}},
	}
	if err := projection.Validate(); err != nil {
		t.Fatalf("valid projection rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*ContextProjection)
	}{
		{"invalid kind", func(p *ContextProjection) { p.Kind = "unknown" }},
		{"missing policy version", func(p *ContextProjection) { p.PolicyVersion = "" }},
		{"invalid source", func(p *ContextProjection) { p.Items[0].Source.Kind = "unknown" }},
		{"hidden lifecycle", func(p *ContextProjection) { p.Items[0].LifecycleState = MemoryStateSuppressed }},
		{"oversize text", func(p *ContextProjection) { p.Items[0].Text = strings.Repeat("x", MaxContextProjectionItemTextBytes+1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := projection
			candidate.Items = append([]ContextProjectionItem(nil), projection.Items...)
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want rejection")
			}
		})
	}
}

func TestContextProjectionItemsSortDeterministically(t *testing.T) {
	items := []ContextProjectionItem{
		{ID: "b", SortKey: "02", Source: ContextProjectionSource{ID: "z"}},
		{ID: "a", SortKey: "01", Source: ContextProjectionSource{ID: "b"}},
		{ID: "c", SortKey: "02", Source: ContextProjectionSource{ID: "a"}},
	}
	SortContextProjectionItems(items)
	if got := []string{items[0].ID, items[1].ID, items[2].ID}; got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Fatalf("sorted ids = %v, want [a c b]", got)
	}
}
