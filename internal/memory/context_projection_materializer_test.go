package memory

import (
	"context"
	"testing"
)

func TestMaterializeContextProjectionIsDeterministicAndOmitsByPolicy(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	input := MaterializeContextProjectionInput{
		Scope: scope, Kind: ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "schema-v1", RendererVersion: "renderer-v1",
		Policy: DefaultContextProjectionPolicy("policy-v1"),
		Sources: []ContextProjectionCandidate{
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "b", Version: 1, Scope: scope}, Class: MemoryClassProfile, State: MemoryStateActive, Content: "second", Confidence: 0.9},
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "a", Version: 1, Scope: scope}, Class: MemoryClassProfile, State: MemoryStateActive, Content: "first", Confidence: 0.9},
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "hidden", Version: 1, Scope: scope}, Class: MemoryClassProfile, State: MemoryStateSuppressed, Content: "secret", Confidence: 1},
		},
	}
	first, err := MaterializeContextProjection(context.Background(), input)
	if err != nil {
		t.Fatalf("first materialization error: %v", err)
	}
	second, err := MaterializeContextProjection(context.Background(), input)
	if err != nil {
		t.Fatalf("second materialization error: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].Text != "first" || first.Items[1].Text != "second" {
		t.Fatalf("items = %+v, want deterministic visible items", first.Items)
	}
	if first.SourceWatermarkHash() != second.SourceWatermarkHash() || first.Items[0].ID != second.Items[0].ID {
		t.Fatalf("identical inputs produced different identity: first=%+v second=%+v", first, second)
	}
}
