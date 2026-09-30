package reasoning

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestOpenAICompatibleProviderDerivesBoundedCandidate(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	req := testInvocation(scope)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		var body openAICompatibleRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "reasoning-model" || body.ResponseFormat.Type != "json_object" || body.Temperature != 0 || len(body.Messages) != 2 {
			t.Fatalf("request = %+v", body)
		}
		if strings.Contains(body.Messages[1].Content, "secret-key") {
			t.Fatal("request leaked adapter credential")
		}
		candidate := testCandidate(scope)
		_ = json.NewEncoder(w).Encode(openAICompatibleResponse{Choices: []openAICompatibleChoice{{Message: openAICompatibleMessage{Role: "assistant", Content: mustJSON(candidate)}}}})
	}))
	defer server.Close()
	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "reasoning-model", APIKey: "secret-key", Timeout: time.Second, Limits: DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := provider.Derive(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "candidate-1" || got.Scope.Normalized() != scope.Normalized() {
		t.Fatalf("candidate = %+v", got)
	}
}

func TestOpenAICompatibleProviderRejectsForeignEvidenceAndScope(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	cases := []struct {
		name      string
		candidate func() Candidate
		category  ErrorCategory
	}{
		{name: "foreign evidence", candidate: func() Candidate {
			c := testCandidate(scope)
			c.Evidence = []Evidence{{Kind: "memory", Reference: "foreign"}}
			return c
		}, category: ErrorCategoryScope},
		{name: "foreign scope", candidate: func() Candidate { c := testCandidate(scope); c.Scope.Namespace = "namespace-b"; return c }, category: ErrorCategoryScope},
		{name: "reserved type", candidate: func() Candidate { c := testCandidate(scope); c.InsightType = "goal"; return c }, category: ErrorCategoryMalformedOutput},
		{name: "direct mutation", candidate: func() Candidate { c := testCandidate(scope); c.DirectMutation = true; return c }, category: ErrorCategoryMalformedOutput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				candidate := tc.candidate()
				_ = json.NewEncoder(w).Encode(openAICompatibleResponse{Choices: []openAICompatibleChoice{{Message: openAICompatibleMessage{Role: "assistant", Content: mustJSON(candidate)}}}})
			}))
			defer server.Close()
			provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "model", APIKey: "key", Timeout: time.Second, Limits: DefaultLimits()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Derive(context.Background(), testInvocation(scope))
			var providerErr *ProviderError
			if !errors.As(err, &providerErr) || providerErr.Category != tc.category {
				t.Fatalf("error = %v, want category %q", err, tc.category)
			}
		})
	}
}

func TestOpenAICompatibleProviderMapsFailuresWithoutLeakingBody(t *testing.T) {
	cases := []struct {
		status   int
		category ErrorCategory
	}{
		{status: http.StatusUnauthorized, category: ErrorCategoryConfiguration},
		{status: http.StatusTooManyRequests, category: ErrorCategoryRateLimit},
		{status: http.StatusGatewayTimeout, category: ErrorCategoryTimeout},
		{status: http.StatusBadGateway, category: ErrorCategoryUnavailable},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("prompt=secret-key raw payload"))
			}))
			defer server.Close()
			provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "model", APIKey: "secret-key", Timeout: time.Second, Limits: DefaultLimits()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Derive(context.Background(), testInvocation(memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}))
			var providerErr *ProviderError
			if !errors.As(err, &providerErr) || providerErr.Category != tc.category {
				t.Fatalf("error = %v, want category %q", err, tc.category)
			}
			if strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "raw payload") {
				t.Fatalf("error leaked provider details: %v", err)
			}
		})
	}
}

func TestOpenAICompatibleProviderRejectsInvalidInvocationBeforeTransport(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "model", APIKey: "key", Timeout: time.Second, Limits: DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	req := testInvocation(memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"})
	req.MaxOutputBytes = 0
	_, err = provider.Derive(context.Background(), req)
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Category != ErrorCategoryValidation {
		t.Fatalf("error = %v, want validation", err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want zero", calls)
	}
}

func TestRunAdapterConformanceRetainsOnlySafeCounters(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	candidate := testCandidate(scope)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openAICompatibleResponse{Choices: []openAICompatibleChoice{{Message: openAICompatibleMessage{Role: "assistant", Content: mustJSON(candidate)}}}})
	}))
	defer server.Close()
	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "model", APIKey: "secret", Timeout: time.Second, Limits: DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	run, err := RunAdapterConformance(context.Background(), provider, ConformanceProfile{ID: "adapter-local", Scope: scope, Fixtures: []Fixture{{ID: "fixture-1", Request: testInvocation(scope), ExpectedCandidate: candidate}}}, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != ConformancePassed || run.Passed != 1 || run.Failed != 0 || run.Prompts != 0 || run.RawPayloads != 0 || run.Credentials != 0 {
		t.Fatalf("conformance run = %+v", run)
	}
}

func TestOpenAICompatibleAdapterUsesBaselineFallbackAndShadowAuthority(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	baseline := testCandidate(scope)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate := baseline
		candidate.Content = "remote candidate"
		_ = json.NewEncoder(w).Encode(openAICompatibleResponse{Choices: []openAICompatibleChoice{{Message: openAICompatibleMessage{Role: "assistant", Content: mustJSON(candidate)}}}})
	}))
	defer server.Close()
	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{Endpoint: server.URL, Model: "model", APIKey: "secret", Timeout: time.Second, Limits: DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := (Executor{Provider: provider, Limits: DefaultLimits()}).Execute(context.Background(), testInvocation(scope), baseline)
	if err != nil || result.Authoritative || result.Candidate.Content != "remote candidate" {
		t.Fatalf("live result = %+v, err=%v", result, err)
	}
	shadow, err := (ShadowExecutor{Provider: provider, Limits: DefaultLimits()}).Execute(context.Background(), testInvocation(scope), baseline)
	if err != nil || shadow.Authoritative || shadow.Candidate.Content != baseline.Content || shadow.Outcome != OutcomeShadow {
		t.Fatalf("shadow result = %+v, err=%v", shadow, err)
	}
}

func testInvocation(scope memory.Scope) InvocationRequest {
	now := time.Now().UTC()
	return InvocationRequest{Metadata: InvocationMetadata{RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idem-1", SchemaVersion: SchemaVersionV1, PolicyVersion: "policy-v1", ProviderVersion: OpenAICompatibleProviderVersion}, Scope: scope, Evidence: []Evidence{{Kind: "memory", Reference: "memory-1"}}, Input: []byte(`{"operation":"derive","content":"bounded input"}`), MaxOutputBytes: 2048, Deadline: now.Add(time.Second), Now: now}
}

func testCandidate(scope memory.Scope) Candidate {
	return Candidate{ID: "candidate-1", Scope: scope, Kind: "candidate", InsightType: "lesson", Content: "bounded candidate", Evidence: []Evidence{{Kind: "memory", Reference: "memory-1"}}, Provenance: map[string]string{"source": "adapter-test"}, PolicyVersion: "policy-v1", ProviderVersion: OpenAICompatibleProviderVersion, ReplayID: "replay-1", Confidence: 0.8}
}

func mustJSON(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(b)
}
