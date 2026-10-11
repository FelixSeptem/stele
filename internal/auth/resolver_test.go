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

type resolverDisclosureStub struct {
	PrincipalAuthorizer
	scope        memory.Scope
	principalID  string
	entitlements DisclosureEntitlements
	err          error
}

func (s *resolverDisclosureStub) AuthorizeScopeDisclosure(_ context.Context, principalID string, scope memory.Scope) (DisclosureEntitlements, error) {
	s.principalID, s.scope = principalID, scope
	return s.entitlements, s.err
}

func TestPrincipalResolverForwardsCurrentDisclosureToDurableAuthorizer(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	durable := &resolverDisclosureStub{entitlements: DisclosureEntitlements{AllowedProfiles: []string{DisclosureProfileContent}, DefaultProfile: DisclosureProfileContent}}
	resolver := NewPrincipalResolver(durable, nil)
	authorizer, ok := any(resolver).(ScopeDisclosureAuthorizer)
	if !ok {
		t.Fatal("production principal resolver must expose scope disclosure authorization")
	}
	got, err := authorizer.AuthorizeScopeDisclosure(context.Background(), "public-principal", scope)
	if err != nil || got.DefaultProfile != DisclosureProfileContent || durable.principalID != "public-principal" || durable.scope != scope {
		t.Fatalf("durable disclosure was not forwarded: %+v, %v", got, err)
	}
	durable.entitlements = DisclosureEntitlements{AllowedProfiles: []string{DisclosureProfileReference}, DefaultProfile: DisclosureProfileReference}
	got, err = authorizer.AuthorizeScopeDisclosure(context.Background(), "public-principal", scope)
	if err != nil || got.DefaultProfile != DisclosureProfileReference {
		t.Fatal("resolver must read the current grant rather than cache the old content ceiling")
	}
	durable.err = context.Canceled
	if _, err := authorizer.AuthorizeScopeDisclosure(context.Background(), "public-principal", scope); err != context.Canceled {
		t.Fatalf("dependency error not preserved: %v", err)
	}
	for _, resolver := range []*PrincipalResolver{nil, NewPrincipalResolver(nil, durable), NewPrincipalResolver(durable, durable)} {
		authorizer := any(resolver).(ScopeDisclosureAuthorizer)
		if _, err := authorizer.AuthorizeScopeDisclosure(context.Background(), bootstrapPrincipalID, scope); err == nil {
			t.Fatal("bootstrap or unavailable durable authorization must not implicitly grant disclosure")
		}
	}
}
