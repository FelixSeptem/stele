package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

type runtimeAuthorizer struct {
	grant  bool
	checks int
}

func (a *runtimeAuthorizer) Authenticate(context.Context, string) (auth.Principal, auth.Credential, error) {
	return auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive, Role: auth.PrincipalRolePublic, Label: "runtime", CreatedAt: time.Now()}, auth.Credential{ID: "credential-1", PrincipalID: "principal-1", Status: auth.CredentialStatusActive, CredentialID: "runtime", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now()}, nil
}
func (a *runtimeAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	a.checks++
	return a.grant, nil
}

func exactScope() memory.Scope {
	return memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
}

func TestRuntimeInitializerResolvesExactScopeAndSeparatesIdentity(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store, BindingTTL: time.Hour, Now: func() time.Time { return time.Unix(10, 0).UTC() }})
	binding, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1", ConversationID: "conversation-1"})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if binding.BindingID == "" || binding.ProviderInstanceID == "" {
		t.Fatal("binding IDs must be opaque and non-empty")
	}
	if binding.Scope != exactScope() || binding.AgentID != "agent-1" || binding.SessionID != "session-1" || binding.ConversationID != "conversation-1" {
		t.Fatalf("binding = %+v", binding)
	}
	if a.checks != 1 {
		t.Fatalf("AuthorizeScope calls = %d, want 1", a.checks)
	}
}

func TestRuntimeInitializerRejectsInvalidOrUnauthorizedBeforeStore(t *testing.T) {
	a := &runtimeAuthorizer{grant: false}
	store := NewMemoryBindingStore()
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	if _, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "", SessionID: "session-1"}); err == nil {
		t.Fatal("missing agent id should fail")
	}
	if store.Count() != 0 {
		t.Fatal("invalid initialization must not persist binding")
	}
	if _, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1"}); err == nil {
		t.Fatal("unauthorized initialization should fail")
	}
	if store.Count() != 0 {
		t.Fatal("unauthorized initialization must not persist binding")
	}
}

func TestRuntimeBindingMiddlewareRejectsWidenedScopeAndRechecksGrant(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	binding, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/provider/op", nil)
	request.Header.Set(HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(HeaderRuntimeSession, "session-1")
	request.Header.Set(auth.HeaderTenant, "tenant-a")
	request.Header.Set(auth.HeaderProject, "project-a")
	request.Header.Set(auth.HeaderNamespace, "namespace-b")
	rec := httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(next).ServeHTTP(rec, request)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("widened scope status = %d, want 403", rec.Code)
	}
	if got, ok := RuntimeBindingFromContext(request.Context()); ok || got.BindingID != "" {
		t.Fatal("binding must not be injected on rejected request")
	}

	a.grant = false
	request.Header.Set(auth.HeaderNamespace, "namespace-a")
	rec = httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(next).ServeHTTP(rec, request)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("revoked grant status = %d, want 403", rec.Code)
	}
	if a.checks < 2 {
		t.Fatal("middleware must revalidate grant on every operation")
	}
}

func TestRuntimeBindingMiddlewareInjectsCanonicalScope(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	binding, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, ok := RuntimeBindingFromContext(r.Context())
		if !ok || got.Scope != exactScope() {
			t.Fatalf("canonical binding = %+v, ok=%v", got, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/provider/op", nil)
	request.Header.Set(HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(HeaderRuntimeSession, "session-1")
	rec := httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(next).ServeHTTP(rec, request)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

func TestRuntimeBindingMiddlewareRequiresExactSession(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	binding, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/provider/op", nil)
	request.Header.Set(HeaderRuntimeBinding, binding.BindingID)
	rec := httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler called") })).ServeHTTP(rec, request)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing session status = %d, want 403", rec.Code)
	}
}

func TestRuntimeBindingMiddlewareRejectsExpiredBinding(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	now := time.Now().UTC().Add(-2 * time.Hour)
	initializer := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store, BindingTTL: time.Hour, Now: func() time.Time { return now }})
	binding, err := initializer.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent-1", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/provider/op", nil)
	request.Header.Set(HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(HeaderRuntimeSession, binding.SessionID)
	rec := httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler called") })).ServeHTTP(rec, request)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expired binding status = %d, want 403", rec.Code)
	}
}
