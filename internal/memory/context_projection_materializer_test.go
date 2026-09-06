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

func TestMaterializeContextProjectionFailsClosedForUnapprovedOrStaleDerivedOutput(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence := CompactionEvidence{ID: "compact-1", Scope: scope, Trigger: "pressure", SourceWatermark: ContextProjectionWatermark{RawEventIDs: []string{"event-1"}}, DerivationVersion: "d1", SummaryVersion: "s1", EvidenceCoverage: 1, State: CompactionEvidenceStateActive, RawEventReferences: []ContextProjectionSource{{Kind: ContextProjectionSourceRawEvent, ID: "event-1", Scope: scope}}}
	input := MaterializeContextProjectionInput{
		Scope: scope, Kind: ContextProjectionKindSession, Version: 1, SchemaVersion: "schema-v1", RendererVersion: "renderer-v1", Policy: DefaultContextProjectionPolicy("policy-v1"),
		Sources: []ContextProjectionCandidate{
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "unapproved", Version: 1, Scope: scope}, Class: MemoryClassSummary, State: MemoryStateActive, Content: "no", Derived: true, Evidence: &evidence},
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "stale", Version: 1, Scope: scope}, Class: MemoryClassSummary, State: MemoryStateActive, Content: "no", Derived: true, Approved: true, Evidence: &CompactionEvidence{ID: "stale", Scope: scope, Trigger: "pressure", SourceWatermark: ContextProjectionWatermark{RawEventIDs: []string{"event-1"}}, DerivationVersion: "d1", SummaryVersion: "s1", EvidenceCoverage: 1, State: CompactionEvidenceStateStale, RawEventReferences: []ContextProjectionSource{{Kind: ContextProjectionSourceRawEvent, ID: "event-1", Scope: scope}}}},
			{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "approved", Version: 1, Scope: scope}, Class: MemoryClassSummary, State: MemoryStateActive, Content: "yes", Derived: true, Approved: true, Evidence: &evidence},
		},
	}
	projection, err := MaterializeContextProjection(context.Background(), input)
	if err != nil {
		t.Fatalf("MaterializeContextProjection() error = %v", err)
	}
	if len(projection.Items) != 1 || projection.Items[0].Text != "yes" {
		t.Fatalf("projection items = %+v, want only approved active derived output", projection.Items)
	}
}
