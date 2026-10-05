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
		{name: "resync", err: errors.New("resync required: cursor expired"), want: provider.ErrorCategoryResyncRequired},
		{name: "scope", err: errors.New("synchronization scope mismatch"), want: provider.ErrorCategoryScope},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerErrorCategory(tc.err); got != tc.want {
				t.Fatalf("providerErrorCategory(%q)=%q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestProviderSynchronizationRouteSupportsInitialAndResume(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	principal := auth.Principal{ID: "public-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"public-key": principal}}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb_sync", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-sync", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	caps := provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})
	syncer := &provider.Synchronizer{Capabilities: caps.Synchronization, Source: &provider.MemorySyncSource{Snapshots: map[string]provider.SyncSnapshot{binding.BindingID: {SnapshotID: "snap-1", Watermark: "w1"}}, EventsByBinding: map[string][]provider.SyncEvent{binding.BindingID: {{Sequence: 1, ReplayID: "evt-1", Kind: provider.SyncEventRawEvent, SchemaVersion: provider.SyncContractVersion, SourceWatermark: "w2", Payload: json.RawMessage(`{"event_type":"message"}`), Available: true}}}}}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCapabilities: caps, ProviderSchemaVersions: []string{"schema-v1"}, ProviderSynchronizer: syncer, ProviderBindings: bindings, PrincipalAuthorizer: authorizer})
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/provider/sync", strings.NewReader(body))
		req.Header.Set("X-API-Key", "public-key")
		req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
		req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
		req.Header.Set(auth.HeaderTenant, scope.Tenant)
		req.Header.Set(auth.HeaderProject, scope.Project)
		req.Header.Set(auth.HeaderNamespace, scope.Namespace)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		return resp
	}
	initial := request(`{"schema_version":"sync-v1"}`)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial status=%d body=%s", initial.Code, initial.Body.String())
	}
	var first provider.SyncResponse
	if err := json.Unmarshal(initial.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.Snapshot == nil || first.NextCursor == "" {
		t.Fatalf("initial response=%+v", first)
	}
	resumed := request(`{"schema_version":"sync-v1","cursor":"` + first.NextCursor + `"}`)
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resumed.Code, resumed.Body.String())
	}
	var second provider.SyncResponse
	if err := json.Unmarshal(resumed.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if len(second.Events) != 1 || second.Events[0].ReplayID != "evt-1" {
		t.Fatalf("resume response=%+v", second)
	}
}

func TestProviderSynchronizationRouteReturnsBoundedResyncAndRejectsMutation(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-r", Project: "project-r", Namespace: "namespace-r"}
	principal := auth.Principal{ID: "public-r", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"public-key-r": principal}}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb-resync", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-r", SessionID: "session-r", ProviderInstanceID: "pi-r", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	caps := provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})
	source := &provider.MemorySyncSource{Snapshots: map[string]provider.SyncSnapshot{binding.BindingID: {SnapshotID: "snap-r", Watermark: "w"}}, RetentionFloors: map[string]int64{binding.BindingID: 2}}
	syncer := &provider.Synchronizer{Capabilities: caps.Synchronization, Source: source}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCapabilities: caps, ProviderSynchronizer: syncer, ProviderBindings: bindings, PrincipalAuthorizer: authorizer})
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/provider/sync", strings.NewReader(body))
		req.Header.Set("X-API-Key", "public-key-r")
		req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
		req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
		req.Header.Set(auth.HeaderTenant, scope.Tenant)
		req.Header.Set(auth.HeaderProject, scope.Project)
		req.Header.Set(auth.HeaderNamespace, scope.Namespace)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		return resp
	}
	initial := request(`{"schema_version":"sync-v1"}`)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial status=%d body=%s", initial.Code, initial.Body.String())
	}
	var first provider.SyncResponse
	if err := json.Unmarshal(initial.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	cursor, err := provider.DecodeSyncCursor(first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	cursor.Sequence = 0
	stale, err := provider.EncodeSyncCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	resync := request(`{"schema_version":"sync-v1","cursor":"` + stale + `"}`)
	if resync.Code != http.StatusConflict || !strings.Contains(resync.Body.String(), `"category":"resync_required"`) || strings.Contains(resync.Body.String(), scope.Tenant) {
		t.Fatalf("resync response status=%d body=%s", resync.Code, resync.Body.String())
	}
	mutation := request(`{"schema_version":"sync-v1","memory_id":"secret","action":"delete"}`)
	if mutation.Code != http.StatusBadRequest || strings.Contains(mutation.Body.String(), "secret") {
		t.Fatalf("mutation response status=%d body=%s", mutation.Code, mutation.Body.String())
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

func TestProviderRetrieveAcceptsOpenAPITopKField(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-retrieve", Project: "project-retrieve", Namespace: "namespace-retrieve"}
	principal := auth.Principal{ID: "public-retrieve", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"retrieve-key": principal}}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb-retrieve", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-retrieve", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	searcher := &stubMemorySearcher{}
	adapter := provider.NewAdapter(provider.AdapterDependencies{Searcher: searcher})
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderAdapter: adapter, ProviderBindings: bindings, PrincipalAuthorizer: authorizer})
	request := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-1","operation_id":"op-1","schema_version":"provider-v1"},"input":{"query":"smoke query","top_k":7}}`))
	request.Header.Set("X-API-Key", "retrieve-key")
	request.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	request.Header.Set(auth.HeaderTenant, scope.Tenant)
	request.Header.Set(auth.HeaderProject, scope.Project)
	request.Header.Set(auth.HeaderNamespace, scope.Namespace)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("retrieve status=%d body=%s; documented top_k must be accepted", response.Code, response.Body.String())
	}
	if searcher.gotInput.Query != "smoke query" || searcher.gotInput.TopK != 7 {
		t.Fatalf("search input=%+v, want query and top_k from OpenAPI request", searcher.gotInput)
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
	for _, route := range []string{"/v1/provider/lifecycle:", "/v1/provider/status:", "/v1/provider/turns:", "/v1/provider/turn-outcomes:", "/v1/provider/sync:"} {
		if !strings.Contains(resp.Body.String(), route) {
			t.Fatalf("OpenAPI missing %s", route)
		}
	}
}
