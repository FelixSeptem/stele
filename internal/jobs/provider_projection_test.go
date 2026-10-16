package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type projectionFeedbackStoreStub struct {
	scope memory.Scope
	limit int
	err   error
}

func (s *projectionFeedbackStoreStub) ListProjectionFeedbackScopes(_ context.Context, limit int) ([]memory.Scope, error) {
	s.limit = limit
	return []memory.Scope{s.scope}, s.err
}

func TestProjectionFeedbackScopeSourceUsesDedicatedBoundedDiscovery(t *testing.T) {
	store := &projectionFeedbackStoreStub{scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "pending"}}
	source := ProjectionFeedbackScopeSource{Store: store}
	for _, invalid := range []ProjectionFeedbackScopeSource{{}, source} {
		if _, err := invalid.ListMaintenanceScopes(context.Background(), 0); err == nil {
			t.Fatal("invalid scope discovery accepted")
		}
	}
	scopes, err := source.ListMaintenanceScopes(context.Background(), 1)
	if err != nil || len(scopes) != 1 || scopes[0] != store.scope || store.limit != 1 {
		t.Fatalf("ledger scope discovery lost: %+v %v", scopes, err)
	}
	store.err = errors.New("unavailable")
	if _, err := source.ListMaintenanceScopes(context.Background(), 1); !errors.Is(err, store.err) {
		t.Fatal("scope discovery failure swallowed")
	}
}

func (s *projectionFeedbackStoreStub) ReconcileProjectionFeedback(_ context.Context, scope memory.Scope, limit int) (int, error) {
	s.scope, s.limit = scope, limit
	return 2, s.err
}
func TestProjectionFeedbackJobIsScopedBoundedAndFailsClosed(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	store := &projectionFeedbackStoreStub{}
	for _, job := range []ProjectionFeedbackJob{{}, {Scope: scope, Store: store}, {Scope: scope, Store: store, Limit: 101}, {Store: store, Limit: 10}} {
		if _, err := job.Run(context.Background()); err == nil {
			t.Fatal("invalid feedback job accepted")
		}
	}
	job := ProjectionFeedbackJob{Scope: scope, Store: store, Limit: 10}
	if n, err := job.Run(context.Background()); err != nil || n != 2 || store.scope != scope || store.limit != 10 {
		t.Fatalf("unscoped job: %d %v %+v", n, err, store)
	}
	store.err = errors.New("unavailable")
	if _, err := job.Run(context.Background()); !errors.Is(err, store.err) {
		t.Fatal("dependency failure swallowed")
	}
	if job.Name() != "provider_projection_feedback" {
		t.Fatal("unstable job name")
	}
}
