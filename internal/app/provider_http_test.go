package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

func TestProviderRoutesRemainAbsentWhenDisabled(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestProviderRuntimeInitializationReturnsServerOwnedBinding(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	principal := auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "runtime", CreatedAt: time.Now()}
	authorizer := stubPrincipalAuthorizer{principal: principal, granted: true}
	bindings := provider.NewMemoryBindingStore()
	initializer := provider.NewRuntimeInitializer(provider.RuntimeInitializerOptions{Authorizer: authorizer, Bindings: bindings, Now: func() time.Time { return time.Unix(100, 0) }})
	handler := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderInitializer: initializer, PrincipalAuthorizer: authorizer, ProviderBindings: bindings})
	req := httptest.NewRequest(http.MethodPost, "/v1/provider/runtimes", strings.NewReader(`{"agent_id":"agent-1","session_id":"session-1","conversation_id":"conversation-1"}`))
	req.Header.Set(auth.HeaderAPIKey, "credential")
	req.Header.Set(auth.HeaderTenant, scope.Tenant)
	req.Header.Set(auth.HeaderProject, scope.Project)
	req.Header.Set(auth.HeaderNamespace, scope.Namespace)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var binding provider.RuntimeBinding
	if err := json.Unmarshal(recorder.Body.Bytes(), &binding); err != nil {
		t.Fatal(err)
	}
	if binding.BindingID == "" || binding.Scope != scope || binding.AgentID != "agent-1" || binding.SessionID != "session-1" {
		t.Fatalf("binding = %+v", binding)
	}
}

func TestProviderOperationRejectsUnsupportedSchemaBeforeDispatch(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	principal := auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "runtime", CreatedAt: time.Now()}
	authorizer := stubPrincipalAuthorizer{principal: principal, granted: true}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "binding-1", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-1", SessionID: "session-1", ProviderInstanceID: "provider-1", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, PrincipalAuthorizer: authorizer, ProviderBindings: bindings, ProviderOperations: provider.NewOperationService(provider.OperationServiceOptions{})})
	req := httptest.NewRequest(http.MethodPost, "/v1/provider/operations/status.read", strings.NewReader(`{"metadata":{"request_id":"request-1","operation_id":"operation-1","event_seq":1,"schema_version":"provider-v999"},"input":{}}`))
	req.Header.Set(auth.HeaderAPIKey, "credential")
	req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	req.Header.Set(auth.HeaderTenant, scope.Tenant)
	req.Header.Set(auth.HeaderProject, scope.Project)
	req.Header.Set(auth.HeaderNamespace, scope.Namespace)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "compatibility") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
