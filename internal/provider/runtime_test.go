package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

type runtimeAuthorizer struct {
	grant           bool
	authenticateErr error
	checks          int
}

func (a *runtimeAuthorizer) Authenticate(context.Context, string) (auth.Principal, auth.Credential, error) {
	if a.authenticateErr != nil {
		return auth.Principal{}, auth.Credential{}, a.authenticateErr
	}
	now := time.Now()
	return auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive, CreatedAt: now}, auth.Credential{ID: "credential-1", PrincipalID: "principal-1", Status: auth.CredentialStatusActive, CredentialID: "cred", Salt: []byte{1}, Digest: []byte{1}, CreatedAt: now}, nil
}
func (a *runtimeAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	a.checks++
	return a.grant, nil
}
func exactScope() memory.Scope {
	return memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
}

func TestRuntimeInitializerAuthenticatesBeforePersistence(t *testing.T) {
	a := &runtimeAuthorizer{grant: true, authenticateErr: errors.New("bad credential")}
	store := NewMemoryBindingStore()
	i := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	if _, err := i.InitializeAuthenticated(context.Background(), "bad", RuntimeInitialization{Scope: exactScope(), AgentID: "agent", SessionID: "session"}); err == nil {
		t.Fatal("expected unauthorized error")
	}
	if store.Count() != 0 || a.checks != 0 {
		t.Fatalf("store=%d checks=%d, want no repository or grant access", store.Count(), a.checks)
	}
	a.authenticateErr = nil
	if _, err := i.InitializeAuthenticated(context.Background(), "ok", RuntimeInitialization{Scope: exactScope(), AgentID: "", SessionID: "session"}); err == nil {
		t.Fatal("expected malformed request rejection")
	}
	if store.Count() != 0 || a.checks != 0 {
		t.Fatalf("store=%d checks=%d, malformed request must fail before lookup", store.Count(), a.checks)
	}
}

func TestRuntimeBindingPersistsSeparatedIdentities(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	i := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store, BindingTTL: time.Hour, Now: func() time.Time { return now }})
	b, err := i.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent", SessionID: "session", ConversationID: "conversation", ProviderInstanceID: "instance"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Lookup(context.Background(), b.BindingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentID != "agent" || got.SessionID != "session" || got.ConversationID != "conversation" || got.ProviderInstanceID != "instance" || got.Scope != exactScope() {
		t.Fatalf("binding identities = %+v", got)
	}
}

func TestRuntimeBindingMiddlewareRejectsMismatchesWithoutDisclosure(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	i := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	b, err := i.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent", SessionID: "session"})
	if err != nil {
		t.Fatal(err)
	}
	handler := RuntimeBindingMiddleware(store, a)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler called") }))
	tests := []struct{ name, binding, session, tenant string }{{"unknown", "rb_unknown", "session", "tenant-a"}, {"missing session", b.BindingID, "", "tenant-a"}, {"cross tenant", b.BindingID, "session", "tenant-b"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Header.Set(auth.HeaderAPIKey, "key")
			r.Header.Set(HeaderRuntimeBinding, tt.binding)
			r.Header.Set(HeaderRuntimeSession, tt.session)
			r.Header.Set(auth.HeaderTenant, tt.tenant)
			r.Header.Set(auth.HeaderProject, "project-a")
			r.Header.Set(auth.HeaderNamespace, "namespace-a")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden || w.Body.String() != "forbidden\n" {
				t.Fatalf("status/body = %d %q", w.Code, w.Body.String())
			}
		})
	}
}

func TestRuntimeBindingMiddlewareRevalidatesRevocation(t *testing.T) {
	a := &runtimeAuthorizer{grant: true}
	store := NewMemoryBindingStore()
	i := NewRuntimeInitializer(RuntimeInitializerOptions{Authorizer: a, Bindings: store})
	b, err := i.Initialize(context.Background(), auth.Principal{ID: "principal-1", Status: auth.PrincipalStatusActive}, RuntimeInitialization{Scope: exactScope(), AgentID: "agent", SessionID: "session"})
	if err != nil {
		t.Fatal(err)
	}
	a.grant = false
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderRuntimeBinding, b.BindingID)
	r.Header.Set(HeaderRuntimeSession, b.SessionID)
	w := httptest.NewRecorder()
	RuntimeBindingMiddleware(store, a)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler called") })).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
}
