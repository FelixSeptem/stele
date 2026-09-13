package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"net/http"
	"net/http/httptest"
)

func TestProviderRoutesDisabledByDefault(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{HTTP: HTTPRuntimeLimits{MaxRequestBodyBytes: 1 << 20}})
	r := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

func TestProviderCapabilitiesRouteReturnsBoundedDocument(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCapabilities: provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})})
	r := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got provider.CapabilityDocument
	if err := provider.DecodeStrict(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ProviderVersion != "provider-v1" {
		t.Fatalf("doc=%+v", got)
	}
}

type providerLifecycleAuthorizer struct {
	principals map[string]auth.Principal
}

func (a providerLifecycleAuthorizer) Authenticate(_ context.Context, secret string) (auth.Principal, auth.Credential, error) {
	p, ok := a.principals[strings.TrimSpace(secret)]
	if !ok {
		return auth.Principal{}, auth.Credential{}, context.Canceled
	}
	return p, auth.Credential{Status: auth.CredentialStatusActive, PrincipalID: p.ID, CreatedAt: p.CreatedAt, Salt: []byte("s"), Digest: []byte("d")}, nil
}

func (providerLifecycleAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	return true, nil
}

type providerLifecycleStub struct{ applied int }

func (s *providerLifecycleStub) Apply(context.Context, memory.LifecycleActionInput) error {
	s.applied++
	return nil
}

func TestProviderLifecycleRouteRequiresAdminBeforeMutation(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	admin := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive, Label: "admin", CreatedAt: now}
	public := auth.Principal{ID: "public-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"admin-key": admin, "public-key": public}}
	store := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb_test", PrincipalID: public.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ConversationID: "conversation-a", ProviderInstanceID: "pi_test", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	lifecycle := &providerLifecycleStub{}
	adapter := provider.NewAdapter(provider.AdapterDependencies{Lifecycle: lifecycle, AllowLifecycle: func(ctx context.Context, b provider.RuntimeBinding) bool {
		p, ok := auth.PrincipalFromContext(ctx)
		return ok && p.Role == auth.PrincipalRoleAdmin && p.ID == b.PrincipalID
	}})
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"schema-v1"}, ProviderAdapter: adapter, ProviderBindings: store, PrincipalAuthorizer: authorizer})

	t.Run("public denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/provider/lifecycle", strings.NewReader(`{"metadata":{"request_id":"r1","operation_id":"o1","schema_version":"schema-v1"},"memory_id":"mem-1","action":"suppress","reason":"privacy"}`))
		req.Header.Set("X-API-Key", "public-key")
		req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
		req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
		req.Header.Set(auth.HeaderTenant, scope.Tenant)
		req.Header.Set(auth.HeaderProject, scope.Project)
		req.Header.Set(auth.HeaderNamespace, scope.Namespace)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s, want 403", resp.Code, resp.Body.String())
		}
		if lifecycle.applied != 0 {
			t.Fatalf("lifecycle applied=%d, want 0", lifecycle.applied)
		}
	})

	adminBinding := binding
	adminBinding.BindingID = "rb_admin"
	adminBinding.PrincipalID = admin.ID
	if err := store.Create(context.Background(), adminBinding); err != nil {
		t.Fatal(err)
	}
	t.Run("admin applies governed action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/provider/lifecycle", strings.NewReader(`{"metadata":{"request_id":"r2","operation_id":"o2","schema_version":"schema-v1"},"memory_id":"mem-1","action":"suppress","reason":"privacy"}`))
		req.Header.Set("X-API-Key", "admin-key")
		req.Header.Set(provider.HeaderRuntimeBinding, adminBinding.BindingID)
		req.Header.Set(provider.HeaderRuntimeSession, adminBinding.SessionID)
		req.Header.Set(auth.HeaderTenant, scope.Tenant)
		req.Header.Set(auth.HeaderProject, scope.Project)
		req.Header.Set(auth.HeaderNamespace, scope.Namespace)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s, want 200", resp.Code, resp.Body.String())
		}
		if lifecycle.applied != 1 {
			t.Fatalf("lifecycle applied=%d, want 1", lifecycle.applied)
		}
	})
}

func TestProviderStatusRouteIsScopedAndRedactsPrincipal(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	principal := auth.Principal{ID: "public-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"public-key": principal}}
	store := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb_status", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ConversationID: "conversation-a", ProviderInstanceID: "pi_status", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"schema-v1"}, ProviderAdapter: provider.NewAdapter(provider.AdapterDependencies{}), ProviderBindings: store, PrincipalAuthorizer: authorizer})
	req := httptest.NewRequest(http.MethodGet, "/v1/provider/status", nil)
	req.Header.Set("X-API-Key", "public-key")
	req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	req.Header.Set(auth.HeaderTenant, scope.Tenant)
	req.Header.Set(auth.HeaderProject, scope.Project)
	req.Header.Set(auth.HeaderNamespace, scope.Namespace)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["scope"] == nil || payload["session_id"] != binding.SessionID {
		t.Fatalf("status payload=%v", payload)
	}
	if _, leaked := payload["principal_id"]; leaked {
		t.Fatalf("status payload leaks principal id: %v", payload)
	}
}

func TestOpenAPIDocumentsProviderLifecycleAndStatusRoutes(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{})
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.Code)
	}
	for _, route := range []string{"/v1/provider/lifecycle:", "/v1/provider/status:"} {
		if !strings.Contains(resp.Body.String(), route) {
			t.Fatalf("OpenAPI missing %s", route)
		}
	}
}
