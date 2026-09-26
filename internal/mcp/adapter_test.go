package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

func TestAdapterIsDisabledByDefaultAndFailsClosed(t *testing.T) {
	adapter := NewAdapter(AdapterOptions{Enabled: false, Authorizer: scopeAuthorizerStub{}})
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled MCP status = %d, want 404", rec.Code)
	}
}

func TestAdapterRequiresPrincipalBeforeProtocolDispatch(t *testing.T) {
	adapter := NewAdapter(AdapterOptions{Enabled: true, Authorizer: scopeAuthorizerStub{}})
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP status = %d, want 401", rec.Code)
	}
}

func TestAdapterRejectsInactiveCredentialsBeforeProtocolDispatch(t *testing.T) {
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	principal := auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	baseCredential := auth.Credential{ID: "credential-1", PrincipalID: principal.ID, Status: auth.CredentialStatusActive, CredentialID: "credential-1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: now.Add(-time.Hour)}
	for name, credential := range map[string]auth.Credential{
		"disabled":    func() auth.Credential { c := baseCredential; c.Status = auth.CredentialStatusDisabled; return c }(),
		"revoked":     func() auth.Credential { c := baseCredential; c.Status = auth.CredentialStatusRevoked; return c }(),
		"expired":     func() auth.Credential { c := baseCredential; c.ExpiresAt = now; return c }(),
		"disabled_at": func() auth.Credential { c := baseCredential; c.DisabledAt = now.Add(-time.Minute); return c }(),
	} {
		t.Run(name, func(t *testing.T) {
			adapter := NewAdapter(AdapterOptions{
				Enabled:    true,
				Authorizer: adapterCredentialAuthorizer{principal: principal, credential: credential},
				Now:        func() time.Time { return now },
			})
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set(auth.HeaderAPIKey, "credential-1.secret")
			rec := httptest.NewRecorder()
			adapter.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("inactive %s credential MCP status = %d, want 401 before protocol dispatch", name, rec.Code)
			}
		})
	}
}

type adapterCredentialAuthorizer struct {
	principal  auth.Principal
	credential auth.Credential
}

func (a adapterCredentialAuthorizer) Authenticate(context.Context, string) (auth.Principal, auth.Credential, error) {
	return a.principal, a.credential, nil
}

func (adapterCredentialAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	return false, nil
}
