package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestProjectionRoutesRequireBindingEvenWithoutDependency(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true})
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if w.Code == 404 || w.Code == 200 {
			t.Fatalf("projection route missing or unguarded: %s %d", path, w.Code)
		}
	}
}

type projectionHTTPStore struct {
	calls   int
	binding provider.RuntimeBinding
	request provider.ProjectionRequest
	record  provider.ProjectionRecord
	history []provider.ProjectionRecord
	err     error
}

func (s *projectionHTTPStore) AdmitProjection(_ context.Context, b provider.RuntimeBinding, r provider.ProjectionRequest) (provider.ProjectionRecord, error) {
	s.calls++
	s.binding = b
	s.request = r
	return s.record, s.err
}
func (s *projectionHTTPStore) ProjectionHistory(_ context.Context, b provider.RuntimeBinding, _ provider.ProjectionHistoryInput) ([]provider.ProjectionRecord, bool, error) {
	s.calls++
	s.binding = b
	if s.history != nil {
		return s.history, false, s.err
	}
	return []provider.ProjectionRecord{s.record}, false, s.err
}
func projectionHTTPRecord() provider.ProjectionRecord {
	return provider.ProjectionRecord{ProjectionID: "11111111-1111-4111-8111-111111111111", LogicalID: "22222222-2222-4222-8222-222222222222", SourceRevision: 1, Disposition: "accepted", EvidenceKind: provider.ProjectionEvidenceKind, References: []provider.ProjectionReferenceAttribution{}, CurrentReferences: []provider.ProjectionReferenceAttribution{}, FeedbackState: "none", ContextDigestAttribution: "none"}
}
func projectionHTTPHandler(t *testing.T, s provider.ProjectionStore, mutate ...func(*provider.RuntimeBinding)) (http.Handler, provider.RuntimeBinding) {
	t.Helper()
	now := time.Now()
	b := provider.RuntimeBinding{BindingID: "binding-context", PrincipalID: "principal-context", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	for _, apply := range mutate {
		apply(&b)
	}
	bindings := provider.NewMemoryBindingStore()
	if err := bindings.Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	p := auth.Principal{ID: b.PrincipalID, Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	return NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderProjections: s, ProviderBindings: bindings, ProviderSchemaVersions: []string{"provider-v1"}, PrincipalAuthorizer: referenceHTTPAuthorizer{providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": p}}}}), b
}
func callProjectionHTTP(h http.Handler, b provider.RuntimeBinding, path, body string, headers http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("X-API-Key", "context-key")
	r.Header.Set(provider.HeaderRuntimeBinding, b.BindingID)
	r.Header.Set(provider.HeaderRuntimeSession, b.SessionID)
	r.Header.Set(auth.HeaderTenant, b.Scope.Tenant)
	r.Header.Set(auth.HeaderProject, b.Scope.Project)
	r.Header.Set(auth.HeaderNamespace, b.Scope.Namespace)
	for k, v := range headers {
		r.Header.Del(k)
		for _, value := range v {
			r.Header.Add(k, value)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const projectionHTTPBody = `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1","idempotency_key":"key"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"succeeded"}}`

func TestProjectionHTTPSerializedStatesAndDedicatedCapability(t *testing.T) {
	s := &projectionHTTPStore{record: projectionHTTPRecord()}
	h, b := projectionHTTPHandler(t, s)
	spec := httptest.NewRecorder()
	h.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if spec.Code != 200 {
		t.Fatalf("served OpenAPI: %d", spec.Code)
	}
	contract, err := assurance.NewProviderProjectionContract(spec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	capability := httptest.NewRecorder()
	capabilityRequest := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	capabilityRequest.Header.Set("X-API-Key", "context-key")
	capabilityRequest.Header.Set(auth.HeaderTenant, b.Scope.Tenant)
	capabilityRequest.Header.Set(auth.HeaderProject, b.Scope.Project)
	capabilityRequest.Header.Set(auth.HeaderNamespace, b.Scope.Namespace)
	h.ServeHTTP(capability, capabilityRequest)
	if capability.Code != http.StatusOK {
		t.Fatalf("served capability: %d %s", capability.Code, capability.Body.String())
	}
	if err = contract.NegotiateDocument(capability.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"accepted", "pending", "failed", "completed", "unavailable"} {
		ref := provider.ProjectionReferenceAttribution{State: state}
		if state != "unavailable" {
			ref.EventID = referenceHTTPRecord().EventID
		}
		if state == "completed" {
			r := referenceHTTPRecord()
			ref.Eligible = true
			ref.Lineage = &provider.ProjectionReferenceLineage{CandidateID: r.CandidateID, MemoryID: r.MemoryID, VersionID: r.VersionID, ProvenanceID: r.ProvenanceID, Version: r.Version}
		}
		s.record.References = []provider.ProjectionReferenceAttribution{ref}
		s.record.CurrentReferences = []provider.ProjectionReferenceAttribution{ref}
		s.record.FeedbackState = "pending"
		w := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
		if w.Code != 202 {
			t.Fatalf("%s: %d %s", state, w.Code, w.Body.String())
		}
		out, err := contract.Validate(w.Body.Bytes(), provider.ProjectionScope(b), "provider-v1")
		if err != nil || out.Record.References[0].State != state || s.binding != b {
			t.Fatalf("binding or serialized state: %s %v", w.Body.String(), err)
		}
	}
	s.record = projectionHTTPRecord()
	first := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	replay := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	if first.Body.String() != replay.Body.String() {
		t.Fatal("replay changed identity")
	}
	older := s.record
	s.record.SupersedesID = s.record.ProjectionID
	s.record.ProjectionID = "44444444-4444-4444-8444-444444444444"
	s.record.Disposition = "corrected"
	s.record.SourceRevision = 2
	w := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	if _, err = contract.Validate(w.Body.Bytes(), provider.ProjectionScope(b), "provider-v1"); err != nil {
		t.Fatal(err)
	}
	// A complete, non-truncated history must retain the accepted revision that
	// the correction names; the repository returns newest first.
	s.history = []provider.ProjectionRecord{s.record, older}
	hist := `{` + referenceMetadata + `,"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","limit":8}}`
	w = callProjectionHTTP(h, b, "/v1/provider/projections/history", hist, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = contract.ValidateHistory(w.Body.Bytes(), provider.ProjectionScope(b), "provider-v1", 8); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionHTTPRejectsBeforeDispatchAndBoundsErrors(t *testing.T) {
	s := &projectionHTTPStore{record: projectionHTTPRecord()}
	h, b := projectionHTTPHandler(t, s)
	for _, tc := range []struct {
		body string
		code int
	}{
		{strings.Replace(projectionHTTPBody, `"source_revision":1`, `"source_revision":null`, 1), 400},
		{strings.Replace(projectionHTTPBody, `"source_revision":1`, `"source_revision":1,"task_state":"done"`, 1), 400},
		{strings.Replace(projectionHTTPBody, `"effect_id"`, `"effectId"`, 1), 400},
		{strings.Replace(projectionHTTPBody, `"source_revision":1`, `"source_revision":1,"source_revision":1`, 1), 400},
		{strings.Replace(projectionHTTPBody, provider.ProjectionContractVersion, "future-v1", 1), 409},
		{strings.Replace(projectionHTTPBody, "provider-v1", "provider-v999", 1), 409},
		{strings.Repeat(" ", 32769), 413},
	} {
		w := callProjectionHTTP(h, b, "/v1/provider/projections", tc.body, nil)
		if w.Code != tc.code || s.calls != 0 {
			t.Fatalf("invalid dispatch %d %s", w.Code, w.Body.String())
		}
	}
	for _, header := range []http.Header{{auth.HeaderTenant: []string{"foreign"}}, {provider.HeaderRuntimeSession: []string{"foreign"}}, {provider.HeaderRuntimeBinding: []string{"foreign"}}} {
		w := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, header)
		if w.Code != 403 || s.calls != 0 {
			t.Fatalf("foreign dispatched %d", w.Code)
		}
	}
	for _, tc := range []struct {
		err      error
		code     int
		category string
	}{{provider.ErrProjectionConflict, 409, "conflict"}, {provider.ErrProjectionStale, 409, "stale"}, {provider.ErrProjectionUnavailable, 410, "lifecycle"}, {errors.New("SQL secret credential"), 503, "dependency"}} {
		s.err = tc.err
		w := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), `"category":"`+tc.category+`"`) || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("unsafe error %d %s", w.Code, w.Body.String())
		}
	}
	s.err = nil
	s.record.ProjectionID = ""
	w := callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	if w.Code != 503 {
		t.Fatal("invalid producer output accepted")
	}
	h, b = projectionHTTPHandler(t, nil)
	w = callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	if w.Code != 503 {
		t.Fatal("dependency not bounded")
	}
	h, b = projectionHTTPHandler(t, s, func(b *provider.RuntimeBinding) { b.AgentID = strings.Repeat("a", 129) })
	before := s.calls
	w = callProjectionHTTP(h, b, "/v1/provider/projections", projectionHTTPBody, nil)
	if w.Code != 400 || s.calls != before {
		t.Fatal("invalid bound scope dispatched")
	}
}

func TestProjectionCapabilityPresentAndDisabledWithoutStore(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"projections":{"enabled":false,"contract_version":"turn-outcome-feedback-projection-v1"`) {
		t.Fatalf("missing independent capability: %d %s", w.Code, w.Body.String())
	}
}

func TestProjectionAuthorizationErrorsAreClosedJSON(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		for _, tc := range []struct {
			name, header, value, category string
			status                        int
		}{
			{"missing key", auth.HeaderAPIKey, "", "authentication", 403},
			{"invalid key", auth.HeaderAPIKey, "unknown", "authentication", 403},
			{"missing tenant", auth.HeaderTenant, "", "validation", 400},
			{"foreign tenant", auth.HeaderTenant, "foreign", "scope", 403},
			{"missing binding", provider.HeaderRuntimeBinding, "", "scope", 403},
			{"foreign binding", provider.HeaderRuntimeBinding, "foreign", "scope", 403},
			{"missing session", provider.HeaderRuntimeSession, "", "scope", 403},
			{"foreign session", provider.HeaderRuntimeSession, "foreign", "scope", 403},
			{"foreign agent", provider.HeaderRuntimeAgent, "foreign", "scope", 403},
			{"foreign conversation", provider.HeaderRuntimeConversation, "foreign", "scope", 403},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				s := &projectionHTTPStore{record: projectionHTTPRecord()}
				h, b := projectionHTTPHandler(t, s)
				w := callProjectionHTTP(h, b, path, projectionHTTPBody, http.Header{tc.header: []string{tc.value}})
				if w.Code != tc.status || s.calls != 0 {
					t.Fatalf("denial %d, dispatch %d: %s", w.Code, s.calls, w.Body.String())
				}
				if w.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("denial is not JSON: %s", w.Body.String())
				}
				var value any
				if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
					t.Fatal(err)
				}
				if err := doc.Components.Schemas["ProviderErrorResponse"].Value.VisitJSON(value); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(w.Body.String(), `"category":"`+tc.category+`"`) {
					t.Fatal(w.Body.String())
				}
			})
		}
	}
}

type projectionDeniedAuthorizer struct {
	referenceHTTPAuthorizer
	grant bool
}

type projectionDisclosureAuthorizer struct {
	referenceHTTPAuthorizer
	entitlements auth.DisclosureEntitlements
}

func (a projectionDisclosureAuthorizer) AuthorizeScopeDisclosure(context.Context, string, memory.Scope) (auth.DisclosureEntitlements, error) {
	return a.entitlements, nil
}

func TestProjectionHTTPUsesCurrentDefaultDisclosureProfile(t *testing.T) {
	for _, route := range []struct {
		path string
		body string
		code int
	}{
		{"/v1/provider/projections", projectionHTTPBody, http.StatusAccepted},
		{"/v1/provider/projections/history", `{` + referenceMetadata + `,"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","limit":8}}`, http.StatusOK},
	} {
		for _, tc := range []struct {
			name         string
			entitlements auth.DisclosureEntitlements
			allowed      bool
		}{
			{"content-only", auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileContent}, true},
			{"reference-only", auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}, true},
			{"default-not-allowed", auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileReference}, false},
			{"empty-grant", auth.DisclosureEntitlements{}, false},
		} {
			t.Run(route.path+"/"+tc.name, func(t *testing.T) {
				s := &projectionHTTPStore{record: projectionHTTPRecord()}
				_, b := projectionHTTPHandler(t, s)
				bindings := provider.NewMemoryBindingStore()
				if err := bindings.Create(context.Background(), b); err != nil {
					t.Fatal(err)
				}
				a := projectionDisclosureAuthorizer{
					referenceHTTPAuthorizer: referenceHTTPAuthorizer{providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": {ID: b.PrincipalID, Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}}}},
					entitlements:            tc.entitlements,
				}
				h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderBindings: bindings, ProviderProjections: s, ProviderSchemaVersions: []string{"provider-v1"}, PrincipalAuthorizer: a})
				w := callProjectionHTTP(h, b, route.path, route.body, nil)
				if tc.allowed {
					if w.Code != route.code || s.calls != 1 || s.binding != b {
						t.Fatalf("valid disclosure grant: status=%d dispatch=%d body=%s", w.Code, s.calls, w.Body.String())
					}
				} else if w.Code != http.StatusForbidden || s.calls != 0 || !strings.Contains(w.Body.String(), `"code":"disclosure_denied"`) {
					t.Fatalf("invalid disclosure grant: status=%d dispatch=%d body=%s", w.Code, s.calls, w.Body.String())
				}
			})
		}
	}
}

func (a projectionDeniedAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	return a.grant, nil
}

func TestProjectionPrincipalAndGrantDenialsNeverDispatch(t *testing.T) {
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		for _, foreignPrincipal := range []bool{false, true} {
			s := &projectionHTTPStore{record: projectionHTTPRecord()}
			_, b := projectionHTTPHandler(t, s)
			bindings := provider.NewMemoryBindingStore()
			if err := bindings.Create(context.Background(), b); err != nil {
				t.Fatal(err)
			}
			principalID := b.PrincipalID
			if foreignPrincipal {
				principalID = "foreign-principal"
			}
			a := projectionDeniedAuthorizer{referenceHTTPAuthorizer: referenceHTTPAuthorizer{providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": {ID: principalID, Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}}}}, grant: foreignPrincipal}
			h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderBindings: bindings, ProviderProjections: s, PrincipalAuthorizer: a})
			w := callProjectionHTTP(h, b, path, projectionHTTPBody, nil)
			if w.Code != 403 || s.calls != 0 || w.Header().Get("Content-Type") != "application/json" || !strings.Contains(w.Body.String(), `"category":"scope"`) {
				t.Fatalf("grant/principal denial: %d %s", w.Code, w.Body.String())
			}
		}
	}
}
