package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	HeaderAPIKey    = "X-API-Key"
	HeaderTenant    = "X-Stele-Tenant"
	HeaderProject   = "X-Stele-Project"
	HeaderNamespace = "X-Stele-Namespace"
)

type StaticAPIKeys map[string]struct{}

type scopeContextKey struct{}

func APIKeyMiddleware(keys StaticAPIKeys) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func ScopeFromContext(ctx context.Context) (memory.Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(memory.Scope)
	return scope, ok
}
