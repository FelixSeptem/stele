package auth

import (
	"context"
	"fmt"
	"github.com/FelixSeptem/stele/internal/memory"
	"strings"
)

type DisclosureGrantAdminStore interface {
	UpdateScopeGrantDisclosure(context.Context, memory.Scope, string, string, DisclosureEntitlements, AuditRecord) error
}

func (s *PrincipalAdminService) UpdateScopeGrantDisclosure(ctx context.Context, scope memory.Scope, principalID, grantID string, e DisclosureEntitlements, actor, reason string) error {
	if err := s.ensureStore(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateIdentifier(principalID, "principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	if err := validateIdentifier(grantID, "grant id", maxPrincipalIDLength); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	actor, reason = strings.TrimSpace(actor), strings.TrimSpace(reason)
	if actor == "" || reason == "" || len(actor) > 128 || len(reason) > 512 {
		return fmt.Errorf("invalid disclosure audit")
	}
	store, ok := s.store.(DisclosureGrantAdminStore)
	if !ok {
		return fmt.Errorf("disclosure store unavailable")
	}
	scope = scope.Normalized()
	return store.UpdateScopeGrantDisclosure(ctx, scope, principalID, grantID, e, AuditRecord{ID: s.newID(), PrincipalID: principalID, Scope: scope, Actor: actor, Reason: reason, Action: "grant_disclosure_updated", Result: "success", CreatedAt: s.now().UTC()})
}
