package assurance

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
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func contextContractFixture() (memory.Scope, provider.DisclosedContextResponse) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	decision, _ := (auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}).Resolve("", "")
	meta := provider.OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "provider-v1"}
	a, _ := provider.NewContextAdmission(provider.ContextInput{Budget: 1}, provider.Discover(provider.CapabilityInput{}).Limits, scope, decision, meta)
	data, _ := a.Finalize(retrieval.AssembledContext{}, meta)
	var r provider.DisclosedContextResponse
	_ = json.Unmarshal(data, &r)
	r.Result.Disclosure.ResponseMode = r.Result.ResponseMode
	return scope, r
}

func TestContextConsumerRejectsMalformedEmptySubstitutes(t *testing.T) {
	contract, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, empty := contextContractFixture()
	data, _ := json.Marshal(empty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err != nil {
		t.Fatal("genuine empty result failed", err)
	}
	cases := map[string]func(map[string]any){
		"missing accounting": func(v map[string]any) { delete(v["result"].(map[string]any), "budget_accounting") },
		"wrong byte count": func(v map[string]any) {
			v["result"].(map[string]any)["budget_accounting"].(map[string]any)["usage"].(map[string]any)["response_bytes"] = 1
		},
		"wrong outcome": func(v map[string]any) {
			v["result"].(map[string]any)["budget_accounting"].(map[string]any)["selection"].(map[string]any)["outcome"] = "selected"
		},
		"missing result":               func(v map[string]any) { delete(v, "result") },
		"missing section":              func(v map[string]any) { delete(v["result"].(map[string]any), "profile") },
		"null section":                 func(v map[string]any) { v["result"].(map[string]any)["profile"] = nil },
		"scalar section":               func(v map[string]any) { v["result"].(map[string]any)["recent_episodes"] = "not-array" },
		"invalid item":                 func(v map[string]any) { v["result"].(map[string]any)["profile"] = []any{"not-item"} },
		"invalid citation":             func(v map[string]any) { v["citations"] = []any{map[string]any{"reference": 17}} },
		"alternate references content": func(v map[string]any) { v["result"] = map[string]any{"references": []any{}, "content": ""} },
		"leaked query":                 func(v map[string]any) { v["result"].(map[string]any)["query"] = "secret-query" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(data, &v)
			mutate(v)
			bad, _ := json.Marshal(v)
			if _, err = contract.Validate(bad, scope, "provider-v1"); err == nil || err.Error() != "provider context failed-contract" {
				t.Fatalf("wanted bounded failed-contract, got %v", err)
			}
		})
	}
	now := time.Now().UTC()
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "m", Scope: scope, MemoryPath: "tasks/one", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, CreatedAt: now, ModifiedAt: now}}
	hit.Memory.SourceMetadata = memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
	hit.Memory.SourceVersion = 1
	nonempty := provider.ShapeDisclosedContextResponse(retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{hit}}, empty.Metadata, empty.Result.Disclosure, 8, scope)
	a, _ := provider.NewContextAdmission(provider.ContextInput{Budget: 1}, provider.Discover(provider.CapabilityInput{}).Limits, scope, empty.Result.Disclosure, empty.Metadata)
	a.AdmitContextItem("recent_episodes", hit)
	data, _ = a.Finalize(retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{hit}}, empty.Metadata)
	_ = json.Unmarshal(data, &nonempty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err != nil {
		t.Fatal(err)
	}
	nonempty.Result.RecentEpisodes[0].Reference.SourceMetadata.Classification = memory.SourceClassificationHigh
	data, _ = json.Marshal(nonempty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err == nil {
		t.Fatal("high identity accepted")
	}
	nonempty.Result.RecentEpisodes[0].Reference.SourceMetadata.Classification = memory.SourceClassificationLow
	nonempty.Citations[0].Reference = "foreign-hidden-marker"
	data, _ = json.Marshal(nonempty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err == nil {
		t.Fatal("unselected evidence accepted")
	}

}

func TestContextPublicHTTPExecutorRequiresSchemaEvidence(t *testing.T) {
	contract, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, empty := contextContractFixture()
	data, _ := json.Marshal(empty)
	response := string(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(provider.HeaderRuntimeBinding) != "binding" {
			t.Error("missing binding")
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	executor := ProviderContextHTTPExecutor{Client: server.Client(), BaseURL: server.URL, Contract: contract, SchemaVersion: "provider-v1"}
	binding := provider.RuntimeBinding{BindingID: "binding", SessionID: "session", Scope: scope}
	fixture := ProviderFixture{ID: "context", Kind: ProviderFixtureContext, Operation: "context", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceContextHTTP, ProviderEvidenceContextSchema}}
	out, err := executor.ExecuteProviderFixture(context.Background(), binding, fixture)
	if err != nil || !out.Passed || !validContextDigest(out.OpenAPIDigest) {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	response = `{"result":{"references":[]}}`
	if _, err = executor.ExecuteProviderFixture(context.Background(), binding, fixture); err == nil {
		t.Fatal("malformed HTTP response passed")
	}
	if _, err = (ProviderHandlerExecutor{Adapter: provider.NewAdapter(provider.AdapterDependencies{})}).ExecuteProviderFixture(context.Background(), binding, fixture); err == nil {
		t.Fatal("adapter-only context passed")
	}
}

func TestContextPublicHTTPExecutorRejectsDifferentAllocation(t *testing.T) {
	contract, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, empty := contextContractFixture()
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "m", Scope: scope, MemoryPath: "tasks/one", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}}}
	a, err := provider.NewContextAdmission(provider.ContextInput{Budget: 2}, provider.Discover(provider.CapabilityInput{}).Limits, scope, empty.Result.Disclosure, empty.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if !a.AdmitContextItem("profile", hit) || !a.AdmitContextItem("recent_session", hit) {
		t.Fatal("fixture did not admit two placements")
	}
	data, err := a.Finalize(retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}, RecentSession: []retrieval.SearchHit{hit}}, empty.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contract.Validate(data, scope, "provider-v1"); err != nil {
		t.Fatal("fixture must be internally consistent", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	executor := ProviderContextHTTPExecutor{Client: server.Client(), BaseURL: server.URL, Contract: contract, SchemaVersion: "provider-v1"}
	if _, err = executor.ExecuteProviderFixture(context.Background(), provider.RuntimeBinding{BindingID: "binding", SessionID: "session", Scope: scope}, ProviderFixture{ID: "context", Kind: ProviderFixtureContext, Operation: "context", Scope: scope}); err == nil {
		t.Fatal("response for budget=2 passed a request for budget=1")
	}
}

func TestContextConsumerBindsLimitsOmissionSectionsAndDiscovery(t *testing.T) {
	contract, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, empty := contextContractFixture()
	zero := 0
	input := provider.ContextInput{Budget: 1, BudgetLimits: &provider.ContextBudgetLimits{MaxContentBytes: &zero}, IncludeDiagnostics: true}
	limits := provider.Discover(provider.CapabilityInput{}).Limits
	a, err := provider.NewContextAdmission(input, limits, scope, empty.Result.Disclosure, empty.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	data, err := a.Finalize(retrieval.AssembledContext{}, empty.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = contract.ValidateForRequest(data, scope, "provider-v1", input, &limits); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []provider.ContextInput{{Budget: 1, IncludeDiagnostics: true}, {Budget: 1, BudgetLimits: input.BudgetLimits}, {Budget: 2, BudgetLimits: input.BudgetLimits, IncludeDiagnostics: true}} {
		if _, err = contract.ValidateForRequest(data, scope, "provider-v1", wrong, &limits); err == nil {
			t.Fatal("different request passed")
		}
	}
	limits.MaxContextBytes--
	if _, err = contract.ValidateForRequest(data, scope, "provider-v1", input, &limits); err == nil {
		t.Fatal("different discovered ceiling passed")
	}
}

func TestContextConformanceCannotPassWithAdapterOnlyOrMissingEvidence(t *testing.T) {
	scope, _ := contextContractFixture()
	now := time.Now().UTC()
	fixture := ProviderFixture{ID: "context", Kind: ProviderFixtureContext, Operation: "context", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceFreshness}}
	for _, out := range []ProviderFixtureOutcome{{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceFreshness}}, {Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceFreshness, ProviderEvidenceContextHTTP, ProviderEvidenceContextSchema}, OpenAPIDigest: "secret-invalid-digest"}, {Passed: true, Hidden: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceFreshness, ProviderEvidenceContextHTTP, ProviderEvidenceContextSchema}, OpenAPIDigest: strings.Repeat("a", 64)}} {
		store := &stubAssuranceStore{}
		executor := &stubProviderFixtureExecutor{outcomes: map[ProviderFixtureKind]ProviderFixtureOutcome{ProviderFixtureContext: out}}
		svc := NewService(ServiceOptions{Store: store, ProviderConformance: executor})
		run, _, err := svc.RunProviderConformance(context.Background(), ProviderConformanceRunInput{Profile: ProviderConformanceProfile{ProfileID: "context", Scope: scope, SchemaVersion: "provider-v1", Fixtures: []ProviderFixture{fixture}}, Binding: provider.RuntimeBinding{Scope: scope}, Dependencies: ProviderDependencySnapshot{Compatible: true, PostgreSQLReady: true, ProjectionFresh: true, WorkerReady: true}, StartedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		if run.Result == ConformanceResultPassed {
			t.Fatal("missing HTTP/schema evidence passed")
		}
		data, _ := json.Marshal(run.EvidenceCounts)
		if strings.Contains(string(data), "secret") {
			t.Fatal("unsafe provenance retained")
		}
		if strings.Contains(string(data), "context_http_schema") {
			t.Fatal("failed fixture retained passing context proof")
		}
	}
}
