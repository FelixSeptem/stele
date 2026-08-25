package auth

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type resolverStoreStub struct {
	principal  Principal
	credential Credential
	err        error
	grant      bool
}

func (s resolverStoreStub) ReadPrincipalCredential(context.Context, string) (Principal, Credential, error) {
	return s.principal, s.credential, s.err
}
func (s resolverStoreStub) HasActiveScopeGrant(context.Context, string, memory.Scope) (bool, error) {
	return s.grant, nil
}

func TestPrincipalResolverFallsBackOnlyForBootstrapOperator(t *testing.T) {
	now := time.Now().UTC()
	durable := NewPrincipalService(resolverStoreStub{err: context.Canceled}, func() time.Time { return now })
	bootstrap := NewBootstrapAuthorizer("bootstrap-key", memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, bootstrapAdminGateStub{})
	resolver := NewPrincipalResolver(durable, bootstrap)
	principal, _, err := resolver.Authenticate(context.Background(), "bootstrap-key")
	if err != nil || principal.ID != bootstrapPrincipalID {
		t.Fatalf("Authenticate(bootstrap) = %+v, %v", principal, err)
	}

	durable = NewPrincipalService(resolverStoreStub{err: context.Canceled}, func() time.Time { return now })
	resolver = NewPrincipalResolver(durable, nil)
	if _, _, err := resolver.Authenticate(context.Background(), "bootstrap-key"); err == nil {
		t.Fatal("Authenticate() error = nil without bootstrap fallback")
	}
}
