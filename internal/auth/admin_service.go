package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type PrincipalAdminStore interface {
	CreatePrincipal(context.Context, Principal, Credential, []ScopeGrant, AuditRecord) error
	ListPrincipals(context.Context, memory.Scope, int) ([]Principal, error)
	ReadPrincipal(context.Context, memory.Scope, string) (Principal, error)
	ListScopeGrants(context.Context, memory.Scope, string) ([]ScopeGrant, error)
	RotateCredential(context.Context, memory.Scope, string, Credential, AuditRecord) error
	DisablePrincipal(context.Context, memory.Scope, string, time.Time, AuditRecord) error
	ExpirePrincipal(context.Context, memory.Scope, string, time.Time, AuditRecord) error
	CreateScopeGrant(context.Context, memory.Scope, ScopeGrant, AuditRecord) error
	RevokeScopeGrant(context.Context, memory.Scope, string, time.Time, AuditRecord) error
	ListAccessAudit(context.Context, memory.Scope, string, int) ([]AuditRecord, error)
}

type CreatePrincipalInput struct {
	Role   PrincipalRole
	Label  string
	Grants []memory.Scope
	Actor  string
	Reason string
}

type IssuedPrincipal struct {
	Principal  Principal            `json:"principal"`
	Credential CredentialProjection `json:"credential"`
	Grants     []ScopeGrant         `json:"grants"`
	Secret     string               `json:"credential_secret"`
}

type IssuedCredential struct {
	Credential CredentialProjection `json:"credential"`
	Secret     string               `json:"credential_secret"`
}

type PrincipalAdminService struct {
	store PrincipalAdminStore
	now   func() time.Time
	newID func() string
}

func NewPrincipalAdminService(store PrincipalAdminStore, now func() time.Time, newID func() string) *PrincipalAdminService {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = func() string { return fmt.Sprintf("access_%d", time.Now().UnixNano()) }
	}
	return &PrincipalAdminService{store: store, now: now, newID: newID}
}

func (s *PrincipalAdminService) ensureStore() error {
	if s == nil || s.store == nil {
		return fmt.Errorf("principal admin store is not configured")
	}
	return nil
}

func (s *PrincipalAdminService) CreatePrincipal(ctx context.Context, input CreatePrincipalInput) (IssuedPrincipal, error) {
	if err := s.ensureStore(); err != nil {
		return IssuedPrincipal{}, err
	}
	now := s.now().UTC()
	principal := Principal{ID: s.newID(), Role: input.Role, Status: PrincipalStatusActive, Label: strings.TrimSpace(input.Label), CreatedAt: now, UpdatedAt: now}
	if err := principal.Validate(); err != nil {
		return IssuedPrincipal{}, err
	}
	if len(input.Grants) == 0 {
		return IssuedPrincipal{}, fmt.Errorf("at least one scope grant is required")
	}
	secret, err := NewCredentialSecret("stl_" + s.newID())
	if err != nil {
		return IssuedPrincipal{}, err
	}
	credential, err := NewCredentialFromSecret(s.newID(), principal.ID, secret, now)
	if err != nil {
		return IssuedPrincipal{}, err
	}
	grants := make([]ScopeGrant, 0, len(input.Grants))
	for _, scope := range input.Grants {
		grant := ScopeGrant{ID: s.newID(), PrincipalID: principal.ID, Scope: scope.Normalized(), Status: ScopeGrantStatusActive, CreatedAt: now}
		if err := grant.Validate(); err != nil {
			return IssuedPrincipal{}, err
		}
		grants = append(grants, grant)
	}
	audit := AuditRecord{ID: s.newID(), PrincipalID: principal.ID, CredentialID: credential.ID, Action: "principal_created", Result: "success", CreatedAt: now}
	if err := s.store.CreatePrincipal(ctx, principal, credential, grants, audit); err != nil {
		return IssuedPrincipal{}, err
	}
	return IssuedPrincipal{Principal: principal, Credential: credential.SafeProjection(), Grants: grants, Secret: secret}, nil
}

func (s *PrincipalAdminService) ListPrincipals(ctx context.Context, scope memory.Scope, limit int) ([]Principal, error) {
	if err := s.ensureStore(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListPrincipals(ctx, scope.Normalized(), limit)
}

func (s *PrincipalAdminService) ReadPrincipal(ctx context.Context, scope memory.Scope, principalID string) (Principal, error) {
	if err := s.ensureStore(); err != nil {
		return Principal{}, err
	}
	if err := scope.Validate(); err != nil {
		return Principal{}, err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return Principal{}, err
	}
	return s.store.ReadPrincipal(ctx, scope.Normalized(), principalID)
}

func (s *PrincipalAdminService) ListScopeGrants(ctx context.Context, scope memory.Scope, principalID string) ([]ScopeGrant, error) {
	if err := s.ensureStore(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return nil, err
	}
	return s.store.ListScopeGrants(ctx, scope.Normalized(), principalID)
}

func (s *PrincipalAdminService) ListAccessAudit(ctx context.Context, scope memory.Scope, principalID string, limit int) ([]AuditRecord, error) {
	if err := s.ensureStore(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if principalID != "" {
		if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
			return nil, err
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.ListAccessAudit(ctx, scope.Normalized(), principalID, limit)
}

func (s *PrincipalAdminService) RotateCredential(ctx context.Context, scope memory.Scope, principalID, actor, reason string) (IssuedCredential, error) {
	if err := s.ensureStore(); err != nil {
		return IssuedCredential{}, err
	}
	if err := scope.Validate(); err != nil {
		return IssuedCredential{}, err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return IssuedCredential{}, err
	}
	now := s.now().UTC()
	lookupID := s.newID()
	secret, err := NewCredentialSecret("stl_" + lookupID)
	if err != nil {
		return IssuedCredential{}, err
	}
	credential, err := NewCredentialFromSecret(s.newID(), principalID, secret, now)
	if err != nil {
		return IssuedCredential{}, err
	}
	audit := AuditRecord{ID: s.newID(), PrincipalID: principalID, CredentialID: credential.ID, Scope: scope.Normalized(), Action: "credential_rotated", Result: "success", CreatedAt: now}
	if err := s.store.RotateCredential(ctx, scope.Normalized(), principalID, credential, audit); err != nil {
		return IssuedCredential{}, err
	}
	return IssuedCredential{Credential: credential.SafeProjection(), Secret: secret}, nil
}

func (s *PrincipalAdminService) DisablePrincipal(ctx context.Context, scope memory.Scope, principalID, actor, reason string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	now := s.now().UTC()
	return s.store.DisablePrincipal(ctx, scope.Normalized(), principalID, now, AuditRecord{ID: s.newID(), PrincipalID: principalID, Scope: scope.Normalized(), Action: "principal_disabled", Result: "success", CreatedAt: now})
}

func (s *PrincipalAdminService) ExpirePrincipal(ctx context.Context, scope memory.Scope, principalID string, expiresAt time.Time, actor, reason string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	expiresAt = expiresAt.UTC()
	if expiresAt.IsZero() || !expiresAt.After(s.now().UTC()) {
		return fmt.Errorf("principal expiry must be in the future")
	}
	now := s.now().UTC()
	audit := AuditRecord{ID: s.newID(), PrincipalID: principalID, Scope: scope.Normalized(), Action: "principal_expiry_set", Result: "success", CreatedAt: now}
	return s.store.ExpirePrincipal(ctx, scope.Normalized(), principalID, expiresAt, audit)
}

func (s *PrincipalAdminService) CreateScopeGrant(ctx context.Context, scope memory.Scope, principalID string, grantScope memory.Scope, actor, reason string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := grantScope.Validate(); err != nil {
		return err
	}
	if scope.Normalized() != grantScope.Normalized() {
		return fmt.Errorf("scope grant must match authorized scope")
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	now := s.now().UTC()
	grant := ScopeGrant{ID: s.newID(), PrincipalID: principalID, Scope: grantScope.Normalized(), Status: ScopeGrantStatusActive, CreatedAt: now}
	return s.store.CreateScopeGrant(ctx, scope.Normalized(), grant, AuditRecord{ID: s.newID(), PrincipalID: principalID, Scope: scope.Normalized(), Action: "scope_grant_created", Result: "success", CreatedAt: now})
}

func (s *PrincipalAdminService) RevokeScopeGrant(ctx context.Context, scope memory.Scope, grantID, actor, reason string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateIdentifier(grantID, "scope grant id", maxPrincipalIDLength); err != nil {
		return err
	}
	now := s.now().UTC()
	return s.store.RevokeScopeGrant(ctx, scope.Normalized(), grantID, now, AuditRecord{ID: s.newID(), Scope: scope.Normalized(), Action: "scope_grant_revoked", Result: "success", CreatedAt: now})
}
