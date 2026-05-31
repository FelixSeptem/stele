package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyMiddlewareRejectsMissingKey(t *testing.T) {
	mw := APIKeyMiddleware(StaticAPIKeys{"test-key": {}})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddlewareRejectsInvalidKey(t *testing.T) {
	mw := APIKeyMiddleware(StaticAPIKeys{"test-key": {}})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	req.Header.Set(HeaderAPIKey, "wrong-key")
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestScopeMiddlewareRejectsMissingScope(t *testing.T) {
	mw := ScopeMiddleware()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestScopeMiddlewareInjectsResolvedScope(t *testing.T) {
	mw := ScopeMiddleware()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		scope, ok := ScopeFromContext(r.Context())
		if !ok {
			t.Fatal("ScopeFromContext() ok = false, want true")
		}

		if scope.Tenant != "tenant-a" || scope.Project != "project-a" || scope.Namespace != "namespace-a" {
			t.Fatalf("scope = %+v, want tenant/project/namespace headers", scope)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	req.Header.Set(HeaderTenant, "tenant-a")
	req.Header.Set(HeaderProject, "project-a")
	req.Header.Set(HeaderNamespace, "namespace-a")
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}
}
