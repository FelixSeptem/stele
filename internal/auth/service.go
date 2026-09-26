package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type PrincipalStore interface {
	ReadPrincipalCredential(ctx context.Context, credentialID string) (Principal, Credential, error)
	HasActiveScopeGrant(ctx context.Context, principalID string, scope memory.Scope) (bool, error)
}

type ScopeAccessAuthorizer interface {
	AuthorizeScopeAccess(context.Context, string, memory.Scope) (ScopeGrantAccessMode, error)
}

type PrincipalService struct {
	store PrincipalStore
	now   func() time.Time
}

func NewPrincipalService(store PrincipalStore, now func() time.Time) *PrincipalService {
	if now == nil {
		now = time.Now
	}
	return &PrincipalService{store: store, now: now}
}

func (s *PrincipalService) Authenticate(ctx context.Context, secret string) (Principal, Credential, error) {
	if s == nil || s.store == nil {
		return Principal{}, Credential{}, fmt.Errorf("principal store is not configured")
	}
	credentialID, err := CredentialIDFromSecret(secret)
	if err != nil {
		return Principal{}, Credential{}, fmt.Errorf("principal credential is invalid")
	}
	principal, credential, err := s.store.ReadPrincipalCredential(ctx, credentialID)
	if err != nil || !PrincipalCredentialActive(principal, credential, s.now().UTC()) || !credential.MatchesSecret(secret) {
		return Principal{}, Credential{}, fmt.Errorf("principal credential is invalid")
	}
	return principal, credential, nil
}

func (s *PrincipalService) AuthorizeScope(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	if s == nil || s.store == nil {
		return false, fmt.Errorf("principal store is not configured")
	}
	return s.store.HasActiveScopeGrant(ctx, principalID, scope)
}

func (s *PrincipalService) AuthorizeScopeAccess(ctx context.Context, principalID string, scope memory.Scope) (ScopeGrantAccessMode, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("principal store is not configured")
	}
	if accessStore, ok := s.store.(interface {
		ScopeGrantAccess(context.Context, string, memory.Scope) (ScopeGrantAccessMode, error)
	}); ok {
		return accessStore.ScopeGrantAccess(ctx, principalID, scope)
	}
	granted, err := s.store.HasActiveScopeGrant(ctx, principalID, scope)
	if err != nil || !granted {
		return "", err
	}
	return ScopeGrantAccessReadWrite, nil
}
