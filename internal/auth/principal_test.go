package auth

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestPrincipalCredentialAndGrantValidation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	principal := Principal{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()}
	credential := Credential{ID: "credential_1", PrincipalID: principal.ID, Status: CredentialStatusActive, CredentialID: "stl_credential_1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now().UTC()}
	grant := ScopeGrant{ID: "grant_1", PrincipalID: principal.ID, Scope: scope, Status: ScopeGrantStatusActive, CreatedAt: time.Now().UTC()}

	if err := principal.Validate(); err != nil {
		t.Fatalf("Principal.Validate() error = %v", err)
	}
	if err := credential.Validate(); err != nil {
		t.Fatalf("Credential.Validate() error = %v", err)
	}
	if err := grant.Validate(); err != nil {
		t.Fatalf("ScopeGrant.Validate() error = %v", err)
	}
	if _, err := NewCredentialSecret("credential_1"); err != nil {
		t.Fatalf("NewCredentialSecret() error = %v", err)
	}
}

func TestPrincipalSafeProjectionNeverIncludesCredentialDigest(t *testing.T) {
	credential := Credential{ID: "credential_1", PrincipalID: "principal_1", Status: CredentialStatusActive, CredentialID: "stl_credential_1", Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now().UTC()}
	public := credential.SafeProjection()
	if public.ID != credential.ID || public.CredentialID != credential.CredentialID {
		t.Fatalf("projection = %+v, want safe identifiers", public)
	}
}

func TestValidateIdempotencyKeyRejectsEmptyAndOversized(t *testing.T) {
	if err := ValidateIdempotencyKey(""); err == nil {
		t.Fatal("ValidateIdempotencyKey(empty) error = nil")
	}
	if err := ValidateIdempotencyKey(string(make([]byte, 257))); err == nil {
		t.Fatal("ValidateIdempotencyKey(oversized) error = nil")
	}
}

func TestCredentialSecretDigestRoundTripRejectsWrongSecret(t *testing.T) {
	secret, err := NewCredentialSecret("stl_credential_1")
	if err != nil {
		t.Fatalf("NewCredentialSecret() error = %v", err)
	}
	credential, err := NewCredentialFromSecret("credential_1", "principal_1", secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewCredentialFromSecret() error = %v", err)
	}
	if !credential.MatchesSecret(secret) {
		t.Fatal("MatchesSecret() = false, want true")
	}
	if credential.MatchesSecret("stl_credential_1.not-the-issued-secret") {
		t.Fatal("MatchesSecret() = true for a different secret")
	}
}
