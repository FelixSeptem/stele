package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

const (
	HeaderAPIKey    = "X-API-Key"
	HeaderTenant    = "X-Stele-Tenant"
	HeaderProject   = "X-Stele-Project"
	HeaderNamespace = "X-Stele-Namespace"
)

type StaticAPIKeys map[string]struct{}

type scopeContextKey struct{}
type principalContextKey struct{}

type PrincipalAuthorizer interface {
	Authenticate(ctx context.Context, secret string) (Principal, Credential, error)
	AuthorizeScope(ctx context.Context, principalID string, scope memory.Scope) (bool, error)
}

func APIKeyMiddleware(keys StaticAPIKeys) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := PrincipalFromContext(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}
			apiKey := strings.TrimSpace(r.Header.Get(HeaderAPIKey))
			if apiKey == "" {
				http.Error(w, "missing api key", http.StatusUnauthorized)
				return
			}

			if _, ok := keys[apiKey]; !ok {
				http.Error(w, "invalid api key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ScopeMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := ScopeFromContext(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}
			scope := memory.Scope{
				Tenant:    r.Header.Get(HeaderTenant),
				Project:   r.Header.Get(HeaderProject),
				Namespace: r.Header.Get(HeaderNamespace),
			}.Normalized()

			if err := scope.Validate(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx := context.WithValue(r.Context(), scopeContextKey{}, scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ProtectedMiddleware(authorizer PrincipalAuthorizer, legacyKeys StaticAPIKeys, requiredRole PrincipalRole) func(http.Handler) http.Handler {
	if authorizer != nil {
		return PrincipalMiddleware(authorizer, requiredRole)
	}
	return APIKeyMiddleware(legacyKeys)
}

func ScopeFromContext(ctx context.Context) (memory.Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(memory.Scope)
	return scope, ok
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func PrincipalMiddleware(authorizer PrincipalAuthorizer, requiredRole PrincipalRole) func(http.Handler) http.Handler {
	return PrincipalMiddlewareWithObserver(authorizer, requiredRole, nil)
}

// PrincipalMiddlewareWithObserver performs the same authorization checks as
// PrincipalMiddleware and emits only bounded operation/status categories.
// The observer is optional so existing callers and tests remain compatible.
func PrincipalMiddlewareWithObserver(authorizer PrincipalAuthorizer, requiredRole PrincipalRole, observer telemetry.Observer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authorizer == nil {
				recordAccess(observer, r, "not_configured")
				http.Error(w, "principal authorization is not configured", http.StatusServiceUnavailable)
				return
			}
			principal, _, err := authorizer.Authenticate(r.Context(), strings.TrimSpace(r.Header.Get(HeaderAPIKey)))
			if err != nil || principal.Status != PrincipalStatusActive || (requiredRole == PrincipalRoleAdmin && principal.Role != PrincipalRoleAdmin) {
				recordAccess(observer, r, "denied")
				http.Error(w, "unauthorized", http.StatusForbidden)
				return
			}
			scope := memory.Scope{Tenant: r.Header.Get(HeaderTenant), Project: r.Header.Get(HeaderProject), Namespace: r.Header.Get(HeaderNamespace)}.Normalized()
			if err := scope.Validate(); err != nil {
				recordAccess(observer, r, "invalid_scope")
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			granted, err := authorizer.AuthorizeScope(r.Context(), principal.ID, scope)
			if err != nil || !granted {
				recordAccess(observer, r, "scope_denied")
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			recordAccess(observer, r, "allowed")
			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			ctx = context.WithValue(ctx, scopeContextKey{}, scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func recordAccess(observer telemetry.Observer, r *http.Request, status string) {
	if observer == nil {
		return
	}
	observer.RecordOperation(r.Context(), telemetry.OperationEvent{
		Mode: "api", Component: "auth", Operation: "principal_access", Status: status, Count: 1,
		ObservedAt: time.Now().UTC(),
	})
}
