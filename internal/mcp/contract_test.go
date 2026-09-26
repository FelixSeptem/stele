package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
	protocolmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPublishedMCPToolsAreRegisteredWithTheSDK(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter(AdapterOptions{Enabled: true})
	session := connectMCPContractSession(t, adapter, auth.Principal{ID: "principal", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}, "")
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	got := make(map[string]bool, len(listed.Tools))
	for _, tool := range listed.Tools {
		got[tool.Name] = true
	}
	for _, want := range ToolDescriptors() {
		if !got[want.Name] {
			t.Errorf("MCP server did not register published tool %q", want.Name)
		}
	}
}

func TestMCPForgetDelegatesToPrivilegedLifecycleAndReplaysDuplicate(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "binding-1", PrincipalID: principal.ID, Scope: scope, AgentID: "agent", SessionID: "session", ProviderInstanceID: "instance", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(ctx, binding); err != nil {
		t.Fatal(err)
	}
	authorizer := scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}
	processor := &contractLifecycleProcessor{}
	providerAdapter := provider.NewAdapter(provider.AdapterDependencies{
		Lifecycle: processor, LifecycleStore: newContractLifecycleStore(),
		AllowLifecycle: func(ctx context.Context, binding provider.RuntimeBinding) bool {
			caller, ok := auth.PrincipalFromContext(ctx)
			return ok && caller.Status == auth.PrincipalStatusActive && caller.Role == auth.PrincipalRoleAdmin && caller.ID == binding.PrincipalID
		},
	})
	adapter := NewAdapter(AdapterOptions{Enabled: true, Authorizer: authorizer, Bindings: bindings, LifecycleAdapter: providerAdapter, Now: func() time.Time { return now }})
	session := connectMCPContractSession(t, adapter, principal, binding.BindingID)
	params := &protocolmcp.CallToolParams{Name: ToolForget, Arguments: map[string]any{
		"memory_id": "memory-1", "action": "suppress", "reason": "user requested removal", "idempotency_key": "forget-1",
	}}
	for range 2 {
		result, err := session.CallTool(ctx, params)
		if err != nil {
			t.Fatalf("CallTool(memory_forget) error = %v", err)
		}
		if result.IsError {
			t.Fatalf("CallTool(memory_forget) returned error result: %+v", result)
		}
	}
	if processor.calls != 1 {
		t.Fatalf("lifecycle mutations = %d, want one durable mutation after equivalent retry", processor.calls)
	}
	if processor.last.Scope != scope || processor.last.MemoryID != "memory-1" || processor.last.Actor != principal.ID || processor.last.Action != policy.ForgettingActionSuppress || processor.last.Reason != "user requested removal" {
		t.Fatalf("lifecycle attribution = %+v, want exact scope, memory, action, reason, and authenticated actor", processor.last)
	}
}

func TestMCPSearchDelegatesExactScopeAndTemporalSelectorWithRedactedShape(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "reader-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	searcher := &contractSearcher{result: retrieval.SearchResult{Hits: []retrieval.SearchHit{{
		Memory:    memory.CanonicalMemory{ID: "memory-1", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "approved content"},
		Score:     retrieval.ScoreBreakdown{Overall: 0.99, Lexical: 0.7},
		Citations: []retrieval.Citation{{MemoryID: "memory-1", RawEventID: "event-1", Operation: "remember"}},
	}}, Temporal: &retrieval.TemporalSelection{Mode: "as_of"}}}
	authorizer := scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}
	adapter := NewAdapter(AdapterOptions{Enabled: true, Authorizer: authorizer, Searcher: searcher, Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10}})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolSearch, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "question", "limit": float64(3), "as_of": "2026-09-25T00:00:00Z",
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_search result=%+v error=%v", result, err)
	}
	if searcher.got.Scope != scope || searcher.got.Query != "question" || searcher.got.TemporalConstraint.Mode != memory.TemporalSelectionAsOf || searcher.got.TemporalConstraint.AsOf == nil {
		t.Fatalf("delegated search input = %+v, want exact scope, query, and explicit as_of", searcher.got)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	response := string(encoded)
	for _, forbidden := range []string{"score", "0.99", "0.7", "tenant", "project", "namespace", "query", "candidate"} {
		if strings.Contains(response, forbidden) {
			t.Errorf("MCP search response leaked %q: %s", forbidden, response)
		}
	}
	for _, required := range []string{"memory-1", "approved content", "event-1", "as_of"} {
		if !strings.Contains(response, required) {
			t.Errorf("MCP search response omitted %q citation-safe evidence: %s", required, response)
		}
	}
}

func TestMCPSearchUsesConfiguredDefaultLimitWhenOmitted(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "reader-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	hits := make([]retrieval.SearchHit, 0, 4)
	for i := 1; i <= 4; i++ {
		hits = append(hits, retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: fmt.Sprintf("memory-%d", i), State: memory.MemoryStateActive, Content: fmt.Sprintf("fact-%d", i)}})
	}
	searcher := &contractSearcher{result: retrieval.SearchResult{Hits: hits}}
	adapter := NewAdapter(AdapterOptions{
		Enabled: true, Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}, Searcher: searcher,
		Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 3, MaxIDs: 5},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolSearch, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "question",
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_search result=%+v error=%v", result, err)
	}
	if searcher.got.TopK != 3 {
		t.Fatalf("search TopK = %d, want configured default MaxResults 3", searcher.got.TopK)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var response SearchResponse
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Hits) != 3 {
		t.Fatalf("search response hits = %d, want configured default MaxResults 3", len(response.Hits))
	}
}

func TestMCPContextDelegatesExactBudgetAndKeepsProfileSectionsSeparate(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "reader-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	assembler := &contractAssembler{result: retrieval.AssembledContext{
		Profile:        []retrieval.SearchHit{{Memory: memory.CanonicalMemory{ID: "profile-1", Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "preferred language"}, Score: retrieval.ScoreBreakdown{Overall: .91}}},
		RecentEpisodes: []retrieval.SearchHit{{Memory: memory.CanonicalMemory{ID: "episode-1", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "recent event"}}},
		Citations:      []retrieval.Citation{{MemoryID: "profile-1", RawEventID: "event-1", Operation: "remember"}},
	}}
	adapter := NewAdapter(AdapterOptions{
		Enabled:    true,
		Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		Assembler:  assembler,
		Limits:     Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolContext, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "question", "budget_bytes": float64(256),
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_context result=%+v error=%v", result, err)
	}
	if assembler.got.Scope != scope || assembler.got.Query != "question" || assembler.got.Budget != 10 || assembler.got.CharacterBudget != 256 {
		t.Fatalf("delegated context input = %+v, want bounded item budget and exact character budget", assembler.got)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	response := string(encoded)
	for _, required := range []string{`"profile"`, `"recent_episodes"`, "preferred language", "recent event", "event-1"} {
		if !strings.Contains(response, required) {
			t.Errorf("MCP context response omitted section/evidence %q: %s", required, response)
		}
	}
	for _, forbidden := range []string{"0.91", "score", "tenant", "project", "namespace"} {
		if strings.Contains(response, forbidden) {
			t.Errorf("MCP context response leaked internal field %q: %s", forbidden, response)
		}
	}
}

func TestMCPBrowseUsesDefaultLimitAfterOffset(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "reader-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	query := &contractMemoryQuery{page: memory.MemoryPage{Items: []memory.MemoryResource{
		{ID: "memory-1", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "first"},
		{ID: "memory-2", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "second"},
		{ID: "memory-3", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "third"},
	}}}
	adapter := NewAdapter(AdapterOptions{
		Enabled:     true,
		Authorizer:  scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		MemoryQuery: query,
		Limits:      Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 2, MaxIDs: 10},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolBrowse, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "offset": float64(1),
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_browse result=%+v error=%v", result, err)
	}
	if query.got.Scope != scope || query.got.Limit != 3 {
		t.Fatalf("browse input = %+v, want exact scope and offset + default limit fetch of 3", query.got)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	response := string(encoded)
	for _, required := range []string{"memory-2", "second", "memory-3", "third"} {
		if !strings.Contains(response, required) {
			t.Errorf("MCP browse response omitted default page item %q: %s", required, response)
		}
	}
	for _, forbidden := range []string{"memory-1", "first"} {
		if strings.Contains(response, forbidden) {
			t.Errorf("MCP browse response retained skipped item %q: %s", forbidden, response)
		}
	}
}

func TestMCPForgetPreviewBoundsPersistedCandidateIDs(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	hits := make([]retrieval.SearchHit, 0, 4)
	for i := 1; i <= 4; i++ {
		hits = append(hits, retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: fmt.Sprintf("memory-%d", i), State: memory.MemoryStateActive, Content: fmt.Sprintf("fact-%d", i)}})
	}
	searcher := &contractSearcher{result: retrieval.SearchResult{Hits: hits}}
	preview := &contractForgetPreviewStore{}
	adapter := NewAdapter(AdapterOptions{
		Enabled: true, Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		Searcher: searcher, PreviewStore: preview,
		Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 2},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolForgetPreview, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "facts", "limit": float64(10),
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_forget_preview result=%+v error=%v", result, err)
	}
	if len(preview.record.MemoryIDs) != 2 || len(preview.record.MemoryIDs) > 2 {
		t.Fatalf("persisted preview IDs = %v, want exactly the configured MaxIDs bound", preview.record.MemoryIDs)
	}
}

func TestMCPForgetPreviewFiltersCandidatesBelowSemanticThreshold(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	searcher := &contractSearcher{result: retrieval.SearchResult{Hits: []retrieval.SearchHit{
		{Memory: memory.CanonicalMemory{ID: "strong", State: memory.MemoryStateActive}, Score: retrieval.ScoreBreakdown{Semantic: 0.9}},
		{Memory: memory.CanonicalMemory{ID: "weak", State: memory.MemoryStateActive}, Score: retrieval.ScoreBreakdown{Semantic: 0.4}},
	}}}
	preview := &contractForgetPreviewStore{}
	adapter := NewAdapter(AdapterOptions{
		Enabled: true, Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		Searcher: searcher, PreviewStore: preview,
		Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolForgetPreview, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "facts", "threshold": 0.8,
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_forget_preview result=%+v error=%v", result, err)
	}
	if got := preview.record.MemoryIDs; len(got) != 1 || got[0] != "strong" {
		t.Fatalf("persisted preview IDs = %v, want only semantic matches at or above threshold", got)
	}
}

func TestMCPForgetPreviewUsesDefaultResultLimitAndBoundsManifest(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	hits := make([]retrieval.SearchHit, 0, 4)
	for i := 1; i <= 4; i++ {
		hits = append(hits, retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: fmt.Sprintf("memory-%d", i), State: memory.MemoryStateActive, Content: fmt.Sprintf("fact-%d", i)}})
	}
	searcher := &contractSearcher{result: retrieval.SearchResult{Hits: hits}}
	preview := &contractForgetPreviewStore{}
	adapter := NewAdapter(AdapterOptions{
		Enabled: true, Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		Searcher: searcher, PreviewStore: preview,
		Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 2, MaxIDs: 4},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolForgetPreview, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "facts",
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_forget_preview result=%+v error=%v", result, err)
	}
	if searcher.got.TopK != 2 {
		t.Fatalf("forget preview TopK = %d, want configured default MaxResults 2", searcher.got.TopK)
	}
	if len(preview.record.MemoryIDs) > 2 {
		t.Fatalf("persisted preview IDs = %v, want at most configured default MaxResults 2", preview.record.MemoryIDs)
	}
}

func TestMCPRememberUsesRequestRuntimeBindingForActiveScope(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	principal := auth.Principal{ID: "writer-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	bindings := provider.NewMemoryBindingStore()
	if err := bindings.Create(ctx, provider.RuntimeBinding{BindingID: "binding-1", PrincipalID: principal.ID, Scope: scope, AgentID: "agent", SessionID: "session", ProviderInstanceID: "instance", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	intent := &contractIntentService{record: memory.MemoryIntentRecord{ID: "intent-1", Status: memory.MemoryIntentStatusAccepted}}
	adapter := NewAdapter(AdapterOptions{
		Enabled: true, Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}, Bindings: bindings, Intent: intent,
		Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10}, Now: func() time.Time { return now },
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolRemember, Arguments: map[string]any{
		"runtime_binding_id": "binding-1", "content": "active scope fact", "reason": "agent request", "idempotency_key": "remember-active-1",
	}})
	if err != nil || result.IsError || intent.got.Scope != scope {
		t.Fatalf("active-scope memory_remember result=%+v error=%v intent=%+v", result, err, intent.got)
	}
}

func TestMCPRememberUsesGovernedIntentBoundaryAndMapsConflict(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "writer-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	authorizer := scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}
	intent := &contractIntentService{record: memory.MemoryIntentRecord{ID: "intent-1", Status: memory.MemoryIntentStatusAccepted}}
	adapter := NewAdapter(AdapterOptions{Enabled: true, Authorizer: authorizer, Intent: intent, Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10}})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolRemember, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "content": "durable governed fact", "reason": "agent request", "idempotency_key": "remember-1",
	}})
	if err != nil || result.IsError {
		t.Fatalf("memory_remember result=%+v error=%v", result, err)
	}
	if intent.got.Scope != scope || intent.got.Actor != principal.ID || intent.got.IdempotencyKey != "remember-1" || intent.got.Type != memory.MemoryIntentRemember || intent.got.Provenance["transport"] != "mcp" {
		t.Fatalf("governed intent input = %+v, want exact scope/actor/key/type/provenance", intent.got)
	}
	intent.err = memory.ErrIdempotencyConflict
	result, err = session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolRemember, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "content": "conflicting fact", "reason": "agent request", "idempotency_key": "remember-1",
	}})
	content, _ := json.Marshal(result.Content)
	if err != nil || !result.IsError || !strings.Contains(string(content), "idempotency_conflict") {
		t.Fatalf("conflicting memory_remember result=%+v error=%v, want bounded conflict", result, err)
	}
}

func TestMCPRememberRejectsReadOnlyScopeGrant(t *testing.T) {
	ctx := context.Background()
	principal := auth.Principal{ID: "reader-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	intent := &contractIntentService{record: memory.MemoryIntentRecord{ID: "intent-1", Status: memory.MemoryIntentStatusAccepted}}
	adapter := NewAdapter(AdapterOptions{
		Enabled:    true,
		Authorizer: scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}, readOnly: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		Intent:     intent, Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10},
	})
	session := connectMCPContractSession(t, adapter, principal, "")
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: ToolRemember, Arguments: map[string]any{
		"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "content": "must not write", "reason": "read-only test", "idempotency_key": "read-only-1",
	}})
	if err != nil || !result.IsError || intent.got.Content != "" {
		t.Fatalf("read-only memory_remember result=%+v error=%v intent=%+v, want bounded denial before submit", result, err, intent.got)
	}
}

func connectMCPContractSession(t *testing.T, adapter *Adapter, principal auth.Principal, bindingID string) *protocolmcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	request := httptest.NewRequest("POST", "/mcp", nil).WithContext(context.WithValue(ctx, requestStateKey{}, requestState{principal: principal, bindingID: bindingID}))
	server := adapter.serverForRequest(request)
	client := protocolmcp.NewClient(&protocolmcp.Implementation{Name: "contract-test", Version: "v1"}, nil)
	clientTransport, serverTransport := protocolmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect test MCP server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect test MCP client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type contractSearcher struct {
	got    retrieval.SearchInput
	result retrieval.SearchResult
}

func (s *contractSearcher) Search(_ context.Context, input retrieval.SearchInput) (retrieval.SearchResult, error) {
	s.got = input
	return s.result, nil
}

type contractAssembler struct {
	got    retrieval.AssembleContextInput
	result retrieval.AssembledContext
}

type contractIntentService struct {
	got    memory.MemoryIntentInput
	record memory.MemoryIntentRecord
	err    error
}

type contractForgetPreviewStore struct{ record ForgetPreviewRecord }

func (s *contractForgetPreviewStore) SaveForgetPreview(_ context.Context, record ForgetPreviewRecord) error {
	s.record = record
	return nil
}

func (s *contractForgetPreviewStore) LoadForgetPreview(context.Context, string) (ForgetPreviewRecord, error) {
	return s.record, nil
}

type contractMemoryQuery struct {
	got  memory.ListMemoriesInput
	page memory.MemoryPage
}

func (q *contractMemoryQuery) ListMemories(_ context.Context, input memory.ListMemoriesInput) (memory.MemoryPage, error) {
	q.got = input
	page := q.page
	if input.Limit >= 0 && input.Limit < len(page.Items) {
		page.Items = append([]memory.MemoryResource(nil), page.Items[:input.Limit]...)
	}
	return page, nil
}

func (s *contractIntentService) Submit(_ context.Context, input memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	s.got = input
	return s.record, s.err
}

func (a *contractAssembler) AssembleContext(_ context.Context, input retrieval.AssembleContextInput) (retrieval.AssembledContext, error) {
	a.got = input
	return a.result, nil
}

type contractLifecycleProcessor struct {
	calls int
	last  memory.LifecycleActionInput
}

func (p *contractLifecycleProcessor) Apply(_ context.Context, input memory.LifecycleActionInput) error {
	p.calls++
	p.last = input
	return nil
}

type contractLifecycleStore struct {
	mu      sync.Mutex
	entries map[string]contractLifecycleEntry
}

type contractLifecycleEntry struct {
	fingerprint string
	completed   bool
	outcome     provider.OperationOutcome
}

func newContractLifecycleStore() *contractLifecycleStore {
	return &contractLifecycleStore{entries: make(map[string]contractLifecycleEntry)}
}

func (s *contractLifecycleStore) ClaimLifecycle(_ context.Context, claim provider.LifecycleClaim) (provider.LifecycleClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := claim.Scope.Tenant + "/" + claim.Scope.Project + "/" + claim.Scope.Namespace + "/" + claim.PrincipalID + "/" + claim.IdempotencyKey
	entry, exists := s.entries[key]
	if exists {
		if entry.fingerprint != claim.RequestFingerprint {
			return provider.LifecycleClaimResult{}, errors.New("idempotency conflict")
		}
		if entry.completed {
			return provider.LifecycleClaimResult{Disposition: provider.LifecycleReplayed, Outcome: entry.outcome}, nil
		}
		return provider.LifecycleClaimResult{Disposition: provider.LifecycleInProgress}, nil
	}
	s.entries[key] = contractLifecycleEntry{fingerprint: claim.RequestFingerprint}
	return provider.LifecycleClaimResult{Disposition: provider.LifecycleClaimed}, nil
}

func (s *contractLifecycleStore) CompleteLifecycle(_ context.Context, claim provider.LifecycleClaim, outcome provider.OperationOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := claim.Scope.Tenant + "/" + claim.Scope.Project + "/" + claim.Scope.Namespace + "/" + claim.PrincipalID + "/" + claim.IdempotencyKey
	entry, exists := s.entries[key]
	if !exists || entry.fingerprint != claim.RequestFingerprint {
		return errors.New("claim unavailable")
	}
	entry.completed, entry.outcome = true, outcome
	s.entries[key] = entry
	return nil
}

func (s *contractLifecycleStore) ReleaseLifecycle(_ context.Context, claim provider.LifecycleClaim) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := claim.Scope.Tenant + "/" + claim.Scope.Project + "/" + claim.Scope.Namespace + "/" + claim.PrincipalID + "/" + claim.IdempotencyKey
	entry, exists := s.entries[key]
	if !exists || entry.fingerprint != claim.RequestFingerprint || entry.completed {
		return nil
	}
	delete(s.entries, key)
	return nil
}
