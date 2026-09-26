package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type scopeAuthorizerStub struct {
	grants   map[string]bool
	readOnly map[string]bool
}

func (s scopeAuthorizerStub) Authenticate(context.Context, string) (auth.Principal, auth.Credential, error) {
	return auth.Principal{}, auth.Credential{}, nil
}
func (s scopeAuthorizerStub) AuthorizeScope(_ context.Context, principalID string, scope memory.Scope) (bool, error) {
	return s.grants[principalID+"/"+scope.Tenant+"/"+scope.Project+"/"+scope.Namespace], nil
}
func (s scopeAuthorizerStub) AuthorizeScopeAccess(_ context.Context, principalID string, scope memory.Scope) (auth.ScopeGrantAccessMode, error) {
	if !s.grants[principalID+"/"+scope.Tenant+"/"+scope.Project+"/"+scope.Namespace] {
		return "", nil
	}
	if s.readOnly[principalID+"/"+scope.Tenant+"/"+scope.Project+"/"+scope.Namespace] {
		return auth.ScopeGrantAccessReadOnly, nil
	}
	return auth.ScopeGrantAccessReadWrite, nil
}

func TestScopeResolverRequiresExactAuthorizedScopeBeforeDispatch(t *testing.T) {
	scope := memory.Scope{Tenant: "t1", Project: "p1", Namespace: "n1"}
	resolver := NewScopeResolver(scopeAuthorizerStub{grants: map[string]bool{"principal-1/t1/p1/n1": true}}, nil, time.Now)
	ctx, err := resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}, &scope, "")
	if err != nil || ctx.Scope != scope {
		t.Fatalf("Resolve() = %+v, %v; want exact authorized scope", ctx, err)
	}
	foreign := memory.Scope{Tenant: "t2", Project: "p2", Namespace: "n2"}
	if _, err := resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, &foreign, ""); err == nil {
		t.Fatal("Resolve() error = nil for foreign scope")
	}
	malformed := memory.Scope{Tenant: "t1"}
	if _, err := resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, &malformed, ""); err == nil {
		t.Fatal("Resolve() error = nil for malformed scope")
	}
}

func TestScopeResolverExplicitScopeTakesPrecedenceOverActiveBinding(t *testing.T) {
	now := time.Now().UTC()
	active := provider.RuntimeBinding{BindingID: "binding-1", PrincipalID: "principal-1", Scope: memory.Scope{Tenant: "t1", Project: "p1", Namespace: "n1"}, AgentID: "agent", SessionID: "session", ProviderInstanceID: "instance", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	explicit := memory.Scope{Tenant: "t2", Project: "p2", Namespace: "n2"}
	resolver := NewScopeResolver(scopeAuthorizerStub{grants: map[string]bool{
		"principal-1/t1/p1/n1": true,
		"principal-1/t2/p2/n2": true,
	}}, provider.NewMemoryBindingStore(), func() time.Time { return now })
	if err := resolver.bindings.Create(context.Background(), active); err != nil {
		t.Fatal(err)
	}
	ctx, err := resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, &explicit, "binding-1")
	if err != nil || ctx.Scope != explicit || ctx.ActiveScope {
		t.Fatalf("Resolve() = %+v, %v; want explicit scope and non-active mode", ctx, err)
	}
	ctx, err = resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, nil, "binding-1")
	if err != nil || ctx.Scope != active.Scope || !ctx.ActiveScope {
		t.Fatalf("Resolve() active = %+v, %v; want binding scope", ctx, err)
	}
	if _, err := resolver.Resolve(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, nil, ""); err == nil {
		t.Fatal("Resolve() error = nil without explicit or active scope")
	}
}
