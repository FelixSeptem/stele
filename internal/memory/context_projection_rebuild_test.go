package memory

import (
	"context"
	"testing"
)

type projectionStoreStub struct {
	latest  ContextProjection
	created []ContextProjection
}

func (s *projectionStoreStub) ReadLatestContextProjection(context.Context, Scope, ContextProjectionKind) (ContextProjection, error) {
	return s.latest, nil
}
func (s *projectionStoreStub) CreateContextProjection(_ context.Context, p ContextProjection) (ContextProjection, error) {
	s.created = append(s.created, p)
	return p, nil
}

func TestRebuildContextProjectionPreservesHistoryWithNewVersion(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &projectionStoreStub{latest: ContextProjection{Version: 3}}
	projection, err := RebuildContextProjection(context.Background(), MaterializeContextProjectionInput{Scope: scope, Kind: ContextProjectionKindAlwaysVisible, Version: 1, SchemaVersion: "schema-v1", RendererVersion: "renderer-v1", Policy: DefaultContextProjectionPolicy("policy-v1"), Sources: []ContextProjectionCandidate{{Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "mem", Version: 1, Scope: scope}, Class: MemoryClassProfile, State: MemoryStateActive, Content: "value", Confidence: 1}}}, store)
	if err != nil {
		t.Fatalf("RebuildContextProjection() error = %v", err)
	}
	if projection.Version != 4 || len(store.created) != 1 {
		t.Fatalf("projection=%+v created=%d, want version 4 and one append", projection, len(store.created))
	}
}

type projectionSourceStoreStub struct {
	projectionStoreStub
	gotScope Scope
	gotKind  ContextProjectionKind
	gotLimit int
	sources  []ContextProjectionCandidate
}

func (s *projectionSourceStoreStub) ListContextProjectionCandidates(_ context.Context, scope Scope, kind ContextProjectionKind, limit int) ([]ContextProjectionCandidate, error) {
	s.gotScope = scope
	s.gotKind = kind
	s.gotLimit = limit
	return s.sources, nil
}

func TestRebuildContextProjectionFromStoreLoadsAuthorizedCandidates(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &projectionSourceStoreStub{
		projectionStoreStub: projectionStoreStub{latest: ContextProjection{Version: 1}},
		sources: []ContextProjectionCandidate{{
			Source:     ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "version-1", MemoryID: "memory-1", Version: 1, Scope: scope},
			Class:      MemoryClassProfile,
			State:      MemoryStateActive,
			Content:    "owned canonical content",
			Confidence: 1,
		}},
	}

	projection, err := RebuildContextProjectionFromStore(context.Background(), ContextProjectionRebuildRequest{
		Scope:           scope,
		Kind:            ContextProjectionKindAlwaysVisible,
		Limit:           12,
		SchemaVersion:   "schema-v1",
		Policy:          DefaultContextProjectionPolicy("policy-v1"),
		RendererVersion: "renderer-v1",
	}, store)
	if err != nil {
		t.Fatalf("RebuildContextProjectionFromStore() error = %v", err)
	}
	if store.gotScope != scope || store.gotKind != ContextProjectionKindAlwaysVisible || store.gotLimit != 12 {
		t.Fatalf("source store input = %#v %#v %d", store.gotScope, store.gotKind, store.gotLimit)
	}
	if projection.Version != 2 || len(projection.Items) != 1 || projection.Items[0].Text != "owned canonical content" {
		t.Fatalf("projection = %+v, want authorized source materialized as version 2", projection)
	}
}

func TestRebuildContextProjectionDoesNotMutateCanonicalSourceSnapshot(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	source := ContextProjectionCandidate{
		Source: ContextProjectionSource{Kind: ContextProjectionSourceCanonicalVersion, ID: "version-1", MemoryID: "memory-1", Version: 1, Scope: scope},
		Class:  MemoryClassProfile, State: MemoryStateActive, Content: "canonical value", Confidence: 1,
	}
	original := source
	store := &projectionSourceStoreStub{projectionStoreStub: projectionStoreStub{latest: ContextProjection{Version: 1}}, sources: []ContextProjectionCandidate{source}}
	if _, err := RebuildContextProjectionFromStore(context.Background(), ContextProjectionRebuildRequest{Scope: scope, Kind: ContextProjectionKindAlwaysVisible, Limit: 10, SchemaVersion: "schema-v1", Policy: DefaultContextProjectionPolicy("policy-v1"), RendererVersion: "renderer-v1"}, store); err != nil {
		t.Fatalf("RebuildContextProjectionFromStore() error = %v", err)
	}
	if store.sources[0].Content != original.Content || store.sources[0].Source.Version != original.Source.Version || store.sources[0].State != original.State {
		t.Fatalf("canonical source snapshot mutated: got %+v want %+v", store.sources[0], original)
	}
}
