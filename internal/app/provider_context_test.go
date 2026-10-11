package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

type contextContractAssembler struct {
	input  retrieval.AssembleContextInput
	calls  int
	result retrieval.AssembledContext
	err    error
}

func (a *contextContractAssembler) AssembleContext(_ context.Context, in retrieval.AssembleContextInput) (retrieval.AssembledContext, error) {
	a.input = in
	a.calls++
	if a.err != nil {
		return retrieval.AssembledContext{}, a.err
	}
	if in.FinalAdmission == nil {
		return a.result, nil
	}
	out := retrieval.AssembledContext{}
	for _, section := range []struct {
		name   string
		hits   []retrieval.SearchHit
		target *[]retrieval.SearchHit
	}{{"profile", a.result.Profile, &out.Profile}, {"relevant_summaries", a.result.RelevantSummaries, &out.RelevantSummaries}, {"related_entities", a.result.RelatedEntities, &out.RelatedEntities}, {"recent_session", a.result.RecentSession, &out.RecentSession}, {"recent_episodes", a.result.RecentEpisodes, &out.RecentEpisodes}} {
		for _, hit := range section.hits {
			if in.FinalAdmission.AdmitContextItem(section.name, hit) {
				*section.target = append(*section.target, hit)
			}
		}
	}
	return out, nil
}

func TestProviderContextAssemblerDependencyFailureIsRetryable503(t *testing.T) {
	a := &contextContractAssembler{err: fmt.Errorf("retrieval dependency unavailable")}
	w := contextContractHandler(t, a)(`{` + contextMetadata + `,"input":{"query":"q","budget":1}}`)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"category":"dependency"`) || !strings.Contains(w.Body.String(), `"retryable":true`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "retrieval dependency unavailable") || strings.Contains(w.Body.String(), `"result"`) {
		t.Fatalf("dependency detail leaked: %s", w.Body.String())
	}
}

func TestProviderContextInvalidMetadataPrecedesBudgetPreflight(t *testing.T) {
	for _, field := range []string{`"request_id":""`, `"operation_id":"invalid token"`, `"event_seq":-1`, `"idempotency_key":"invalid token"`, `"memory_path":"../secret"`} {
		for _, tiny := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tiny=%v", field, tiny), func(t *testing.T) {
				metadata := map[string]any{"request_id": "r", "operation_id": "o", "schema_version": "provider-v1"}
				var invalid map[string]any
				_ = json.Unmarshal([]byte("{"+field+"}"), &invalid)
				for key, value := range invalid {
					metadata[key] = value
				}
				input := map[string]any{"query": "q", "budget": 1}
				if tiny {
					input["budget_limits"] = map[string]int{"max_response_bytes": 1}
				}
				body, _ := json.Marshal(map[string]any{"metadata": metadata, "input": input})
				a := &contextContractAssembler{}
				w := contextContractHandler(t, a)(string(body))
				if w.Code != http.StatusBadRequest || a.calls != 0 || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
					t.Fatalf("status=%d calls=%d body=%s", w.Code, a.calls, w.Body.String())
				}
			})
		}
	}
}

func contextContractHandler(t *testing.T, a *contextContractAssembler, overrides ...http.Header) func(string) *httptest.ResponseRecorder {
	return contextContractHandlerWithDependencies(t, a, nil, overrides...)
}

func contextContractHandlerWithDependencies(t *testing.T, a *contextContractAssembler, configure func(*HTTPDependencies), overrides ...http.Header) func(string) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	p := auth.Principal{ID: "principal-context", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, CreatedAt: now}
	b := provider.RuntimeBinding{BindingID: "binding-context", PrincipalID: p.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	store := provider.NewMemoryBindingStore()
	if err := store.Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	deps := HTTPDependencies{ProviderEnabled: true, ProviderSchemaVersions: []string{"provider-v1"}, ProviderAdapter: provider.NewAdapter(provider.AdapterDependencies{Assembler: a}), ProviderBindings: store, PrincipalAuthorizer: providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": p}}}
	if configure != nil {
		configure(&deps)
	}
	h := NewHTTPHandler(deps)
	return func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/v1/provider/context", strings.NewReader(body))
		r.Header.Set("X-API-Key", "context-key")
		r.Header.Set(provider.HeaderRuntimeBinding, b.BindingID)
		r.Header.Set(provider.HeaderRuntimeSession, b.SessionID)
		r.Header.Set(auth.HeaderTenant, scope.Tenant)
		r.Header.Set(auth.HeaderProject, scope.Project)
		r.Header.Set(auth.HeaderNamespace, scope.Namespace)
		for _, headers := range overrides {
			for key, values := range headers {
				r.Header.Del(key)
				for _, value := range values {
					r.Header.Add(key, value)
				}
			}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
}

func TestProviderCapabilityDigestMatchesServedOpenAPI(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCapabilities: provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "provider-v1"})})
	spec := httptest.NewRecorder()
	h.ServeHTTP(spec, httptest.NewRequest("GET", "/openapi.yaml", nil))
	caps := httptest.NewRecorder()
	h.ServeHTTP(caps, httptest.NewRequest("GET", "/v1/provider/capabilities", nil))
	var document provider.CapabilityDocument
	if err := json.Unmarshal(caps.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if spec.Code != 200 || caps.Code != 200 || document.SchemaDigest != fmt.Sprintf("sha256:%x", sha256.Sum256(spec.Body.Bytes())) {
		t.Fatalf("capability digest does not describe served OpenAPI: %q", document.SchemaDigest)
	}
}

func TestProviderContextDeniedBindingDoesNotDispatch(t *testing.T) {
	for _, headers := range []http.Header{
		{"X-API-Key": {"invalid"}},
		{auth.HeaderTenant: {"foreign"}},
		{provider.HeaderRuntimeSession: {"foreign"}},
		{provider.HeaderRuntimeBinding: {"foreign"}},
	} {
		a := &contextContractAssembler{}
		w := contextContractHandler(t, a, headers)(`{` + contextMetadata + `,"input":{"query":"q","budget":1}}`)
		if (w.Code != 401 && w.Code != 403) || a.calls != 0 {
			t.Fatalf("denied request status=%d calls=%d", w.Code, a.calls)
		}
	}
}

func TestProviderContextNonemptyHTTPResponseConforms(t *testing.T) {
	now := time.Now().UTC()
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "memory-context", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, MemoryPath: "tasks/a", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "selected context", SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}, CreatedAt: now, ModifiedAt: now}, Score: retrieval.ScoreBreakdown{Overall: 0.987654}, Citations: []retrieval.Citation{{MemoryID: "memory-context", RawEventID: "event-context", Operation: "promote"}}}
	// The assembler aggregates citations before the item budget is applied.
	// Its candidate-only evidence must not escape through the public projection.
	candidateCitations := append(append([]retrieval.Citation{}, hit.Citations...), retrieval.Citation{MemoryID: "omitted-memory", RawEventID: "omitted-event", Operation: "promote"})
	a := &contextContractAssembler{result: retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{hit}, Citations: candidateCitations}}
	w := contextContractHandler(t, a)(`{` + contextMetadata + `,"input":{"query":"q","budget":1,"path":"tasks/a"}}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), "score") || strings.Contains(w.Body.String(), "0.987654") {
		t.Fatalf("unsafe nonempty response status=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "omitted-memory") || strings.Contains(w.Body.String(), "omitted-event") {
		t.Fatal("budget-omitted candidate evidence escaped public context")
	}
	doc, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if err = doc.Components.Schemas["ProviderContextResponse"].Value.VisitJSON(value); err != nil {
		t.Fatal(err)
	}
}

const contextMetadata = `"metadata":{"request_id":"req-context","operation_id":"op-context","schema_version":"provider-v1"}`

func TestProviderContextCanonicalContract(t *testing.T) {
	a := &contextContractAssembler{}
	request := contextContractHandler(t, a)
	body := `{` + contextMetadata + `,"input":{"query":"query","budget":7,"path_prefix":"/projects/demo/","include_relations":true,"include_experience_insights":true,"include_goal_context":true,"include_diagnostics":true,"include_feedback_diagnostics":true,"feedback_aware_ranking":true}}`
	w := request(body)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	in := a.input
	if a.calls != 1 || in.Budget != 7 || in.Query != "query" || in.PathPrefix != "projects/demo" || in.SessionID != "session-a" || in.Scope.Tenant != "tenant-a" || in.UserID != "" || !in.IncludeRelations || in.IncludeExperienceInsights || in.IncludeGoalContext || in.IncludeDiagnostics || in.IncludeFeedbackDiagnostics || !in.DisclosureRequired || !in.FeedbackAwareRanking {
		t.Fatalf("mapped input=%+v calls=%d", in, a.calls)
	}
	var value map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	result := value["result"].(map[string]any)
	for _, section := range []string{"profile", "recent_session", "recent_episodes", "relevant_summaries", "related_entities", "citations"} {
		if v, ok := result[section].([]any); !ok || len(v) != 0 {
			t.Fatalf("%s must be []: %s", section, w.Body.String())
		}
	}
	doc, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	schema := doc.Paths.Value("/v1/provider/context").Post.Responses.Value("200").Value.Content.Get("application/json").Schema.Value
	if err := schema.VisitJSON(value); err != nil {
		t.Fatalf("public response schema: %v", err)
	}
}

func TestProviderContextRejectsNoncanonicalRequestsBeforeAssembly(t *testing.T) {
	cases := []string{`null`, `{}`, `{"metadata":null,"input":{}}`, `{` + contextMetadata + `,"input":null}`, `{` + contextMetadata + `,"input":[]}`, `{` + contextMetadata + `,"input":{"query":"query"}}`, `{` + contextMetadata + `,"input":{"query":" ","budget":1}}`, `{` + contextMetadata + `,"input":{"query":"query","budget":0}}`, `{` + contextMetadata + `,"input":{"query":"query","budget":1,"path":"a","path_prefix":"b"}}`, `{` + contextMetadata + `,"input":{"query":"query","budget":1}} {}`}
	for _, field := range []string{`"Query":"q"`, `"Budget":2`, `"PathPrefix":"a"`, `"IncludeRelations":true`, `"scope":{}`, `"session_id":"other"`, `"user_id":"other"`, `"role":"compiler"`, `"reference_only":true`, `"character_budget":1`, `"use_projections":true`, `"feedback_ranking_policy":"global"`, `"include_relations":null`, `"budget":null`} {
		cases = append(cases, `{`+contextMetadata+`,"input":{"query":"query","budget":1,`+field+`}}`)
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			a := &contextContractAssembler{}
			w := contextContractHandler(t, a)(body)
			if w.Code != 400 || a.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, a.calls, w.Body.String())
			}
		})
	}
}

func TestProviderContextUnsupportedSchemaDoesNotDispatch(t *testing.T) {
	a := &contextContractAssembler{}
	w := contextContractHandler(t, a)(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"future"},"input":{"query":"q","budget":1}}`)
	if w.Code != 400 || a.calls != 0 || !strings.Contains(w.Body.String(), `"category":"compatibility"`) {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, a.calls, w.Body.String())
	}
}
