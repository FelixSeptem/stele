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
	principal := auth.Principal{ID: "public-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
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
	principal := auth.Principal{ID: "public-r", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive, Label: "public", CreatedAt: now}
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

func TestProviderRuntimeInitializationUsesExactScopeForDisclosure(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-runtime", Project: "project-runtime", Namespace: "namespace-runtime"}
	principal := auth.Principal{ID: "public-runtime", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "runtime", CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"runtime-key": principal}}
	bindings := provider.NewMemoryBindingStore()
	initializer := provider.NewRuntimeInitializer(provider.RuntimeInitializerOptions{Authorizer: authorizer, Bindings: bindings, BindingTTL: time.Hour, Now: func() time.Time { return now }})
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderInitializer: initializer, ProviderBindings: bindings, PrincipalAuthorizer: authorizer})
	req := httptest.NewRequest(http.MethodPost, "/v1/provider/runtime", strings.NewReader(`{"scope":{"tenant":"tenant-runtime","project":"project-runtime","namespace":"namespace-runtime"},"agent_id":"agent","session_id":"session"}`))
	req.Header.Set("X-API-Key", "runtime-key")
	req.Header.Set(auth.HeaderTenant, scope.Tenant)
	req.Header.Set(auth.HeaderProject, scope.Project)
	req.Header.Set(auth.HeaderNamespace, scope.Namespace)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("runtime initialization status=%d body=%s, want 201", resp.Code, resp.Body.String())
	}
	var binding provider.RuntimeBinding
	if err := json.Unmarshal(resp.Body.Bytes(), &binding); err != nil {
		t.Fatal(err)
	}
	if binding.Scope != scope {
		t.Fatalf("binding scope=%+v, want %+v", binding.Scope, scope)
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
	searcher := &stubMemorySearcher{result: retrieval.SearchResult{Hits: []retrieval.SearchHit{{
		Memory:    memory.CanonicalMemory{ID: "mem-retrieve", Scope: scope, MemoryPath: "compilations/run-1", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "private-retrieve-content", SourceVersion: 3, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}},
		Score:     retrieval.ScoreBreakdown{Overall: 0.98, Lexical: 0.8, Semantic: 0.9},
		Citations: []retrieval.Citation{{MemoryID: "mem-retrieve"}, {MemoryID: "hidden-citation-identity-marker"}},
	}, {
		Memory: memory.CanonicalMemory{ID: "hidden-high-marker", Scope: scope, MemoryPath: "private/high", Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "high-class-marker", SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationHigh}},
	}, {
		Memory: memory.CanonicalMemory{ID: "hidden-foreign-marker", Scope: memory.Scope{Tenant: "foreign", Project: scope.Project, Namespace: scope.Namespace}, MemoryPath: "foreign/path", Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "foreign-content-marker", SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}},
	}}}}
	adapter := provider.NewAdapter(provider.AdapterDependencies{Searcher: searcher})
	readObserver := &stubProviderReadObserver{}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderAdapter: adapter, ProviderBindings: bindings, PrincipalAuthorizer: referenceHTTPAuthorizer{authorizer}, ProviderReadObserver: readObserver})
	request := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-1","operation_id":"op-1","schema_version":"provider-v1"},"input":{"query":"smoke query","path":"compilations/run-1","top_k":7}}`))
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
	if len(readObserver.memoryIDs) != 1 || readObserver.memoryIDs[0] != "mem-retrieve" || readObserver.scope != scope || readObserver.observedAt.IsZero() {
		t.Fatalf("public read observation=%+v ids=%v, want only the disclosed same-scope match", readObserver, readObserver.memoryIDs)
	}
	if searcher.gotInput.Query != "smoke query" || searcher.gotInput.TopK != 7 {
		t.Fatalf("search input=%+v, want query and top_k from OpenAPI request", searcher.gotInput)
	}
	if searcher.gotInput.Path != "compilations/run-1" || !searcher.gotInput.DisclosureRequired {
		t.Fatalf("search input=%+v, want exact path under disclosure filtering", searcher.gotInput)
	}
	for _, marker := range []string{"private-retrieve-content", "hidden-citation-identity-marker", "hidden-high-marker", "high-class-marker", "hidden-foreign-marker", "foreign-content-marker", `"score"`, `"diagnostics"`} {
		if strings.Contains(response.Body.String(), marker) {
			t.Fatalf("reference-only retrieval leaked content or internals: %s", response.Body.String())
		}
	}
	if !strings.Contains(response.Body.String(), `"memory_path":"compilations/run-1"`) || !strings.Contains(response.Body.String(), `"reference":"mem-retrieve"`) || !strings.Contains(response.Body.String(), `"disclosure_profile":"memory-reference-v1"`) || !strings.Contains(response.Body.String(), `"response_mode":"reference_only"`) {
		t.Fatalf("reference projection/citation missing: %s", response.Body.String())
	}
	feedbackOverride := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-feedback","operation_id":"op-feedback","schema_version":"provider-v1"},"input":{"query":"smoke query","top_k":7,"feedback_aware_ranking":true}}`))
	feedbackOverride.Header = request.Header.Clone()
	priorQuery := searcher.gotInput.Query
	feedbackResponse := httptest.NewRecorder()
	h.ServeHTTP(feedbackResponse, feedbackOverride)
	if feedbackResponse.Code != http.StatusBadRequest || searcher.gotInput.Query != priorQuery || strings.Contains(feedbackResponse.Body.String(), "feedback_aware_ranking") {
		t.Fatalf("undocumented ranking override was accepted or echoed: %d %s", feedbackResponse.Code, feedbackResponse.Body.String())
	}
	contentRequest := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-2","operation_id":"op-2","schema_version":"provider-v1"},"input":{"query":"smoke query","path":"compilations/run-1","top_k":7,"disclosure_profile":"memory-content-v1","response_mode":"bounded_content"}}`))
	contentRequest.Header = request.Header.Clone()
	contentResponse := httptest.NewRecorder()
	h.ServeHTTP(contentResponse, contentRequest)
	if contentResponse.Code != http.StatusOK || !strings.Contains(contentResponse.Body.String(), "private-retrieve-content") || strings.Contains(contentResponse.Body.String(), `"score"`) || strings.Contains(contentResponse.Body.String(), "hidden-citation-identity-marker") {
		t.Fatalf("bounded content projection failed or exposed internals: %d %s", contentResponse.Code, contentResponse.Body.String())
	}
	limits := provider.Discover(provider.CapabilityInput{}).Limits
	limits.MaxContextBytes, limits.MaxContextContentBytes = 256, 256
	boundedHandler := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderAdapter: adapter, ProviderBindings: bindings, PrincipalAuthorizer: referenceHTTPAuthorizer{authorizer}, ProviderLimits: limits})
	largeResponseRequest := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-3","operation_id":"op-3","schema_version":"provider-v1"},"input":{"query":"smoke query","top_k":7,"disclosure_profile":"memory-content-v1","response_mode":"bounded_content"}}`))
	largeResponseRequest.Header = request.Header.Clone()
	largeResponse := httptest.NewRecorder()
	boundedHandler.ServeHTTP(largeResponse, largeResponseRequest)
	if largeResponse.Code != http.StatusRequestEntityTooLarge || !strings.Contains(largeResponse.Body.String(), `"code":"provider_response_limit_exceeded"`) || strings.Contains(largeResponse.Body.String(), `"result"`) {
		t.Fatalf("oversized Provider retrieval was not refused whole: %d %s", largeResponse.Code, largeResponse.Body.String())
	}
}

type stubProviderReadObserver struct {
	scope      memory.Scope
	memoryIDs  []string
	observedAt time.Time
	err        error
}

func (s *stubProviderReadObserver) RecordProviderMemoryRead(_ context.Context, scope memory.Scope, memoryIDs []string, observedAt time.Time) error {
	s.scope = scope
	s.memoryIDs = append([]string(nil), memoryIDs...)
	s.observedAt = observedAt
	return s.err
}

func TestProviderTurnsAreBoundToCurrentDisclosureAndRuntimeSession(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-turn", Project: "project-turn", Namespace: "namespace-turn"}
	principal := auth.Principal{ID: "public-turn", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"turn-key": principal}}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb-turn", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-bound", ProviderInstanceID: "pi-turn", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	session := &stubMemorySessionService{turn: memory.MemorySessionTurn{ID: "turn-created", SessionID: binding.SessionID, Scope: scope, Query: "private turn query", ContextEvidence: map[string]any{"summary": "private turn summary", "diagnostics": []any{"private diagnostic"}, "memory_ids": []any{"mem-visible"}, "citations": []any{"citation-visible"}}}}
	adapter := provider.NewAdapter(provider.AdapterDependencies{Session: session})
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderAdapter: adapter, ProviderBindings: bindings, PrincipalAuthorizer: referenceHTTPAuthorizer{authorizer}})
	request := httptest.NewRequest(http.MethodPost, "/v1/provider/turns", strings.NewReader(`{"metadata":{"request_id":"req-turn","operation_id":"op-turn","schema_version":"provider-v1"},"input":{"query":"private turn query","context_budget":5}}`))
	request.Header.Set("X-API-Key", "turn-key")
	request.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	request.Header.Set(auth.HeaderTenant, scope.Tenant)
	request.Header.Set(auth.HeaderProject, scope.Project)
	request.Header.Set(auth.HeaderNamespace, scope.Namespace)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("turn status=%d body=%s", response.Code, response.Body.String())
	}
	input := session.gotCreateTurnInput
	if input.SessionID != binding.SessionID || input.Scope != binding.Scope || input.ResponseMode != auth.ResponseModeReferenceOnly || input.DisclosureProfile != auth.DisclosureProfileReference || input.IncludeExperienceInsights || input.IncludeDiagnostics {
		t.Fatalf("turn input=%+v, want runtime-bound session and exact scope", input)
	}
	if strings.Contains(response.Body.String(), "private turn query") || strings.Contains(response.Body.String(), "private turn summary") || strings.Contains(response.Body.String(), "private diagnostic") || !strings.Contains(response.Body.String(), `"turn_id":"turn-created"`) || !strings.Contains(response.Body.String(), `"response_mode":"reference_only"`) {
		t.Fatalf("turn response did not use the closed reference projection: %s", response.Body.String())
	}
	contentRequest := httptest.NewRequest(http.MethodPost, "/v1/provider/turns", strings.NewReader(`{"metadata":{"request_id":"req-turn-content","operation_id":"op-turn-content","schema_version":"provider-v1"},"input":{"query":"private turn query","disclosure_profile":"memory-content-v1","response_mode":"bounded_content"}}`))
	contentRequest.Header = request.Header.Clone()
	contentResponse := httptest.NewRecorder()
	h.ServeHTTP(contentResponse, contentRequest)
	if contentResponse.Code != http.StatusCreated || !strings.Contains(contentResponse.Body.String(), "private turn summary") || strings.Contains(contentResponse.Body.String(), "private diagnostic") || !strings.Contains(contentResponse.Body.String(), `"response_mode":"bounded_content"`) {
		t.Fatalf("authorized bounded-content turn projection failed: %d %s", contentResponse.Code, contentResponse.Body.String())
	}

	wrongSession := httptest.NewRequest(http.MethodPost, "/v1/provider/turns", strings.NewReader(`{"metadata":{"request_id":"req-turn-2","operation_id":"op-turn-2","schema_version":"provider-v1"},"input":{"query":"private turn query"}}`))
	wrongSession.Header = request.Header.Clone()
	wrongSession.Header.Set(provider.HeaderRuntimeSession, "session-foreign")
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, wrongSession)
	if denied.Code != http.StatusForbidden || strings.Contains(denied.Body.String(), "session-foreign") {
		t.Fatalf("foreign session was not denied before turn creation: %d %s", denied.Code, denied.Body.String())
	}
	for _, override := range []string{`"session_id":"session-bound"`, `"turn_id":"caller-turn"`, `"role":"admin"`, `"include_diagnostics":true`} {
		invalid := httptest.NewRequest(http.MethodPost, "/v1/provider/turns", strings.NewReader(`{"metadata":{"request_id":"req-override","operation_id":"op-override","schema_version":"provider-v1"},"input":{"query":"q",`+override+`}}`))
		invalid.Header = request.Header.Clone()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, invalid)
		if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "caller-turn") || strings.Contains(w.Body.String(), "session-bound") {
			t.Fatalf("caller authority override was accepted or echoed: %s -> %d %s", override, w.Code, w.Body.String())
		}
	}
}

func TestProviderRetrieveRejectsBoundedContentWithoutCurrentGrant(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-retrieve-denied", Project: "project-retrieve-denied", Namespace: "namespace-retrieve-denied"}
	principal := auth.Principal{ID: "public-retrieve-denied", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, CreatedAt: now}
	authorizer := providerLifecycleAuthorizer{principals: map[string]auth.Principal{"retrieve-denied-key": principal}}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "rb-retrieve-denied", PrincipalID: principal.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-retrieve-denied", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	searcher := &stubMemorySearcher{}
	h := NewHTTPHandler(HTTPDependencies{
		ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"},
		ProviderAdapter: provider.NewAdapter(provider.AdapterDependencies{Searcher: searcher}), ProviderBindings: bindings,
		PrincipalAuthorizer: referenceOnlyProviderAuthorizer{authorizer},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/provider/retrieve", strings.NewReader(`{"metadata":{"request_id":"req-denied","operation_id":"op-denied","schema_version":"provider-v1"},"input":{"query":"private-marker","top_k":1,"disclosure_profile":"memory-content-v1","response_mode":"bounded_content"}}`))
	request.Header.Set("X-API-Key", "retrieve-denied-key")
	request.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	request.Header.Set(auth.HeaderTenant, scope.Tenant)
	request.Header.Set(auth.HeaderProject, scope.Project)
	request.Header.Set(auth.HeaderNamespace, scope.Namespace)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"disclosure_denied"`) || strings.Contains(response.Body.String(), "private-marker") || searcher.gotInput.Query != "" {
		t.Fatalf("unauthorized bounded content reached retrieval: %d %s input=%+v", response.Code, response.Body.String(), searcher.gotInput)
	}
}

type referenceOnlyProviderAuthorizer struct{ providerLifecycleAuthorizer }

func (referenceOnlyProviderAuthorizer) AuthorizeScopeDisclosure(context.Context, string, memory.Scope) (auth.DisclosureEntitlements, error) {
	return auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}, nil
}

func TestMemorySessionContextAdapterForwardsDisclosureRequirement(t *testing.T) {
	assembler := &stubContextAssembler{}
	adapter := memorySessionContextAdapter{assembler: assembler}
	_, err := adapter.AssembleSessionContext(context.Background(), memory.MemorySessionContextRequest{
		Scope:     memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		SessionID: "session-a", Query: "query", Budget: 3, DisclosureRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !assembler.gotInput.DisclosureRequired || assembler.gotInput.SessionID != "session-a" {
		t.Fatalf("assemble input=%+v, want disclosure and session binding", assembler.gotInput)
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

func (providerLifecycleAuthorizer) AuthorizeScopeDisclosure(context.Context, string, memory.Scope) (auth.DisclosureEntitlements, error) {
	return auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference, auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileReference}, nil
}
