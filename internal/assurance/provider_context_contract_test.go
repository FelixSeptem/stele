package assurance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func contextContractFixture() (memory.Scope, provider.ContextResponse) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	return scope, provider.ShapeContextResponse(retrieval.AssembledContext{}, provider.OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "provider-v1"}, provider.ContextInput{}, 8)
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
	nonempty := provider.ShapeContextResponse(retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{hit}}, empty.Metadata, provider.ContextInput{}, 8)
	data, _ = json.Marshal(nonempty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err != nil {
		t.Fatal(err)
	}
	foreign := scope
	foreign.Tenant = "other"
	if _, err = contract.Validate(data, foreign, "provider-v1"); err == nil {
		t.Fatal("foreign scope accepted")
	}
	nonempty.Result.RecentEpisodes[0].Memory.State = memory.MemoryStateSuppressed
	data, _ = json.Marshal(nonempty)
	if _, err = contract.Validate(data, scope, "provider-v1"); err == nil {
		t.Fatal("hidden memory accepted")
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
