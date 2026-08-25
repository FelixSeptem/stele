package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const bootstrapPrincipalID = "bootstrap-operator"

type BootstrapAdminGate interface {
	HasActiveAdminPrincipal(ctx context.Context) (bool, error)
}

type BootstrapAuthorizer struct {
	key          string
	defaultScope memory.Scope
	gate         BootstrapAdminGate
}

func NewBootstrapAuthorizer(key string, defaultScope memory.Scope, gate BootstrapAdminGate) *BootstrapAuthorizer {
	return &BootstrapAuthorizer{key: strings.TrimSpace(key), defaultScope: defaultScope.Normalized(), gate: gate}
}

func (a *BootstrapAuthorizer) Authenticate(ctx context.Context, secret string) (Principal, Credential, error) {
	if a == nil || a.key == "" || strings.TrimSpace(secret) != a.key {
		return Principal{}, Credential{}, fmt.Errorf("bootstrap credential is invalid")
	}
	if err := a.defaultScope.Validate(); err != nil {
		return Principal{}, Credential{}, fmt.Errorf("bootstrap scope is invalid")
	}
	if a.gate != nil {
		hasAdmin, err := a.gate.HasActiveAdminPrincipal(ctx)
		if err != nil {
			return Principal{}, Credential{}, fmt.Errorf("check durable admin principal: %w", err)
		}
		if hasAdmin {
			return Principal{}, Credential{}, fmt.Errorf("bootstrap operator is disabled")
		}
	}
	now := time.Now().UTC()
	return Principal{ID: bootstrapPrincipalID, Role: PrincipalRoleAdmin, Status: PrincipalStatusActive, Label: "bootstrap", CreatedAt: now}, Credential{ID: bootstrapPrincipalID, PrincipalID: bootstrapPrincipalID, Status: CredentialStatusActive, CredentialID: bootstrapPrincipalID, Salt: []byte{1}, Digest: []byte{1}, CreatedAt: now}, nil
}

func (a *BootstrapAuthorizer) AuthorizeScope(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	if a == nil || principalID != bootstrapPrincipalID {
		return false, nil
	}
	if a.gate != nil {
		hasAdmin, err := a.gate.HasActiveAdminPrincipal(ctx)
		if err != nil {
			return false, fmt.Errorf("check durable admin principal: %w", err)
		}
		if hasAdmin {
			return false, nil
		}
	}
	return scope.Normalized() == a.defaultScope, nil
}
