package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"net/http"
	"net/http/httptest"
)

func TestProviderErrorCategoryMapsBoundedContractErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want provider.ErrorCategory
	}{
		{name: "conflict", err: memory.ErrIdempotencyConflict, want: provider.ErrorCategoryConflict},
		{name: "retryable", err: errors.New("lifecycle operation in progress"), want: provider.ErrorCategoryRetryable},
		{name: "dependency", err: errors.New("provider search service is not configured"), want: provider.ErrorCategoryDependency},
		{name: "stale", err: errors.New("stale event sequence"), want: provider.ErrorCategoryStale},
		{name: "lifecycle", err: errors.New("provider lifecycle operation requires privileged authorization"), want: provider.ErrorCategoryLifecycle},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerErrorCategory(tc.err); got != tc.want {
				t.Fatalf("providerErrorCategory(%q)=%q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

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

func TestProviderRetrievalPlannerRolloutPreservesOperationContracts(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	binding := provider.RuntimeBinding{BindingID: "rb-planner", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "provider-a", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	metadata := provider.OperationMetadata{RequestID: "request-1", OperationID: "operation-1", SchemaVersion: "schema-v1"}
	for _, stage := range []struct {
		name   string
		status memory.RankingRolloutPolicyStatus
		mode   memory.RankingRolloutMode
	}{
		{name: "diagnostics-only", status: memory.RankingRolloutPolicyStatusDiagnosticsOnly, mode: memory.RankingRolloutModeDiagnosticsOnly},
		{name: "shadow", status: memory.RankingRolloutPolicyStatusDryRun, mode: memory.RankingRolloutModeDryRun},
		{name: "active", status: memory.RankingRolloutPolicyStatusActiveForScope, mode: memory.RankingRolloutModeActiveForScope},
		{name: "disabled", status: memory.RankingRolloutPolicyStatusDisabled, mode: memory.RankingRolloutModeActiveForScope},
		{name: "rolled-back", status: memory.RankingRolloutPolicyStatusRolledBack, mode: memory.RankingRolloutModeActiveForScope},
	} {
		t.Run(stage.name, func(t *testing.T) {
			policy := plannerHTTPRollout(scope, stage.status, stage.mode)
			policy.RetrievalPlannerSelector = memory.RetrievalPlannerRolloutSelector{SessionID: binding.SessionID}
			lexical := &plannerHTTPRecordingLexical{}
			service := retrieval.NewService(retrieval.ServiceDependencies{Lexical: lexical, Citations: queryAnalysisHTTPCitations{}, RankingRolloutPolicyReader: queryAnalysisHTTPPolicyReader{policy: policy}, RetrievalPlanPolicy: plannerHTTPPlanPolicy(4)})
			adapter := provider.NewAdapter(provider.AdapterDependencies{Searcher: service, Assembler: service})
			searchStart := len(lexical.inputs)
			search, _, err := adapter.Search(context.Background(), binding, metadata, retrieval.SearchInput{Query: "private planner query", TopK: 10})
			if err != nil {
				t.Fatal(err)
			}
			if stage.name == "active" && !lexical.plannedSince(searchStart, 4) {
				t.Fatalf("matching provider search did not execute active plan: inputs=%+v", lexical.inputs[searchStart:])
			}
			contextStart := len(lexical.inputs)
			assembled, _, err := adapter.AssembleContext(context.Background(), binding, metadata, retrieval.AssembleContextInput{Query: "private planner query", Budget: 10})
			if err != nil {
				t.Fatal(err)
			}
			if stage.name == "active" && !lexical.plannedSince(contextStart, 4) {
				t.Fatalf("matching provider context did not execute active plan: inputs=%+v", lexical.inputs[contextStart:])
			}
			for _, result := range []any{search, assembled} {
				payload, marshalErr := json.Marshal(result)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				assertNoPlannerInternals(t, string(payload))
				if !strings.Contains(string(payload), `"id":"mem-original"`) {
					t.Fatalf("provider result lost public shape: %s", payload)
				}
			}
			if stage.name == "active" {
				foreign := binding
				foreign.SessionID = "session-foreign"
				foreignStart := len(lexical.inputs)
				if _, _, err := adapter.Search(context.Background(), foreign, metadata, retrieval.SearchInput{Query: "private planner query", TopK: 10, SessionID: binding.SessionID}); err != nil {
					t.Fatal(err)
				}
				if lexical.plannedSince(foreignStart, 4) || len(lexical.inputs) != foreignStart+1 || lexical.inputs[foreignStart].SessionID != foreign.SessionID || lexical.inputs[foreignStart].TopK != 10 {
					t.Fatalf("foreign provider selector affected retrieval: inputs=%+v", lexical.inputs[foreignStart:])
				}
			}
		})
	}
}

func TestProviderActivePlannerRerankerFallbackReasonsRemainPrivate(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	binding := provider.RuntimeBinding{BindingID: "rb-rerank", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "provider-a", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	metadata := provider.OperationMetadata{RequestID: "request-1", OperationID: "operation-1", SchemaVersion: "schema-v1"}
	for _, scenario := range []string{"ineligible", "insufficient_headroom", "latency_exhausted"} {
		t.Run(scenario, func(t *testing.T) {
			service := plannerRerankerHTTPService(scope, scenario, memory.RetrievalPlannerRolloutSelector{SessionID: binding.SessionID})
			adapter := provider.NewAdapter(provider.AdapterDependencies{Searcher: service, Assembler: service})
			search, _, err := adapter.Search(context.Background(), binding, metadata, retrieval.SearchInput{Query: "ordinary", TopK: 10, IncludeFeedbackDiagnostics: true})
			if err != nil {
				t.Fatal(err)
			}
			assembled, _, err := adapter.AssembleContext(context.Background(), binding, metadata, retrieval.AssembleContextInput{Query: "ordinary", Budget: 10, IncludeDiagnostics: true, IncludeFeedbackDiagnostics: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, result := range []any{search, assembled} {
				payload, marshalErr := json.Marshal(result)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				assertNoPlannerRerankerCause(t, string(payload))
				if !strings.Contains(string(payload), `"reason":"optional reranker not applied"`) {
					t.Fatalf("missing generic reranker fallback: %s", payload)
				}
			}
		})
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
		req := httptest.NewRequest(http.MethodPost, "/v1/provider/lifecycle", strings.NewReader(`{"metadata":{"request_id":"r2","operation_id":"o2","idempotency_key":"life-2","schema_version":"schema-v1"},"memory_id":"mem-1","action":"suppress","reason":"privacy"}`))
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
	for _, route := range []string{"/v1/provider/lifecycle:", "/v1/provider/status:", "/v1/provider/turns:", "/v1/provider/turn-outcomes:"} {
		if !strings.Contains(resp.Body.String(), route) {
			t.Fatalf("OpenAPI missing %s", route)
		}
	}
}
