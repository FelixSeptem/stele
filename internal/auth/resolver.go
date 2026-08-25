package auth

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

type PrincipalResolver struct {
	durable   PrincipalAuthorizer
	bootstrap PrincipalAuthorizer
}

func NewPrincipalResolver(durable PrincipalAuthorizer, bootstrap PrincipalAuthorizer) *PrincipalResolver {
	return &PrincipalResolver{durable: durable, bootstrap: bootstrap}
}

func (r *PrincipalResolver) Authenticate(ctx context.Context, secret string) (Principal, Credential, error) {
	if r == nil {
		return Principal{}, Credential{}, fmt.Errorf("principal authorization is not configured")
	}
	if r.durable != nil {
		principal, credential, err := r.durable.Authenticate(ctx, secret)
		if err == nil {
			return principal, credential, nil
		}
	}
	if r.bootstrap != nil {
		return r.bootstrap.Authenticate(ctx, secret)
	}
	return Principal{}, Credential{}, fmt.Errorf("principal credential is invalid")
}

func (r *PrincipalResolver) AuthorizeScope(ctx context.Context, principalID string, scope memory.Scope) (bool, error) {
	if r == nil {
		return false, fmt.Errorf("principal authorization is not configured")
	}
	if principalID == bootstrapPrincipalID && r.bootstrap != nil {
		return r.bootstrap.AuthorizeScope(ctx, principalID, scope)
	}
	if r.durable == nil {
		return false, fmt.Errorf("principal authorization is not configured")
	}
	return r.durable.AuthorizeScope(ctx, principalID, scope)
}
