package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestProviderDisclosureDeniesAlternateHTTPRoutesWithoutBinding(t *testing.T) {
	principal := auth.Principal{ID: "principal-context", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	h := NewHTTPHandler(HTTPDependencies{PrincipalAuthorizer: providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": principal}}})
	for _, route := range []struct{ method, path string }{
		{"POST", "/v1/memories/search"}, {"POST", "/v1/context"}, {"GET", "/v1/memories"},
		{"GET", "/v1/memories/hidden-identity-marker/history"}, {"GET", "/v1/memories/hidden-identity-marker/provenance"},
		{"GET", "/v1/memory-sessions/hidden-identity-marker"}, {"GET", "/v1/memory-intents/hidden-identity-marker"},
		{"POST", "/v1/provider/retrieve"}, {"POST", "/v1/provider/sync"}, {"POST", "/v1/provider/turns"},
		{"POST", "/v1/provider/forget"}, {"POST", "/v1/mcp"},
		{"GET", "/v1/future-content-route/hidden-identity-marker"},
	} {
		 t.Run(route.method+route.path, func(t *testing.T) {
			r := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"query":"private-content-marker"}`))
			r.Header.Set(auth.HeaderAPIKey, "context-key")
			r.Header.Set(auth.HeaderTenant, "tenant-a")
			r.Header.Set(auth.HeaderProject, "project-a")
			r.Header.Set(auth.HeaderNamespace, "namespace-a")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"code":"disclosure_denied"`) || strings.Contains(w.Body.String(), "marker") {
				t.Fatalf("alternate route disclosed data or reached dispatch: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestProviderContextSerializedCeilingRejectsWholeResponse(t *testing.T) {
	for _, mode := range []string{auth.ResponseModeReferenceOnly, auth.ResponseModeBoundedContent} {
		t.Run(mode, func(t *testing.T) {
			a := &contextContractAssembler{result: retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{{Memory: memory.CanonicalMemory{
				ID: "selected-identity-marker", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, MemoryPath: "tasks/one", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive,
				Content: "selected-content-marker", SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow},
			}}}}}
			body := `{` + contextMetadata + `,"input":{"query":"q","budget":1,"disclosure_profile":"memory-content-v1","response_mode":"` + mode + `"}}`
			baseline := contextContractHandler(t, a)(body)
			if baseline.Code != http.StatusOK || !strings.Contains(baseline.Body.String(), "selected-identity-marker") {
				t.Fatalf("invalid baseline: %d", baseline.Code)
			}
			for _, delta := range []int{0, -1} {
				request := contextContractHandlerWithDependencies(t, a, func(deps *HTTPDependencies) { deps.ProviderLimits.MaxContextBytes = baseline.Body.Len() + delta })
				w := request(body)
				if delta == 0 {
					if w.Code != http.StatusOK || w.Body.String() != baseline.Body.String() {
						t.Fatal("response at exact serialized ceiling changed or was rejected")
					}
					continue
				}
				if w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), `"code":"context_limit_exceeded"`) || strings.Contains(w.Body.String(), "selected-") || strings.Contains(w.Body.String(), `"result"`) {
					t.Fatalf("oversize response was partially disclosed: %d %s", w.Code, w.Body.String())
				}
				if !json.Valid(w.Body.Bytes()) {
					t.Fatal("refusal is not a complete JSON response")
				}
			}
		})
	}
}

type mutableContextDisclosureAuthorizer struct {
	providerLifecycleAuthorizer
	entitlements auth.DisclosureEntitlements
	err          error
	reads        int
}

func (a *mutableContextDisclosureAuthorizer) AuthorizeScopeDisclosure(_ context.Context, principalID string, scope memory.Scope) (auth.DisclosureEntitlements, error) {
	if principalID != "principal-context" || scope != (memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}) {
		panic("incorrect disclosure scope")
	}
	a.reads++
	return a.entitlements, a.err
}

func TestProviderContextOldBindingReadsCurrentAuthorityAndFailsClosed(t *testing.T) {
	a := &contextContractAssembler{}
	authority := &mutableContextDisclosureAuthorizer{entitlements: auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileContent}}
	request := contextContractHandlerWithDependencies(t, a, func(deps *HTTPDependencies) {
		authority.providerLifecycleAuthorizer = deps.PrincipalAuthorizer.(providerLifecycleAuthorizer)
		deps.PrincipalAuthorizer = authority
	})
	body := `{` + contextMetadata + `,"input":{"query":"private-query-marker","budget":1,"disclosure_profile":"memory-content-v1","response_mode":"bounded_content"}}`
	if w := request(body); w.Code != 200 || a.calls != 1 {
		t.Fatalf("initial content request failed: %d", w.Code)
	}
	for _, tc := range []struct {
		name         string
		entitlements auth.DisclosureEntitlements
		err          error
		status       int
		category     string
	}{
		{"downgraded", auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}, nil, 403, "scope"},
		{"revoked", auth.DisclosureEntitlements{}, auth.ErrDisclosureDenied, 403, "scope"},
		{"unconfigured", auth.DisclosureEntitlements{}, nil, 403, "scope"},
		{"dependency", auth.DisclosureEntitlements{}, errors.New("private-store-error-marker"), 503, "dependency"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authority.entitlements, authority.err = tc.entitlements, tc.err
			reads := authority.reads
			w := request(body)
			if w.Code != tc.status || authority.reads != reads+1 || a.calls != 1 || !strings.Contains(w.Body.String(), `"category":"`+tc.category+`"`) || strings.Contains(w.Body.String(), "marker") || strings.Contains(w.Body.String(), `"result"`) {
				t.Fatalf("current authority did not fail closed: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
