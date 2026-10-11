package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

type disclosureAdminStub struct {
	PrincipalAdministrationService
	calls int
	value auth.DisclosureEntitlements
}

func (s *disclosureAdminStub) UpdateScopeGrantDisclosure(_ context.Context, scope memory.Scope, principalID, grantID string, e auth.DisclosureEntitlements, actor, reason string) error {
	if scope.Tenant != "tenant-a" || principalID != "principal-a" || grantID != "grant-a" || actor != "operator" || reason != "least privilege" {
		panic("incorrect admin scope/audit")
	}
	s.calls++
	s.value = e
	return nil
}
func TestDisclosureAdministrationClosedAndAuthenticated(t *testing.T) {
	valid := `{"disclosure":{"allowed_profiles":["memory-reference-v1"],"default_profile":"memory-reference-v1","source_attestation":false},"actor":"operator","reason":"least privilege"}`
	for _, tc := range []struct {
		body, key     string
		status, calls int
	}{
		{valid, "admin", 204, 1}, {valid, "public", 401, 0},
		{strings.Replace(valid, `"source_attestation":false`, `"source_attestation":false,"role":"admin"`, 1), "admin", 400, 0},
		{strings.Replace(valid, "memory-reference-v1", "caller-admin", -1), "admin", 400, 0},
		{strings.Replace(valid, `"actor":"operator"`, `"Actor":"operator"`, 1), "admin", 400, 0},
		{strings.Replace(valid, `"source_attestation":false`, `"source_attestation":false,"source_attestation":true`, 1), "admin", 400, 0},
	} {
		s := &disclosureAdminStub{}
		h := NewHTTPHandler(HTTPDependencies{AdminAPIKeys: auth.StaticAPIKeys{"admin": {}}, PrincipalAdmin: s})
		r := httptest.NewRequest(http.MethodPut, "/v1/admin/principals/principal-a/grants/grant-a/disclosure", strings.NewReader(tc.body))
		r.Header.Set(auth.HeaderAPIKey, tc.key)
		r.Header.Set(auth.HeaderTenant, "tenant-a")
		r.Header.Set(auth.HeaderProject, "project-a")
		r.Header.Set(auth.HeaderNamespace, "namespace-a")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || s.calls != tc.calls {
			t.Fatalf("status %d calls %d, want %d/%d", w.Code, s.calls, tc.status, tc.calls)
		}
	}
}
