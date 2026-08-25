package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type principalStoreStub struct {
	principal    Principal
	credential   Credential
	readErr      error
	authorized   bool
	authorizeErr error
}

func (s principalStoreStub) ReadPrincipalCredential(context.Context, string) (Principal, Credential, error) {
	return s.principal, s.credential, s.readErr
}

func (s principalStoreStub) HasActiveScopeGrant(context.Context, string, memory.Scope) (bool, error) {
	return s.authorized, s.authorizeErr
}

func TestPrincipalServiceAuthenticatesOnlyActiveMatchingCredentials(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	secret, err := NewCredentialSecret("stl_credential_1")
	if err != nil {
		t.Fatalf("NewCredentialSecret() error = %v", err)
	}
	credential, err := NewCredentialFromSecret("credential_1", "principal_1", secret, now)
	if err != nil {
		t.Fatalf("NewCredentialFromSecret() error = %v", err)
	}
	principal := Principal{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "integration-a", CreatedAt: now}
	service := NewPrincipalService(principalStoreStub{principal: principal, credential: credential}, func() time.Time { return now })

	if _, _, err := service.Authenticate(context.Background(), secret); err != nil {
		t.Fatalf("Authenticate(valid secret) error = %v", err)
	}
	if _, _, err := service.Authenticate(context.Background(), "stl_credential_1.invalid"); err == nil {
		t.Fatal("Authenticate(wrong secret) error = nil")
	}

	credential.Status = CredentialStatusDisabled
	service = NewPrincipalService(principalStoreStub{principal: principal, credential: credential}, func() time.Time { return now })
	if _, _, err := service.Authenticate(context.Background(), secret); err == nil {
		t.Fatal("Authenticate(disabled credential) error = nil")
	}

	credential.Status = CredentialStatusActive
	credential.ExpiresAt = now
	service = NewPrincipalService(principalStoreStub{principal: principal, credential: credential}, func() time.Time { return now })
	if _, _, err := service.Authenticate(context.Background(), secret); err == nil {
		t.Fatal("Authenticate(expired credential) error = nil")
	}
}

func TestPrincipalServiceAuthorizesExactGrant(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewPrincipalService(principalStoreStub{authorized: true}, time.Now)
	authorized, err := service.AuthorizeScope(context.Background(), "principal_1", scope)
	if err != nil || !authorized {
		t.Fatalf("AuthorizeScope() = %v, %v; want true, nil", authorized, err)
	}

	service = NewPrincipalService(principalStoreStub{authorizeErr: errors.New("unavailable")}, time.Now)
	if _, err := service.AuthorizeScope(context.Background(), "principal_1", scope); err == nil {
		t.Fatal("AuthorizeScope(store error) error = nil")
	}
}

func TestPrincipalServiceRejectsMissingStore(t *testing.T) {
	service := NewPrincipalService(nil, time.Now)
	if _, _, err := service.Authenticate(context.Background(), "credential.invalid"); err == nil {
		t.Fatal("Authenticate() error = nil")
	}
}
