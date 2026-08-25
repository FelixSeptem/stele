package auth

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type bootstrapAdminGateStub struct {
	hasAdmin bool
	err      error
}

func (s bootstrapAdminGateStub) HasActiveAdminPrincipal(context.Context) (bool, error) {
	return s.hasAdmin, s.err
}

func TestBootstrapAuthorizerOnlyAuthorizesConfiguredDefaultScopeBeforeDurableAdmin(t *testing.T) {
	defaultScope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	authorizer := NewBootstrapAuthorizer("bootstrap-key", defaultScope, bootstrapAdminGateStub{})
	principal, _, err := authorizer.Authenticate(context.Background(), "bootstrap-key")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.Role != PrincipalRoleAdmin {
		t.Fatalf("principal role = %q, want admin", principal.Role)
	}
	granted, err := authorizer.AuthorizeScope(context.Background(), principal.ID, defaultScope)
	if err != nil || !granted {
		t.Fatalf("AuthorizeScope(default) = %v, %v; want true, nil", granted, err)
	}
	granted, err = authorizer.AuthorizeScope(context.Background(), principal.ID, memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"})
	if err != nil || granted {
		t.Fatalf("AuthorizeScope(other) = %v, %v; want false, nil", granted, err)
	}
}

func TestBootstrapAuthorizerRejectsAccessAfterDurableAdminExists(t *testing.T) {
	authorizer := NewBootstrapAuthorizer("bootstrap-key", memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, bootstrapAdminGateStub{hasAdmin: true})
	if _, _, err := authorizer.Authenticate(context.Background(), "bootstrap-key"); err == nil {
		t.Fatal("Authenticate() error = nil after durable admin exists")
	}
}
