package auth

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type principalAdminStoreStub struct {
	createdPrincipal  Principal
	createdCredential Credential
	createdGrants     []ScopeGrant
	principals        []Principal
	grants            []ScopeGrant
	audits            []AuditRecord
	rotated           Credential
	disabled          string
	revokedGrant      string
}

func (s *principalAdminStoreStub) CreatePrincipal(context.Context, Principal, Credential, []ScopeGrant, AuditRecord) error {
	return nil
}
func (s *principalAdminStoreStub) ListPrincipals(context.Context, memory.Scope, int) ([]Principal, error) {
	return s.principals, nil
}
func (s *principalAdminStoreStub) ReadPrincipal(context.Context, memory.Scope, string) (Principal, error) {
	return s.createdPrincipal, nil
}
func (s *principalAdminStoreStub) ListScopeGrants(context.Context, memory.Scope, string) ([]ScopeGrant, error) {
	return s.grants, nil
}
func (s *principalAdminStoreStub) RotateCredential(context.Context, memory.Scope, string, Credential, AuditRecord) error {
	s.rotated = Credential{Status: CredentialStatusActive}
	return nil
}

func (s *principalAdminStoreStub) DisablePrincipal(_ context.Context, _ memory.Scope, principalID string, _ time.Time, _ AuditRecord) error {
	s.disabled = principalID
	return nil
}
func (s *principalAdminStoreStub) ExpirePrincipal(_ context.Context, _ memory.Scope, _ string, _ time.Time, _ AuditRecord) error {
	return nil
}
func (s *principalAdminStoreStub) CreateScopeGrant(_ context.Context, _ memory.Scope, grant ScopeGrant, _ AuditRecord) error {
	s.createdGrants = append(s.createdGrants, grant)
	return nil
}
func (s *principalAdminStoreStub) RevokeScopeGrant(_ context.Context, _ memory.Scope, grantID string, _ time.Time, _ AuditRecord) error {
	s.revokedGrant = grantID
	return nil
}
func (s *principalAdminStoreStub) ListAccessAudit(context.Context, memory.Scope, string, int) ([]AuditRecord, error) {
	return s.audits, nil
}

func TestPrincipalAdminServiceRotatesCredentialAndReturnsSecretOnce(t *testing.T) {
	store := &principalAdminStoreStub{}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	sequence := 0
	service := NewPrincipalAdminService(store, func() time.Time { return now }, func() string {
		sequence++
		return "id_" + string(rune('0'+sequence))
	})
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	issued, err := service.RotateCredential(context.Background(), scope, "principal_1", "operator_1", "rotation")
	if err != nil {
		t.Fatalf("RotateCredential() error = %v", err)
	}
	if issued.Secret == "" || issued.Credential.CredentialID == "" {
		t.Fatalf("issued credential = %+v", issued)
	}
	raw, err := json.Marshal(issued.Credential)
	if err != nil || strings.Contains(string(raw), "digest") || strings.Contains(string(raw), "salt") {
		t.Fatalf("issued credential projection contains digest material: %s", raw)
	}
}

func TestPrincipalAdminServiceReadsOnlyScopedSafeProjections(t *testing.T) {
	store := &principalAdminStoreStub{
		createdPrincipal: Principal{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "integration-a"},
		principals:       []Principal{{ID: "principal_1", Role: PrincipalRolePublic, Status: PrincipalStatusActive, Label: "integration-a"}},
		grants:           []ScopeGrant{{ID: "grant_1", PrincipalID: "principal_1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Status: ScopeGrantStatusActive}},
		audits:           []AuditRecord{{ID: "audit_1", PrincipalID: "principal_1", Action: "credential_rotated", Result: "success"}},
	}
	service := NewPrincipalAdminService(store, time.Now, nil)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	if principals, err := service.ListPrincipals(context.Background(), scope, 10); err != nil || len(principals) != 1 {
		t.Fatalf("ListPrincipals() = %+v, %v", principals, err)
	}
	if grants, err := service.ListScopeGrants(context.Background(), scope, "principal_1"); err != nil || len(grants) != 1 {
		t.Fatalf("ListScopeGrants() = %+v, %v", grants, err)
	}
	if audits, err := service.ListAccessAudit(context.Background(), scope, "principal_1", 10); err != nil || len(audits) != 1 {
		t.Fatalf("ListAccessAudit() = %+v, %v", audits, err)
	}
	if principal, err := service.ReadPrincipal(context.Background(), scope, "principal_1"); err != nil || principal.ID != "principal_1" {
		t.Fatalf("ReadPrincipal() = %+v, %v", principal, err)
	}
}

func TestPrincipalAdminServiceDisablesAndRevokesScopedAccess(t *testing.T) {
	store := &principalAdminStoreStub{}
	service := NewPrincipalAdminService(store, time.Now, nil)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	if err := service.DisablePrincipal(context.Background(), scope, "principal_1", "operator_1", "disable"); err != nil {
		t.Fatalf("DisablePrincipal() error = %v", err)
	}
	if err := service.RevokeScopeGrant(context.Background(), scope, "grant_1", "operator_1", "revoke"); err != nil {
		t.Fatalf("RevokeScopeGrant() error = %v", err)
	}

	if err := service.CreateScopeGrant(context.Background(), scope, "principal_1", scope, "operator_1", "grant"); err != nil {
		t.Fatalf("CreateScopeGrant() error = %v", err)
	}
	if store.disabled != "principal_1" || store.revokedGrant != "grant_1" {
		t.Fatalf("store lifecycle state disabled=%q revoked=%q", store.disabled, store.revokedGrant)
	}
	if len(store.createdGrants) != 1 {
		t.Fatalf("created grants = %+v", store.createdGrants)
	}
}

func TestPrincipalAdminServiceIssuesOneTimeCredentialAndSafeInspection(t *testing.T) {
	store := &principalAdminStoreStub{}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	service := NewPrincipalAdminService(store, func() time.Time { return now }, func() string { return "id_1" })
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	issued, err := service.CreatePrincipal(context.Background(), CreatePrincipalInput{Role: PrincipalRolePublic, Label: "integration-a", Grants: []memory.Scope{scope}, Actor: "bootstrap-operator", Reason: "initial setup"})
	if err != nil {
		t.Fatalf("CreatePrincipal() error = %v", err)
	}
	if issued.Secret == "" || issued.Principal.Role != PrincipalRolePublic || len(issued.Grants) != 1 {
		t.Fatalf("issued = %+v", issued)
	}
	if issued.Credential.CredentialID == "" {
		t.Fatalf("credential projection = %+v, want safe lookup identifier", issued.Credential)
	}
}
