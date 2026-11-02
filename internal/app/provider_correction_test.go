package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type correctionHTTPResolver struct {
	calls   int
	binding provider.RuntimeBinding
	request provider.CorrectionRequest
	result  provider.CorrectionResult
	err     error
}

func (s *correctionHTTPResolver) SubmitCorrection(_ context.Context, binding provider.RuntimeBinding, request provider.CorrectionRequest) (provider.CorrectionResult, error) {
	s.calls++
	s.binding, s.request = binding, request
	return s.result, s.err
}

func correctionHTTPHandler(t *testing.T, resolver provider.CorrectionResolver, policies ...memory.MemoryIntentPolicyGate) (http.Handler, provider.RuntimeBinding) {
	t.Helper()
	now := time.Now().UTC()
	binding := provider.RuntimeBinding{BindingID: "binding-correction", PrincipalID: "principal-correction", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "provider-a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	policy := memory.MemoryIntentPolicyGate(memory.StaticMemoryIntentPolicy{Scope: binding.Scope, PolicyVersion: "correction-policy-v1", Enabled: true})
	if len(policies) > 0 {
		policy = policies[0]
	}
	bindings := provider.NewMemoryBindingStore()
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	principal := auth.Principal{ID: binding.PrincipalID, Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCorrections: resolver, MemoryIntentPolicy: policy, ProviderBindings: bindings, ProviderSchemaVersions: []string{"provider-v1"}, PrincipalAuthorizer: referenceHTTPAuthorizer{providerLifecycleAuthorizer{principals: map[string]auth.Principal{"correction-key": principal}}}})
	return h, binding
}

func callCorrectionHTTP(h http.Handler, binding provider.RuntimeBinding, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/v1/provider/corrections", strings.NewReader(body))
	r.Header.Set(auth.HeaderAPIKey, "correction-key")
	r.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	r.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	r.Header.Set(auth.HeaderTenant, binding.Scope.Tenant)
	r.Header.Set(auth.HeaderProject, binding.Scope.Project)
	r.Header.Set(auth.HeaderNamespace, binding.Scope.Namespace)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const correctionHTTPBody = "{\"metadata\":{\"request_id\":\"r1\",\"operation_id\":\"o1\",\"idempotency_key\":\"correction-key\",\"schema_version\":\"provider-v1\"},\"input\":{\"contract_version\":\"provider-correction-governed-version-v1\",\"projection_id\":\"11111111-1111-4111-8111-111111111111\",\"target_memory_id\":\"22222222-2222-4222-8222-222222222222\",\"expected_version\":2,\"content\":\"Corrected durable fact\",\"reason\":\"source correction\"}}"

func TestProviderCorrectionRouteUsesBoundResolverAndCapability(t *testing.T) {
	resolver := &correctionHTTPResolver{result: provider.CorrectionResult{IntentID: "33333333-3333-4333-8333-333333333333", Status: memory.MemoryIntentStatusAccepted, AttributionState: "verified"}}
	h, binding := correctionHTTPHandler(t, resolver)

	capability := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	request.Header.Set(auth.HeaderAPIKey, "correction-key")
	request.Header.Set(auth.HeaderTenant, binding.Scope.Tenant)
	request.Header.Set(auth.HeaderProject, binding.Scope.Project)
	request.Header.Set(auth.HeaderNamespace, binding.Scope.Namespace)
	h.ServeHTTP(capability, request)
	if capability.Code != http.StatusOK || !strings.Contains(capability.Body.String(), "\"corrections\":{\"enabled\":true") || !strings.Contains(capability.Body.String(), "\"operations\":[\"ingest\",\"intent\",\"retrieve\",\"context\",\"forget\",\"status\",\"sync\",\"corrections\"]") {
		t.Fatalf("capability = %d %s", capability.Code, capability.Body.String())
	}

	w := callCorrectionHTTP(h, binding, correctionHTTPBody)
	if w.Code != http.StatusAccepted || resolver.calls != 1 || resolver.binding != binding || !strings.Contains(w.Body.String(), "\"intent_id\":\"33333333-3333-4333-8333-333333333333\"") {
		t.Fatalf("response = %d %s calls=%d", w.Code, w.Body.String(), resolver.calls)
	}
	if resolver.request.PolicyVersion != "correction-policy-v1" {
		t.Fatalf("resolver policy version = %q, want server policy version", resolver.request.PolicyVersion)
	}
	if !strings.Contains(w.Body.String(), "\"contract_version\":\"provider-correction-governed-version-v1\"") || !strings.Contains(w.Body.String(), "\"attribution_state\":\"verified\"") {
		t.Fatalf("response does not match the published correction contract: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "projection_id") || strings.Contains(w.Body.String(), "target_memory_id") {
		t.Fatalf("response leaks correction source or target: %s", w.Body.String())
	}
}

func TestProviderCorrectionRouteBoundsInvalidAndResolverErrors(t *testing.T) {
	resolver := &correctionHTTPResolver{}
	h, binding := correctionHTTPHandler(t, resolver)
	for _, body := range []string{strings.Replace(correctionHTTPBody, "\"expected_version\":2", "\"expected_version\":0", 1), strings.Replace(correctionHTTPBody, "\"content\"", "\"binding_id\":\"foreign\",\"content\"", 1)} {
		w := callCorrectionHTTP(h, binding, body)
		if w.Code != http.StatusBadRequest || resolver.calls != 0 {
			t.Fatalf("invalid correction dispatched: %d %s", w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		err      error
		code     int
		category string
	}{{provider.ErrCorrectionUnavailable, http.StatusGone, "lifecycle"}, {provider.ErrCorrectionStale, http.StatusConflict, "stale"}, {provider.ErrCorrectionConflict, http.StatusConflict, "conflict"}, {errors.New("postgres credential marker"), http.StatusServiceUnavailable, "dependency"}} {
		resolver.err = tc.err
		w := callCorrectionHTTP(h, binding, correctionHTTPBody)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), "\"category\":\""+tc.category+"\"") || strings.Contains(w.Body.String(), "credential") {
			t.Fatalf("%s response = %d %s", tc.category, w.Code, w.Body.String())
		}
	}

	h, binding = correctionHTTPHandler(t, nil)
	capabilityResponse := httptest.NewRecorder()
	capabilityRequest := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	capabilityRequest.Header.Set(auth.HeaderAPIKey, "correction-key")
	capabilityRequest.Header.Set(auth.HeaderTenant, binding.Scope.Tenant)
	capabilityRequest.Header.Set(auth.HeaderProject, binding.Scope.Project)
	capabilityRequest.Header.Set(auth.HeaderNamespace, binding.Scope.Namespace)
	h.ServeHTTP(capabilityResponse, capabilityRequest)
	if capabilityResponse.Code != http.StatusOK || !strings.Contains(capabilityResponse.Body.String(), "\"corrections\":{\"enabled\":false") || strings.Contains(capabilityResponse.Body.String(), "\"operations\":[\"ingest\",\"intent\",\"retrieve\",\"context\",\"forget\",\"status\",\"sync\",\"corrections\"]") {
		t.Fatalf("unavailable correction capability was advertised: %d %s", capabilityResponse.Code, capabilityResponse.Body.String())
	}
	w := callCorrectionHTTP(h, binding, correctionHTTPBody)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "\"provider_unavailable\"") {
		t.Fatalf("missing resolver = %d %s", w.Code, w.Body.String())
	}
}

func TestProviderCorrectionRouteRequiresCurrentMemoryIntentPolicy(t *testing.T) {
	resolver := &correctionHTTPResolver{result: provider.CorrectionResult{IntentID: "33333333-3333-4333-8333-333333333333", Status: memory.MemoryIntentStatusAccepted, AttributionState: "verified"}}
	policy := memory.StaticMemoryIntentPolicy{Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, PolicyVersion: "correction-policy-v1", Enabled: false}
	h, binding := correctionHTTPHandler(t, resolver, policy)
	w := callCorrectionHTTP(h, binding, correctionHTTPBody)
	if w.Code != http.StatusForbidden || resolver.calls != 0 || !strings.Contains(w.Body.String(), "policy_disabled") {
		t.Fatalf("disabled intent policy reached correction resolver: status=%d calls=%d body=%s", w.Code, resolver.calls, w.Body.String())
	}
}
