package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type ScopeResolver struct {
	authorizer auth.PrincipalAuthorizer
	bindings   provider.RuntimeBindingStore
	now        func() time.Time
}

func NewScopeResolver(authorizer auth.PrincipalAuthorizer, bindings provider.RuntimeBindingStore, now func() time.Time) *ScopeResolver {
	if now == nil {
		now = time.Now
	}
	return &ScopeResolver{authorizer: authorizer, bindings: bindings, now: now}
}

func (r *ScopeResolver) Resolve(ctx context.Context, principal auth.Principal, explicit *memory.Scope, bindingID string) (DispatchContext, error) {
	if r == nil || r.authorizer == nil || principal.Status != auth.PrincipalStatusActive || strings.TrimSpace(principal.ID) == "" {
		return DispatchContext{}, mcpError(ErrorAuth, "unauthorized")
	}
	if explicit != nil {
		scope := explicit.Normalized()
		if err := scope.Validate(); err != nil {
			return DispatchContext{}, mcpError(ErrorScope, "invalid_scope")
		}
		granted, err := r.authorizer.AuthorizeScope(ctx, principal.ID, scope)
		if err != nil || !granted {
			return DispatchContext{}, mcpError(ErrorScope, "forbidden")
		}
		writable, err := r.writable(ctx, principal.ID, scope)
		if err != nil {
			return DispatchContext{}, err
		}
		return DispatchContext{PrincipalID: principal.ID, Role: string(principal.Role), Scope: scope, AccessMode: "explicit", Writable: writable}, nil
	}
	if strings.TrimSpace(bindingID) == "" || r.bindings == nil {
		return DispatchContext{}, mcpError(ErrorScope, "active_scope_unavailable")
	}
	binding, err := r.bindings.Lookup(ctx, strings.TrimSpace(bindingID))
	if err != nil || binding.PrincipalID != principal.ID || binding.Validate(r.now().UTC()) != nil {
		return DispatchContext{}, mcpError(ErrorScope, "active_scope_unavailable")
	}
	granted, err := r.authorizer.AuthorizeScope(ctx, principal.ID, binding.Scope.Normalized())
	if err != nil || !granted {
		return DispatchContext{}, mcpError(ErrorScope, "forbidden")
	}
	writable, err := r.writable(ctx, principal.ID, binding.Scope.Normalized())
	if err != nil {
		return DispatchContext{}, err
	}
	return DispatchContext{PrincipalID: principal.ID, Role: string(principal.Role), Scope: binding.Scope.Normalized(), AccessMode: "active", ActiveScope: true, Writable: writable}, nil
}

func (r *ScopeResolver) writable(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	access, ok := r.authorizer.(auth.ScopeAccessAuthorizer)
	if !ok {
		return false, mcpError(ErrorScope, "scope_access_unavailable")
	}
	mode, err := access.AuthorizeScopeAccess(ctx, principalID, scope)
	if err != nil || !mode.Valid() {
		return false, mcpError(ErrorScope, "forbidden")
	}
	return mode == auth.ScopeGrantAccessReadWrite, nil
}
