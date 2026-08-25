package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type stubPrincipalAuthorizer struct {
	principal  Principal
	credential Credential
	grant      bool
	err        error
}

func (s stubPrincipalAuthorizer) Authenticate(ctx context.Context, secret string) (Principal, Credential, error) {
	if s.err != nil {
		return Principal{}, Credential{}, s.err
	}
	return s.principal, s.credential, nil
}
func (s stubPrincipalAuthorizer) AuthorizeScope(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	return s.grant, nil
}

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

func TestPrincipalMiddlewareRejectsUngrantScopeBeforeHandler(t *testing.T) {
	called := false
	mw := PrincipalMiddleware(stubPrincipalAuthorizer{principal: Principal{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "a", CreatedAt: time.Now()}, credential: Credential{ID: "credential_1", PrincipalID: "principal_1", Status: CredentialStatusActive, CredentialID: "stl_credential_1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now()}, grant: false}, PrincipalRolePublic)
	req := httptest.NewRequest(http.MethodGet, "/v1/memories", nil)
	req.Header.Set(HeaderAPIKey, "stl_credential_1.secret")
	req.Header.Set(HeaderTenant, "tenant-b")
	req.Header.Set(HeaderProject, "project-a")
	req.Header.Set(HeaderNamespace, "namespace-a")
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })).ServeHTTP(rec, req)
	if called || rec.Code != http.StatusForbidden {
		t.Fatalf("called=%v status=%d, want denied before handler", called, rec.Code)
	}
}

func TestPrincipalMiddlewareRejectsPublicRoleOnAdminRoute(t *testing.T) {
	mw := PrincipalMiddleware(stubPrincipalAuthorizer{principal: Principal{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "a", CreatedAt: time.Now()}, credential: Credential{ID: "credential_1", PrincipalID: "principal_1", Status: CredentialStatusActive, CredentialID: "stl_credential_1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now()}, grant: true}, PrincipalRoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/status", nil)
	req.Header.Set(HeaderAPIKey, "stl_credential_1.secret")
	req.Header.Set(HeaderTenant, "tenant-a")
	req.Header.Set(HeaderProject, "project-a")
	req.Header.Set(HeaderNamespace, "namespace-a")
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
